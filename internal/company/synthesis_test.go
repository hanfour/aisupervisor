package company

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// --- mergeCouncilFindings tests ---

func TestMergeCouncilFindings_DeduplicateSameFileLine(t *testing.T) {
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "main.go", Line: 10, Severity: "MEDIUM", Body: "minor issue"},
			Expert:     DomainBackend,
			Confidence: 0.8,
		},
		{
			Finding:    Finding{File: "main.go", Line: 10, Severity: "HIGH", Body: "serious issue"},
			Expert:     DomainSecurity,
			Confidence: 0.9,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 1 {
		t.Fatalf("expected 1 finding after merge, got %d", len(merged))
	}
	if merged[0].Severity != "HIGH" {
		t.Errorf("expected highest severity HIGH, got %s", merged[0].Severity)
	}
}

func TestMergeCouncilFindings_SameSeverityKeepHigherConfidence(t *testing.T) {
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "main.go", Line: 10, Severity: "HIGH", Body: "issue A"},
			Expert:     DomainBackend,
			Confidence: 0.7,
		},
		{
			Finding:    Finding{File: "main.go", Line: 10, Severity: "HIGH", Body: "issue B"},
			Expert:     DomainSecurity,
			Confidence: 0.95,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 1 {
		t.Fatalf("expected 1 finding after merge, got %d", len(merged))
	}
	if merged[0].Confidence != 0.95 {
		t.Errorf("expected highest confidence 0.95, got %f", merged[0].Confidence)
	}
}

func TestMergeCouncilFindings_KeepDifferentFiles(t *testing.T) {
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "main.go", Line: 10, Severity: "HIGH", Body: "missing error handling"},
			Expert:     DomainBackend,
			Confidence: 0.9,
		},
		{
			Finding:    Finding{File: "server.go", Line: 10, Severity: "HIGH", Body: "missing error handling"},
			Expert:     DomainBackend,
			Confidence: 0.9,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 2 {
		t.Fatalf("expected 2 findings (different files), got %d", len(merged))
	}
}

func TestMergeCouncilFindings_DomainPriority(t *testing.T) {
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "auth.go", Line: 42, Severity: "HIGH", Body: "SQL injection risk in user input"},
			Expert:     DomainBackend,
			Confidence: 0.85,
		},
		{
			Finding:    Finding{File: "auth.go", Line: 42, Severity: "HIGH", Body: "SQL injection vulnerability in user input"},
			Expert:     DomainSecurity,
			Confidence: 0.9,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 1 {
		t.Fatalf("expected 1 finding after domain priority merge, got %d", len(merged))
	}
	if merged[0].Expert != DomainSecurity {
		t.Errorf("expected security domain to win over backend, got %s", merged[0].Expert)
	}
}

func TestMergeCouncilFindings_DomainPriority_Reverse(t *testing.T) {
	// Test bidirectional: order {backend, security} should also resolve to security
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "auth.go", Line: 42, Severity: "HIGH", Body: "SQL injection vulnerability in user input"},
			Expert:     DomainSecurity,
			Confidence: 0.9,
		},
		{
			Finding:    Finding{File: "auth.go", Line: 42, Severity: "HIGH", Body: "SQL injection risk in user input"},
			Expert:     DomainBackend,
			Confidence: 0.85,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 1 {
		t.Fatalf("expected 1 finding after domain priority merge, got %d", len(merged))
	}
	if merged[0].Expert != DomainSecurity {
		t.Errorf("expected security domain to win, got %s", merged[0].Expert)
	}
}

func TestMergeCouncilFindings_SimilarBodySameFile(t *testing.T) {
	// Same file, different lines, similar body (>80% similarity) → merge into one
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "handler.go", Line: 10, Severity: "MEDIUM", Body: "error return value not checked in handler"},
			Expert:     DomainBackend,
			Confidence: 0.8,
		},
		{
			Finding:    Finding{File: "handler.go", Line: 50, Severity: "MEDIUM", Body: "error return value not checked in handler function"},
			Expert:     DomainBackend,
			Confidence: 0.7,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 1 {
		t.Fatalf("expected 1 finding after similar body merge, got %d", len(merged))
	}
	// Should keep earlier line
	if merged[0].Line != 10 {
		t.Errorf("expected earlier line 10, got %d", merged[0].Line)
	}
}

func TestMergeCouncilFindings_AssignsIDs(t *testing.T) {
	findings := []ExpertFinding{
		{
			Finding:    Finding{File: "a.go", Line: 1, Severity: "CRITICAL", Body: "critical bug"},
			Expert:     DomainSecurity,
			Confidence: 0.95,
		},
		{
			Finding:    Finding{File: "b.go", Line: 2, Severity: "HIGH", Body: "high issue"},
			Expert:     DomainBackend,
			Confidence: 0.8,
		},
		{
			Finding:    Finding{File: "c.go", Line: 3, Severity: "MEDIUM", Body: "medium note"},
			Expert:     DomainRefactoring,
			Confidence: 0.6,
		},
	}

	merged := mergeCouncilFindings(findings)

	if len(merged) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(merged))
	}

	for i, f := range merged {
		expected := fmt.Sprintf("#%d", i+1)
		if f.ID != expected {
			t.Errorf("finding %d: expected ID %q, got %q", i, expected, f.ID)
		}
	}

	// Also verify sorted by severity descending
	if merged[0].Severity != "CRITICAL" {
		t.Errorf("expected first finding CRITICAL, got %s", merged[0].Severity)
	}
	if merged[1].Severity != "HIGH" {
		t.Errorf("expected second finding HIGH, got %s", merged[1].Severity)
	}
	if merged[2].Severity != "MEDIUM" {
		t.Errorf("expected third finding MEDIUM, got %s", merged[2].Severity)
	}
}

// --- applyCarmackFilter tests ---

func TestCarmackFilter_ConventionFiltering(t *testing.T) {
	dir := t.TempDir()
	store, err := NewConventionStore(dir)
	if err != nil {
		t.Fatalf("create convention store: %v", err)
	}

	// Add a convention matching error handling pattern
	store.Propose(Convention{
		Domain:   DomainBackend,
		Pattern:  "error return value not checked",
		FileGlob: "*.go",
	})
	// Accept it twice so it becomes active
	store.Accept("conv-001")
	store.Accept("conv-001")

	findings := []ExpertFinding{
		{
			Finding:    Finding{ID: "#1", File: "main.go", Line: 10, Severity: "MEDIUM", Body: "error return value not checked"},
			Expert:     DomainBackend,
			Confidence: 0.8,
		},
		{
			Finding:    Finding{ID: "#2", File: "server.go", Line: 20, Severity: "HIGH", Body: "SQL injection vulnerability"},
			Expert:     DomainSecurity,
			Confidence: 0.9,
		},
	}

	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		ConventionStore: store,
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 finding after convention filtering, got %d", len(filtered))
	}
	if filtered[0].Body != "SQL injection vulnerability" {
		t.Errorf("expected SQL injection finding to survive, got %q", filtered[0].Body)
	}
}

func TestCarmackFilter_ScaleFiltering_Small(t *testing.T) {
	findings := []ExpertFinding{
		{
			Finding: Finding{ID: "#1", File: "main.go", Line: 10, Severity: "MEDIUM", Body: "slow loop"},
			Expert:  DomainPerformance,
		},
		{
			Finding: Finding{ID: "#2", File: "main.go", Line: 20, Severity: "HIGH", Body: "SQL injection"},
			Expert:  DomainSecurity,
		},
		{
			Finding: Finding{ID: "#3", File: "handler.go", Line: 5, Severity: "MEDIUM", Body: "missing test"},
			Expert:  DomainTesting,
		},
	}

	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		ProjectScale: ScaleSmall,
	})

	// MEDIUM performance findings should be removed for small projects
	for _, f := range filtered {
		if f.Severity == "MEDIUM" && f.Expert == DomainPerformance {
			t.Errorf("MEDIUM performance finding should be removed for small projects")
		}
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 findings after small scale filtering, got %d", len(filtered))
	}
}

func TestCarmackFilter_ScaleFiltering_Medium(t *testing.T) {
	var findings []ExpertFinding
	// 2 CRITICAL
	findings = append(findings, ExpertFinding{
		Finding: Finding{ID: "#1", File: "a.go", Line: 1, Severity: "CRITICAL", Body: "critical A"},
		Expert:  DomainSecurity,
	})
	findings = append(findings, ExpertFinding{
		Finding: Finding{ID: "#2", File: "b.go", Line: 2, Severity: "CRITICAL", Body: "critical B"},
		Expert:  DomainSecurity,
	})
	// 2 HIGH
	findings = append(findings, ExpertFinding{
		Finding: Finding{ID: "#3", File: "c.go", Line: 3, Severity: "HIGH", Body: "high A"},
		Expert:  DomainBackend,
	})
	findings = append(findings, ExpertFinding{
		Finding: Finding{ID: "#4", File: "d.go", Line: 4, Severity: "HIGH", Body: "high B"},
		Expert:  DomainBackend,
	})
	// 7 MEDIUM — should be capped at 5
	for i := 0; i < 7; i++ {
		findings = append(findings, ExpertFinding{
			Finding: Finding{
				ID:       fmt.Sprintf("#%d", 5+i),
				File:     fmt.Sprintf("file%d.go", i),
				Line:     i + 10,
				Severity: "MEDIUM",
				Body:     fmt.Sprintf("medium issue %d", i),
			},
			Expert: DomainRefactoring,
		})
	}

	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		ProjectScale: ScaleMedium,
		MaxFindings:  50, // high cap so scale filtering is the constraint
	})

	// Count MEDIUM findings
	mediumCount := 0
	for _, f := range filtered {
		if f.Severity == "MEDIUM" {
			mediumCount++
		}
	}
	if mediumCount > 5 {
		t.Errorf("expected max 5 MEDIUM findings for medium scale, got %d", mediumCount)
	}
	// All CRITICAL and HIGH should survive
	critCount := 0
	highCount := 0
	for _, f := range filtered {
		if f.Severity == "CRITICAL" {
			critCount++
		}
		if f.Severity == "HIGH" {
			highCount++
		}
	}
	if critCount != 2 {
		t.Errorf("expected 2 CRITICAL, got %d", critCount)
	}
	if highCount != 2 {
		t.Errorf("expected 2 HIGH, got %d", highCount)
	}
}

func TestCarmackFilter_PatternCompression(t *testing.T) {
	// 4 findings with same domain and very similar body → compress to 1
	findings := []ExpertFinding{
		{
			Finding: Finding{ID: "#1", File: "a.go", Line: 10, Severity: "MEDIUM", Body: "error return value not checked"},
			Expert:  DomainBackend,
		},
		{
			Finding: Finding{ID: "#2", File: "b.go", Line: 20, Severity: "MEDIUM", Body: "error return value not checked"},
			Expert:  DomainBackend,
		},
		{
			Finding: Finding{ID: "#3", File: "c.go", Line: 30, Severity: "MEDIUM", Body: "error return value not checked"},
			Expert:  DomainBackend,
		},
		{
			Finding: Finding{ID: "#4", File: "d.go", Line: 40, Severity: "MEDIUM", Body: "error return value not checked"},
			Expert:  DomainBackend,
		},
	}

	filtered := applyCarmackFilter(findings, CarmackFilterConfig{})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 finding after pattern compression, got %d", len(filtered))
	}
	if !strings.Contains(filtered[0].Body, "4 occurrences") {
		t.Errorf("expected body to mention 4 occurrences, got %q", filtered[0].Body)
	}
	// Should list all locations
	for _, file := range []string{"a.go:10", "b.go:20", "c.go:30", "d.go:40"} {
		if !strings.Contains(filtered[0].Body, file) {
			t.Errorf("expected body to contain %q, got %q", file, filtered[0].Body)
		}
	}
}

func TestCarmackFilter_MaxFindings(t *testing.T) {
	var findings []ExpertFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, ExpertFinding{
			Finding: Finding{
				ID:       fmt.Sprintf("#%d", i+1),
				File:     fmt.Sprintf("file%d.go", i),
				Line:     i + 1,
				Severity: "HIGH",
				Body:     fmt.Sprintf("unique issue number %d in the codebase", i),
			},
			Expert: ExpertDomain(fmt.Sprintf("domain%d", i%5)), // spread across domains to avoid compression
		})
	}

	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		MaxFindings: 15,
	})

	if len(filtered) > 15 {
		t.Errorf("expected max 15 findings, got %d", len(filtered))
	}
}

func TestCarmackFilter_MaxFindings_DefaultCap(t *testing.T) {
	var findings []ExpertFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, ExpertFinding{
			Finding: Finding{
				ID:       fmt.Sprintf("#%d", i+1),
				File:     fmt.Sprintf("file%d.go", i),
				Line:     i + 1,
				Severity: "HIGH",
				Body:     fmt.Sprintf("unique problem %d with different words each time here", i),
			},
			Expert: ExpertDomain(fmt.Sprintf("domain%d", i%5)),
		})
	}

	// MaxFindings=0 → defaults to 15
	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		MaxFindings: 0,
	})

	if len(filtered) > 15 {
		t.Errorf("expected default max 15 findings, got %d", len(filtered))
	}
}

// --- determineVerdict tests ---

func TestDetermineVerdict_NoFindings(t *testing.T) {
	result := determineVerdict(nil)
	if result != "APPROVED" {
		t.Errorf("expected APPROVED for no findings, got %s", result)
	}
}

func TestDetermineVerdict_OnlyCritical(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "CRITICAL", Body: "critical bug"}},
	}
	result := determineVerdict(findings)
	if result != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED for critical, got %s", result)
	}
}

func TestDetermineVerdict_ThreeHigh(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "HIGH", Body: "issue 1"}},
		{Finding: Finding{Severity: "HIGH", Body: "issue 2"}},
		{Finding: Finding{Severity: "HIGH", Body: "issue 3"}},
	}
	result := determineVerdict(findings)
	if result != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED for 3 HIGH, got %s", result)
	}
}

func TestDetermineVerdict_TwoHigh(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "HIGH", Body: "issue 1"}},
		{Finding: Finding{Severity: "HIGH", Body: "issue 2"}},
	}
	result := determineVerdict(findings)
	if result != "APPROVED" {
		t.Errorf("expected APPROVED for 2 HIGH (with notes), got %s", result)
	}
}

func TestDetermineVerdict_OneHigh(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "HIGH", Body: "issue 1"}},
		{Finding: Finding{Severity: "MEDIUM", Body: "issue 2"}},
	}
	result := determineVerdict(findings)
	if result != "APPROVED" {
		t.Errorf("expected APPROVED for 1 HIGH, got %s", result)
	}
}

func TestDetermineVerdict_OnlyMedium(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "MEDIUM", Body: "issue 1"}},
		{Finding: Finding{Severity: "MEDIUM", Body: "issue 2"}},
	}
	result := determineVerdict(findings)
	if result != "APPROVED" {
		t.Errorf("expected APPROVED for only MEDIUM, got %s", result)
	}
}

// --- detectProjectScale tests ---

func TestDetectProjectScale(t *testing.T) {
	tests := []struct {
		fileCount int
		expected  ProjectScale
	}{
		{10, ScaleSmall},
		{0, ScaleSmall},
		{49, ScaleSmall},
		{50, ScaleMedium},
		{100, ScaleMedium},
		{500, ScaleMedium},
		{501, ScaleLarge},
		{1000, ScaleLarge},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("files=%d", tt.fileCount), func(t *testing.T) {
			result := detectProjectScale(tt.fileCount)
			if result != tt.expected {
				t.Errorf("detectProjectScale(%d) = %s, want %s", tt.fileCount, result, tt.expected)
			}
		})
	}
}

// --- synthesizeWithAI test ---

// synthMockChat implements ai.ChatProvider for synthesis tests.
type synthMockChat struct {
	response string
	err      error
}

func (m *synthMockChat) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	return m.response, m.err
}

func TestSynthesizeWithAI(t *testing.T) {
	mock := &synthMockChat{
		response: "This PR has 2 HIGH issues: a potential race condition in handler.go and a missing nil check in parser.go. Both are worth addressing but not blocking.",
	}

	findings := []ExpertFinding{
		{
			Finding: Finding{File: "handler.go", Line: 10, Severity: "HIGH", Body: "potential race condition"},
			Expert:  DomainConcurrency,
		},
		{
			Finding: Finding{File: "parser.go", Line: 20, Severity: "HIGH", Body: "missing nil check"},
			Expert:  DomainBackend,
		},
	}

	brief := &ContextBrief{
		TaskID:    "task-1",
		ProjectID: "proj-1",
	}

	summary, err := synthesizeWithAI(context.Background(), mock, findings, brief, "", "en")
	if err != nil {
		t.Fatalf("synthesizeWithAI error: %v", err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !strings.Contains(summary, "race condition") {
		t.Errorf("expected summary to contain AI response content, got %q", summary)
	}
}

func TestSynthesizeWithAI_Error(t *testing.T) {
	mock := &synthMockChat{
		err: fmt.Errorf("API error"),
	}

	_, err := synthesizeWithAI(context.Background(), mock, nil, nil, "", "en")
	if err == nil {
		t.Error("expected error from synthesizeWithAI")
	}
}
