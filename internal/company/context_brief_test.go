package company

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBriefSection_Truncate(t *testing.T) {
	// 10000 chars content, 100 token budget → ~400 chars max
	content := strings.Repeat("x", 10000)
	s := BriefSection{
		Name:        "test",
		Content:     content,
		TokenBudget: 100,
		Priority:    2,
	}

	s.truncate()

	maxChars := 100 * charsPerToken // 400
	if len(s.Content) > maxChars {
		t.Errorf("expected truncated content <= %d chars, got %d", maxChars, len(s.Content))
	}
	if !strings.Contains(s.Content, "... (truncated) ...") {
		t.Error("expected truncation marker in content")
	}
	// Should contain both beginning and end of original
	if !strings.HasPrefix(s.Content, "xxx") {
		t.Error("expected content to start with original prefix")
	}
	if !strings.HasSuffix(s.Content, "xxx") {
		t.Error("expected content to end with original suffix")
	}
}

func TestBriefSection_Truncate_ShortContent(t *testing.T) {
	// Content shorter than budget should not be truncated
	s := BriefSection{
		Name:        "test",
		Content:     "short content",
		TokenBudget: 100,
		Priority:    2,
	}

	s.truncate()

	if s.Content != "short content" {
		t.Errorf("short content should not be truncated, got %q", s.Content)
	}
}

func TestContextBriefBuilder_Build_BasicSections(t *testing.T) {
	builder := &ContextBriefBuilder{
		totalBudget:     6000,
		taskID:          "task-123",
		projectID:       "proj-456",
		projectName:     "MyProject",
		techStack:       "Go + Svelte",
		baseBranch:      "main",
		diffStats:       "3 files changed, 100 insertions(+), 20 deletions(-)",
		changedFiles:    []string{"main.go", "app.go"},
		architectureCtx: "Wails v2 desktop app with Go backend",
		phase0Summary:   "All checks passed",
		conventions:     "Use gofmt",
	}

	brief, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	if brief.TaskID != "task-123" {
		t.Errorf("expected TaskID 'task-123', got %q", brief.TaskID)
	}
	if brief.ProjectID != "proj-456" {
		t.Errorf("expected ProjectID 'proj-456', got %q", brief.ProjectID)
	}
	if len(brief.Sections) == 0 {
		t.Fatal("expected at least one section")
	}

	// Verify expected section names exist
	sectionNames := make(map[string]bool)
	for _, s := range brief.Sections {
		sectionNames[s.Name] = true
	}
	for _, name := range []string{"project_summary", "diff_stats", "architecture", "phase0_results", "conventions"} {
		if !sectionNames[name] {
			t.Errorf("expected section %q to exist", name)
		}
	}
}

func TestContextBriefBuilder_Build_PriorityTruncation(t *testing.T) {
	// Very small total budget to force truncation of low-priority sections
	builder := &ContextBriefBuilder{
		totalBudget:      500, // 500 tokens = 2000 chars total
		taskID:           "task-123",
		projectID:        "proj-456",
		projectName:      "MyProject",
		techStack:        "Go + Svelte",
		baseBranch:       "main",
		diffStats:        strings.Repeat("d", 800),
		changedFiles:     []string{"main.go"},
		architectureCtx:  strings.Repeat("a", 6000),
		phase0Summary:    strings.Repeat("p", 3200),
		conventions:      strings.Repeat("c", 4000),
		rejectionHistory: strings.Repeat("r", 2000),
		karpathyOverlay:  strings.Repeat("k", 2000),
	}

	brief, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	// Low-priority sections (rejection_history, karpathy_overlay, conventions)
	// should be compressed more aggressively than high-priority ones
	totalChars := 0
	for _, s := range brief.Sections {
		totalChars += len(s.Content)
	}

	maxChars := 500 * charsPerToken
	if totalChars > maxChars {
		t.Errorf("total chars %d exceeds budget %d", totalChars, maxChars)
	}
}

func TestContextBrief_Render(t *testing.T) {
	brief := &ContextBrief{
		TaskID:    "task-abc",
		ProjectID: "proj-xyz",
		Sections: []BriefSection{
			{Name: "project_summary", Content: "This is the summary", TokenBudget: 500, Priority: 1},
			{Name: "diff_stats", Content: "3 files changed", TokenBudget: 200, Priority: 1},
			{Name: "architecture", Content: "", TokenBudget: 1500, Priority: 2}, // empty — should be skipped
		},
	}

	rendered := brief.Render()

	if !strings.Contains(rendered, "# Context Brief") {
		t.Error("expected '# Context Brief' header")
	}
	if !strings.Contains(rendered, "task-abc") {
		t.Error("expected task ID in rendered output")
	}
	if !strings.Contains(rendered, "proj-xyz") {
		t.Error("expected project ID in rendered output")
	}
	if !strings.Contains(rendered, "## Project Summary") {
		t.Error("expected 'Project Summary' section title")
	}
	if !strings.Contains(rendered, "This is the summary") {
		t.Error("expected project summary content")
	}
	if !strings.Contains(rendered, "## Diff Stats") {
		t.Error("expected 'Diff Stats' section title")
	}
	// Empty architecture section should be skipped
	if strings.Contains(rendered, "## Architecture") {
		t.Error("empty architecture section should be skipped in render")
	}
}

func TestContextBrief_WriteFile(t *testing.T) {
	dir := t.TempDir()

	brief := &ContextBrief{
		TaskID:    "task-write",
		ProjectID: "proj-write",
		Sections: []BriefSection{
			{Name: "project_summary", Content: "Test summary", TokenBudget: 500, Priority: 1},
		},
	}

	path, err := brief.WriteToDir(dir)
	if err != nil {
		t.Fatalf("WriteToDir() returned error: %v", err)
	}

	expectedDir := filepath.Join(dir, ".ais-review")
	expectedFile := filepath.Join(expectedDir, "context-brief.md")

	if path != expectedFile {
		t.Errorf("expected path %q, got %q", expectedFile, path)
	}
	if brief.FilePath != expectedFile {
		t.Errorf("expected FilePath to be set to %q, got %q", expectedFile, brief.FilePath)
	}

	// Verify directory was created
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("expected .ais-review dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected .ais-review to be a directory")
	}

	// Verify file was written
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("expected context-brief.md to exist: %v", err)
	}
	if !strings.Contains(string(data), "Test summary") {
		t.Error("expected file to contain summary content")
	}
}

func TestContextBriefBuilder_Build_EmptySections(t *testing.T) {
	builder := &ContextBriefBuilder{
		totalBudget: 6000,
		taskID:      "task-empty",
		projectID:   "proj-empty",
		// All content fields left empty
	}

	brief, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	// Even with empty fields, sections should be created
	if len(brief.Sections) == 0 {
		t.Fatal("expected sections to be created even with empty content")
	}

	// Render should skip empty sections
	rendered := brief.Render()
	if strings.Contains(rendered, "## Architecture") {
		t.Error("empty architecture section should be skipped in render")
	}
}

func TestContextBriefBuilder_Defaults(t *testing.T) {
	builder := &ContextBriefBuilder{
		taskID:    "task-def",
		projectID: "proj-def",
		// totalBudget left as 0 — should default to 6000
	}

	brief, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}

	// Should not error with zero budget (defaults to 6000)
	if brief == nil {
		t.Fatal("expected non-nil brief")
	}

	// Verify the builder applied the default
	if builder.totalBudget != 6000 {
		t.Errorf("expected totalBudget to default to 6000, got %d", builder.totalBudget)
	}
}

func TestSectionTitle(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"project_summary", "Project Summary"},
		{"diff_stats", "Diff Stats"},
		{"architecture", "Architecture"},
		{"phase0_results", "Phase 0 Results"},
		{"conventions", "Conventions"},
		{"graph_context", "Graph Context"},
		{"rejection_history", "Rejection History"},
		{"karpathy_overlay", "Karpathy Overlay"},
		{"unknown_section", "Unknown Section"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sectionTitle(tt.name)
			if got != tt.expected {
				t.Errorf("sectionTitle(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}
