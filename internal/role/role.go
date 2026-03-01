package role

import (
	"context"

	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
)

// Mode defines how a role operates.
type Mode string

const (
	ModeReactive  Mode = "reactive"
	ModeProactive Mode = "proactive"
	ModeHybrid    Mode = "hybrid"
)

// InterventionType defines the kind of action a role takes.
type InterventionType string

const (
	InterventionSelectOption InterventionType = "select_option"
	InterventionFreeText     InterventionType = "free_text"
	InterventionNone         InterventionType = "none"
)

// Intervention is the output of a role evaluation.
type Intervention struct {
	Type       InterventionType
	OptionKey  string  // for select_option
	Text       string  // for free_text
	Reasoning  string
	Confidence float64
	RoleID     string
	Priority   int
}

// Observation is the input to a role evaluation.
type Observation struct {
	PaneContent       string
	Prompt            *detector.PromptMatch
	SessionContext    *sessionctx.SessionContext
	SessionName       string
	TaskGoal          string
	DiscussionContext string // injected during roundtable with prior opinions summary
}

// Role is the core abstraction for a supervisor role.
type Role interface {
	ID() string
	Name() string
	Mode() Mode
	Priority() int
	ShouldEvaluate(obs Observation) bool
	Evaluate(ctx context.Context, obs Observation) (*Intervention, error)
}

// Avatarer is an optional interface for roles that have an avatar icon.
type Avatarer interface {
	Avatar() string
}

// GetAvatar returns the avatar for a role, or empty string if not set.
func GetAvatar(r Role) string {
	if a, ok := r.(Avatarer); ok {
		return a.Avatar()
	}
	return ""
}
