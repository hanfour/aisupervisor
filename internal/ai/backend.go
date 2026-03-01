package ai

import (
	"context"

	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
)

type AnalysisRequest struct {
	PaneContent          string
	Prompt               *detector.PromptMatch
	SessionName          string
	TaskGoal             string                    // optional user-provided goal for this session
	SessionContext       *sessionctx.SessionContext // optional per-session context
	SystemPromptOverride string                    // if set, replaces the default system prompt
	DiscussionContext    string                    // injected during group discussion roundtable
}

type Decision struct {
	ChosenOption detector.ResponseOption
	Reasoning    string
	Confidence   float64
}

type Backend interface {
	Name() string
	Analyze(ctx context.Context, req AnalysisRequest) (*Decision, error)
	Healthy(ctx context.Context) error
}
