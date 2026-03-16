package company

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/binpath"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// paneSnapshot records captured pane content and when it was first seen unchanged.
type paneSnapshot struct {
	content string
	since   time.Time
}

// HealthReport summarises what the startup health check found and fixed.
type HealthReport struct {
	StaleWorkersReset  int      `json:"staleWorkersReset"`
	OrphanedTasksFixed int      `json:"orphanedTasksFixed"`
	MissingDeps        []string `json:"missingDeps"`
	Warnings           []string `json:"warnings"`
	GatesCleaned       int      `json:"gatesCleaned"`
}

// RunHealthCheck performs a one-time startup health check:
//   - resets orphaned tasks (in_progress/assigned but worker is idle with no tmux)
//   - checks for required external dependencies
//   - cleans old resolved gate requests
func (m *Manager) RunHealthCheck() *HealthReport {
	report := &HealthReport{}

	// 1. Check external dependencies
	for _, dep := range []string{"tmux", "claude", "git"} {
		if _, err := exec.LookPath(dep); err != nil {
			report.MissingDeps = append(report.MissingDeps, dep)
		}
	}

	// 2. Fix orphaned tasks: in_progress/assigned but assignee worker is idle and has no tmux session
	m.mu.RLock()
	workersCopy := make(map[string]*worker.Worker)
	for id, w := range m.workers {
		workersCopy[id] = w
	}
	m.mu.RUnlock()

	for _, p := range m.projectStore.ListProjects() {
		for _, t := range m.projectStore.TasksForProject(p.ID) {
			if t.Status != project.TaskInProgress && t.Status != project.TaskAssigned {
				continue
			}
			if t.AssigneeID == "" {
				continue
			}
			w, ok := workersCopy[t.AssigneeID]
			if !ok {
				// Assignee worker doesn't exist — reset task
				log.Printf("HEALTH: task %s assigned to nonexistent worker %s — resetting to ready", t.ID, t.AssigneeID)
				m.projectStore.ForceUpdateTaskStatus(t.ID, project.TaskReady)
				m.projectStore.UpdateTaskAssignee(t.ID, "")
				report.OrphanedTasksFixed++
				continue
			}
			if w.Status == worker.WorkerIdle && w.TmuxSession == "" {
				log.Printf("HEALTH: task %s assigned to idle worker %s with no tmux — resetting to ready", t.ID, w.ID)
				m.projectStore.ForceUpdateTaskStatus(t.ID, project.TaskReady)
				m.projectStore.UpdateTaskAssignee(t.ID, "")
				report.OrphanedTasksFixed++
			}
		}
	}

	// 3. Clean old resolved gate requests (older than 7 days)
	if m.humanGate != nil {
		report.GatesCleaned = m.humanGate.CleanOldRequests(7 * 24 * time.Hour)
	}

	if len(report.MissingDeps) > 0 {
		report.Warnings = append(report.Warnings, "Missing dependencies: "+joinStrings(report.MissingDeps))
	}
	if report.OrphanedTasksFixed > 0 {
		log.Printf("HEALTH: fixed %d orphaned tasks", report.OrphanedTasksFixed)
	}

	return report
}

// CheckDependencies returns a list of missing required external dependencies.
// It checks the bundled bin directory first, then falls back to system PATH.
func CheckDependencies() []string {
	var missing []string
	for _, dep := range []string{"tmux", "claude", "git"} {
		if !findDep(dep) {
			missing = append(missing, dep)
		}
	}
	return missing
}

// findDep checks for a dependency in bundled bin first, then system PATH.
func findDep(name string) bool {
	// Check bundled bin directory
	if bundled := binpath.BundledBinDir(); bundled != "" {
		candidate := filepath.Join(bundled, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	// Fallback to system PATH
	_, err := exec.LookPath(name)
	return err == nil
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// StartHealthCheck runs a periodic background health check every 60 seconds.
// It detects orphaned tmux sessions, stalled workers, and proactively assigns tasks.
func (m *Manager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runHealthCycle()
			m.proactiveTaskDiscovery()
		}
	}
}

// runHealthCycle performs a single health check iteration.
func (m *Manager) runHealthCycle() {
	if m.tmuxClient == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.workers {
		if w.Status != worker.WorkerWorking && w.Status != worker.WorkerWaiting {
			continue
		}
		if w.TmuxSession == "" {
			continue
		}

		// Check if tmux session still exists
		has, err := m.tmuxClient.HasSession(w.TmuxSession)
		if err != nil || !has {
			log.Printf("HEALTH: worker %s (%s) has orphaned session %q — attempting auto-recovery", w.ID, w.Name, w.TmuxSession)

			// Try to respawn if task is still valid
			if m.autoRecover(w) {
				continue
			}

			// Recovery not possible — reset worker
			if cancel, ok := m.cancels[w.ID]; ok {
				cancel()
				delete(m.cancels, w.ID)
			}

			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			w.TmuxSession = ""
			w.SessionID = ""
			w.RecoveryAttempts = 0
			m.saveWorkers()

			m.emit(Event{
				Type:     EventHumanInterventionRequired,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s auto-reset: orphaned tmux session", "員工 %s 已自動重設：孤立的 tmux session", w.Name),
			})

			delete(m.lastPaneContent, w.ID)
			continue
		}

		// Capture pane content and check for stalled output
		content, err := m.tmuxClient.CapturePane(w.TmuxSession, 0, 0, 20)
		if err != nil {
			continue
		}

		prev, exists := m.lastPaneContent[w.ID]
		if !exists || prev.content != content {
			// Content changed — update snapshot and reset recovery attempts
			m.lastPaneContent[w.ID] = paneSnapshot{content: content, since: time.Now()}
			if w.RecoveryAttempts > 0 {
				w.RecoveryAttempts = 0
			}
		} else if time.Since(prev.since) > 5*time.Minute {
			// Content unchanged for 5+ minutes — attempt auto-recovery
			m.autoRecoverStuck(w, content)
			// Reset the timer so we don't spam every 60s
			m.lastPaneContent[w.ID] = paneSnapshot{content: content, since: time.Now()}
		}
	}

	// Check for timed-out reviews
	m.checkReviewTimeouts()
}

// autoRecover attempts to respawn a worker whose tmux session died.
// Must be called with m.mu held. Returns true if recovery was initiated.
func (m *Manager) autoRecover(w *worker.Worker) bool {
	if w.CurrentTaskID == "" || m.spawner == nil {
		return false
	}

	t, ok := m.projectStore.GetTask(w.CurrentTaskID)
	if !ok || t.Status == project.TaskDone || t.Status == project.TaskFailed {
		return false
	}

	p, ok := m.projectStore.GetProject(t.ProjectID)
	if !ok {
		return false
	}

	w.RecoveryAttempts++
	if w.RecoveryAttempts > 3 {
		// Too many recovery attempts — mark task as failed
		log.Printf("HEALTH: worker %s recovery exhausted (%d attempts) — marking task %s failed", w.ID, w.RecoveryAttempts, t.ID)
		m.projectStore.ForceUpdateTaskStatus(t.ID, project.TaskFailed)

		if cancel, ok := m.cancels[w.ID]; ok {
			cancel()
			delete(m.cancels, w.ID)
		}

		w.Status = worker.WorkerIdle
		w.CurrentTaskID = ""
		w.TmuxSession = ""
		w.SessionID = ""
		w.RecoveryAttempts = 0
		m.saveWorkers()

		m.emit(Event{
			Type:     EventWorkerRecoveryFailed,
			WorkerID: w.ID,
			TaskID:   t.ID,
			Message:  m.msgf("Worker %s recovery failed after %d attempts — task marked failed", "員工 %s 恢復失敗（%d 次嘗試）— 任務標記為失敗", w.Name, 3),
		})
		return true
	}

	// Capture last output for context before killing
	var lastOutput string
	if prev, exists := m.lastPaneContent[w.ID]; exists {
		lastOutput = prev.content
	}
	// Truncate to ~2000 runes for context (rune-safe to avoid splitting UTF-8)
	if rs := []rune(lastOutput); len(rs) > 2000 {
		lastOutput = string(rs[len(rs)-2000:])
	}

	// Cancel existing monitor
	if cancel, ok := m.cancels[w.ID]; ok {
		cancel()
		delete(m.cancels, w.ID)
	}

	// Reset worker state for respawn
	w.TmuxSession = ""
	w.SessionID = ""
	w.LastRecoveryAt = time.Now()

	log.Printf("HEALTH: respawning worker %s for task %s (attempt %d)", w.ID, t.ID, w.RecoveryAttempts)

	// Capture IDs before releasing lock — respawnWorker will re-acquire lock and
	// look up the worker/task fresh to avoid data races on shared pointers.
	workerID := w.ID
	taskID := t.ID
	projectID := p.ID

	// Respawn in a goroutine (we hold m.mu, spawner needs it released)
	go func() {
		m.respawnWorkerByID(workerID, taskID, projectID, lastOutput)
	}()

	return true
}

// respawnWorkerByID re-spawns a worker by looking up fresh references under lock.
// This avoids data races from holding stale pointers across goroutine boundaries.
func (m *Manager) respawnWorkerByID(workerID, taskID, projectID, lastOutput string) {
	ctx := context.Background()

	// Look up fresh references under lock
	m.mu.RLock()
	w, wOK := m.workers[workerID]
	m.mu.RUnlock()
	if !wOK {
		log.Printf("HEALTH: respawn aborted — worker %s no longer exists", workerID)
		return
	}

	t, tOK := m.projectStore.GetTask(taskID)
	if !tOK {
		log.Printf("HEALTH: respawn aborted — task %s no longer exists", taskID)
		return
	}

	p, pOK := m.projectStore.GetProject(projectID)
	if !pOK {
		log.Printf("HEALTH: respawn aborted — project %s no longer exists", projectID)
		return
	}

	// Build recovery prompt prefix
	recoveryPrefix := ""
	lang := m.GetLanguage()
	if lang == "en" {
		recoveryPrefix = fmt.Sprintf("NOTE: Your previous session was interrupted. Here is the last progress captured:\n\n%s\n\n--- Continue from where you left off ---\n\n", lastOutput)
	} else {
		recoveryPrefix = fmt.Sprintf("注意：你之前的工作階段中斷了。以下是最後捕捉到的進度：\n\n%s\n\n--- 請從中斷處繼續 ---\n\n", lastOutput)
	}

	// Use a shallow copy of the task to avoid mutating shared state
	taskCopy := *t
	taskCopy.Prompt = recoveryPrefix + t.Prompt

	err := m.spawner.SpawnForTask(ctx, w, &taskCopy, p)

	if err != nil {
		log.Printf("HEALTH: respawn failed for worker %s: %v", workerID, err)
		m.mu.Lock()
		w.Status = worker.WorkerIdle
		w.CurrentTaskID = ""
		w.RecoveryAttempts = 0
		m.saveWorkers()
		m.projectStore.ForceUpdateTaskStatus(taskID, project.TaskReady)
		m.mu.Unlock()

		m.emit(Event{
			Type:     EventWorkerRecoveryFailed,
			WorkerID: workerID,
			TaskID:   taskID,
			Message:  m.msgf("Worker %s respawn failed: %v", "員工 %s 重新啟動失敗：%v", w.Name, err),
		})
		return
	}

	m.mu.Lock()
	m.saveWorkers()
	m.mu.Unlock()

	m.emit(Event{
		Type:     EventWorkerRecovered,
		WorkerID: workerID,
		TaskID:   taskID,
		Message:  m.msgf("Worker %s auto-recovered (attempt %d)", "員工 %s 已自動恢復（第 %d 次）", w.Name, w.RecoveryAttempts),
	})

	// Start new completion monitoring
	workerCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancels[workerID] = cancel
	m.mu.Unlock()

	m.wg.Add(1)
	go m.watchCompletion(workerCtx, w, t, p)
}

// autoRecoverStuck handles a worker whose pane output hasn't changed for 5+ minutes.
// Must be called with m.mu held.
func (m *Manager) autoRecoverStuck(w *worker.Worker, content string) {
	w.RecoveryAttempts++

	switch w.RecoveryAttempts {
	case 1:
		// First attempt: send Enter key (might be waiting for permission prompt)
		log.Printf("HEALTH: worker %s stuck — sending Enter key (attempt 1)", w.ID)
		m.tmuxClient.SendKeys(w.TmuxSession, w.Window, w.Pane, "Enter")
		m.emit(Event{
			Type:     EventHumanInterventionRequired,
			WorkerID: w.ID,
			Message:  m.msgf("Worker %s may be stuck — sent Enter key to wake up", "員工 %s 可能卡住了 — 已送出 Enter 喚醒", w.Name),
		})

	case 2:
		// Second attempt: send Enter again (avoids sending 'y' which could confirm destructive prompts)
		log.Printf("HEALTH: worker %s still stuck — sending Enter key again (attempt 2)", w.ID)
		m.tmuxClient.SendKeys(w.TmuxSession, w.Window, w.Pane, "Enter")

	default:
		// Third+ attempt: kill session; next health cycle will detect the missing
		// session via autoRecover(), which handles respawn or marks task failed.
		log.Printf("HEALTH: worker %s stuck for too long — killing session (attempt %d)", w.ID, w.RecoveryAttempts)
		m.tmuxClient.KillSession(w.TmuxSession)
		w.TmuxSession = ""
	}
}

// checkReviewTimeouts auto-approves reviews that have been running too long.
// Must be called with m.mu held.
func (m *Manager) checkReviewTimeouts() {
	reviewTimeout := 15 * time.Minute
	if m.reviewTimeoutMinutes > 0 {
		reviewTimeout = time.Duration(m.reviewTimeoutMinutes) * time.Minute
	}

	for _, p := range m.projectStore.ListProjects() {
		for _, t := range m.projectStore.TasksForProject(p.ID) {
			if t.Status != project.TaskReview {
				continue
			}
			if t.ReviewStartedAt == nil {
				continue
			}
			if time.Since(*t.ReviewStartedAt) <= reviewTimeout {
				continue
			}

			log.Printf("HEALTH: review for task %s timed out after %v — auto-approving", t.ID, time.Since(*t.ReviewStartedAt).Round(time.Second))

			// Auto-approve the task
			m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)

			// Reset the reviewer worker (not the original assignee)
			reviewerID := t.ReviewerID
			if reviewerID != "" {
				if rw, ok := m.workers[reviewerID]; ok && rw.Status == worker.WorkerWorking {
					if cancel, ok := m.cancels[rw.ID]; ok {
						cancel()
						delete(m.cancels, rw.ID)
					}
					rw.Status = worker.WorkerIdle
					rw.CurrentTaskID = ""
					if rw.TmuxSession != "" {
						m.tmuxClient.KillSession(rw.TmuxSession)
						rw.TmuxSession = ""
						rw.SessionID = ""
					}
					m.saveWorkers()
				}
			}

			m.emit(Event{
				Type:      EventReviewTimeout,
				ProjectID: p.ID,
				TaskID:    t.ID,
				Message:   m.msgf("Review for task %q auto-approved (timeout after %d min)", "任務 %q 的審查已自動核准（%d 分鐘超時）", t.Title, int(reviewTimeout.Minutes())),
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

			// Also mark any review sub-tasks for this task as done
			for _, st := range m.projectStore.TasksForProject(p.ID) {
				if st.ParentTaskID == t.ID && st.Status != project.TaskDone && st.Status != project.TaskFailed {
					m.projectStore.UpdateTaskStatus(st.ID, project.TaskDone)
				}
			}

			// checkProjectCompletion reads from projectStore (no mu needed), safe to call in goroutine
			projectID := p.ID
			go m.checkProjectCompletion(projectID)
		}
	}
}
