package knowledge

import (
	"strings"
	"testing"
	"time"
)

func TestInjector_BuildContext(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	store.Add(Entry{
		ProjectID: "p1",
		Type:      KnowledgeArchitecture,
		Summary:   "MVC architecture with Go backend",
		Relevance: 1.0,
	})
	store.Add(Entry{
		ProjectID: "p1",
		WorkerID:  "w1",
		Type:      KnowledgeTaskSummary,
		Summary:   "Implemented auth middleware",
		Relevance: 0.9,
	})
	store.Add(Entry{
		ProjectID: "p1",
		WorkerID:  "w1",
		Type:      KnowledgeLessonLearnt,
		Summary:   "Always check error returns from yaml.Unmarshal",
		Relevance: 0.7,
	})

	injector := NewInjector(store, 500)
	ctx, err := injector.BuildContext("w1", "p1", TierL3DeepSearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Fatal("context should not be empty")
	}
	if len(ctx) > 7500 {
		t.Errorf("context too long: %d chars", len(ctx))
	}
}

func TestInjector_RespectsTokenBudget(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	for i := 0; i < 50; i++ {
		store.Add(Entry{
			ProjectID: "p1",
			WorkerID:  "w1",
			Type:      KnowledgeTaskSummary,
			Summary:   "This is a moderately long summary for testing token budget limits in the injector.",
			Relevance: 0.5,
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}

	injector := NewInjector(store, 200)
	ctx, _ := injector.BuildContext("w1", "p1", TierL3DeepSearch)
	if len(ctx) > 1500 {
		t.Errorf("should respect token budget, got %d chars", len(ctx))
	}
}

func TestInjector_EmptyKnowledge(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	injector := NewInjector(store, 500)
	ctx, err := injector.BuildContext("w1", "p1", TierL2RoomRecall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "" {
		t.Error("should return empty string for no knowledge")
	}
}

func TestTierConstants(t *testing.T) {
	tests := []struct {
		tier KnowledgeTier
		want int
	}{
		{TierL0Identity, 0},
		{TierL1Essential, 1},
		{TierL2RoomRecall, 2},
		{TierL3DeepSearch, 3},
	}
	for _, tt := range tests {
		if int(tt.tier) != tt.want {
			t.Errorf("tier %d != %d", tt.tier, tt.want)
		}
	}
}

func TestBuildContextTierFiltering(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	entries := []Entry{
		{ProjectID: "p1", Type: KnowledgeArchitecture, Summary: "Uses hexagonal arch", Relevance: 0.9},
		{ProjectID: "p1", Type: KnowledgeDecision, Summary: "Use YAML for config", Relevance: 0.8},
		{ProjectID: "p1", Type: KnowledgeLessonLearnt, Summary: "Avoid global state", Relevance: 0.7},
	}
	for _, e := range entries {
		if err := store.Add(e); err != nil {
			t.Fatal(err)
		}
	}

	inj := NewInjector(store, 2000)

	tests := []struct {
		tier         KnowledgeTier
		wantContains []string
		wantMissing  []string
	}{
		{TierL1Essential, []string{"YAML for config"}, []string{"hexagonal", "global state"}},
		{TierL2RoomRecall, []string{"YAML for config", "hexagonal"}, []string{"global state"}},
		{TierL3DeepSearch, []string{"YAML for config", "hexagonal", "global state"}, nil},
	}

	for _, tt := range tests {
		ctx, err := inj.BuildContext("", "p1", tt.tier)
		if err != nil {
			t.Fatalf("tier %d: %v", tt.tier, err)
		}
		for _, s := range tt.wantContains {
			if !strings.Contains(ctx, s) {
				t.Errorf("tier %d: missing %q in:\n%s", tt.tier, s, ctx)
			}
		}
		for _, s := range tt.wantMissing {
			if strings.Contains(ctx, s) {
				t.Errorf("tier %d: should not contain %q", tt.tier, s)
			}
		}
	}
}

func TestTierForType(t *testing.T) {
	tests := []struct {
		kt   KnowledgeType
		want KnowledgeTier
	}{
		{KnowledgeDecision, TierL1Essential},
		{KnowledgeFeedback, TierL1Essential},
		{KnowledgeArchitecture, TierL2RoomRecall},
		{KnowledgeGotcha, TierL2RoomRecall},
		{KnowledgeTaskSummary, TierL2RoomRecall},
		{KnowledgeLessonLearnt, TierL3DeepSearch},
	}
	for _, tt := range tests {
		if got := TierForType(tt.kt); got != tt.want {
			t.Errorf("TierForType(%s) = %d, want %d", tt.kt, got, tt.want)
		}
	}
}
