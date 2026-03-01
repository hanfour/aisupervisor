package role

import (
	"context"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/detector"
)

func TestAIRole_ShouldEvaluate_TriggerPatterns(t *testing.T) {
	cfg := config.RoleConfig{
		ID:              "test_role",
		Name:            "Test Role",
		Mode:            "reactive",
		Enabled:         true,
		Priority:        50,
		TriggerPatterns: []string{`(?i)delete|remove|drop`},
		ResponseFormat:  "option",
	}

	r, err := NewAIRole(cfg, &fakeBackend{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		content  string
		hasPrompt bool
		expected bool
	}{
		{"DELETE FROM users", true, true},
		{"remove file.txt", true, true},
		{"DROP TABLE", true, true},
		{"reading file", true, false},
		{"normal output", false, false},
	}

	for _, tt := range tests {
		var prompt *detector.PromptMatch
		if tt.hasPrompt {
			prompt = &detector.PromptMatch{Summary: "test"}
		}
		obs := Observation{PaneContent: tt.content, Prompt: prompt}
		result := r.ShouldEvaluate(obs)
		if result != tt.expected {
			t.Errorf("content=%q hasPrompt=%v: expected %v, got %v", tt.content, tt.hasPrompt, tt.expected, result)
		}
	}
}

func TestAIRole_Cooldown(t *testing.T) {
	cfg := config.RoleConfig{
		ID:          "cooldown_role",
		Name:        "Cooldown Role",
		Mode:        "proactive",
		Enabled:     true,
		Priority:    30,
		CooldownSec: 1,
	}

	backend := &fakeBackend{
		decision: &ai.Decision{
			ChosenOption: detector.ResponseOption{Key: "y", Label: "Yes"},
			Reasoning:    "ok",
			Confidence:   0.9,
		},
	}

	r, err := NewAIRole(cfg, backend)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obs := Observation{PaneContent: "some content"}

	// First evaluation: should work
	if !r.ShouldEvaluate(obs) {
		t.Error("first evaluation should be allowed")
	}

	_, err = r.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Immediately after: should be blocked by cooldown
	if r.ShouldEvaluate(obs) {
		t.Error("second evaluation should be blocked by cooldown")
	}

	// Wait for cooldown to expire
	time.Sleep(1100 * time.Millisecond)
	if !r.ShouldEvaluate(obs) {
		t.Error("evaluation should be allowed after cooldown")
	}
}

func TestAIRole_Evaluate(t *testing.T) {
	cfg := config.RoleConfig{
		ID:             "eval_role",
		Name:           "Eval Role",
		Mode:           "reactive",
		Enabled:        true,
		Priority:       75,
		SystemPrompt:   "You are a code reviewer.",
		ResponseFormat: "option",
	}

	backend := &fakeBackend{
		decision: &ai.Decision{
			ChosenOption: detector.ResponseOption{Key: "2", Label: "No"},
			Reasoning:    "unsafe operation",
			Confidence:   0.85,
		},
	}

	r, err := NewAIRole(cfg, backend)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obs := Observation{
		PaneContent: "rm -rf /",
		Prompt: &detector.PromptMatch{
			Type:    detector.PromptTypeGeneric,
			Summary: "Execute: rm -rf /",
			Options: []detector.ResponseOption{
				{Key: "1", Label: "Yes"},
				{Key: "2", Label: "No"},
			},
		},
	}

	intervention, err := r.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intervention.Type != InterventionSelectOption {
		t.Errorf("expected select_option, got %s", intervention.Type)
	}
	if intervention.OptionKey != "2" {
		t.Errorf("expected key '2', got %q", intervention.OptionKey)
	}
	if intervention.RoleID != "eval_role" {
		t.Errorf("expected role ID 'eval_role', got %q", intervention.RoleID)
	}
	if intervention.Priority != 75 {
		t.Errorf("expected priority 75, got %d", intervention.Priority)
	}
}

func TestAIRole_InvalidPattern(t *testing.T) {
	cfg := config.RoleConfig{
		ID:              "bad_pattern",
		Name:            "Bad",
		Mode:            "reactive",
		Enabled:         true,
		TriggerPatterns: []string{`[invalid`},
	}

	_, err := NewAIRole(cfg, &fakeBackend{})
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestAIRole_ModeMapping(t *testing.T) {
	tests := []struct {
		configMode string
		expected   Mode
	}{
		{"reactive", ModeReactive},
		{"proactive", ModeProactive},
		{"hybrid", ModeHybrid},
		{"", ModeReactive}, // default
		{"unknown", ModeReactive},
	}

	for _, tt := range tests {
		cfg := config.RoleConfig{
			ID:      "mode_test",
			Name:    "Mode Test",
			Mode:    tt.configMode,
			Enabled: true,
		}
		r, err := NewAIRole(cfg, &fakeBackend{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Mode() != tt.expected {
			t.Errorf("configMode=%q: expected %s, got %s", tt.configMode, tt.expected, r.Mode())
		}
	}
}
