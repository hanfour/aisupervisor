package role

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
)

// GatekeeperRole wraps the existing permission gatekeeper logic as a Role.
type GatekeeperRole struct {
	backend     ai.Backend
	autoApprove []config.AutoApproveRule
	threshold   float64
}

// NewGatekeeperRole creates a gatekeeper role that preserves existing behavior.
func NewGatekeeperRole(backend ai.Backend, autoApprove []config.AutoApproveRule, threshold float64) *GatekeeperRole {
	return &GatekeeperRole{
		backend:     backend,
		autoApprove: autoApprove,
		threshold:   threshold,
	}
}

func (r *GatekeeperRole) ID() string       { return "permission_gatekeeper" }
func (r *GatekeeperRole) Name() string     { return "Permission Gatekeeper" }
func (r *GatekeeperRole) Mode() Mode       { return ModeReactive }
func (r *GatekeeperRole) Priority() int    { return 100 }

func (r *GatekeeperRole) ShouldEvaluate(obs Observation) bool {
	return obs.Prompt != nil
}

func (r *GatekeeperRole) Evaluate(ctx context.Context, obs Observation) (*Intervention, error) {
	if obs.Prompt == nil {
		return &Intervention{Type: InterventionNone, RoleID: r.ID()}, nil
	}

	// Check auto-approve rules (global + project)
	if intervention, ok := r.checkAutoApprove(obs.Prompt, obs.SessionContext); ok {
		return intervention, nil
	}

	// AI analysis
	req := ai.AnalysisRequest{
		PaneContent:       obs.PaneContent,
		Prompt:            obs.Prompt,
		SessionName:       obs.SessionName,
		TaskGoal:          obs.TaskGoal,
		SessionContext:    obs.SessionContext,
		DiscussionContext: obs.DiscussionContext,
	}
	decision, err := r.backend.Analyze(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gatekeeper analyze: %w", err)
	}

	return &Intervention{
		Type:       InterventionSelectOption,
		OptionKey:  decision.ChosenOption.Key,
		Reasoning:  decision.Reasoning,
		Confidence: decision.Confidence,
		RoleID:     r.ID(),
		Priority:   r.Priority(),
	}, nil
}

func (r *GatekeeperRole) checkAutoApprove(match *detector.PromptMatch, sc *sessionctx.SessionContext) (*Intervention, bool) {
	// Check global rules
	for _, rule := range r.autoApprove {
		if intervention, ok := r.matchRule(match, rule.PatternContains, rule.Response, rule.Label); ok {
			return intervention, true
		}
	}

	// Check project-specific rules
	if sc != nil {
		snap := sc.Snapshot()
		for _, rule := range snap.Rules {
			if intervention, ok := r.matchRule(match, rule.PatternContains, rule.Response, rule.Label); ok {
				return intervention, true
			}
		}
	}

	return nil, false
}

func (r *GatekeeperRole) matchRule(match *detector.PromptMatch, pattern, response, label string) (*Intervention, bool) {
	if strings.Contains(match.Summary, pattern) ||
		strings.Contains(match.FullContext, pattern) {
		return &Intervention{
			Type:       InterventionSelectOption,
			OptionKey:  response,
			Reasoning:  fmt.Sprintf("Auto-approve rule: %s", label),
			Confidence: 1.0,
			RoleID:     r.ID(),
			Priority:   r.Priority(),
		}, true
	}
	return nil, false
}

// IsAutoApproved returns true if the intervention was created by an auto-approve rule.
func IsAutoApproved(i *Intervention) bool {
	return i != nil && i.Confidence == 1.0 && strings.HasPrefix(i.Reasoning, "Auto-approve rule:")
}
