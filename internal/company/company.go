package company

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/claudecli"
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
	narrator         *personality.Narrator
	review           *ReviewPipeline
	collector        *training.Collector
	finetuneRunner *training.FinetuneRunner
	finetuneCfg    training.FinetuneConfig
	maxWorkers     map[worker.WorkerTier]int // per-tier worker limits (0 = unlimited)
	shutdownCancel context.CancelFunc
	language       string // "en" or "zh-TW"
	langMu         sync.RWMutex
	chatProvider   ai.ChatProvider
	ollamaEndpoint string // kept for personality narrator
	ollamaModel    string // kept for personality narrator
	modelStrategy  *ModelStrategy
	circuitBreaker *CircuitBreaker
	dataDir          string
	humanGate        *HumanGate
	commMatrix       *CommunicationMatrix
	lastPaneContent  map[string]paneSnapshot
	lastHealthReport *HealthReport
	objectives       []Objective
	objectivesPath   string
	budgets          []MonthlyBudget
	budgetsPath      string
	analytics        []PerformanceSnapshot
	draining         atomic.Bool // debounce flag for drainReadyQueue
	trainingLoop     *training.TrainingLoop
	agenticLoopCfg   config.AgenticLoopConfig
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
	chatProvider ai.ChatProvider,
) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	personalityStore := personality.NewStore(dataDir)
	if err := personalityStore.Load(); err != nil {
		// Just log, don't fail startup
	}

	// Initialize narrator: prefer chatProvider > Claude CLI > Ollama
	ollamaEndpoint := os.Getenv("OLLAMA_ENDPOINT")
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://localhost:11434"
	}
	if ollamaModel == "" {
		ollamaModel = "llama3.2"
	}
	var aiGen personality.AIGenerator
	if chatProvider != nil {
		aiGen = personality.NewChatAdapter(chatProvider)
	} else if cli := claudecli.New(); cli != nil {
		aiGen = personality.NewChatAdapter(cli)
	} else {
		aiGen = personality.NewOllamaAdapter(ollamaEndpoint, ollamaModel)
	}
	narrator := personality.NewNarrator(aiGen)

	m := &Manager{
		projectStore:     projectStore,
		spawner:          spawner,
		gitOps:           gitOps,
		monitor:          monitor,
		tmuxClient:       tmuxClient,
		autoSchedule:     true,
		workers:          make(map[string]*worker.Worker),
		cancels:          make(map[string]context.CancelFunc),
		dataDir:          dataDir,
		workersPath:      filepath.Join(dataDir, "workers.yaml"),
		maxWorkers:       make(map[worker.WorkerTier]int),
		personalityStore: personalityStore,
		narrator:         narrator,
		chatProvider:     chatProvider,
		ollamaEndpoint:   ollamaEndpoint,
		ollamaModel:      ollamaModel,
		lastPaneContent:  make(map[string]paneSnapshot),
		objectivesPath:   objectivesFilePath(dataDir),
		budgetsPath:      budgetsFilePath(dataDir),
	}
	m.review = newReviewPipeline(m)
	m.trainingLoop = &training.TrainingLoop{}
	m.modelStrategy = NewModelStrategy()
	m.circuitBreaker = NewCircuitBreaker(m)
	m.commMatrix = NewCommunicationMatrix(m)
	m.humanGate = NewHumanGate(m, DefaultHumanGateConfig(), dataDir)

	bgCtx, bgCancel := context.WithCancel(context.Background())
	m.shutdownCancel = bgCancel

	if err := m.loadWorkers(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := m.loadObjectives(); err != nil {
		log.Printf("warning: failed to load objectives: %v", err)
	}
	if err := m.loadBudgets(); err != nil {
		log.Printf("warning: failed to load budgets: %v", err)
	}

	// Recovery: reset workers with stale tmux sessions to idle
	m.recoverStaleWorkers()

	// Startup health check: fix orphaned tasks, check deps, clean old gates
	m.lastHealthReport = m.RunHealthCheck()

	// Periodically persist personality data
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				m.personalityStore.SaveIfDirty()
			}
		}
	}()

	// Relationship decay: reduce affinity for stale relationships
	go func() {
		// For simulation purposes, decay runs every hour (not every 24h)
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				if m.personalityStore == nil {
					continue
				}
				for _, rel := range m.personalityStore.ListRelationships() {
					daysSince := time.Since(rel.LastInteraction).Hours() / 24
					if daysSince > 1 {
						delta := -1 * int(daysSince)
						m.personalityStore.UpdateRelationship(rel.WorkerA, rel.WorkerB, func(r *personality.Relationship) {
							r.AdjustAffinity(delta)
						})
					}
				}
				m.personalityStore.SaveIfDirty()
			}
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
		Phase:       project.PhasePRD,
	}
	if err := m.projectStore.SaveProject(p); err != nil {
		return nil, err
	}

	m.emit(Event{
		Type:      EventProjectCreated,
		ProjectID: p.ID,
		Message:   m.msgf("Project created: %s", "專案已建立：%s", name),
	})

	// Start PRD pipeline: create a PRD task instead of directly decomposing goals
	if len(goals) > 0 {
		go m.createPRDTask(p)
	}

	return p, nil
}

// DeleteProject removes a project and all its tasks.
// Refuses if any task is currently in progress or assigned.
func (m *Manager) DeleteProject(projectID string) error {
	// Check for active tasks
	tasks := m.projectStore.TasksForProject(projectID)
	for _, t := range tasks {
		if t.Status == project.TaskInProgress || t.Status == project.TaskAssigned {
			return fmt.Errorf("cannot delete project %q: task %q is %s", projectID, t.ID, t.Status)
		}
	}

	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return fmt.Errorf("project %q not found", projectID)
	}
	name := p.Name

	if err := m.projectStore.DeleteProject(projectID); err != nil {
		return err
	}

	m.emit(Event{
		Type:      EventProjectDeleted,
		ProjectID: projectID,
		Message:   m.msgf("Project deleted: %s", "專案已刪除：%s", name),
	})
	return nil
}

// ActiveWorkerCount returns the number of workers currently working on tasks.
func (m *Manager) ActiveWorkerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, w := range m.workers {
		if w.Status == worker.WorkerWorking || w.Status == worker.WorkerWaiting {
			count++
		}
	}
	return count
}

// ClearAllProjects deletes all projects and their tasks.
// If force is true, it also stops any workers currently working on tasks.
func (m *Manager) ClearAllProjects(force bool) error {
	m.mu.Lock()

	// Collect active workers
	activeWorkers := make([]*worker.Worker, 0)
	for _, w := range m.workers {
		if w.Status == worker.WorkerWorking || w.Status == worker.WorkerWaiting {
			activeWorkers = append(activeWorkers, w)
		}
	}

	if !force && len(activeWorkers) > 0 {
		m.mu.Unlock()
		return fmt.Errorf("%d workers are currently active", len(activeWorkers))
	}

	// Stop active workers: cancel monitoring, kill tmux sessions, reset state
	for _, w := range activeWorkers {
		if cancel, ok := m.cancels[w.ID]; ok {
			cancel()
			delete(m.cancels, w.ID)
		}
	}

	// Clean up ALL workers: kill any tmux sessions and reset state
	dirty := false
	for _, w := range m.workers {
		if w.TmuxSession != "" {
			m.tmuxClient.KillSession(w.TmuxSession)
			w.TmuxSession = ""
			dirty = true
		}
		if w.SessionID != "" {
			w.SessionID = ""
			dirty = true
		}
		if w.CurrentTaskID != "" {
			w.CurrentTaskID = ""
			dirty = true
		}
		if w.Status != worker.WorkerIdle {
			w.Status = worker.WorkerIdle
			dirty = true
		}
	}
	if dirty {
		m.saveWorkers()
		m.emit(Event{
			Type:    EventWorkerIdle,
			Message: m.msgf("All workers reset to idle", "所有員工已重設為閒置"),
		})
	}

	// Delete all projects
	projects := m.projectStore.ListProjects()
	m.mu.Unlock()

	for _, p := range projects {
		// Force-delete: update any non-idle tasks to ready first so DeleteProject won't reject
		tasks := m.projectStore.TasksForProject(p.ID)
		for _, t := range tasks {
			if t.Status == project.TaskInProgress || t.Status == project.TaskAssigned {
				m.projectStore.UpdateTaskStatus(t.ID, project.TaskReady)
			}
		}
		if err := m.DeleteProject(p.ID); err != nil {
			return fmt.Errorf("deleting project %q: %w", p.ID, err)
		}
	}

	m.emit(Event{
		Type:    EventProjectDeleted,
		Message: m.msgf("All projects cleared", "已清除全部專案"),
	})
	return nil
}

func (m *Manager) ListProjects() []*project.Project {
	return m.projectStore.ListProjects()
}

func (m *Manager) GetProject(id string) (*project.Project, bool) {
	return m.projectStore.GetProject(id)
}

// --- Task operations ---

func (m *Manager) AddTask(projectID, title, description, prompt string, dependsOn []string, priority int, milestone string, taskType string) (*project.Task, error) {
	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return nil, fmt.Errorf("project %q not found", projectID)
	}

	tt := project.TaskType(taskType)
	switch tt {
	case project.TaskTypeResearch, project.TaskTypePRD, project.TaskTypeDesign,
		project.TaskTypeAdmin, project.TaskTypeHR, project.TaskTypeTraining:
		// keep as-is
	default:
		tt = project.TaskTypeCode
	}

	slug := slugify(title)
	t := &project.Task{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Prompt:      prompt,
		Type:        tt,
		Priority:    priority,
		DependsOn:   dependsOn,
		Milestone:   milestone,
	}

	// Code and training tasks need a git branch
	if tt == project.TaskTypeCode || tt == project.TaskTypeTraining {
		t.BranchName = gitops.BranchName(p.ID, "", slug)
	}

	// Determine initial status based on dependencies
	if len(dependsOn) == 0 {
		t.Status = project.TaskReady
	}

	if err := m.projectStore.SaveTask(t); err != nil {
		return nil, err
	}

	// Fix branch name with actual task ID (only for code tasks)
	if tt == project.TaskTypeCode {
		t.BranchName = gitops.BranchName(p.ID, t.ID, slug)
		if err := m.projectStore.SaveTask(t); err != nil {
			return nil, err
		}
	}

	m.emit(Event{
		Type:      EventTaskCreated,
		ProjectID: projectID,
		TaskID:    t.ID,
		Message:   m.msgf("Task created: %s", "任務已建立：%s", title),
	})

	if t.Status == project.TaskReady && m.autoSchedule {
		go m.drainReadyQueue(context.Background())
	}

	return t, nil
}

func (m *Manager) ListTasks(projectID string) []*project.Task {
	return m.projectStore.TasksForProject(projectID)
}

func (m *Manager) GetTask(taskID string) (*project.Task, bool) {
	return m.projectStore.GetTask(taskID)
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

	profile := personality.NewCharacterProfile(w.ID, string(w.EffectiveTier()))
	m.personalityStore.SetProfile(profile)

	if m.narrator != nil {
		go func(workerID, workerName string, traits personality.PersonalityTraits) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			narrative, err := m.narrator.GeneratePersonality(ctx, workerName, traits)
			if err == nil && narrative != nil {
				m.personalityStore.UpdateProfile(workerID, func(p *personality.CharacterProfile) {
					p.Narrative = *narrative
				})
				m.emit(Event{
					Type:     EventNarrativeGenerated,
					WorkerID: workerID,
					Message:  m.msgf("Narrative generated for %s", "已為 %s 生成性格描述", workerName),
				})
			}
		}(w.ID, w.Name, profile.Traits)
	}

	if err := m.saveWorkers(); err != nil {
		return nil, err
	}

	m.emit(Event{
		Type:     EventWorkerSpawned,
		WorkerID: w.ID,
		Message:  m.msgf("Worker hired: %s (tier: %s)", "已雇用員工：%s（等級：%s）", name, w.EffectiveTier()),
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
		Message:  m.msgf("Worker removed: %s", "已移除員工：%s", w.Name),
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

// UpdateWorkerAppearance updates the pixel office appearance for a worker.
func (m *Manager) UpdateWorkerAppearance(workerID string, bodyRow int, outfit, hair string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %q not found", workerID)
	}

	w.Appearance = &worker.WorkerAppearance{
		BodyRow: bodyRow,
		Outfit:  outfit,
		Hair:    hair,
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
	if m.spawner == nil {
		m.projectStore.UpdateTaskStatus(taskID, project.TaskReady)
		return fmt.Errorf("spawner not configured")
	}
	if err := m.spawner.SpawnForTask(ctx, w, t, p); err != nil {
		m.projectStore.UpdateTaskStatus(taskID, project.TaskReady)
		// Schedule a delayed retry via drainReadyQueue
		go func() {
			time.Sleep(10 * time.Second)
			m.drainReadyQueue(context.Background())
		}()
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
		Message:   m.msgf("Task %q assigned to %s", "任務 %q 已分配給 %s", t.Title, w.Name),
	})

	m.emit(Event{
		Type:      EventBranchCreated,
		ProjectID: p.ID,
		TaskID:    taskID,
		Message:   m.msgf("Branch created: %s", "分支已建立：%s", t.BranchName),
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
		// Context cancelled (shutdown or manual pause) — reset worker to idle
		// so it doesn't remain stuck in "working" state
		m.mu.Lock()
		if w.Status == worker.WorkerWorking {
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			m.saveWorkers()
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskReady)
		}
		m.mu.Unlock()
		return
	}

	m.handleTaskCompletion(w, t, p, result)
}

func (m *Manager) handleTaskCompletion(w *worker.Worker, t *project.Task, p *project.Project, result worker.CompletionResult) {
	m.mu.Lock()

	if result.Success && t.Type == project.TaskTypeTraining {
		m.handleTrainingIteration(w, t, p)
		return
	}

	if result.Success && t.Type == project.TaskTypeResearch {
		m.handleResearchCompletion(w, t, p)
		return
	}

	if result.Success && t.Type == project.TaskTypePRD {
		m.handlePRDCompletion(w, t, p)
		return
	}

	if result.Success && t.Type == project.TaskTypeDesign {
		m.handleDesignCompletion(w, t, p)
		return
	}

	if result.Success && t.Type == project.TaskTypeAdmin {
		m.handleDesignCompletion(w, t, p)
		return
	}

	if result.Success && t.Type == project.TaskTypeHR {
		m.handleDesignCompletion(w, t, p)
		return
	}

	if result.Success {
		// Check if this is a review task (has a parent task)
		if t.ParentTaskID != "" {
			// Record token usage and analytics for review sub-task
			m.recordCompletionMetrics(w, t, true)

			// Reset manager to idle
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			m.saveWorkers()
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)

			m.personalityStore.UpdateProfile(w.ID, func(p *personality.CharacterProfile) {
				personality.ApplyEvent(p, personality.EventTaskCompleted)
				personality.UpdateAutoMood(p)
			})
			m.emit(Event{
				Type:     EventMoodChanged,
				WorkerID: w.ID,
				Message:  fmt.Sprintf("Mood changed for %s", w.Name),
			})

			m.mu.Unlock()

			// Handle review result
			m.review.HandleReviewResult(w, t, p, result)

			m.emit(Event{
				Type:     EventWorkerIdle,
				WorkerID: w.ID,
				Message:  m.msgf("Manager %s is idle", "管理員 %s 已閒置", w.Name),
			})

			// Try to drain review queue and engage idle managers
			go m.engageIdleManagers(context.Background(), p.ID)
			return
		}

		// Check if engineer with a parent → route to manager review
		// Guard: skip review for tasks that are themselves review sub-tasks (have ParentTaskID)
		if w.EffectiveTier() == worker.TierEngineer && w.ParentID != "" && t.ParentTaskID == "" {
			// Record token usage and analytics before routing to review
			m.recordCompletionMetrics(w, t, true)

			m.projectStore.UpdateTaskStatus(t.ID, project.TaskCodeReview)

			m.personalityStore.UpdateProfile(w.ID, func(prof *personality.CharacterProfile) {
				personality.ApplyEvent(prof, personality.EventTaskCompleted)
				personality.UpdateAutoMood(prof)
			})
			m.emit(Event{
				Type:     EventMoodChanged,
				WorkerID: w.ID,
				Message:  fmt.Sprintf("Mood changed for %s", w.Name),
			})

			m.emit(Event{
				Type:      EventTaskCompleted,
				ProjectID: p.ID,
				TaskID:    t.ID,
				WorkerID:  w.ID,
				Message:   m.msgf("Task %q completed by engineer, routing to review", "任務 %q 已由工程師完成，轉至審查", t.Title),
			})

			// Reset engineer to idle
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			m.saveWorkers()
			m.mu.Unlock()

			m.emit(Event{
				Type:     EventWorkerIdle,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s is idle", "員工 %s 已閒置", w.Name),
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
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)

		m.personalityStore.UpdateProfile(w.ID, func(prof *personality.CharacterProfile) {
			personality.ApplyEvent(prof, personality.EventTaskCompleted)
			personality.UpdateAutoMood(prof)
		})
		m.emit(Event{
			Type:     EventMoodChanged,
			WorkerID: w.ID,
			Message:  fmt.Sprintf("Mood changed for %s", w.Name),
		})

		m.emit(Event{
			Type:      EventTaskCompleted,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   m.msgf("Task %q completed (reason: %s)", "任務 %q 已完成（原因：%s）", t.Title, result.Reason),
		})
	} else {
		const maxRetries = 2

		m.personalityStore.UpdateProfile(w.ID, func(prof *personality.CharacterProfile) {
			personality.ApplyEvent(prof, personality.EventTaskFailed)
			personality.UpdateAutoMood(prof)
		})
		m.emit(Event{
			Type:     EventMoodChanged,
			WorkerID: w.ID,
			Message:  fmt.Sprintf("Mood changed for %s", w.Name),
		})

		// Auto-retry if under the retry limit
		if t.RetryCount < maxRetries {
			t.RetryCount++
			m.projectStore.SaveTask(t)
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskReady)
			m.emit(Event{
				Type:      EventTaskFailed,
				ProjectID: p.ID,
				TaskID:    t.ID,
				WorkerID:  w.ID,
				Message:   m.msgf("Task %q failed, auto-retrying (%d/%d)", "任務 %q 失敗，自動重試（%d/%d）", t.Title, t.RetryCount, maxRetries),
			})
		} else {
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskFailed)
			m.emit(Event{
				Type:      EventTaskFailed,
				ProjectID: p.ID,
				TaskID:    t.ID,
				WorkerID:  w.ID,
				Message:   m.msgf("Task %q failed after %d retries", "任務 %q 重試 %d 次後仍然失敗", t.Title, maxRetries),
			})
		}
	}

	// Record token usage, analytics, and budget checks
	m.recordCompletionMetrics(w, t, result.Success)

	// Check for delegation output from managers
	if result.Success && (w.EffectiveTier() == worker.TierManager || w.EffectiveTier() == worker.TierConsultant) {
		m.handleDelegationOutput(w, t, p)
	}

	// Reset worker to idle
	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	m.saveWorkers()

	m.emit(Event{
		Type:     EventWorkerIdle,
		WorkerID: w.ID,
		Message:  m.msgf("Worker %s is idle", "員工 %s 已閒置", w.Name),
	})

	// Promote newly unblocked tasks
	promoted, _ := m.projectStore.PromoteReady(p.ID)
	for _, pt := range promoted {
		m.emit(Event{
			Type:      EventTaskCreated,
			ProjectID: p.ID,
			TaskID:    pt.ID,
			Message:   m.msgf("Task %q is now ready (dependencies resolved)", "任務 %q 已就緒（依賴已解決）", pt.Title),
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
		go m.drainReadyQueue(context.Background())
	}

	// Check if project is fully completed
	go m.checkProjectCompletion(projectID)
}

// handleTrainingIteration runs the test command, evaluates the score,
// and either commits (improved) or rolls back (no improvement).
// If iterations remain and the pass threshold is not reached, it re-spawns the worker.
// Must be called with m.mu held (acquired in handleTaskCompletion).
func (m *Manager) handleTrainingIteration(w *worker.Worker, t *project.Task, p *project.Project) {
	cfg := t.TrainingConfig
	if cfg == nil {
		log.Printf("training: task %s has no TrainingConfig, treating as done", t.ID)
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)
		w.Status = worker.WorkerIdle
		w.CurrentTaskID = ""
		m.saveWorkers()
		m.mu.Unlock()
		return
	}

	// Record metrics
	m.recordCompletionMetrics(w, t, true)

	// Run test evaluation (release lock during I/O)
	m.mu.Unlock()

	result, err := m.trainingLoop.EvaluateAndDecide(p.RepoPath, t.BranchName, cfg)
	if err != nil {
		log.Printf("training: evaluate failed for task %s: %v", t.ID, err)
		m.emit(Event{
			Type:      EventTaskFailed,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   m.msgf("Training evaluation failed: %v", "訓練評估失敗：%v", err),
		})
		m.mu.Lock()
		m.projectStore.UpdateTaskStatus(t.ID, project.TaskFailed)
		w.Status = worker.WorkerIdle
		w.CurrentTaskID = ""
		m.saveWorkers()
		m.mu.Unlock()
		return
	}

	// Update training config with iteration results
	m.mu.Lock()
	cfg.CurrentIter = result.Iteration
	cfg.LastTestOutput = result.TestOutput
	if result.Improved {
		cfg.BestScore = result.Score
		cfg.BestCommit = result.CommitHash
	}
	m.projectStore.SaveTask(t)

	if result.Improved {
		m.emit(Event{
			Type:      EventTaskCompleted,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   m.msgf("Training iteration %d/%d: improved to %.4f", "訓練迭代 %d/%d：進步至 %.4f", result.Iteration, cfg.MaxIterations, result.Score),
		})
	} else {
		m.emit(Event{
			Type:      EventTaskFailed,
			ProjectID: p.ID,
			TaskID:    t.ID,
			WorkerID:  w.ID,
			Message:   m.msgf("Training iteration %d/%d: rolled back (score %.4f <= best %.4f)", "訓練迭代 %d/%d：已回退（分數 %.4f <= 最佳 %.4f）", result.Iteration, cfg.MaxIterations, result.Score, cfg.BestScore),
		})
	}

	// Check if we should continue iterating
	if cfg.CurrentIter < cfg.MaxIterations && cfg.BestScore < cfg.PassThreshold {
		// Re-spawn worker for next iteration
		w.Status = worker.WorkerIdle
		w.CurrentTaskID = ""
		m.saveWorkers()
		m.mu.Unlock()

		go func() {
			ctx := context.Background()
			if err := m.AssignTask(ctx, t.ID, w.ID); err != nil {
				log.Printf("training: failed to re-assign task %s to %s: %v", t.ID, w.ID, err)
			}
		}()
		return
	}

	// Training complete
	m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)
	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	m.saveWorkers()

	m.emit(Event{
		Type:      EventTaskCompleted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		WorkerID:  w.ID,
		Message:   m.msgf("Training complete: %d iterations, best score %.4f", "訓練完成：%d 輪迭代，最佳分數 %.4f", cfg.CurrentIter, cfg.BestScore),
	})

	m.mu.Unlock()

	if m.autoSchedule {
		go m.tryAutoAssign(w.ID)
	}
}

// SaveTask persists a task to the store.
func (m *Manager) SaveTask(t *project.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projectStore.SaveTask(t)
}

// SetAgenticLoopConfig sets the agentic loop configuration.
func (m *Manager) SetAgenticLoopConfig(cfg config.AgenticLoopConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agenticLoopCfg = cfg
}

// GetAgenticLoopConfig returns the current agentic loop configuration.
func (m *Manager) GetAgenticLoopConfig() config.AgenticLoopConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agenticLoopCfg
}

// recordCompletionMetrics captures token usage, analytics snapshot, and budget checks.
// Must be called with m.mu held.
func (m *Manager) recordCompletionMetrics(w *worker.Worker, t *project.Task, success bool) {
	if w.TmuxSession != "" && m.tmuxClient != nil {
		paneOut, err := m.tmuxClient.CapturePane(w.TmuxSession, w.Window, w.Pane, 500)
		if err == nil {
			tokens := worker.ParseTokenUsage(paneOut)
			if tokens > 0 {
				m.RecordTokenUsage(t.ID, tokens)
			}
		}
	}
	m.recordAnalyticsSnapshot(w.ID, success)

	// Reload task to get updated TokensConsumed after RecordTokenUsage
	if updated, ok := m.projectStore.GetTask(t.ID); ok {
		if updated.BudgetLimit > 0 {
			if m.circuitBreaker.CheckBudget(updated) {
				m.emit(Event{
					Type:      EventTaskEscalated,
					ProjectID: t.ProjectID,
					TaskID:    t.ID,
					Message:   m.msgf("Task %q exceeded budget limit", "任務 %q 已超出預算上限", t.Title),
				})
			} else if m.circuitBreaker.BudgetWarning(updated) {
				m.emit(Event{
					Type:      EventBudgetWarning,
					ProjectID: t.ProjectID,
					TaskID:    t.ID,
					Message:   m.msgf("Task %q approaching budget limit (>80%% used)", "任務 %q 即將達到預算上限（已使用 >80%%）", t.Title),
				})
			}
		}
	}
}

// handleResearchCompletion processes a completed research task: extracts the JSON
// report from the tmux pane output, saves it, and marks the task as done.
// Must be called with m.mu held. Releases m.mu before returning (does not re-acquire).
func (m *Manager) handleResearchCompletion(w *worker.Worker, t *project.Task, p *project.Project) {
	// Try to extract report from tmux pane content
	var rawOutput string
	if m.tmuxClient != nil && w.TmuxSession != "" {
		content, err := m.tmuxClient.CapturePane(w.TmuxSession, 0, 0, 500)
		if err == nil {
			rawOutput = content
		}
	}

	// Parse JSON report from output
	report := parseResearchReport(rawOutput)
	report.TaskID = t.ID
	report.ProjectID = p.ID
	report.WorkerID = w.ID

	// Save report
	if err := m.projectStore.SaveReport(report); err != nil {
		log.Printf("failed to save research report for task %s: %v", t.ID, err)
	}

	// Record token usage and analytics
	m.recordCompletionMetrics(w, t, true)

	// Mark task as done (research tasks skip review)
	if err := m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone); err != nil {
		log.Printf("failed to update task %s status to done: %v", t.ID, err)
	}

	m.personalityStore.UpdateProfile(w.ID, func(prof *personality.CharacterProfile) {
		personality.ApplyEvent(prof, personality.EventTaskCompleted)
		personality.UpdateAutoMood(prof)
	})
	m.emit(Event{
		Type:     EventMoodChanged,
		WorkerID: w.ID,
		Message:  fmt.Sprintf("Mood changed for %s", w.Name),
	})

	m.emit(Event{
		Type:      EventResearchCompleted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		WorkerID:  w.ID,
		Message:   m.msgf("%s completed research task %q — check the report.", "%s 完成了研究任務「%s」，請查看報告。", w.Name, t.Title),
	})

	// Reset worker to idle
	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	m.saveWorkers()

	m.emit(Event{
		Type:     EventWorkerIdle,
		WorkerID: w.ID,
		Message:  m.msgf("Worker %s is idle", "員工 %s 已閒置", w.Name),
	})

	// Promote newly unblocked tasks
	promoted, _ := m.projectStore.PromoteReady(p.ID)
	for _, pt := range promoted {
		m.emit(Event{
			Type:      EventTaskCreated,
			ProjectID: p.ID,
			TaskID:    pt.ID,
			Message:   m.msgf("Task %q is now ready (dependencies resolved)", "任務 %q 已就緒（依賴已解決）", pt.Title),
		})
	}

	shouldAutoSchedule := m.autoSchedule
	workerID := w.ID
	projectID := p.ID

	m.mu.Unlock()

	if shouldAutoSchedule {
		go m.tryAutoAssign(workerID)
	}
	if len(promoted) > 0 {
		go m.engageIdleManagers(context.Background(), projectID)
		go m.drainReadyQueue(context.Background())
	}

	// Check if project is fully completed
	go m.checkProjectCompletion(projectID)
}

// parseResearchReport attempts to extract a structured research report from raw output.
func parseResearchReport(raw string) *project.ResearchReport {
	report := &project.ResearchReport{
		RawContent: raw,
	}

	// Try to find JSON in the output
	type reportJSON struct {
		Summary         string   `json:"summary"`
		KeyFindings     []string `json:"keyFindings"`
		Recommendations []string `json:"recommendations"`
		References      []string `json:"references"`
		RawContent      string   `json:"rawContent"`
	}

	jsonStr := extractJSON(raw)
	if jsonStr != "" {
		var parsed reportJSON
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			report.Summary = parsed.Summary
			report.KeyFindings = parsed.KeyFindings
			report.Recommendations = parsed.Recommendations
			report.References = parsed.References
			if parsed.RawContent != "" {
				report.RawContent = parsed.RawContent
			}
		}
	}

	return report
}

// extractJSON finds the first complete JSON object in a string.
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(text); i++ {
		if escape {
			escape = false
			continue
		}
		ch := text[i]
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

// idleWorkerSnapshot holds the immutable fields needed for task matching.
type idleWorkerSnapshot struct {
	ID           string
	SkillProfile string
	Tier         worker.WorkerTier
}

// drainReadyQueue assigns ready tasks to idle workers until no more matches.
func (m *Manager) drainReadyQueue(ctx context.Context) {
	// Debounce: only one drain can run at a time
	if !m.draining.CompareAndSwap(false, true) {
		return
	}
	defer m.draining.Store(false)

	readyTasks := m.projectStore.ReadyTasksByPriority()
	if len(readyTasks) == 0 {
		return
	}

	// Snapshot idle workers (IDs, skill profiles, and tiers)
	m.mu.RLock()
	var idle []idleWorkerSnapshot
	for _, w := range m.workers {
		if w.Status == worker.WorkerIdle {
			idle = append(idle, idleWorkerSnapshot{ID: w.ID, SkillProfile: w.SkillProfile, Tier: w.EffectiveTier()})
		}
	}
	m.mu.RUnlock()

	assignedMap := make(map[string]bool)
	for _, t := range readyTasks {
		best := matchWorker(t, idle, assignedMap)
		if best == "" {
			continue
		}
		if err := m.AssignTask(ctx, best, t.ID); err != nil {
			continue // worker might have become busy, skip
		}
		assignedMap[best] = true
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
			Message:  m.msgf("Auto-assign failed for task %q to worker %s: %v", "自動分配任務 %q 給員工 %s 失敗：%v", task.Title, workerID, err),
		})
		return
	}

	m.emit(Event{
		Type:     EventAutoAssigned,
		TaskID:   task.ID,
		WorkerID: workerID,
		Message:  m.msgf("Auto-assigned task %q to worker %s", "已自動分配任務 %q 給員工 %s", task.Title, workerID),
	})
}

// UpdateTaskStatusDirect updates a task's status directly (used by board drag-and-drop).
func (m *Manager) UpdateTaskStatusDirect(taskID string, status string) error {
	m.mu.Lock()

	t, ok := m.projectStore.GetTask(taskID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("task %q not found", taskID)
	}

	if err := m.projectStore.ForceUpdateTaskStatus(taskID, project.TaskStatus(status)); err != nil {
		m.mu.Unlock()
		return err
	}

	m.emit(Event{
		Type:      EventTaskCompleted,
		ProjectID: t.ProjectID,
		TaskID:    taskID,
		Message:   m.msgf("Task %q status changed to %s", "任務 %q 狀態已變更為 %s", t.Title, status),
	})

	shouldDrain := project.TaskStatus(status) == project.TaskReady && m.autoSchedule
	m.mu.Unlock()

	if shouldDrain {
		go m.drainReadyQueue(context.Background())
	}

	return nil
}

// CompleteTask manually marks a task as done (used by supervisor/UI for review → done).
func (m *Manager) CompleteTask(taskID string) error {
	m.mu.Lock()

	t, ok := m.projectStore.GetTask(taskID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("task %q not found", taskID)
	}

	if err := m.projectStore.ForceUpdateTaskStatus(taskID, project.TaskDone); err != nil {
		m.mu.Unlock()
		return err
	}

	// Reset assignee worker to idle
	if t.AssigneeID != "" {
		if w, ok := m.workers[t.AssigneeID]; ok && w.Status == worker.WorkerWorking {
			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			m.saveWorkers()
			m.emit(Event{
				Type:     EventWorkerIdle,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s is idle", "員工 %s 已閒置", w.Name),
			})
		}
	}

	m.emit(Event{
		Type:      EventTaskCompleted,
		ProjectID: t.ProjectID,
		TaskID:    taskID,
		Message:   m.msgf("Task %q marked as done", "任務 %q 已標記為完成", t.Title),
	})

	// Promote newly unblocked tasks
	promoted, _ := m.projectStore.PromoteReady(t.ProjectID)
	for _, pt := range promoted {
		m.emit(Event{
			Type:      EventTaskCreated,
			ProjectID: t.ProjectID,
			TaskID:    pt.ID,
			Message:   m.msgf("Task %q is now ready (dependencies resolved)", "任務 %q 已就緒（依賴已解決）", pt.Title),
		})
	}

	shouldDrain := len(promoted) > 0 && m.autoSchedule
	m.mu.Unlock()

	if shouldDrain {
		go m.drainReadyQueue(context.Background())
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
		case project.TaskDone, project.TaskDeployed:
			dto.Done++
		case project.TaskInProgress, project.TaskAssigned, project.TaskCodeReview, project.TaskReview, project.TaskTesting, project.TaskRevision:
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

// checkProjectCompletion checks if all tasks in a project are done/failed and triggers retro.
func (m *Manager) checkProjectCompletion(projectID string) {
	progress := m.ProjectProgress(projectID)
	if progress.Total == 0 {
		return
	}
	// Project is complete when all tasks are either done or failed
	if progress.Done+progress.Failed < progress.Total {
		return
	}

	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return
	}
	// Avoid re-triggering if already completed
	if p.Status == project.ProjectCompleted {
		return
	}

	p.Status = project.ProjectCompleted
	m.projectStore.SaveProject(p)

	m.emit(Event{
		Type:      EventProjectCompleted,
		ProjectID: projectID,
		Message:   m.msgf("Project %q completed (%d done, %d failed)", "專案「%s」已完成（%d 完成、%d 失敗）", p.Name, progress.Done, progress.Failed),
	})

	// Auto-update linked objective progress
	m.updateObjectiveProgress(projectID)

	// Trigger retro automatically
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		if err := m.RunRetro(ctx, projectID); err != nil {
			log.Printf("WARNING: auto-retro for project %s failed: %v", projectID, err)
		}
	}()
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

// LoadHumanGateConfig applies human gate settings from config.
func (m *Manager) LoadHumanGateConfig(cfg config.HumanGateConfig) {
	m.humanGate = NewHumanGate(m, HumanGateConfig{
		Enabled:               cfg.Enabled,
		TokenBudgetThreshold:  cfg.TokenBudgetThreshold,
		RequireDeployApproval: cfg.RequireDeployApproval,
		ConfidenceFloor:       cfg.ConfidenceFloor,
	}, m.dataDir)
}

// SetLanguage sets the prompt language for the company system.
func (m *Manager) SetLanguage(lang string) {
	m.langMu.Lock()
	defer m.langMu.Unlock()
	m.language = lang
}

// GetLanguage returns the current prompt language, defaulting to "zh-TW".
// Uses a separate lock (langMu) to avoid deadlocks when called while m.mu is held.
func (m *Manager) GetLanguage() string {
	m.langMu.RLock()
	defer m.langMu.RUnlock()
	if m.language == "" {
		return "zh-TW"
	}
	return m.language
}

// msg returns en or zh string based on current language setting.
// Safe to call without holding m.mu (uses GetLanguage which acquires its own lock).
func (m *Manager) msg(en, zh string) string {
	if m.GetLanguage() == "en" {
		return en
	}
	return zh
}

// msgf returns a formatted bilingual string.
func (m *Manager) msgf(enFmt, zhFmt string, args ...interface{}) string {
	if m.GetLanguage() == "en" {
		return fmt.Sprintf(enFmt, args...)
	}
	return fmt.Sprintf(zhFmt, args...)
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
		Message:  m.msgf("Worker %s promoted from %s to %s", "員工 %s 已從 %s 升遷至 %s", w.Name, oldTier, newTier),
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

// GetNarrator returns the narrator instance (may be nil if Ollama is not configured).
func (m *Manager) GetNarrator() *personality.Narrator {
	return m.narrator
}

// GetModelStrategy returns the model strategy.
func (m *Manager) GetModelStrategy() *ModelStrategy {
	return m.modelStrategy
}

// GetCircuitBreaker returns the circuit breaker.
func (m *Manager) GetCircuitBreaker() *CircuitBreaker {
	return m.circuitBreaker
}

// GetHumanGate returns the human gate.
func (m *Manager) GetHumanGate() *HumanGate {
	return m.humanGate
}

// GetCommunicationMatrix returns the communication matrix.
func (m *Manager) GetCommunicationMatrix() *CommunicationMatrix {
	return m.commMatrix
}

// GetLastHealthReport returns the health report from the last startup check.
func (m *Manager) GetLastHealthReport() *HealthReport {
	return m.lastHealthReport
}

// NeedsOnboarding returns true if no workers have been created yet.
func (m *Manager) NeedsOnboarding() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers) == 0
}

// SetChatProvider replaces the current chat provider (used for runtime backend switching).
func (m *Manager) SetChatProvider(cp ai.ChatProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatProvider = cp
}

// GetChatProvider returns the current chat provider.
func (m *Manager) GetChatProvider() ai.ChatProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.chatProvider
}

// Shutdown cancels all active workers and waits for goroutines to exit.
func (m *Manager) Shutdown() {
	if m.shutdownCancel != nil {
		m.shutdownCancel()
	}

	m.mu.Lock()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.mu.Unlock()

	m.wg.Wait()

	if m.personalityStore != nil {
		m.personalityStore.Save()
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

// ResetWorker forces a worker back to idle, killing its tmux session if active.
func (m *Manager) ResetWorker(workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Cancel completion monitor
	if cancel, ok := m.cancels[workerID]; ok {
		cancel()
		delete(m.cancels, workerID)
	}

	// Kill tmux session
	if w.TmuxSession != "" && m.tmuxClient != nil {
		_ = m.tmuxClient.KillSession(w.TmuxSession)
	}

	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	w.TmuxSession = ""
	w.SessionID = ""
	m.saveWorkers()

	delete(m.lastPaneContent, workerID)

	m.emit(Event{
		Type:     EventWorkerIdle,
		WorkerID: workerID,
		Message:  m.msgf("Worker %s has been reset to idle", "員工 %s 已重設為閒置", w.Name),
	})
	return nil
}

// PauseWorker sends Ctrl+C to the worker's tmux pane, cancels the monitor, and sets status to paused.
func (m *Manager) PauseWorker(workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workers[workerID]
	if !ok {
		return fmt.Errorf("worker %s not found", workerID)
	}
	if w.Status != worker.WorkerWorking {
		return fmt.Errorf("worker %s is not working (status: %s)", workerID, w.Status)
	}

	// Cancel completion monitor
	if cancel, ok := m.cancels[workerID]; ok {
		cancel()
		delete(m.cancels, workerID)
	}

	// Send Ctrl+C to tmux pane and verify it stopped
	if w.TmuxSession != "" && m.tmuxClient != nil {
		_ = m.tmuxClient.SendKeys(w.TmuxSession, w.Window, w.Pane, "C-c")
		// Send a second Ctrl+C after a short delay to handle cases where the first
		// is caught (e.g., during file writes)
		time.Sleep(500 * time.Millisecond)
		_ = m.tmuxClient.SendKeys(w.TmuxSession, w.Window, w.Pane, "C-c")
	}

	w.Status = worker.WorkerPaused
	m.saveWorkers()

	m.emit(Event{
		Type:     EventWorkerPaused,
		WorkerID: workerID,
		Message:  m.msgf("Worker %s has been paused", "員工 %s 已暫停", w.Name),
	})
	return nil
}

// ResumeWorker re-sends the task prompt to a paused worker and restarts the monitor.
func (m *Manager) ResumeWorker(ctx context.Context, workerID string) error {
	m.mu.Lock()

	w, ok := m.workers[workerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("worker %s not found", workerID)
	}
	if w.Status != worker.WorkerPaused {
		m.mu.Unlock()
		return fmt.Errorf("worker %s is not paused (status: %s)", workerID, w.Status)
	}

	taskID := w.CurrentTaskID
	w.Status = worker.WorkerWorking
	m.saveWorkers()
	m.mu.Unlock()

	// Get the task and project to rebuild the prompt
	t, ok := m.projectStore.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	p, ok := m.projectStore.GetProject(t.ProjectID)
	if !ok {
		return fmt.Errorf("project %s not found", t.ProjectID)
	}

	// Try to resume in the existing tmux session first.
	// If session is still alive, re-send the prompt directly instead of spawning a new session.
	sessionAlive := false
	if w.TmuxSession != "" && m.tmuxClient != nil {
		if exists, _ := m.tmuxClient.HasSession(w.TmuxSession); exists {
			sessionAlive = true
		}
	}

	if sessionAlive {
		// Existing session still alive — re-send the prompt to continue
		resumePrompt := t.Prompt
		if m.GetLanguage() == "en" {
			resumePrompt = "RESUME: You were previously interrupted. Continue the following task from where you left off.\n\n" + resumePrompt
		} else {
			resumePrompt = "繼續：你先前被中斷了。請從上次中斷的地方繼續以下任務。\n\n" + resumePrompt
		}
		if err := m.spawner.SendPromptToExisting(w, resumePrompt); err != nil {
			// Fall through to full re-spawn
			sessionAlive = false
		}
	}

	if !sessionAlive {
		// Session dead or prompt send failed — full re-spawn
		if err := m.spawner.SpawnForTask(ctx, w, t, p); err != nil {
			m.mu.Lock()
			w.Status = worker.WorkerPaused
			m.saveWorkers()
			m.mu.Unlock()
			return fmt.Errorf("resume worker: %w", err)
		}
	}

	// Restart completion monitor (use background context so it survives frontend reloads)
	monCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancels[workerID] = cancel
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		result, err := m.monitor.WatchForCompletion(monCtx, w)
		if err != nil {
			return
		}
		m.handleTaskCompletion(w, t, p, result)
	}()

	m.emit(Event{
		Type:     EventWorkerResumed,
		WorkerID: workerID,
		Message:  m.msgf("Worker %s has been resumed", "員工 %s 已恢復", w.Name),
	})
	return nil
}

// ReassignTask unassigns a task from its current worker and reassigns it to a new one.
func (m *Manager) ReassignTask(ctx context.Context, taskID, newWorkerID string) error {
	task, ok := m.projectStore.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Reset old assignee if any
	if task.AssigneeID != "" {
		if err := m.ResetWorker(task.AssigneeID); err != nil {
			log.Printf("WARN: failed to reset old assignee %s: %v", task.AssigneeID, err)
		}
	}

	// Reset task to ready
	if err := m.projectStore.ForceUpdateTaskStatus(taskID, project.TaskReady); err != nil {
		return fmt.Errorf("reset task status: %w", err)
	}
	task.AssigneeID = ""
	if err := m.projectStore.SaveTask(task); err != nil {
		return fmt.Errorf("clear assignee: %w", err)
	}

	// Assign to new worker
	return m.AssignTask(ctx, newWorkerID, taskID)
}

// DrainReviewQueue forces processing of all pending review requests.
func (m *Manager) DrainReviewQueue(ctx context.Context) {
	m.review.DrainQueue(ctx)
}
