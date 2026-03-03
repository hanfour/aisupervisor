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

	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/training"
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
	wg           sync.WaitGroup
	workersPath  string
	personalityStore *personality.Store
	review           *ReviewPipeline
	collector        *training.Collector
	finetuneRunner *training.FinetuneRunner
	finetuneCfg    training.FinetuneConfig
	maxWorkers     map[worker.WorkerTier]int // per-tier worker limits (0 = unlimited)
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

	personalityStore := personality.NewStore(dataDir)
	if err := personalityStore.Load(); err != nil {
		// Just log, don't fail startup
	}

	m := &Manager{
		projectStore:     projectStore,
		spawner:          spawner,
		gitOps:           gitOps,
		monitor:          monitor,
		tmuxClient:       tmuxClient,
		autoSchedule:     true,
		workers:          make(map[string]*worker.Worker),
		cancels:          make(map[string]context.CancelFunc),
		workersPath:      filepath.Join(dataDir, "workers.yaml"),
		maxWorkers:       make(map[worker.WorkerTier]int),
		personalityStore: personalityStore,
	}
	m.review = newReviewPipeline(m)

	if err := m.loadWorkers(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Recovery: reset workers with stale tmux sessions to idle
	m.recoverStaleWorkers()

	// Periodically persist personality data
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.personalityStore.Save()
		}
	}()

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

func (m *Manager) CreateWorker(name, avatar string, opts ...WorkerOption) (*worker.Worker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w := &worker.Worker{
		ID:        fmt.Sprintf("w%d-%d", time.Now().UnixMilli(), workerIDCounter.Add(1)),
		Name:      name,
		Avatar:    avatar,
		Status:    worker.WorkerIdle,
		CreatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(w)
	}

	// Validate hierarchy constraints
	if err := m.validateHierarchy(w); err != nil {
		return nil, err
	}

	// Enforce MaxWorkers per tier
	if max, ok := m.maxWorkers[w.EffectiveTier()]; ok && max > 0 {
		count := 0
		for _, existing := range m.workers {
			if existing.EffectiveTier() == w.EffectiveTier() {
				count++
			}
		}
		if count >= max {
			return nil, fmt.Errorf("max workers (%d) reached for tier %s", max, w.EffectiveTier())
		}
	}

	m.workers[w.ID] = w

	profile := personality.NewCharacterProfile(w.ID)
	m.personalityStore.SetProfile(profile)
	m.personalityStore.Save()

	if err := m.saveWorkers(); err != nil {
		return nil, err
	}

	m.emit(Event{
		Type:     EventWorkerSpawned,
		WorkerID: w.ID,
		Message:  fmt.Sprintf("Worker hired: %s (tier: %s)", name, w.EffectiveTier()),
	})
	return w, nil
}

// validateHierarchy checks that parent-child tier relationships are correct.
func (m *Manager) validateHierarchy(w *worker.Worker) error {
	if w.ParentID == "" {
		return nil
	}
	parent, ok := m.workers[w.ParentID]
	if !ok {
		return fmt.Errorf("parent worker %q not found", w.ParentID)
	}
	tier := w.EffectiveTier()
	parentTier := parent.EffectiveTier()

	switch tier {
	case worker.TierEngineer:
		if parentTier != worker.TierManager {
			return fmt.Errorf("engineer's parent must be a manager, got %s", parentTier)
		}
	case worker.TierManager:
		if parentTier != worker.TierConsultant {
			return fmt.Errorf("manager's parent must be a consultant, got %s", parentTier)
		}
	case worker.TierConsultant:
		return fmt.Errorf("consultant cannot have a parent")
	}
	return nil
}

// GetSubordinates returns workers whose ParentID matches the given worker ID.
func (m *Manager) GetSubordinates(workerID string) []*worker.Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*worker.Worker
	for _, w := range m.workers {
		if w.ParentID == workerID {
			result = append(result, w)
		}
	}
	return result
}

// GetManager returns the parent (manager) of a worker.
func (m *Manager) GetManager(workerID string) (*worker.Worker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, ok := m.workers[workerID]
	if !ok || w.ParentID == "" {
		return nil, false
	}
	parent, ok := m.workers[w.ParentID]
	return parent, ok
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

// DeleteWorker removes a worker by ID.
func (m *Manager) DeleteWorker(workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %q not found", workerID)
	}
	if w.Status != worker.WorkerIdle {
		return fmt.Errorf("cannot delete worker %q: status is %s (must be idle)", workerID, w.Status)
	}

	delete(m.workers, workerID)
	if err := m.saveWorkers(); err != nil {
		return err
	}

	m.emit(Event{
		Type:     EventWorkerSpawned,
		WorkerID: workerID,
		Message:  fmt.Sprintf("Worker removed: %s", w.Name),
	})
	return nil
}

// UpdateWorkerFields updates optional fields on a worker (parentID, modelVersion, backendID, skillProfile).
func (m *Manager) UpdateWorkerFields(workerID, parentID, modelVersion, backendID, skillProfile string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if parentID != "" {
		w.ParentID = parentID
	}
	if modelVersion != "" {
		w.ModelVersion = modelVersion
	}
	if backendID != "" {
		w.BackendID = backendID
	}
	// Allow clearing skill profile with special value "-"
	if skillProfile == "-" {
		w.SkillProfile = ""
	} else if skillProfile != "" {
		w.SkillProfile = skillProfile
	}

	// Re-validate hierarchy after changes
	if err := m.validateHierarchy(w); err != nil {
		return err
	}

	return m.saveWorkers()
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

	m.wg.Add(1)
	go m.watchCompletion(workerCtx, w, t, p)

	return nil
}

func (m *Manager) watchCompletion(ctx context.Context, w *worker.Worker, t *project.Task, p *project.Project) {
	defer m.wg.Done()
	defer func() {
		// Clean up cancel func from map to prevent leaks
		m.mu.Lock()
		delete(m.cancels, w.ID)
		m.mu.Unlock()
	}()

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
		// Check if this is a review task completed by a manager
		if t.ParentTaskID != "" && w.EffectiveTier() == worker.TierManager {
			// Reset manager to idle
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			m.saveWorkers()
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)

			if profile := m.personalityStore.GetProfile(w.ID); profile != nil {
				personality.ApplyEvent(profile, personality.EventTaskCompleted)
				personality.UpdateAutoMood(profile)
				m.personalityStore.Save()
			}

			m.mu.Unlock()

			// Handle review result
			m.review.HandleReviewResult(w, t, p, result)

			m.emit(Event{
				Type:     EventWorkerIdle,
				WorkerID: w.ID,
				Message:  fmt.Sprintf("Manager %s is idle", w.Name),
			})

			// Try to drain review queue and engage idle managers
			go m.engageIdleManagers(context.Background(), p.ID)
			return
		}

		// Check if engineer with a parent → route to manager review
		if w.EffectiveTier() == worker.TierEngineer && w.ParentID != "" {
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskReview)

			if profile := m.personalityStore.GetProfile(w.ID); profile != nil {
				personality.ApplyEvent(profile, personality.EventTaskCompleted)
				personality.UpdateAutoMood(profile)
				m.personalityStore.Save()
			}

			m.emit(Event{
				Type:      EventTaskCompleted,
				ProjectID: p.ID,
				TaskID:    t.ID,
				WorkerID:  w.ID,
				Message:   fmt.Sprintf("Task %q completed by engineer, routing to review", t.Title),
			})

			// Reset engineer to idle
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			m.saveWorkers()
			m.mu.Unlock()

			m.emit(Event{
				Type:     EventWorkerIdle,
				WorkerID: w.ID,
				Message:  fmt.Sprintf("Worker %s is idle", w.Name),
			})

			// Start manager review
			go func() {
				ctx := context.Background()
				m.review.StartReview(ctx, w, t, p)
			}()

			if m.autoSchedule {
				go m.tryAutoAssign(w.ID)
			}
			return
		}

		// Default: no review needed (consultant or engineer without parent)
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskReview)

		if profile := m.personalityStore.GetProfile(w.ID); profile != nil {
			personality.ApplyEvent(profile, personality.EventTaskCompleted)
			personality.UpdateAutoMood(profile)
			m.personalityStore.Save()
		}

		m.emit(Event{
			Type:      EventTaskCompleted,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   fmt.Sprintf("Task %q completed (reason: %s)", t.Title, result.Reason),
		})
	} else {
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskFailed)

		if profile := m.personalityStore.GetProfile(w.ID); profile != nil {
			personality.ApplyEvent(profile, personality.EventTaskFailed)
			personality.UpdateAutoMood(profile)
			m.personalityStore.Save()
		}

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
	projectID := p.ID

	m.mu.Unlock()

	if shouldAutoSchedule {
		go m.tryAutoAssign(workerID)
	}

	// Engage idle managers after task completion
	if len(promoted) > 0 {
		go m.engageIdleManagers(context.Background(), projectID)
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
		m.emit(Event{
			Type:     EventTaskFailed,
			TaskID:   task.ID,
			WorkerID: workerID,
			Message:  fmt.Sprintf("Auto-assign failed for task %q to worker %s: %v", task.Title, workerID, err),
		})
		return
	}

	m.emit(Event{
		Type:     EventAutoAssigned,
		TaskID:   task.ID,
		WorkerID: workerID,
		Message:  fmt.Sprintf("Auto-assigned task %q to worker %s", task.Title, workerID),
	})
}

// UpdateTaskStatusDirect updates a task's status directly (used by board drag-and-drop).
func (m *Manager) UpdateTaskStatusDirect(taskID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.projectStore.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}

	if err := m.projectStore.UpdateTaskStatus(taskID, project.TaskStatus(status)); err != nil {
		return err
	}

	m.emit(Event{
		Type:      EventTaskCompleted,
		ProjectID: t.ProjectID,
		TaskID:    taskID,
		Message:   fmt.Sprintf("Task %q status changed to %s", t.Title, status),
	})

	return nil
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

// AssignAllReady matches all ready tasks for a project to idle engineers.
func (m *Manager) AssignAllReady(ctx context.Context, projectID string) (int, error) {
	readyTasks := m.projectStore.ReadyTasksByPriority()
	idleWorkers := m.idleEngineers()

	assigned := 0
	for _, t := range readyTasks {
		if t.ProjectID != projectID {
			continue
		}
		if assigned >= len(idleWorkers) {
			break
		}
		if err := m.AssignTask(ctx, idleWorkers[assigned].ID, t.ID); err != nil {
			continue
		}
		assigned++
	}
	return assigned, nil
}

// LaunchWave assigns ready tasks for a specific milestone to idle engineers.
func (m *Manager) LaunchWave(ctx context.Context, projectID, milestone string) (int, error) {
	readyTasks := m.projectStore.ReadyTasksByPriority()
	idleWorkers := m.idleEngineers()

	assigned := 0
	for _, t := range readyTasks {
		if t.ProjectID != projectID || t.Milestone != milestone {
			continue
		}
		if assigned >= len(idleWorkers) {
			break
		}
		if err := m.AssignTask(ctx, idleWorkers[assigned].ID, t.ID); err != nil {
			continue
		}
		assigned++
	}
	return assigned, nil
}

// engageIdleManagers tries to utilize idle managers by draining review queue
// and assigning ready tasks that managers can handle.
func (m *Manager) engageIdleManagers(ctx context.Context, projectID string) {
	// 1. Drain review queue first
	m.review.DrainQueue(ctx)

	// 2. Check if idle managers can pick up ready tasks
	m.mu.RLock()
	var idleManagers []*worker.Worker
	for _, w := range m.workers {
		if w.Status == worker.WorkerIdle && w.EffectiveTier() == worker.TierManager {
			idleManagers = append(idleManagers, w)
		}
	}
	m.mu.RUnlock()

	if len(idleManagers) == 0 {
		return
	}

	readyTasks := m.projectStore.ReadyTasksByPriority()
	assigned := 0
	for _, t := range readyTasks {
		if t.ProjectID != projectID {
			continue
		}
		if assigned >= len(idleManagers) {
			break
		}
		if err := m.AssignTask(ctx, idleManagers[assigned].ID, t.ID); err != nil {
			continue
		}
		assigned++
	}
}

// idleEngineers returns all idle engineer workers.
func (m *Manager) idleEngineers() []*worker.Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*worker.Worker
	for _, w := range m.workers {
		if w.Status == worker.WorkerIdle && w.EffectiveTier() == worker.TierEngineer {
			result = append(result, w)
		}
	}
	return result
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

// SetCollector attaches a training data collector.
func (m *Manager) SetCollector(c *training.Collector) {
	m.collector = c
}

// SetFinetuneRunner sets the fine-tune runner and config for auto-trigger.
func (m *Manager) SetFinetuneRunner(runner *training.FinetuneRunner, cfg training.FinetuneConfig) {
	m.finetuneRunner = runner
	m.finetuneCfg = cfg
}

// SetMaxWorkers sets the maximum number of workers per tier.
func (m *Manager) SetMaxWorkers(tier worker.WorkerTier, max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxWorkers[tier] = max
}

// LoadMaxWorkers loads per-tier limits from config.
func (m *Manager) LoadMaxWorkers(tiers []config.WorkerTierConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, tc := range tiers {
		if tc.MaxWorkers > 0 {
			m.maxWorkers[worker.WorkerTier(tc.Tier)] = tc.MaxWorkers
		}
	}
}

// PromoteWorker upgrades a worker's tier (e.g. engineer → manager).
func (m *Manager) PromoteWorker(workerID string, newTier worker.WorkerTier) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %q not found", workerID)
	}

	oldTier := w.EffectiveTier()
	w.Tier = newTier
	if err := m.saveWorkers(); err != nil {
		return err
	}

	m.emit(Event{
		Type:     EventWorkerPromoted,
		WorkerID: workerID,
		Message:  fmt.Sprintf("Worker %s promoted from %s to %s", w.Name, oldTier, newTier),
	})
	return nil
}

// ReviewPipeline returns the review pipeline for external integration.
func (m *Manager) ReviewPipeline() *ReviewPipeline {
	return m.review
}

// PendingReviews returns the current review queue.
func (m *Manager) PendingReviews() []ReviewRequest {
	if m.review == nil {
		return nil
	}
	return m.review.PendingReviews()
}

// ProjectStore returns the underlying project store.
func (m *Manager) ProjectStore() *project.Store {
	return m.projectStore
}

// GetPersonalityStore returns the personality store.
func (m *Manager) GetPersonalityStore() *personality.Store {
	return m.personalityStore
}

// Shutdown cancels all active workers and waits for goroutines to exit.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.mu.Unlock()

	m.wg.Wait()
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
