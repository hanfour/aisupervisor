package company

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConventionStore_NewEmpty(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}
	if cs == nil {
		t.Fatal("expected non-nil store")
	}
	if len(cs.conventions) != 0 {
		t.Errorf("expected 0 conventions, got %d", len(cs.conventions))
	}

	// Verify conventions/ subdir was created
	convDir := filepath.Join(dir, "conventions")
	info, err := os.Stat(convDir)
	if err != nil {
		t.Fatalf("conventions dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("conventions path is not a directory")
	}
}

func TestConventionStore_ProposeAndSave(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	id := cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml.Unmarshal error ignored",
		Description: "Config loading allows silent YAML errors",
		FileGlob:    "*.go",
		Source:      "review:task-123",
	})
	if id != "conv-001" {
		t.Errorf("expected conv-001, got %s", id)
	}

	if err := cs.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify index.yaml exists
	indexPath := filepath.Join(dir, "conventions", "index.yaml")
	if _, err := os.Stat(indexPath); err != nil {
		t.Errorf("index.yaml not created: %v", err)
	}

	// Verify conventions.md exists
	mdPath := filepath.Join(dir, "conventions", "conventions.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("conventions.md not created: %v", err)
	}
}

func TestConventionStore_LoadExisting(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml error ignored",
		Description: "Config loading allows silent YAML errors",
		FileGlob:    "*.go",
		Source:      "review:task-123",
	})
	cs.Propose(Convention{
		Domain:      DomainFrontend,
		Pattern:     "missing reactive cleanup",
		Description: "Svelte components should unsubscribe on destroy",
		FileGlob:    "*.svelte",
		Source:      "review:task-456",
	})
	if err := cs.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload from same directory
	cs2, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("reload NewConventionStore: %v", err)
	}
	if len(cs2.conventions) != 2 {
		t.Fatalf("expected 2 conventions after reload, got %d", len(cs2.conventions))
	}
	if cs2.conventions[0].ID != "conv-001" {
		t.Errorf("expected conv-001, got %s", cs2.conventions[0].ID)
	}
	if cs2.conventions[1].ID != "conv-002" {
		t.Errorf("expected conv-002, got %s", cs2.conventions[1].ID)
	}
	if cs2.conventions[0].Domain != DomainBackend {
		t.Errorf("expected backend domain, got %s", cs2.conventions[0].Domain)
	}

	// idSeq should be initialized from max existing ID
	id := cs2.Propose(Convention{
		Domain:  DomainTesting,
		Pattern: "test pattern",
		FileGlob: "*_test.go",
		Source:  "review:task-789",
	})
	if id != "conv-003" {
		t.Errorf("expected conv-003 after reload, got %s", id)
	}
}

func TestConventionStore_Accept(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	id := cs.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "yaml error",
		FileGlob: "*.go",
		Source:  "review:task-1",
	})

	cs.Accept(id)
	cs.Accept(id)

	var found *Convention
	for i := range cs.conventions {
		if cs.conventions[i].ID == id {
			found = &cs.conventions[i]
			break
		}
	}
	if found == nil {
		t.Fatal("convention not found")
	}
	if found.AcceptCount != 2 {
		t.Errorf("expected AcceptCount=2, got %d", found.AcceptCount)
	}
}

func TestConventionStore_FindRelevant(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	// Backend convention for Go files (accepted)
	id1 := cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml error ignored",
		Description: "Config loading allows silent YAML errors",
		FileGlob:    "*.go",
		Source:      "review:task-1",
	})
	cs.Accept(id1)
	cs.Accept(id1)

	// Frontend convention for Svelte files (accepted)
	id2 := cs.Propose(Convention{
		Domain:      DomainFrontend,
		Pattern:     "missing cleanup",
		Description: "Missing reactive cleanup",
		FileGlob:    "*.svelte",
		Source:      "review:task-2",
	})
	cs.Accept(id2)
	cs.Accept(id2)

	// Proposed but not accepted (AcceptCount=0)
	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "proposed only",
		Description: "This should not appear",
		FileGlob:    "*.go",
		Source:      "review:task-3",
	})

	// Query for backend + main.go → should get only the backend convention
	results := cs.FindRelevant(DomainBackend, "main.go")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != id1 {
		t.Errorf("expected %s, got %s", id1, results[0].ID)
	}

	// Query for frontend + App.svelte → should get only the frontend convention
	results = cs.FindRelevant(DomainFrontend, "App.svelte")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != id2 {
		t.Errorf("expected %s, got %s", id2, results[0].ID)
	}

	// Query for backend + App.svelte → no match (glob mismatch)
	results = cs.FindRelevant(DomainBackend, "App.svelte")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestConventionStore_FindRelevant_WildcardGlob(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	id := cs.Propose(Convention{
		Domain:  DomainSecurity,
		Pattern: "hardcoded secrets",
		FileGlob: "*",
		Source:  "review:task-1",
	})
	cs.Accept(id)
	cs.Accept(id)

	results := cs.FindRelevant(DomainSecurity, "anything.txt")
	if len(results) != 1 {
		t.Errorf("expected 1 result for wildcard glob, got %d", len(results))
	}
}

func TestConventionStore_MatchesFinding(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	id := cs.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "yaml.Unmarshal error ignored",
		FileGlob: "*.go",
		Source:  "review:task-1",
	})
	cs.Accept(id)
	cs.Accept(id)

	ef := ExpertFinding{
		Finding: Finding{
			File:     "config.go",
			Severity: "MEDIUM",
			Body:     "yaml.Unmarshal return value is discarded",
		},
		Expert: DomainBackend,
	}

	match := cs.MatchesFinding(ef)
	if match == nil {
		t.Fatal("expected a match, got nil")
	}
	if match.ID != id {
		t.Errorf("expected %s, got %s", id, match.ID)
	}
}

func TestConventionStore_MatchesFinding_NoMatch(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	id := cs.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "yaml.Unmarshal error ignored",
		FileGlob: "*.go",
		Source:  "review:task-1",
	})
	cs.Accept(id)
	cs.Accept(id)

	ef := ExpertFinding{
		Finding: Finding{
			File:     "handler.go",
			Severity: "HIGH",
			Body:     "SQL injection via string concatenation in query builder",
		},
		Expert: DomainSecurity,
	}

	match := cs.MatchesFinding(ef)
	if match != nil {
		t.Errorf("expected nil match for unrelated finding, got %s", match.ID)
	}
}

func TestConventionStore_MatchesFinding_ProposedNotMatched(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	// Only proposed, not accepted (AcceptCount=0)
	cs.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "yaml.Unmarshal error ignored",
		FileGlob: "*.go",
		Source:  "review:task-1",
	})

	ef := ExpertFinding{
		Finding: Finding{
			File:     "config.go",
			Severity: "MEDIUM",
			Body:     "yaml.Unmarshal error ignored silently",
		},
		Expert: DomainBackend,
	}

	match := cs.MatchesFinding(ef)
	if match != nil {
		t.Errorf("expected nil for proposed-only convention, got %s", match.ID)
	}
}

func TestConventionStore_Decay(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	// Old convention with low uses — should be decayed
	id1 := cs.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "old pattern",
		FileGlob: "*.go",
		Source:  "review:task-1",
	})
	// Manually set LastUsed to 60 days ago
	for i := range cs.conventions {
		if cs.conventions[i].ID == id1 {
			cs.conventions[i].LastUsed = time.Now().Add(-60 * 24 * time.Hour)
			cs.conventions[i].AcceptCount = 1
			break
		}
	}

	// Frequently used convention — should survive
	id2 := cs.Propose(Convention{
		Domain:  DomainFrontend,
		Pattern: "active pattern",
		FileGlob: "*.svelte",
		Source:  "review:task-2",
	})
	cs.Accept(id2)
	cs.Accept(id2)
	cs.Accept(id2)
	cs.Accept(id2)
	cs.Accept(id2) // AcceptCount=5

	// Recently used, low accept — should survive (not old enough)
	cs.Propose(Convention{
		Domain:  DomainTesting,
		Pattern: "recent pattern",
		FileGlob: "*_test.go",
		Source:  "review:task-3",
	})

	removed := cs.Decay(30*24*time.Hour, 3) // maxAge=30 days, minUses=3
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if len(cs.conventions) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(cs.conventions))
	}

	// Verify the old one was removed
	for _, c := range cs.conventions {
		if c.ID == id1 {
			t.Error("old convention should have been removed")
		}
	}
}

func TestConventionStore_MarkdownRender(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("NewConventionStore: %v", err)
	}

	id1 := cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml error ignored",
		Description: "Config loading allows silent YAML errors",
		FileGlob:    "internal/config/*.go",
		Source:      "review:task-123",
	})
	cs.Accept(id1)
	cs.Accept(id1)
	cs.Accept(id1)
	cs.Accept(id1) // AcceptCount=4

	cs.Propose(Convention{
		Domain:      DomainFrontend,
		Pattern:     "missing reactive cleanup",
		Description: "Svelte components should unsubscribe on destroy",
		FileGlob:    "*.svelte",
		Source:      "review:task-456",
	})

	if err := cs.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	mdPath := filepath.Join(dir, "conventions", "conventions.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read conventions.md: %v", err)
	}
	md := string(data)

	// Check header
	if !strings.Contains(md, "# Project Conventions") {
		t.Error("missing header in conventions.md")
	}
	if !strings.Contains(md, "Auto-maintained by council review system") {
		t.Error("missing subtitle in conventions.md")
	}

	// Check domain grouping
	if !strings.Contains(md, "## backend") {
		t.Error("missing backend domain header")
	}
	if !strings.Contains(md, "## frontend") {
		t.Error("missing frontend domain header")
	}

	// Check convention entry format
	if !strings.Contains(md, "### conv-001: yaml error ignored") {
		t.Error("missing convention title")
	}
	if !strings.Contains(md, "**Applicable**: `internal/config/*.go`") {
		t.Error("missing applicable glob")
	}
	if !strings.Contains(md, "**Description**: Config loading allows silent YAML errors") {
		t.Error("missing description")
	}
	if !strings.Contains(md, "**Source**: review:task-123") {
		t.Error("missing source")
	}
	if !strings.Contains(md, "**References**: 4") {
		t.Error("missing references count")
	}
}

func TestWordSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"yaml.Unmarshal error ignored", "yaml.Unmarshal return value is discarded", 0.20, 0.5},
		{"hello world", "hello world", 1.0, 1.0},
		{"completely different", "nothing similar here", 0.0, 0.01},
		{"", "", 0.0, 0.01}, // edge case: empty strings
		{"one", "two three four five", 0.0, 0.01},
	}
	for _, tc := range tests {
		sim := wordSimilarity(tc.a, tc.b)
		if sim < tc.min || sim > tc.max {
			t.Errorf("wordSimilarity(%q, %q) = %.3f, expected [%.3f, %.3f]",
				tc.a, tc.b, sim, tc.min, tc.max)
		}
	}
}
