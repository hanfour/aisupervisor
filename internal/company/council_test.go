package company

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
)

// councilMockChat is a test double for ai.ChatProvider.
type councilMockChat struct {
	response string
	err      error
	// callCount tracks how many times Chat was called.
	callCount int
}

func (m *councilMockChat) Chat(_ context.Context, _ []ai.ChatMessage) (string, error) {
	m.callCount++
	return m.response, m.err
}

// --- RunCouncil tests ---

func TestCouncilEngine_RunCouncil_Phase0CriticalFail(t *testing.T) {
	engine := &CouncilEngine{
		registry:    NewExpertRegistry(),
		conventions: nil,
		language:    "en",
		reviewCfg:   config.ReviewConfig{},
	}

	// All phase0 checks failed.
	phase0 := &Phase0Report{
		Results: []Phase0Result{
			{Check: Phase0Check{Name: "go-vet", Command: "go vet ./..."}, Passed: false, Output: "vet error"},
			{Check: Phase0Check{Name: "lint", Command: "npm run lint"}, Passed: false, Output: "lint error"},
		},
		AllGreen: false,
	}

	req := CouncilRequest{
		Diff:      "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n-old\n+new",
		DiffLines: 5,
		FileCount: 10,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED, got %s", result.Status)
	}
	if len(result.Findings) == 0 {
		t.Error("expected phase0 findings, got none")
	}
	for _, f := range result.Findings {
		if f.Finding.Source != "phase0" {
			t.Errorf("expected source phase0, got %s", f.Finding.Source)
		}
	}
}

func TestCouncilEngine_RunCouncil_AllGreen(t *testing.T) {
	mockResp := `[{"file":"main.go","line":10,"severity":"MEDIUM","body":"consider renaming var"}]`
	mock := &councilMockChat{response: mockResp}

	engine := &CouncilEngine{
		chatProvider: mock,
		registry:     NewExpertRegistry(),
		conventions:  nil,
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 5},
	}

	phase0 := &Phase0Report{
		Results:  []Phase0Result{{Check: Phase0Check{Name: "go-vet"}, Passed: true}},
		AllGreen: true,
	}

	req := CouncilRequest{
		Diff:      "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n-old\n+new",
		DiffLines: 5,
		FileCount: 10,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", result.Status)
	}
}

func TestCouncilEngine_RunCouncil_CriticalFinding(t *testing.T) {
	mockResp := `[{"file":"main.go","line":5,"severity":"CRITICAL","body":"hardcoded password"}]`
	mock := &councilMockChat{response: mockResp}

	engine := &CouncilEngine{
		chatProvider: mock,
		registry:     NewExpertRegistry(),
		conventions:  nil,
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 5},
	}

	phase0 := &Phase0Report{
		Results:  []Phase0Result{{Check: Phase0Check{Name: "go-vet"}, Passed: true}},
		AllGreen: true,
	}

	req := CouncilRequest{
		Diff:      "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n-old\n+new",
		DiffLines: 5,
		FileCount: 10,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED, got %s", result.Status)
	}

	foundCritical := false
	for _, f := range result.Findings {
		if f.Severity == "CRITICAL" {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Error("expected at least one CRITICAL finding")
	}
}

func TestCouncilEngine_RunCouncil_NoFindings(t *testing.T) {
	mock := &councilMockChat{response: "[]"}

	engine := &CouncilEngine{
		chatProvider: mock,
		registry:     NewExpertRegistry(),
		conventions:  nil,
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 5},
	}

	phase0 := &Phase0Report{
		Results:  []Phase0Result{{Check: Phase0Check{Name: "go-vet"}, Passed: true}},
		AllGreen: true,
	}

	req := CouncilRequest{
		Diff:      "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n-old\n+new",
		DiffLines: 5,
		FileCount: 10,
		Phase0:    phase0,
	}

	result, err := engine.RunCouncil(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", result.Status)
	}
}

// --- parseExpertFindings tests ---

func TestCouncilEngine_ParseExpertResponse_Direct(t *testing.T) {
	raw := `[{"file":"auth.go","line":12,"severity":"HIGH","body":"missing input validation"}]`
	findings, err := parseExpertFindings(raw, DomainSecurity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Expert != DomainSecurity {
		t.Errorf("expected expert %s, got %s", DomainSecurity, findings[0].Expert)
	}
	if findings[0].File != "auth.go" {
		t.Errorf("expected file auth.go, got %s", findings[0].File)
	}
	if findings[0].Severity != "HIGH" {
		t.Errorf("expected severity HIGH, got %s", findings[0].Severity)
	}
}

func TestCouncilEngine_ParseExpertResponse_Markdown(t *testing.T) {
	raw := "Here are my findings:\n\n```json\n" +
		`[{"file":"db.go","line":5,"severity":"CRITICAL","body":"SQL injection"}]` +
		"\n```\n\nPlease fix these."

	findings, err := parseExpertFindings(raw, DomainDatabase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Expert != DomainDatabase {
		t.Errorf("expected expert %s, got %s", DomainDatabase, findings[0].Expert)
	}
	if findings[0].Body != "SQL injection" {
		t.Errorf("expected body 'SQL injection', got %q", findings[0].Body)
	}
}

func TestCouncilEngine_ParseExpertResponse_NoFindings(t *testing.T) {
	raw := "I reviewed the code and found no issues. The implementation looks correct."
	findings, err := parseExpertFindings(raw, DomainSecurity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestCouncilEngine_ParseExpertResponse_Nested(t *testing.T) {
	raw := "After analysis, here are the issues:\n" +
		`[{"file":"handler.go","line":20,"severity":"MEDIUM","body":"missing timeout"},` +
		`{"file":"handler.go","line":30,"severity":"HIGH","body":"goroutine leak"}]` +
		"\nThese should be addressed before merge."

	findings, err := parseExpertFindings(raw, DomainConcurrency)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Expert != DomainConcurrency {
		t.Errorf("expected expert %s, got %s", DomainConcurrency, findings[0].Expert)
	}
	if findings[1].Severity != "HIGH" {
		t.Errorf("expected severity HIGH on 2nd finding, got %s", findings[1].Severity)
	}
}

// --- filterDiffForFiles tests ---

func TestCouncilEngine_FilterDiffForFiles(t *testing.T) {
	fullDiff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,3 +1,3 @@",
		"-old line",
		"+new line",
		"diff --git a/util.go b/util.go",
		"--- a/util.go",
		"+++ b/util.go",
		"@@ -10,3 +10,4 @@",
		" context",
		"+added line",
	}, "\n")

	filtered := filterDiffForFiles(fullDiff, []string{"main.go"})

	if !strings.Contains(filtered, "main.go") {
		t.Error("expected filtered diff to contain main.go")
	}
	if strings.Contains(filtered, "util.go") {
		t.Error("expected filtered diff to NOT contain util.go")
	}
}

func TestCouncilEngine_FilterDiffForFiles_WildcardPassthrough(t *testing.T) {
	fullDiff := "diff --git a/main.go b/main.go\n+++ b/main.go\n"

	// Empty files list should return full diff.
	filtered := filterDiffForFiles(fullDiff, nil)
	if filtered != fullDiff {
		t.Errorf("expected full diff for nil files, got %q", filtered)
	}

	// Wildcard should return full diff.
	filtered = filterDiffForFiles(fullDiff, []string{"*"})
	if filtered != fullDiff {
		t.Errorf("expected full diff for wildcard, got %q", filtered)
	}
}

// --- dispatchExperts tests ---

func TestCouncilEngine_DispatchExperts_SingleFailure(t *testing.T) {
	findings := []Finding{{File: "ok.go", Line: 1, Severity: "MEDIUM", Body: "found something"}}
	findingsJSON, _ := json.Marshal(findings)

	var mu sync.Mutex
	callIndex := 0

	mockProvider := &funcChatProvider{
		chatFn: func(ctx context.Context, msgs []ai.ChatMessage) (string, error) {
			mu.Lock()
			idx := callIndex
			callIndex++
			mu.Unlock()

			// First call fails, subsequent calls succeed.
			if idx == 0 {
				return "", errors.New("api timeout")
			}
			return string(findingsJSON), nil
		},
	}

	engine := &CouncilEngine{
		chatProvider: mockProvider,
		registry:     NewExpertRegistry(),
		language:     "en",
		reviewCfg:    config.ReviewConfig{APIExpertTimeoutS: 2},
	}

	experts := []SelectedExpert{
		{
			Expert: Expert{Domain: DomainSecurity, Name: "sec", SystemPrompt: "security review"},
			Mode:   ExecAPI,
		},
		{
			Expert: Expert{Domain: DomainPerformance, Name: "perf", SystemPrompt: "perf review"},
			Mode:   ExecAPI,
		},
	}

	brief := &ContextBrief{
		TaskID:      "test-task",
		ProjectID:   "test-proj",
		GeneratedAt: time.Now(),
	}

	results, err := engine.dispatchExperts(context.Background(), experts, brief, "diff content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// At least one expert should have returned findings (the second one).
	// The first one failed, so we should have results from at least the successful expert.
	if len(results) == 0 {
		t.Error("expected at least some findings from successful expert, got 0")
	}
}

// funcChatProvider wraps a function to implement ai.ChatProvider for testing.
type funcChatProvider struct {
	chatFn func(ctx context.Context, msgs []ai.ChatMessage) (string, error)
}

func (f *funcChatProvider) Chat(ctx context.Context, msgs []ai.ChatMessage) (string, error) {
	return f.chatFn(ctx, msgs)
}

