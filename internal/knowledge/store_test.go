package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_AddAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	entry := Entry{
		ProjectID: "p1",
		WorkerID:  "w1",
		Type:      KnowledgeTaskSummary,
		Summary:   "Implemented login page with OAuth",
		Files:     []string{"src/login.go"},
		Relevance: 0.8,
	}

	err := store.Add(entry)
	if err != nil {
		t.Fatalf("failed to add: %v", err)
	}

	entries, err := store.GetForWorker("w1", "p1")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Summary != "Implemented login page with OAuth" {
		t.Errorf("unexpected summary: %s", entries[0].Summary)
	}
}

func TestStore_GetShared(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Add(Entry{
		ProjectID: "p1",
		Type:      KnowledgeArchitecture,
		Summary:   "Uses MVC pattern",
		Relevance: 1.0,
	})
	store.Add(Entry{
		ProjectID: "p1",
		WorkerID:  "w1",
		Type:      KnowledgeTaskSummary,
		Summary:   "Personal note",
		Relevance: 0.5,
	})

	shared, _ := store.GetShared("p1")
	if len(shared) != 1 {
		t.Errorf("expected 1 shared entry, got %d", len(shared))
	}
}

func TestStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	store1 := NewStore(dir)
	store1.Add(Entry{
		ProjectID: "p1",
		Type:      KnowledgeGotcha,
		Summary:   "tmux capture needs -S flag",
		Relevance: 0.9,
	})

	store2 := NewStore(dir)
	entries, _ := store2.GetShared("p1")
	if len(entries) != 1 {
		t.Errorf("expected persistence, got %d entries", len(entries))
	}

	sharedPath := filepath.Join(dir, "projects", "p1", "shared.yaml")
	if _, err := os.Stat(sharedPath); os.IsNotExist(err) {
		t.Error("shared.yaml should exist on disk")
	}
}
