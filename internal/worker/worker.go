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
	CreatedAt     time.Time    `yaml:"created_at" json:"createdAt"`
}
