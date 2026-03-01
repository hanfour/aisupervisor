package worker

import "time"

type WorkerStatus string

const (
	WorkerIdle     WorkerStatus = "idle"
	WorkerWorking  WorkerStatus = "working"
	WorkerWaiting  WorkerStatus = "waiting"
	WorkerFinished WorkerStatus = "finished"
	WorkerError    WorkerStatus = "error"
)

type WorkerTier string

const (
	TierConsultant WorkerTier = "consultant"
	TierManager    WorkerTier = "manager"
	TierEngineer   WorkerTier = "engineer"
)

type Worker struct {
	ID            string       `yaml:"id" json:"id"`
	Name          string       `yaml:"name" json:"name"`
	Avatar        string       `yaml:"avatar" json:"avatar"`
	Status        WorkerStatus `yaml:"status" json:"status"`
	CurrentTaskID string       `yaml:"current_task_id,omitempty" json:"currentTaskId,omitempty"`
	TmuxSession   string       `yaml:"tmux_session" json:"tmuxSession"`
	Window        int          `yaml:"window" json:"window"`
	Pane          int          `yaml:"pane" json:"pane"`
	SessionID     string       `yaml:"session_id,omitempty" json:"sessionId,omitempty"`
	Tier          WorkerTier   `yaml:"tier,omitempty" json:"tier,omitempty"`
	BackendID     string       `yaml:"backend_id,omitempty" json:"backendId,omitempty"`
	ParentID      string       `yaml:"parent_id,omitempty" json:"parentId,omitempty"`
	ModelVersion  string       `yaml:"model_version,omitempty" json:"modelVersion,omitempty"`
	CLITool       string       `yaml:"cli_tool,omitempty" json:"cliTool,omitempty"`
	CreatedAt     time.Time    `yaml:"created_at" json:"createdAt"`
}

// EffectiveTier returns the worker's tier, defaulting to TierEngineer if unset.
func (w *Worker) EffectiveTier() WorkerTier {
	if w.Tier == "" {
		return TierEngineer
	}
	return w.Tier
}
