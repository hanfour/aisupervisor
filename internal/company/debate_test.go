package company

import (
	"context"
	"sync"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

func TestSelectStrategy(t *testing.T) {
	tests := []struct {
		name      string
		diffLines int
		fileCount int
		want      ReviewStrategy
	}{
		{"tiny change", 10, 1, ReviewLight},
		{"small multi-file", 30, 4, ReviewStandard},
		{"medium change", 100, 5, ReviewStandard},
		{"large change", 400, 10, ReviewDebate},
		{"exactly at threshold", 300, 5, ReviewDebate},
		{"light boundary files", 40, 3, ReviewLight},
		{"light boundary lines", 50, 2, ReviewStandard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectStrategy(tt.diffLines, tt.fileCount, 300, 50, 3)
			if got != tt.want {
				t.Errorf("selectStrategy(%d, %d) = %s, want %s", tt.diffLines, tt.fileCount, got, tt.want)
			}
		})
	}
}

func TestMergeFindings(t *testing.T) {
	all := []Finding{
		{File: "main.go", Line: 10, Severity: "HIGH", Body: "issue A", Source: "impact"},
		{File: "main.go", Line: 10, Severity: "MEDIUM", Body: "issue A dup", Source: "quality"},
		{File: "util.go", Line: 5, Severity: "CRITICAL", Body: "issue B", Source: "impact"},
	}
	merged := mergeFindings(all)
	if len(merged) != 2 {
		t.Fatalf("mergeFindings: got %d, want 2", len(merged))
	}
	for _, f := range merged {
		if f.File == "main.go" && f.Severity != "HIGH" {
			t.Errorf("dedup should keep higher severity: got %s", f.Severity)
		}
	}
	// CRITICAL should be first (higher rank)
	if merged[0].Severity != "CRITICAL" {
		t.Errorf("first finding should be CRITICAL, got %s", merged[0].Severity)
	}
}

func TestTallyVotes(t *testing.T) {
	findings := []Finding{
		{ID: "#1"}, {ID: "#2"}, {ID: "#3"},
	}
	votes1 := map[string]string{"#1": "KEEP", "#2": "DROP", "#3": "KEEP"}
	votes2 := map[string]string{"#1": "KEEP", "#2": "DROP", "#3": "DROP"}

	survived := tallyVotes(findings, votes1, votes2)
	if len(survived) != 1 || survived[0].ID != "#1" {
		t.Errorf("tallyVotes: got %v, want only #1", survived)
	}
}

type mockChatProvider struct {
	responses []string
	callIdx   int
	mu        sync.Mutex
}

func (m *mockChatProvider) Chat(ctx context.Context, msgs []ai.ChatMessage) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callIdx >= len(m.responses) {
		return `{"findings": []}`, nil
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return resp, nil
}

func TestRunDebateFastConverge(t *testing.T) {
	mock := &mockChatProvider{
		responses: []string{
			`{"findings": []}`,
			`{"findings": []}`,
		},
	}
	result, err := runDebate(context.Background(), mock, "diff content", "", "opus", "haiku", "sonnet", 5, "en")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "APPROVED" {
		t.Errorf("0 findings should be APPROVED, got %s", result.Status)
	}
}

func TestRunSinglePassReview(t *testing.T) {
	mock := &mockChatProvider{
		responses: []string{
			`{"status":"APPROVED","summary":"clean code","comments":[]}`,
		},
	}
	result, err := runSinglePassReview(context.Background(), mock, "diff", "", "sonnet", "en", ReviewLight)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "APPROVED" {
		t.Errorf("got %s, want APPROVED", result.Status)
	}
}

func TestDebateFullPipeline(t *testing.T) {
	mock := &mockChatProvider{
		responses: []string{
			// Agent A (impact): finds SQL injection
			`{"findings": [{"file": "main.go", "line": 10, "severity": "HIGH", "body": "SQL injection in user input"}]}`,
			// Agent B (quality): finds same + naming issue
			`{"findings": [{"file": "main.go", "line": 10, "severity": "HIGH", "body": "SQL injection dup"}, {"file": "util.go", "line": 5, "severity": "MEDIUM", "body": "poor naming"}]}`,
			// Voter 1
			`{"#1": "KEEP", "#2": "DROP"}`,
			// Voter 2
			`{"#1": "KEEP", "#2": "DROP"}`,
			// Synthesizer
			`{"status": "CHANGES_REQUESTED", "summary": "SQL injection found", "comments": [{"file": "main.go", "line": 10, "severity": "HIGH", "body": "SQL injection in user input"}]}`,
		},
	}
	// fastConverge=1 forces voting round (merged findings > 1)
	result, err := runDebate(context.Background(), mock, "big diff...", "", "opus", "haiku", "sonnet", 1, "en")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "CHANGES_REQUESTED" {
		t.Errorf("status = %q, want CHANGES_REQUESTED", result.Status)
	}
	if len(result.Comments) == 0 {
		t.Error("expected comments in result")
	}
}

func TestDebateAllApproved(t *testing.T) {
	mock := &mockChatProvider{
		responses: []string{
			// Agent A: one medium finding
			`{"findings": [{"file": "x.go", "line": 1, "severity": "MEDIUM", "body": "minor style"}]}`,
			// Agent B: no findings
			`{"findings": []}`,
			// Synthesizer (fast converge with 1 finding <= 5)
			`{"status": "APPROVED", "summary": "only minor style issue", "comments": [{"file": "x.go", "line": 1, "severity": "MEDIUM", "body": "minor style"}]}`,
		},
	}
	result, err := runDebate(context.Background(), mock, "small diff", "", "opus", "haiku", "sonnet", 5, "zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "APPROVED" {
		t.Errorf("MEDIUM-only should be APPROVED, got %s", result.Status)
	}
}

func TestParseDebateResult(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{"valid approved", `{"status":"APPROVED","summary":"looks good","comments":[]}`, "APPROVED", true},
		{"wrapped in markdown", "```json\n{\"status\":\"CHANGES_REQUESTED\",\"summary\":\"issues\",\"comments\":[{\"file\":\"x.go\",\"severity\":\"HIGH\",\"body\":\"bad\"}]}\n```", "CHANGES_REQUESTED", true},
		{"garbage", "no json here", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDebateResult(tt.input)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.Status != tt.want {
					t.Errorf("status = %q, want %q", result.Status, tt.want)
				}
			} else if err == nil {
				t.Error("expected error for garbage input")
			}
		})
	}
}
