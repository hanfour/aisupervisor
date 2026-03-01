package group

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/role"
)

// mockRole implements role.Role for testing.
type mockRole struct {
	id       string
	name     string
	mode     role.Mode
	priority int
	action   string
	text     string
	reasoning string
	confidence float64
	evalDelay  time.Duration
}

func (r *mockRole) ID() string    { return r.id }
func (r *mockRole) Name() string  { return r.name }
func (r *mockRole) Mode() role.Mode { return r.mode }
func (r *mockRole) Priority() int { return r.priority }
func (r *mockRole) ShouldEvaluate(obs role.Observation) bool { return obs.Prompt != nil }

func (r *mockRole) Evaluate(ctx context.Context, obs role.Observation) (*role.Intervention, error) {
	if r.evalDelay > 0 {
		time.Sleep(r.evalDelay)
	}

	ivType := role.InterventionSelectOption
	if r.text != "" {
		ivType = role.InterventionFreeText
	}

	return &role.Intervention{
		Type:       ivType,
		OptionKey:  r.action,
		Text:       r.text,
		Reasoning:  r.reasoning,
		Confidence: r.confidence,
		RoleID:     r.id,
		Priority:   r.priority,
	}, nil
}

func makeObs() role.Observation {
	return role.Observation{
		PaneContent: "test pane content",
		Prompt: &detector.PromptMatch{
			Type:    "test",
			Summary: "test prompt",
			Options: []detector.ResponseOption{
				{Key: "y", Label: "Yes"},
				{Key: "n", Label: "No"},
			},
		},
		SessionName: "test-session",
	}
}

func TestDetectDivergence_NoDivergence(t *testing.T) {
	opinions := []Opinion{
		{RoleID: "r1", Action: "y", Confidence: 0.9},
		{RoleID: "r2", Action: "y", Confidence: 0.85},
	}
	result := DetectDivergence(opinions, 0.3)
	if result.Divergent {
		t.Error("expected no divergence when all actions agree and confidence spread is small")
	}
}

func TestDetectDivergence_DifferentActions(t *testing.T) {
	opinions := []Opinion{
		{RoleID: "r1", Action: "y", Confidence: 0.9},
		{RoleID: "r2", Action: "n", Confidence: 0.9},
	}
	result := DetectDivergence(opinions, 0.3)
	if !result.Divergent {
		t.Error("expected divergence when actions differ")
	}
	if len(result.UniqueActions) != 2 {
		t.Errorf("expected 2 unique actions, got %d", len(result.UniqueActions))
	}
}

func TestDetectDivergence_HighConfidenceSpread(t *testing.T) {
	opinions := []Opinion{
		{RoleID: "r1", Action: "y", Confidence: 0.95},
		{RoleID: "r2", Action: "y", Confidence: 0.3},
	}
	result := DetectDivergence(opinions, 0.3)
	if !result.Divergent {
		t.Error("expected divergence when confidence spread exceeds threshold")
	}
	if result.ConfidenceSpread < 0.6 {
		t.Errorf("expected confidence spread ~0.65, got %f", result.ConfidenceSpread)
	}
}

func TestDetectDivergence_SingleOpinion(t *testing.T) {
	opinions := []Opinion{
		{RoleID: "r1", Action: "y", Confidence: 0.5},
	}
	result := DetectDivergence(opinions, 0.3)
	if result.Divergent {
		t.Error("expected no divergence for single opinion")
	}
}

func TestDetectDivergence_Empty(t *testing.T) {
	result := DetectDivergence(nil, 0.3)
	if result.Divergent {
		t.Error("expected no divergence for empty opinions")
	}
}

func TestRunDiscussion_NoDivergence_SkipsRoundtable(t *testing.T) {
	r1 := &mockRole{id: "r1", name: "Role1", mode: role.ModeReactive, priority: 100, action: "y", reasoning: "safe", confidence: 0.9}
	r2 := &mockRole{id: "r2", name: "Role2", mode: role.ModeReactive, priority: 80, action: "y", reasoning: "looks good", confidence: 0.85}

	rm := role.NewManager(r1, r2)
	grp := &Group{
		ID:                  "g1",
		Name:                "Test Group",
		LeaderID:            "r1",
		RoleIDs:             []string{"r1", "r2"},
		DivergenceThreshold: 0.3,
	}
	mgr := NewManager(rm, []*Group{grp})

	// Collect events
	var events []DiscussionEvent
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		for e := range mgr.DiscussionEvents() {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}
	}()

	obs := makeObs()
	iv, err := mgr.RunDiscussion(context.Background(), grp, obs, "sess1")
	close(done)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iv == nil {
		t.Fatal("expected intervention, got nil")
	}
	if iv.OptionKey != "y" {
		t.Errorf("expected action 'y', got %q", iv.OptionKey)
	}

	// Wait a bit for events to be processed
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have opinion events + decision event, no roundtable
	hasRoundtable := false
	for _, e := range events {
		if e.Phase == PhaseRoundtable {
			hasRoundtable = true
		}
	}
	if hasRoundtable {
		t.Error("expected no roundtable phase when opinions agree")
	}
}

func TestRunDiscussion_WithDivergence_TriggersRoundtable(t *testing.T) {
	r1 := &mockRole{id: "leader", name: "Leader", mode: role.ModeReactive, priority: 100, action: "y", reasoning: "approve", confidence: 0.9}
	r2 := &mockRole{id: "skeptic", name: "Skeptic", mode: role.ModeReactive, priority: 80, action: "n", reasoning: "deny", confidence: 0.8}

	rm := role.NewManager(r1, r2)
	grp := &Group{
		ID:                  "g1",
		Name:                "Test Group",
		LeaderID:            "leader",
		RoleIDs:             []string{"leader", "skeptic"},
		DivergenceThreshold: 0.3,
	}
	mgr := NewManager(rm, []*Group{grp})

	// Collect events
	var events []DiscussionEvent
	var mu sync.Mutex
	go func() {
		for e := range mgr.DiscussionEvents() {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}
	}()

	obs := makeObs()
	iv, err := mgr.RunDiscussion(context.Background(), grp, obs, "sess1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iv == nil {
		t.Fatal("expected intervention, got nil")
	}

	// Wait for events
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have opinion events, roundtable events, and decision event
	phases := make(map[DiscussionPhase]int)
	for _, e := range events {
		phases[e.Phase]++
	}

	if phases[PhaseOpinion] < 2 {
		t.Errorf("expected at least 2 opinion events, got %d", phases[PhaseOpinion])
	}
	if phases[PhaseRoundtable] < 1 {
		t.Errorf("expected at least 1 roundtable event, got %d", phases[PhaseRoundtable])
	}
	if phases[PhaseDecision] < 1 {
		t.Errorf("expected at least 1 decision event, got %d", phases[PhaseDecision])
	}
}

func TestDiscussionEventStreamOrder(t *testing.T) {
	r1 := &mockRole{id: "r1", name: "R1", mode: role.ModeReactive, priority: 100, action: "y", reasoning: "yes", confidence: 0.9}
	r2 := &mockRole{id: "r2", name: "R2", mode: role.ModeReactive, priority: 80, action: "n", reasoning: "no", confidence: 0.8}

	rm := role.NewManager(r1, r2)
	grp := &Group{
		ID:                  "g1",
		Name:                "Test",
		LeaderID:            "r1",
		RoleIDs:             []string{"r1", "r2"},
		DivergenceThreshold: 0.3,
	}
	mgr := NewManager(rm, []*Group{grp})

	var events []DiscussionEvent
	var mu sync.Mutex
	go func() {
		for e := range mgr.DiscussionEvents() {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}
	}()

	obs := makeObs()
	_, err := mgr.RunDiscussion(context.Background(), grp, obs, "sess1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Verify phase order: all opinions before roundtable, roundtable before decision
	lastOpinionIdx := -1
	firstRoundtableIdx := len(events)
	lastRoundtableIdx := -1
	firstDecisionIdx := len(events)

	for i, e := range events {
		switch e.Phase {
		case PhaseOpinion:
			lastOpinionIdx = i
		case PhaseRoundtable:
			if i < firstRoundtableIdx {
				firstRoundtableIdx = i
			}
			lastRoundtableIdx = i
		case PhaseDecision:
			if i < firstDecisionIdx {
				firstDecisionIdx = i
			}
		}
	}

	if lastOpinionIdx >= firstRoundtableIdx {
		t.Error("opinion events should come before roundtable events")
	}
	if lastRoundtableIdx >= firstDecisionIdx {
		t.Error("roundtable events should come before decision events")
	}
}

func TestEvaluateWithGroups_Fallback(t *testing.T) {
	r1 := &mockRole{id: "r1", name: "R1", mode: role.ModeReactive, priority: 100, action: "y", reasoning: "yes", confidence: 0.9}

	rm := role.NewManager(r1)
	// No groups configured
	mgr := NewManager(rm, nil)

	obs := makeObs()
	iv, err := mgr.EvaluateWithGroups(context.Background(), obs, "sess1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iv == nil {
		t.Fatal("expected intervention from fallback")
	}
	if iv.OptionKey != "y" {
		t.Errorf("expected action 'y', got %q", iv.OptionKey)
	}
}
