package role

import (
	"context"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
)

type fakeBackend struct {
	decision *ai.Decision
	err      error
}

func (f *fakeBackend) Name() string { return "fake" }
func (f *fakeBackend) Analyze(_ context.Context, _ ai.AnalysisRequest) (*ai.Decision, error) {
	return f.decision, f.err
}
func (f *fakeBackend) Healthy(_ context.Context) error { return nil }

func TestGatekeeperRole_AutoApprove(t *testing.T) {
	backend := &fakeBackend{}
	rules := []config.AutoApproveRule{
		{Label: "Approve reads", PatternContains: "Reading file", Response: "1"},
	}

	gk := NewGatekeeperRole(backend, rules, 0.7)

	obs := Observation{
		PaneContent: "some content",
		Prompt: &detector.PromptMatch{
			Type:    detector.PromptTypeClaudeCode,
			Summary: "Reading file: main.go",
			Options: []detector.ResponseOption{
				{Key: "1", Label: "Yes"},
				{Key: "2", Label: "No"},
			},
		},
	}

	if !gk.ShouldEvaluate(obs) {
		t.Fatal("gatekeeper should evaluate when prompt is present")
	}

	intervention, err := gk.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intervention.Type != InterventionSelectOption {
		t.Errorf("expected select_option, got %s", intervention.Type)
	}
	if intervention.OptionKey != "1" {
		t.Errorf("expected option key '1', got %q", intervention.OptionKey)
	}
	if !IsAutoApproved(intervention) {
		t.Error("expected auto-approved intervention")
	}
}

func TestGatekeeperRole_ProjectRules(t *testing.T) {
	backend := &fakeBackend{}
	gk := NewGatekeeperRole(backend, nil, 0.7)

	sc := sessionctx.NewSessionContext("test", 20, 10)
	sc.SetRules([]sessionctx.ProjectRule{
		{
			Label:           "Allow go build",
			PatternContains: "go build",
			Response:        "y",
		},
	})

	obs := Observation{
		PaneContent:    "running go build ./...",
		Prompt: &detector.PromptMatch{
			Type:    detector.PromptTypeGeneric,
			Summary: "Execute: go build ./...",
			Options: []detector.ResponseOption{
				{Key: "y", Label: "Yes"},
				{Key: "n", Label: "No"},
			},
		},
		SessionContext: sc,
	}

	intervention, err := gk.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intervention.OptionKey != "y" {
		t.Errorf("expected option key 'y', got %q", intervention.OptionKey)
	}
	if !IsAutoApproved(intervention) {
		t.Error("expected auto-approved")
	}
}

func TestGatekeeperRole_AIDecision(t *testing.T) {
	backend := &fakeBackend{
		decision: &ai.Decision{
			ChosenOption: detector.ResponseOption{Key: "1", Label: "Yes"},
			Reasoning:    "safe operation",
			Confidence:   0.9,
		},
	}
	gk := NewGatekeeperRole(backend, nil, 0.7)

	obs := Observation{
		PaneContent: "some content",
		Prompt: &detector.PromptMatch{
			Type:    detector.PromptTypeClaudeCode,
			Summary: "Write file: output.txt",
			Options: []detector.ResponseOption{
				{Key: "1", Label: "Yes"},
				{Key: "2", Label: "No"},
			},
		},
	}

	intervention, err := gk.Evaluate(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if intervention.OptionKey != "1" {
		t.Errorf("expected '1', got %q", intervention.OptionKey)
	}
	if intervention.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", intervention.Confidence)
	}
	if IsAutoApproved(intervention) {
		t.Error("should not be auto-approved")
	}
}

func TestGatekeeperRole_NoPrompt(t *testing.T) {
	gk := NewGatekeeperRole(&fakeBackend{}, nil, 0.7)
	obs := Observation{PaneContent: "just some output"}

	if gk.ShouldEvaluate(obs) {
		t.Error("gatekeeper should not evaluate without a prompt")
	}
}

func TestManager_PriorityResolution(t *testing.T) {
	// Create two roles with different priorities
	high := &staticRole{id: "high", priority: 100, mode: ModeReactive, shouldEval: true, intervention: &Intervention{
		Type: InterventionSelectOption, OptionKey: "1", RoleID: "high", Priority: 100, Confidence: 0.9,
	}}
	low := &staticRole{id: "low", priority: 50, mode: ModeReactive, shouldEval: true, intervention: &Intervention{
		Type: InterventionSelectOption, OptionKey: "2", RoleID: "low", Priority: 50, Confidence: 0.8,
	}}

	mgr := NewManager(low, high)

	obs := Observation{
		Prompt: &detector.PromptMatch{
			Type: detector.PromptTypeGeneric,
			Summary: "test",
		},
	}

	intervention, err := mgr.EvaluateReactive(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intervention == nil {
		t.Fatal("expected intervention")
	}
	if intervention.RoleID != "high" {
		t.Errorf("expected high priority role to win, got %q", intervention.RoleID)
	}
}

func TestManager_AddRemoveList(t *testing.T) {
	mgr := NewManager()

	r1 := &staticRole{id: "r1", mode: ModeReactive}
	r2 := &staticRole{id: "r2", mode: ModeProactive}

	mgr.Add(r1)
	mgr.Add(r2)

	if len(mgr.List()) != 2 {
		t.Errorf("expected 2 roles, got %d", len(mgr.List()))
	}

	mgr.Remove("r1")
	if len(mgr.List()) != 1 {
		t.Errorf("expected 1 role after remove, got %d", len(mgr.List()))
	}

	if _, ok := mgr.Get("r2"); !ok {
		t.Error("expected to find r2")
	}
	if _, ok := mgr.Get("r1"); ok {
		t.Error("r1 should have been removed")
	}
}

func TestManager_EvaluateProactive(t *testing.T) {
	r1 := &staticRole{id: "p1", priority: 80, mode: ModeProactive, shouldEval: true, intervention: &Intervention{
		Type: InterventionFreeText, Text: "looks good", RoleID: "p1", Priority: 80,
	}}
	r2 := &staticRole{id: "p2", priority: 60, mode: ModeProactive, shouldEval: true, intervention: &Intervention{
		Type: InterventionFreeText, Text: "review needed", RoleID: "p2", Priority: 60,
	}}

	mgr := NewManager(r1, r2)
	obs := Observation{PaneContent: "some code output"}

	interventions, err := mgr.EvaluateProactive(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interventions) != 2 {
		t.Fatalf("expected 2 interventions, got %d", len(interventions))
	}
	// Should be sorted by priority descending
	if interventions[0].RoleID != "p1" {
		t.Errorf("expected p1 first (higher priority), got %s", interventions[0].RoleID)
	}
}

func TestManager_SkipsNonMatchingModes(t *testing.T) {
	proactive := &staticRole{id: "pro", mode: ModeProactive, shouldEval: true, intervention: &Intervention{
		Type: InterventionFreeText, Text: "hi", RoleID: "pro",
	}}

	mgr := NewManager(proactive)
	obs := Observation{Prompt: &detector.PromptMatch{Summary: "test"}}

	// EvaluateReactive should not pick up proactive-only role
	intervention, err := mgr.EvaluateReactive(context.Background(), obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intervention != nil {
		t.Error("proactive-only role should not be evaluated in reactive path")
	}
}

// staticRole is a test helper that returns fixed values.
type staticRole struct {
	id           string
	name         string
	mode         Mode
	priority     int
	shouldEval   bool
	intervention *Intervention
}

func (r *staticRole) ID() string       { return r.id }
func (r *staticRole) Name() string     { return r.name }
func (r *staticRole) Mode() Mode       { return r.mode }
func (r *staticRole) Priority() int    { return r.priority }
func (r *staticRole) ShouldEvaluate(_ Observation) bool { return r.shouldEval }
func (r *staticRole) Evaluate(_ context.Context, _ Observation) (*Intervention, error) {
	return r.intervention, nil
}
