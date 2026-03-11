package company

import (
	"context"
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
// It detects orphaned tmux sessions and stalled workers.
func (m *Manager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runHealthCycle()
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
			log.Printf("HEALTH: worker %s (%s) has orphaned session %q — resetting", w.ID, w.Name, w.TmuxSession)

			// Cancel completion monitor
			if cancel, ok := m.cancels[w.ID]; ok {
				cancel()
				delete(m.cancels, w.ID)
			}

			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			w.TmuxSession = ""
			w.SessionID = ""
			m.saveWorkers()

			m.emit(Event{
				Type:     EventHumanInterventionRequired,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s auto-reset: orphaned tmux session", "員工 %s 已自動重設：孤立的 tmux session", w.Name),
			})

			// Clean up stale pane snapshot
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
			// Content changed — update snapshot
			m.lastPaneContent[w.ID] = paneSnapshot{content: content, since: time.Now()}
		} else if time.Since(prev.since) > 5*time.Minute {
			// Content unchanged for 5+ minutes — emit warning (don't auto-reset)
			log.Printf("HEALTH: worker %s (%s) pane output unchanged for %v", w.ID, w.Name, time.Since(prev.since).Round(time.Second))
			m.emit(Event{
				Type:     EventHumanInterventionRequired,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s may be stuck: no output change for 5+ minutes", "員工 %s 可能卡住了：超過 5 分鐘沒有輸出變化", w.Name),
			})
			// Reset the timer so we don't spam warnings every 60s
			m.lastPaneContent[w.ID] = paneSnapshot{content: content, since: time.Now()}
		}
	}
}
