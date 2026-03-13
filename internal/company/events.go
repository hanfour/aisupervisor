package company

import "time"

type EventType string

const (
	EventProjectCreated  EventType = "project_created"
	EventTaskCreated     EventType = "task_created"
	EventTaskAssigned    EventType = "task_assigned"
	EventTaskCompleted   EventType = "task_completed"
	EventTaskFailed      EventType = "task_failed"
	EventWorkerSpawned   EventType = "worker_spawned"
	EventWorkerIdle      EventType = "worker_idle"
	EventBranchCreated   EventType = "branch_created"
	EventCommitDetected  EventType = "commit_detected"
	EventAutoAssigned    EventType = "auto_assigned"
	EventReviewStarted   EventType = "review_started"
	EventReviewApproved  EventType = "review_approved"
	EventReviewRejected  EventType = "review_rejected"
	EventTaskRevision        EventType = "task_revision"
	EventWorkerPromoted      EventType = "worker_promoted"
	EventTrainingCaptured    EventType = "training_captured"
	EventFinetuneStarted     EventType = "finetune_started"
	EventFinetuneCompleted   EventType = "finetune_completed"
	EventBenchmarkCompleted  EventType = "benchmark_completed"
	EventNarrativeGenerated  EventType = "narrative_generated"
	EventMoodChanged         EventType = "mood_changed"
	EventRelationshipUpdated EventType = "relationship_updated"
	EventProjectDeleted      EventType = "project_deleted"
	EventResearchCompleted   EventType = "research_completed"
	EventSpecReviewStarted   EventType = "spec_review_started"
	EventSpecApproved        EventType = "spec_approved"
	EventTestingStarted      EventType = "testing_started"
	EventTestingPassed       EventType = "testing_passed"
	EventSecurityScanStart   EventType = "security_scan_start"
	EventSecurityPassed      EventType = "security_passed"
	EventStagingStarted      EventType = "staging_started"
	EventStagingAccepted     EventType = "staging_accepted"
	EventTaskEscalated       EventType = "task_escalated"
	EventTaskDeployed        EventType = "task_deployed"
	EventHumanInterventionRequired EventType = "human_intervention_required"
	EventProjectCompleted          EventType = "project_completed"
	EventRetroStarted              EventType = "retro_started"
	EventRetroCompleted            EventType = "retro_completed"
	EventPRDCompleted              EventType = "prd_completed"
	EventPRDApproved               EventType = "prd_approved"
	EventDesignCompleted           EventType = "design_completed"
	EventObjectiveCreated          EventType = "objective_created"
	EventDelegationCreated         EventType = "delegation_created"
	EventWorkerPaused              EventType = "worker_paused"
	EventWorkerResumed             EventType = "worker_resumed"
	EventBudgetWarning             EventType = "budget_warning"
	EventObjectiveCompleted        EventType = "objective_completed"
)

type Event struct {
	Type      EventType `json:"type"`
	ProjectID string    `json:"projectId,omitempty"`
	TaskID    string    `json:"taskId,omitempty"`
	WorkerID  string    `json:"workerId,omitempty"`
	Payload  *StructuredMessage `json:"payload,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// MessageType represents the type of a structured message.
type MessageType string

const (
	MsgTaskAssignment  MessageType = "task_assignment"
	MsgReviewRequest   MessageType = "review_request"
	MsgEscalation      MessageType = "escalation"
	MsgQuestion        MessageType = "question"
	MsgStatusUpdate    MessageType = "status_update"
	MsgFeedback        MessageType = "feedback"
)

// MessagePriority represents the urgency of a message.
type MessagePriority string

const (
	PriorityLow      MessagePriority = "low"
	PriorityNormal   MessagePriority = "normal"
	PriorityHigh     MessagePriority = "high"
	PriorityCritical MessagePriority = "critical"
)

// StructuredMessage provides structured context for inter-agent communication.
type StructuredMessage struct {
	ID          string          `json:"id"`
	From        string          `json:"from"`
	To          string          `json:"to"`
	Type        MessageType     `json:"type"`
	Priority    MessagePriority `json:"priority"`
	ContextRefs []string        `json:"contextRefs,omitempty"`
	Content     string          `json:"content"`
	ParentMsgID string          `json:"parentMsgId,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
}
