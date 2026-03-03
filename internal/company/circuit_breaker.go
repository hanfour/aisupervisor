package company

import (
	"fmt"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

const (
	// MaxBouncesPerPair is the maximum bounces allowed between the same agent pair.
	MaxBouncesPerPair = 3
	// MaxTotalBounces is the maximum total bounces allowed for a task.
	MaxTotalBounces = 6
	// BudgetWarnPercent triggers a warning when this percentage of budget remains.
	BudgetWarnPercent = 0.20
)

// CircuitBreaker detects infinite loops and budget overruns in task processing.
type CircuitBreaker struct {
	mgr *Manager
}

// NewCircuitBreaker creates a new CircuitBreaker attached to the given manager.
func NewCircuitBreaker(mgr *Manager) *CircuitBreaker {
	return &CircuitBreaker{mgr: mgr}
}

// CheckBounceLoop returns true if the task is in a bounce loop between the given agents.
// It also records the bounce in the task's history.
func (cb *CircuitBreaker) CheckBounceLoop(t *project.Task, fromID, toID string) bool {
	// Count bounces between this specific pair
	pairCount := 0
	for _, b := range t.BounceHistory {
		if (b.FromID == fromID && b.ToID == toID) || (b.FromID == toID && b.ToID == fromID) {
			pairCount++
		}
	}

	if pairCount >= MaxBouncesPerPair {
		return true
	}

	if len(t.BounceHistory) >= MaxTotalBounces {
		return true
	}

	return false
}

// RecordBounce adds a bounce record to the task.
func (cb *CircuitBreaker) RecordBounce(t *project.Task, fromID, toID string, stage project.TaskStatus, reason string) {
	t.BounceHistory = append(t.BounceHistory, project.BounceRecord{
		FromID:    fromID,
		ToID:      toID,
		Stage:     stage,
		Reason:    reason,
		Timestamp: time.Now(),
	})
}

// CheckBudget returns true if the task has exceeded its budget limit.
func (cb *CircuitBreaker) CheckBudget(t *project.Task) bool {
	if t.BudgetLimit <= 0 {
		return false
	}
	return t.TokensConsumed >= t.BudgetLimit
}

// BudgetWarning returns true if the task is within the warning threshold of its budget.
func (cb *CircuitBreaker) BudgetWarning(t *project.Task) bool {
	if t.BudgetLimit <= 0 {
		return false
	}
	remaining := float64(t.BudgetLimit - t.TokensConsumed)
	return remaining/float64(t.BudgetLimit) <= BudgetWarnPercent
}

// Escalate triggers an escalation for a task that is stuck in a loop.
func (cb *CircuitBreaker) Escalate(t *project.Task, reason string) {
	t.Status = project.TaskEscalation
	cb.mgr.emit(Event{
		Type:      EventTaskEscalated,
		ProjectID: t.ProjectID,
		TaskID:    t.ID,
		Message:   fmt.Sprintf("Task %q escalated: %s (bounces: %d, rejections: %d)", t.Title, reason, len(t.BounceHistory), t.RejectionCount),
	})
}
