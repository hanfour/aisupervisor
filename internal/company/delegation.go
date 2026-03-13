package company

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// delegationCommand represents a task delegation instruction from a manager.
type delegationCommand struct {
	Title    string `json:"title"`
	Prompt   string `json:"prompt"`
	Priority int    `json:"priority"`
}

type delegationOutput struct {
	Delegate []delegationCommand `json:"delegate"`
}

const maxDelegatedTasksPerProject = 20

// handleDelegationOutput parses the manager's pane output for delegation commands
// and creates sub-tasks assigned to idle subordinates.
// Must be called with m.mu held.
func (m *Manager) handleDelegationOutput(w *worker.Worker, t *project.Task, p *project.Project) {
	// Prevent runaway delegation: limit total delegated tasks per project
	delegatedCount := 0
	for _, pt := range m.projectStore.TasksForProject(p.ID) {
		if pt.Type == project.TaskTypeCode && pt.Priority > 0 {
			delegatedCount++
		}
	}
	if delegatedCount >= maxDelegatedTasksPerProject {
		log.Printf("delegation: project %s has %d tasks, skipping further delegation", p.ID, delegatedCount)
		return
	}

	if w.TmuxSession == "" || m.tmuxClient == nil {
		return
	}

	paneContent, err := m.tmuxClient.CapturePane(w.TmuxSession, w.Window, w.Pane, 500)
	if err != nil {
		return
	}

	// Try to find delegation JSON in pane output.
	// Search for the specific "delegate" key to avoid false positives from other JSON.
	idx := strings.Index(paneContent, `"delegate"`)
	if idx == -1 {
		return
	}
	// Walk back to find the opening brace
	start := strings.LastIndex(paneContent[:idx], "{")
	if start == -1 {
		return
	}
	extracted := extractChatJSON(paneContent[start:])
	if extracted == "" {
		return
	}

	var output delegationOutput
	if err := json.Unmarshal([]byte(extracted), &output); err != nil {
		return
	}

	if len(output.Delegate) == 0 {
		return
	}

	// Find idle subordinates (m.mu is already held by caller)
	idleSubs := make([]*worker.Worker, 0)
	for _, sub := range m.workers {
		if sub.ParentID == w.ID && sub.Status == worker.WorkerIdle {
			idleSubs = append(idleSubs, sub)
		}
	}

	subIdx := 0
	for _, cmd := range output.Delegate {
		priority := cmd.Priority
		if priority <= 0 {
			priority = 1
		}

		newTask, err := m.AddTask(p.ID, cmd.Title, "", cmd.Prompt, nil, priority, "", "code")
		if err != nil {
			log.Printf("delegation: failed to create task %q: %v", cmd.Title, err)
			continue
		}

		m.emit(Event{
			Type:      EventDelegationCreated,
			ProjectID: p.ID,
			TaskID:    newTask.ID,
			WorkerID:  w.ID,
			Message:   m.msgf("Manager %s delegated task %q", "管理員 %s 委派任務 %q", w.Name, cmd.Title),
		})

		// Auto-assign to an idle subordinate if available
		if subIdx < len(idleSubs) {
			go func(wID, tID string) {
				if err := m.AssignTask(context.Background(), wID, tID); err != nil {
					log.Printf("delegation: failed to assign task %s to %s: %v", tID, wID, err)
				}
			}(idleSubs[subIdx].ID, newTask.ID)
			subIdx++
		}
	}

	log.Printf("delegation: manager %s created %d tasks from delegation output", w.Name, len(output.Delegate))
}

