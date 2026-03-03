package project

import "fmt"

// Extended task statuses for the full lifecycle.
const (
	TaskDraft        TaskStatus = "draft"
	TaskSpecReview   TaskStatus = "spec_review"
	TaskApproved     TaskStatus = "approved"
	TaskCodeReview   TaskStatus = "code_review"
	TaskTesting      TaskStatus = "testing"
	TaskSecurityScan TaskStatus = "security_scan"
	TaskStaging      TaskStatus = "staging"
	TaskAccepted     TaskStatus = "accepted"
	TaskDeployed     TaskStatus = "deployed"
	TaskEscalation   TaskStatus = "escalation"
)

// ValidTransitions defines the allowed state transitions for tasks.
var ValidTransitions = map[TaskStatus][]TaskStatus{
	TaskBacklog:      {TaskDraft, TaskReady},       // TaskReady preserves backward compatibility
	TaskDraft:        {TaskSpecReview, TaskReady},   // small tasks can skip spec_review
	TaskSpecReview:   {TaskApproved, TaskRevision},
	TaskApproved:     {TaskReady},
	TaskReady:        {TaskAssigned},
	TaskAssigned:     {TaskInProgress, TaskReady},
	TaskInProgress:   {TaskCodeReview, TaskReview, TaskDone, TaskFailed}, // TaskDone for no-review path
	TaskCodeReview:   {TaskTesting, TaskDone, TaskRevision},             // TaskDone when verification pipeline disabled
	TaskReview:       {TaskDone, TaskRevision},                          // legacy review status
	TaskTesting:      {TaskSecurityScan, TaskRevision},
	TaskSecurityScan: {TaskStaging, TaskRevision},
	TaskStaging:      {TaskAccepted, TaskRevision},
	TaskAccepted:     {TaskDone, TaskDeployed},
	TaskRevision:     {TaskInProgress, TaskReady, TaskFailed},
	TaskDone:         {TaskDeployed},
	TaskFailed:       {TaskReady, TaskBacklog},
	TaskEscalation:   {TaskReady, TaskBacklog, TaskFailed},
}

// MaxRejectionsBeforeEscalation is the threshold for automatic escalation.
const MaxRejectionsBeforeEscalation = 3

// CanTransition checks if a transition from `from` to `to` is valid.
func CanTransition(from, to TaskStatus) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the transition is not allowed.
func ValidateTransition(from, to TaskStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid task transition: %s → %s", from, to)
	}
	return nil
}

// NormalizeStatus maps legacy status values to their current equivalents.
// This provides backward compatibility when loading tasks from YAML.
func NormalizeStatus(status TaskStatus) TaskStatus {
	switch status {
	case "review":
		// Legacy "review" maps to "code_review" for new workflows,
		// but we keep it as-is for backward compat since TaskReview is still a valid status.
		return TaskReview
	default:
		return status
	}
}

// ShouldEscalate returns true if the task has been rejected enough times to warrant escalation.
func ShouldEscalate(t *Task) bool {
	return t.RejectionCount >= MaxRejectionsBeforeEscalation
}

// StageForRole returns the task statuses that correspond to a given worker role.
func StageForRole(role string) []TaskStatus {
	switch role {
	case "architect":
		return []TaskStatus{TaskSpecReview}
	case "coder":
		return []TaskStatus{TaskInProgress}
	case "qa":
		return []TaskStatus{TaskTesting}
	case "security":
		return []TaskStatus{TaskSecurityScan}
	case "devops":
		return []TaskStatus{TaskStaging}
	default:
		return []TaskStatus{TaskInProgress}
	}
}
