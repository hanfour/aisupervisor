package company

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/worker"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	mu           sync.RWMutex
	projectStore *project.Store
	spawner      *worker.Spawner
	gitOps       gitops.GitOps
	monitor      *worker.CompletionMonitor
	tmuxClient   tmux.TmuxClient
	subscribers  []chan Event
	subMu        sync.Mutex
	autoSchedule bool
	workers      map[string]*worker.Worker
	cancels      map[string]context.CancelFunc
	workersPath  string
}

type workersFile struct {
	Workers []*worker.Worker `yaml:"workers"`
}

// ProgressDTO reports project completion progress.
type ProgressDTO struct {
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	InProgress int     `json:"inProgress"`
	Failed     int     `json:"failed"`
	Percent    float64 `json:"percent"`
}

func New(
	projectStore *project.Store,
	spawner *worker.Spawner,
	gitOps gitops.GitOps,
	monitor *worker.CompletionMonitor,
	tmuxClient tmux.TmuxClient,
	dataDir string,
) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	m := &Manager{
		projectStore: projectStore,
		spawner:      spawner,
		gitOps:       gitOps,
		monitor:      monitor,
		tmuxClient:   tmuxClient,
		autoSchedule: true,
		workers:      make(map[string]*worker.Worker),
		cancels:      make(map[string]context.CancelFunc),
		workersPath:  filepath.Join(dataDir, "workers.yaml"),
	}

	if err := m.loadWorkers(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Recovery: reset workers with stale tmux sessions to idle
	m.recoverStaleWorkers()

	return m, nil
}

// recoverStaleWorkers detects workers that were persisted as working/waiting
// but whose tmux sessions no longer exist (e.g. after a restart), and resets
// them to idle.
func (m *Manager) recoverStaleWorkers() {
	if m.tmuxClient == nil {
		return
	}
	changed := false
	for _, w := range m.workers {
		if w.Status == worker.WorkerIdle || w.Status == worker.WorkerFinished {
			continue
		}
		if w.TmuxSession == "" {
			continue
		}
		has, err := m.tmuxClient.HasSession(w.TmuxSession)
		if err != nil || !has {
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			w.TmuxSession = ""
			w.SessionID = ""
			changed = true
		}
	}
	if changed {
		m.saveWorkers()
	}
}

// --- Project operations ---

func (m *Manager) CreateProject(name, description, repoPath, baseBranch string, goals []string) (*project.Project, error) {
	p := &project.Project{
		Name:        name,
		Description: description,
		RepoPath:    repoPath,
		BaseBranch:  baseBranch,
		Goals:       goals,
	}
	if err := m.projectStore.SaveProject(p); err != nil {
		return nil, err
	}

	m.emit(Event{
		Type:      EventProjectCreated,
		ProjectID: p.ID,
		Message:   fmt.Sprintf("Project created: %s", name),
	})
	return p, nil
}

func (m *Manager) ListProjects() []*project.Project {
	return m.projectStore.ListProjects()
}

func (m *Manager) GetProject(id string) (*project.Project, bool) {
	return m.projectStore.GetProject(id)
}

// --- Task operations ---

func (m *Manager) AddTask(projectID, title, description, prompt string, dependsOn []string, priority int, milestone string) (*project.Task, error) {
	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return nil, fmt.Errorf("project %q not found", projectID)
	}

	slug := slugify(title)
	t := &project.Task{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Prompt:      prompt,
		Priority:    priority,
		DependsOn:   dependsOn,
		Milestone:   milestone,
		BranchName:  gitops.BranchName(p.ID, "", slug), // ID set after save
	}

	// Determine initial status based on dependencies
	if len(dependsOn) == 0 {
		t.Status = project.TaskReady
	}

	if err := m.projectStore.SaveTask(t); err != nil {
		return nil, err
	}

	// Fix branch name with actual task ID
	t.BranchName = gitops.BranchName(p.ID, t.ID, slug)
	if err := m.projectStore.SaveTask(t); err != nil {
		return nil, err
	}

	m.emit(Event{
		Type:      EventTaskCreated,
		ProjectID: projectID,
		TaskID:    t.ID,
		Message:   fmt.Sprintf("Task created: %s", title),
	})
	return t, nil
}

func (m *Manager) ListTasks(projectID string) []*project.Task {
	return m.projectStore.TasksForProject(projectID)
}

// --- Worker operations ---

func (m *Manager) CreateWorker(name, avatar string) (*worker.Worker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w := &worker.Worker{
		ID:        fmt.Sprintf("w%d-%d", time.Now().UnixMilli(), workerIDCounter.Add(1)),
		Name:      name,
		Avatar:    avatar,
		Status:    worker.WorkerIdle,
		CreatedAt: time.Now(),
	}

	m.workers[w.ID] = w
	if err := m.saveWorkers(); err != nil {
		return nil, err
	}

	m.emit(Event{
		Type:     EventWorkerSpawned,
		WorkerID: w.ID,
		Message:  fmt.Sprintf("Worker hired: %s", name),
	})
	return w, nil
}

func (m *Manager) ListWorkers() []*worker.Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*worker.Worker, 0, len(m.workers))
	for _, w := range m.workers {
		result = append(result, w)
	}
	return result
}

func (m *Manager) GetWorker(id string) (*worker.Worker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[id]
	return w, ok
}

// --- Assignment + lifecycle ---

func (m *Manager) AssignTask(ctx context.Context, workerID, taskID string) error {
	m.mu.Lock()

	w, ok := m.workers[workerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("worker %q not found", workerID)
	}
	if w.Status != worker.WorkerIdle {
		m.mu.Unlock()
		return fmt.Errorf("worker %q is not idle (status: %s)", workerID, w.Status)
	}

	t, ok := m.projectStore.GetTask(taskID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.Status != project.TaskReady {
		m.mu.Unlock()
		return fmt.Errorf("task %q is not ready (status: %s)", taskID, t.Status)
	}

	p, ok := m.projectStore.GetProject(t.ProjectID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("project %q not found", t.ProjectID)
	}

	// Update task status
	t.AssigneeID = workerID
	if err := m.projectStore.UpdateTaskStatus(taskID, project.TaskAssigned); err != nil {
		m.mu.Unlock()
		return err
	}

	m.mu.Unlock()

	// Spawn worker (this creates tmux session, git branch, launches Claude Code)
	if err := m.spawner.SpawnForTask(ctx, w, t, p); err != nil {
		m.projectStore.UpdateTaskStatus(taskID, project.TaskReady)
		return fmt.Errorf("spawning worker: %w", err)
	}

	m.mu.Lock()
	m.projectStore.UpdateTaskStatus(taskID, project.TaskInProgress)
	m.saveWorkers()
	m.mu.Unlock()

	m.emit(Event{
		Type:      EventTaskAssigned,
		ProjectID: p.ID,
		TaskID:    taskID,
		WorkerID:  workerID,
		Message:   fmt.Sprintf("Task %q assigned to %s", t.Title, w.Name),
	})

	m.emit(Event{
		Type:      EventBranchCreated,
		ProjectID: p.ID,
		TaskID:    taskID,
		Message:   fmt.Sprintf("Branch created: %s", t.BranchName),
	})

	// Start completion monitoring in background
	workerCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancels[workerID] = cancel
	m.mu.Unlock()

	go m.watchCompletion(workerCtx, w, t, p)

	return nil
}

func (m *Manager) watchCompletion(ctx context.Context, w *worker.Worker, t *project.Task, p *project.Project) {
	result, err := m.monitor.WatchForCompletion(ctx, w)
	if err != nil {
		// Context cancelled — not an error
		return
	}

	m.handleTaskCompletion(w, t, p, result)
}

func (m *Manager) handleTaskCompletion(w *worker.Worker, t *project.Task, p *project.Project, result worker.CompletionResult) {
	m.mu.Lock()

	if result.Success {
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskReview)
		m.emit(Event{
			Type:      EventTaskCompleted,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   fmt.Sprintf("Task %q completed (reason: %s)", t.Title, result.Reason),
		})
	} else {
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskFailed)
		m.emit(Event{
			Type:      EventTaskFailed,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   fmt.Sprintf("Task %q failed", t.Title),
		})
	}

	// Reset worker to idle
	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	m.saveWorkers()

	m.emit(Event{
		Type:     EventWorkerIdle,
		WorkerID: w.ID,
		Message:  fmt.Sprintf("Worker %s is idle", w.Name),
	})

	// Promote newly unblocked tasks
	promoted, _ := m.projectStore.PromoteReady(p.ID)
	for _, pt := range promoted {
		m.emit(Event{
			Type:      EventTaskCreated,
			ProjectID: p.ID,
			TaskID:    pt.ID,
			Message:   fmt.Sprintf("Task %q is now ready (dependencies resolved)", pt.Title),
		})
	}

	shouldAutoSchedule := m.autoSchedule
	workerID := w.ID

	m.mu.Unlock()

	if shouldAutoSchedule {
		go m.tryAutoAssign(workerID)
	}
}

// tryAutoAssign picks the highest-priority ready task and assigns it to the given idle worker.
func (m *Manager) tryAutoAssign(workerID string) {
	candidates := m.projectStore.ReadyTasksByPriority()
	if len(candidates) == 0 {
		return
	}

	task := candidates[0]
	ctx := context.Background()
	if err := m.AssignTask(ctx, workerID, task.ID); err != nil {
		return
	}

	m.emit(Event{
		Type:     EventAutoAssigned,
		TaskID:   task.ID,
		WorkerID: workerID,
		Message:  fmt.Sprintf("Auto-assigned task %q to worker %s", task.Title, workerID),
	})
}

// CompleteTask manually marks a task as done (used by supervisor/UI for review → done).
func (m *Manager) CompleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.projectStore.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}

	if err := m.projectStore.UpdateTaskStatus(taskID, project.TaskDone); err != nil {
		return err
	}

	m.emit(Event{
		Type:      EventTaskCompleted,
		ProjectID: t.ProjectID,
		TaskID:    taskID,
		Message:   fmt.Sprintf("Task %q marked as done", t.Title),
	})

	// Promote newly unblocked tasks
	promoted, _ := m.projectStore.PromoteReady(t.ProjectID)
	for _, pt := range promoted {
		m.emit(Event{
			Type:      EventTaskCreated,
			ProjectID: t.ProjectID,
			TaskID:    pt.ID,
			Message:   fmt.Sprintf("Task %q is now ready (dependencies resolved)", pt.Title),
		})
	}

	return nil
}

// ProjectProgress returns completion statistics for a project.
func (m *Manager) ProjectProgress(projectID string) ProgressDTO {
	tasks := m.projectStore.TasksForProject(projectID)
	dto := ProgressDTO{Total: len(tasks)}
	for _, t := range tasks {
		switch t.Status {
		case project.TaskDone:
			dto.Done++
		case project.TaskInProgress, project.TaskAssigned:
			dto.InProgress++
		case project.TaskFailed:
			dto.Failed++
		}
	}
	if dto.Total > 0 {
		dto.Percent = float64(dto.Done) / float64(dto.Total) * 100
	}
	return dto
}

// Subscribe creates a new event channel that receives all future events.
func (m *Manager) Subscribe() <-chan Event {
	ch := make(chan Event, 100)
	m.subMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subMu.Unlock()
	return ch
}

// Unsubscribe removes a previously subscribed channel and closes it.
func (m *Manager) Unsubscribe(ch <-chan Event) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for i, sub := range m.subscribers {
		if sub == ch {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// Events returns a new subscriber channel (backwards compatible).
func (m *Manager) Events() <-chan Event {
	return m.Subscribe()
}

// ProjectStore returns the underlying project store.
func (m *Manager) ProjectStore() *project.Store {
	return m.projectStore
}

// Shutdown cleans up all active workers.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cancel := range m.cancels {
		cancel()
	}
}

func (m *Manager) emit(e Event) {
	e.Timestamp = time.Now()
	m.subMu.Lock()
	for _, ch := range m.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
	m.subMu.Unlock()
}

func (m *Manager) loadWorkers() error {
	data, err := os.ReadFile(m.workersPath)
	if err != nil {
		return err
	}
	var f workersFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, w := range f.Workers {
		m.workers[w.ID] = w
	}
	return nil
}

func (m *Manager) saveWorkers() error {
	f := workersFile{
		Workers: make([]*worker.Worker, 0, len(m.workers)),
	}
	for _, w := range m.workers {
		f.Workers = append(f.Workers, w)
	}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(m.workersPath, data, 0o644)
}

var (
	nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)
	workerIDCounter atomic.Int64
)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}
