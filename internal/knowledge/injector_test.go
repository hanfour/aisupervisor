package knowledge

import (
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
	ctx, err := injector.BuildContext("w1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Fatal("context should not be empty")
	}
	if len(ctx) > 2500 {
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
	ctx, _ := injector.BuildContext("w1", "p1")
	if len(ctx) > 1500 {
		t.Errorf("should respect token budget, got %d chars", len(ctx))
	}
}

func TestInjector_EmptyKnowledge(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	injector := NewInjector(store, 500)
	ctx, err := injector.BuildContext("w1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "" {
		t.Error("should return empty string for no knowledge")
	}
}
