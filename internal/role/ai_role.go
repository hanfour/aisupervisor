package role

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
)

// AIRole is an AI-driven custom role with its own system prompt and trigger patterns.
type AIRole struct {
	cfg             config.RoleConfig
	backend         ai.Backend
	triggerPatterns []*regexp.Regexp
	mu              sync.Mutex
	lastEvaluation  time.Time
}

// NewAIRole creates an AI-backed custom role.
func NewAIRole(cfg config.RoleConfig, backend ai.Backend) (*AIRole, error) {
	var patterns []*regexp.Regexp
	for _, p := range cfg.TriggerPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid trigger pattern %q: %w", p, err)
		}
		patterns = append(patterns, re)
	}

	return &AIRole{
		cfg:             cfg,
		backend:         backend,
		triggerPatterns: patterns,
	}, nil
}

func (r *AIRole) ID() string     { return r.cfg.ID }
func (r *AIRole) Name() string   { return r.cfg.Name }
func (r *AIRole) Priority() int  { return r.cfg.Priority }
func (r *AIRole) Avatar() string { return r.cfg.Avatar }

func (r *AIRole) Mode() Mode {
	switch r.cfg.Mode {
	case "proactive":
		return ModeProactive
	case "hybrid":
		return ModeHybrid
	default:
		return ModeReactive
	}
}

func (r *AIRole) ShouldEvaluate(obs Observation) bool {
	// Check cooldown
	if r.cfg.CooldownSec > 0 {
		r.mu.Lock()
		elapsed := time.Since(r.lastEvaluation)
		r.mu.Unlock()
		if elapsed < time.Duration(r.cfg.CooldownSec)*time.Second {
			return false
		}
	}

	// For reactive mode, require a prompt
	if r.Mode() == ModeReactive && obs.Prompt == nil {
		return false
	}

	// Check trigger patterns if configured
	if len(r.triggerPatterns) > 0 {
		for _, re := range r.triggerPatterns {
			if re.MatchString(obs.PaneContent) {
				return true
			}
		}
		return false
	}

	// No trigger patterns: evaluate if there's relevant content
	if r.Mode() == ModeReactive {
		return obs.Prompt != nil
	}
	return obs.PaneContent != ""
}

func (r *AIRole) Evaluate(ctx context.Context, obs Observation) (*Intervention, error) {
	r.mu.Lock()
	r.lastEvaluation = time.Now()
	r.mu.Unlock()

	req := ai.AnalysisRequest{
		PaneContent:          obs.PaneContent,
		Prompt:               obs.Prompt,
		SessionName:          obs.SessionName,
		TaskGoal:             obs.TaskGoal,
		SessionContext:       obs.SessionContext,
		SystemPromptOverride: r.cfg.SystemPrompt,
		DiscussionContext:    obs.DiscussionContext,
	}

	decision, err := r.backend.Analyze(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ai role %s analyze: %w", r.cfg.ID, err)
	}

	interventionType := r.determineInterventionType(decision)

	return &Intervention{
		Type:       interventionType,
		OptionKey:  decision.ChosenOption.Key,
		Text:       extractFreeText(decision),
		Reasoning:  decision.Reasoning,
		Confidence: decision.Confidence,
		RoleID:     r.cfg.ID,
		Priority:   r.cfg.Priority,
	}, nil
}

func (r *AIRole) determineInterventionType(decision *ai.Decision) InterventionType {
	switch r.cfg.ResponseFormat {
	case "option":
		return InterventionSelectOption
	case "freetext":
		return InterventionFreeText
	case "either":
		// If there's a chosen key that looks like an option, use select
		if decision.ChosenOption.Key != "" {
			return InterventionSelectOption
		}
		return InterventionFreeText
	default:
		if decision.ChosenOption.Key != "" {
			return InterventionSelectOption
		}
		return InterventionNone
	}
}

type freeTextResponse struct {
	Text string `json:"text"`
}

func extractFreeText(decision *ai.Decision) string {
	// Try to extract free text from reasoning if it contains a JSON text field
	var ft freeTextResponse
	if err := json.Unmarshal([]byte(decision.Reasoning), &ft); err == nil && ft.Text != "" {
		return ft.Text
	}
	return ""
}
