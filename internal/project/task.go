package project

import "time"

type TaskStatus string

const (
	TaskBacklog    TaskStatus = "backlog"
	TaskReady      TaskStatus = "ready"
	TaskAssigned   TaskStatus = "assigned"
	TaskInProgress TaskStatus = "in_progress"
	TaskReview     TaskStatus = "review"
	TaskDone       TaskStatus = "done"
	TaskFailed     TaskStatus = "failed"
)

type Task struct {
	ID          string     `yaml:"id" json:"id"`
	ProjectID   string     `yaml:"project_id" json:"projectId"`
	Title       string     `yaml:"title" json:"title"`
	Description string     `yaml:"description" json:"description"`
	Prompt      string     `yaml:"prompt" json:"prompt"`
	Status      TaskStatus `yaml:"status" json:"status"`
	Priority    int        `yaml:"priority" json:"priority"` // 1=highest
	BranchName  string     `yaml:"branch_name" json:"branchName"`
	AssigneeID  string     `yaml:"assignee_id,omitempty" json:"assigneeId,omitempty"`
	DependsOn   []string   `yaml:"depends_on,omitempty" json:"dependsOn,omitempty"`
	Milestone   string     `yaml:"milestone,omitempty" json:"milestone,omitempty"`
	CreatedAt   time.Time  `yaml:"created_at" json:"createdAt"`
	StartedAt   *time.Time `yaml:"started_at,omitempty" json:"startedAt,omitempty"`
	CompletedAt *time.Time `yaml:"completed_at,omitempty" json:"completedAt,omitempty"`
}
