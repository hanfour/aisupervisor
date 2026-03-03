package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewEntry(t *testing.T) {
	e := NewEntry(EntryBugFix, "backend", "Fix null check", "When handling API responses", "Add nil guard", "w1")

	if e.ID == "" {
		t.Error("ID should be generated")
	}
	if e.Type != EntryBugFix {
		t.Errorf("Type = %s, want bug_fix", e.Type)
	}
	if e.Confidence != 0.5 {
		t.Errorf("Confidence = %f, want 0.5", e.Confidence)
	}
}

func TestIsExpired(t *testing.T) {
	e := NewEntry(EntryPattern, "backend", "Test", "", "", "w1")
	if e.IsExpired() {
		t.Error("entry without expiry should not be expired")
	}

	past := time.Now().Add(-1 * time.Hour)
	e.ExpiresAt = &past
	if !e.IsExpired() {
		t.Error("entry with past expiry should be expired")
	}

	future := time.Now().Add(1 * time.Hour)
	e.ExpiresAt = &future
	if e.IsExpired() {
		t.Error("entry with future expiry should not be expired")
	}
}

func TestIsValid(t *testing.T) {
	e := NewEntry(EntryPattern, "backend", "Test", "", "", "w1")
	e.Confidence = 0.8

	if !e.IsValid(0.5) {
		t.Error("should be valid with confidence above threshold")
	}
	if e.IsValid(0.9) {
		t.Error("should not be valid with confidence below threshold")
	}

	past := time.Now().Add(-1 * time.Hour)
	e.ExpiresAt = &past
	if e.IsValid(0.5) {
		t.Error("expired entry should not be valid")
	}
}

func TestStoreAddAndSearch(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	e1 := NewEntry(EntryBugFix, "backend", "Fix 1", "", "", "w1")
	e1.Tags = []string{"go", "api"}
	e1.Confidence = 0.8
	store.Add(e1)

	e2 := NewEntry(EntryPattern, "frontend", "Pattern 1", "", "", "w2")
	e2.Tags = []string{"react", "ui"}
	e2.Confidence = 0.9
	store.Add(e2)

	// Search by domain
	results := store.SearchByDomain("backend", 0.5)
	if len(results) != 1 {
		t.Errorf("SearchByDomain(backend) = %d results, want 1", len(results))
	}

	// Search by tags
	results = store.SearchByTags([]string{"go"}, 0.5)
	if len(results) != 1 {
		t.Errorf("SearchByTags(go) = %d results, want 1", len(results))
	}

	// Search with both
	results = store.Search("backend", []string{"go"}, 0.5)
	if len(results) != 1 {
		t.Errorf("Search(backend, go) = %d results, want 1", len(results))
	}

	// List all
	all := store.ListAll()
	if len(all) != 2 {
		t.Errorf("ListAll = %d, want 2", len(all))
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create and save
	store1 := NewStore(dir)
	e := NewEntry(EntryLessonLearned, "backend", "Lesson", "ctx", "res", "w1")
	e.Confidence = 0.9
	store1.Add(e)

	// Verify file exists
	path := filepath.Join(dir, "knowledge.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("knowledge.yaml not created")
	}

	// Load in new store
	store2 := NewStore(dir)
	if err := store2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	got, ok := store2.Get(e.ID)
	if !ok {
		t.Fatal("entry not found after load")
	}
	if got.Title != "Lesson" {
		t.Errorf("Title = %q, want Lesson", got.Title)
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	e := NewEntry(EntryBugFix, "backend", "Fix", "", "", "w1")
	store.Add(e)

	store.Delete(e.ID)

	_, ok := store.Get(e.ID)
	if ok {
		t.Error("entry should be deleted")
	}
}
