package session

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusPaused  Status = "paused"
	StatusStopped Status = "stopped"
)

type MonitoredSession struct {
	ID          string    `yaml:"id" json:"id"`
	Name        string    `yaml:"name" json:"name"`
	TmuxSession string    `yaml:"tmux_session" json:"tmux_session"`
	Window      int       `yaml:"window" json:"window"`
	Pane        int       `yaml:"pane" json:"pane"`
	ToolType    string    `yaml:"tool_type" json:"tool_type"` // claude_code, gemini, auto
	TaskGoal    string    `yaml:"task_goal,omitempty" json:"task_goal,omitempty"`
	ProjectDir  string    `yaml:"project_dir,omitempty" json:"project_dir,omitempty"`
	Status      Status    `yaml:"status" json:"status"`
	CreatedAt   time.Time `yaml:"created_at" json:"created_at"`
}
