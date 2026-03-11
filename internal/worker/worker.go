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

type WorkerGender string

const (
	GenderMale   WorkerGender = "male"
	GenderFemale WorkerGender = "female"
)

type WorkerTier string

const (
	TierConsultant WorkerTier = "consultant"
	TierManager    WorkerTier = "manager"
	TierEngineer   WorkerTier = "engineer"
)

// WorkerRole represents the functional role of a worker within the pipeline.
type WorkerRole string

const (
	RoleArchitect WorkerRole = "architect"   // spec_review stage
	RoleCoder     WorkerRole = "coder"       // in_progress stage (default)
	RoleQA        WorkerRole = "qa"          // testing stage
	RoleSecurity  WorkerRole = "security"    // security_scan stage
	RoleDevOps    WorkerRole = "devops"      // staging stage
	RoleDesigner  WorkerRole = "designer"    // UI/UX related tasks
)

// WorkerAppearance stores pixel office visual customization.
type WorkerAppearance struct {
	BodyRow int    `yaml:"body_row" json:"bodyRow"`   // 0-5 skin tone
	Outfit  string `yaml:"outfit" json:"outfit"`       // "outfit1".."outfit6"
	Hair    string `yaml:"hair" json:"hair"`           // "hair1".."hair7"
}

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
	SkillProfile  string       `yaml:"skill_profile,omitempty" json:"skillProfile,omitempty"`
	Role          WorkerRole   `yaml:"role,omitempty" json:"role,omitempty"`
	Gender        WorkerGender    `yaml:"gender,omitempty" json:"gender,omitempty"`
	Appearance    *WorkerAppearance `yaml:"appearance,omitempty" json:"appearance,omitempty"`
	CreatedAt     time.Time       `yaml:"created_at" json:"createdAt"`
}

// EffectiveTier returns the worker's tier, defaulting to TierEngineer if unset.
func (w *Worker) EffectiveTier() WorkerTier {
	if w.Tier == "" {
		return TierEngineer
	}
	return w.Tier
}

// EffectiveRole returns the worker's role, defaulting to RoleCoder if unset.
func (w *Worker) EffectiveRole() WorkerRole {
	if w.Role == "" {
		return RoleCoder
	}
	return w.Role
}
