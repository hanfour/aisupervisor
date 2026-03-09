package project

import "time"

type TaskType string

const (
	TaskTypeCode     TaskType = "code"
	TaskTypeResearch TaskType = "research"
	TaskTypePRD      TaskType = "prd"
	TaskTypeDesign   TaskType = "design"
)

type TaskStatus string

const (
	TaskBacklog    TaskStatus = "backlog"
	TaskReady      TaskStatus = "ready"
	TaskAssigned   TaskStatus = "assigned"
	TaskInProgress TaskStatus = "in_progress"
	TaskReview     TaskStatus = "review"
	TaskRevision   TaskStatus = "revision"
	TaskDone       TaskStatus = "done"
	TaskFailed     TaskStatus = "failed"
)

type Task struct {
	ID           string     `yaml:"id" json:"id"`
	ProjectID    string     `yaml:"project_id" json:"projectId"`
	Title        string     `yaml:"title" json:"title"`
	Description  string     `yaml:"description" json:"description"`
	Prompt       string     `yaml:"prompt" json:"prompt"`
	Status       TaskStatus `yaml:"status" json:"status"`
	Type         TaskType   `yaml:"type,omitempty" json:"type,omitempty"` // "code" (default) or "research"
	Priority     int        `yaml:"priority" json:"priority"`            // 1=highest
	BranchName   string     `yaml:"branch_name" json:"branchName"`
	AssigneeID   string     `yaml:"assignee_id,omitempty" json:"assigneeId,omitempty"`
	DependsOn    []string   `yaml:"depends_on,omitempty" json:"dependsOn,omitempty"`
	Milestone    string     `yaml:"milestone,omitempty" json:"milestone,omitempty"`
	ReviewerID   string     `yaml:"reviewer_id,omitempty" json:"reviewerId,omitempty"`
	ParentTaskID string     `yaml:"parent_task_id,omitempty" json:"parentTaskId,omitempty"`
	ReviewCount      int            `yaml:"review_count,omitempty" json:"reviewCount,omitempty"`
	RejectionCount   int            `yaml:"rejection_count,omitempty" json:"rejectionCount,omitempty"`
	RejectionHistory []Rejection    `yaml:"rejection_history,omitempty" json:"rejectionHistory,omitempty"`
	GateRequestID    string         `yaml:"gate_request_id,omitempty" json:"gateRequestId,omitempty"`
	BounceHistory    []BounceRecord `yaml:"bounce_history,omitempty" json:"bounceHistory,omitempty"`
	TokensConsumed   int64          `yaml:"tokens_consumed,omitempty" json:"tokensConsumed,omitempty"`
	BudgetLimit      int64          `yaml:"budget_limit,omitempty" json:"budgetLimit,omitempty"`
	CreatedAt        time.Time      `yaml:"created_at" json:"createdAt"`
	StartedAt        *time.Time     `yaml:"started_at,omitempty" json:"startedAt,omitempty"`
	CompletedAt      *time.Time     `yaml:"completed_at,omitempty" json:"completedAt,omitempty"`
}

// Rejection records a single review rejection event.
type Rejection struct {
	Stage      TaskStatus `yaml:"stage" json:"stage"`
	RejectorID string     `yaml:"rejector_id" json:"rejectorId"`
	Reason     string     `yaml:"reason" json:"reason"`
	Timestamp  time.Time  `yaml:"timestamp" json:"timestamp"`
}

// BounceRecord tracks a task being bounced between agents.
type BounceRecord struct {
	FromID    string     `yaml:"from_id" json:"fromId"`
	ToID      string     `yaml:"to_id" json:"toId"`
	Stage     TaskStatus `yaml:"stage" json:"stage"`
	Reason    string     `yaml:"reason" json:"reason"`
	Timestamp time.Time  `yaml:"timestamp" json:"timestamp"`
}
