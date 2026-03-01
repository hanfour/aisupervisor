package company

import "time"

type EventType string

const (
	EventProjectCreated EventType = "project_created"
	EventTaskCreated    EventType = "task_created"
	EventTaskAssigned   EventType = "task_assigned"
	EventTaskCompleted  EventType = "task_completed"
	EventTaskFailed     EventType = "task_failed"
	EventWorkerSpawned  EventType = "worker_spawned"
	EventWorkerIdle     EventType = "worker_idle"
	EventBranchCreated  EventType = "branch_created"
	EventCommitDetected EventType = "commit_detected"
)

type Event struct {
	Type      EventType `json:"type"`
	ProjectID string    `json:"projectId,omitempty"`
	TaskID    string    `json:"taskId,omitempty"`
	WorkerID  string    `json:"workerId,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
