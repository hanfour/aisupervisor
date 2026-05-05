package worker

import (
	"time"

	"github.com/hanfourmini/aisupervisor/internal/growth"
)

type WorkerStatus string

const (
	WorkerIdle     WorkerStatus = "idle"
	WorkerWorking  WorkerStatus = "working"
	WorkerWaiting  WorkerStatus = "waiting"
	WorkerFinished WorkerStatus = "finished"
	WorkerError    WorkerStatus = "error"
	WorkerPaused   WorkerStatus = "paused"
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
//
// The legacy layered fields (BodyRow / Outfit / Hair) drive the
// MetroCity static-spritesheet pipeline (frontend/src/lib/office/sprites.js).
// They remain for backward compatibility and are still used as the
// fallback when no AI sprite is available.
//
// SpriteSheetPath, when set, points at a per-worker PixelLab AI
// generated sprite sheet on disk. The frontend prefers it over the
// layered fields; if the file is missing or unreadable it falls back
// to the layered renderer.
type WorkerAppearance struct {
	BodyRow         int    `yaml:"body_row" json:"bodyRow"`                   // 0-5 skin tone (layered fallback)
	Outfit          string `yaml:"outfit" json:"outfit"`                       // "outfit1".."outfit6" (layered fallback)
	Hair            string `yaml:"hair" json:"hair"`                           // "hair1".."hair7" (layered fallback)
	SpriteSheetPath string `yaml:"sprite_sheet_path,omitempty" json:"spriteSheetPath,omitempty"` // absolute path to AI-generated sheet, empty = use layered
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
	Title         string       `yaml:"title,omitempty" json:"title,omitempty"`
	Gender        WorkerGender    `yaml:"gender,omitempty" json:"gender,omitempty"`
	Appearance       *WorkerAppearance `yaml:"appearance,omitempty" json:"appearance,omitempty"`
	RecoveryAttempts int               `yaml:"-" json:"-"` // transient: recovery attempts for current task
	LastRecoveryAt   time.Time         `yaml:"-" json:"-"` // transient: last recovery attempt time
	SkillTree        *growth.SkillTree `yaml:"skill_tree,omitempty" json:"skillTree,omitempty"`
	CreatedAt        time.Time         `yaml:"created_at" json:"createdAt"`
	LastCommunityID  int               `yaml:"last_community_id,omitempty" json:"lastCommunityId,omitempty"`
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
