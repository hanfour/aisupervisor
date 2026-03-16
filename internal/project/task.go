package project

import "time"

type TaskType string

const (
	TaskTypeCode     TaskType = "code"
	TaskTypeResearch TaskType = "research"
	TaskTypePRD      TaskType = "prd"
	TaskTypeDesign   TaskType = "design"
	TaskTypeAdmin    TaskType = "admin"
	TaskTypeHR       TaskType = "hr"
	TaskTypeTraining TaskType = "training"
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

// TrainingTaskConfig holds iteration state for training tasks.
type TrainingTaskConfig struct {
	MaxIterations  int     `yaml:"max_iterations" json:"maxIterations"`
	TestCmd        string  `yaml:"test_cmd" json:"testCmd"`
	BenchmarkCmd   string  `yaml:"benchmark_cmd,omitempty" json:"benchmarkCmd,omitempty"`
	PassThreshold  float64 `yaml:"pass_threshold" json:"passThreshold"`
	CurrentIter    int     `yaml:"current_iter" json:"currentIter"`
	BestScore      float64 `yaml:"best_score" json:"bestScore"`
	BestCommit     string  `yaml:"best_commit,omitempty" json:"bestCommit,omitempty"`
	LastTestOutput string  `yaml:"last_test_output,omitempty" json:"lastTestOutput,omitempty"`

	// ScoreHistory records the score of each iteration for trend analysis.
	ScoreHistory []float64 `yaml:"score_history,omitempty" json:"scoreHistory,omitempty"`
	// TestTimeoutSec is the max seconds a test command can run (default 300).
	TestTimeoutSec int `yaml:"test_timeout_sec,omitempty" json:"testTimeoutSec,omitempty"`
	// PlateauLimit is how many consecutive non-improving iterations trigger early-stop (default 3).
	PlateauLimit int `yaml:"plateau_limit,omitempty" json:"plateauLimit,omitempty"`
}

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
	TrainingConfig   *TrainingTaskConfig `yaml:"training_config,omitempty" json:"trainingConfig,omitempty"`

	// VerifyCmd overrides the project-level verify command for this task.
	// If set, runs after the worker completes to validate the result.
	VerifyCmd       string  `yaml:"verify_cmd,omitempty" json:"verifyCmd,omitempty"`
	// VerifyScore tracks the last verification score (0.0-1.0).
	VerifyScore     float64 `yaml:"verify_score,omitempty" json:"verifyScore,omitempty"`
	// BestVerifyScore is the best verification score across iterations.
	BestVerifyScore float64 `yaml:"best_verify_score,omitempty" json:"bestVerifyScore,omitempty"`
	// BestVerifyCommit is the commit hash with the best verification score.
	BestVerifyCommit string `yaml:"best_verify_commit,omitempty" json:"bestVerifyCommit,omitempty"`
	// IterationCount tracks how many verify-improve cycles this task has gone through.
	IterationCount  int     `yaml:"iteration_count,omitempty" json:"iterationCount,omitempty"`
	// OriginalPrompt preserves the initial prompt before verification feedback is appended.
	OriginalPrompt string `yaml:"original_prompt,omitempty" json:"-"`
	// PreTaskCommit is the HEAD commit before the worker started, used for rollback.
	PreTaskCommit string `yaml:"pre_task_commit,omitempty" json:"-"`
	// HelpRequestHandled stores the help content already processed, to prevent duplicate handling.
	HelpRequestHandled string `yaml:"help_request_handled,omitempty" json:"-"`

	RetryCount       int            `yaml:"retry_count,omitempty" json:"retryCount,omitempty"`
	CreatedAt        time.Time      `yaml:"created_at" json:"createdAt"`
	StartedAt        *time.Time     `yaml:"started_at,omitempty" json:"startedAt,omitempty"`
	CompletedAt      *time.Time     `yaml:"completed_at,omitempty" json:"completedAt,omitempty"`
	ReviewStartedAt  *time.Time     `yaml:"review_started_at,omitempty" json:"reviewStartedAt,omitempty"`
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
