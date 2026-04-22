package company

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhase0_DetectGoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0644); err != nil {
		t.Fatal(err)
	}

	checks := detectChecks(dir, "")
	found := false
	for _, c := range checks {
		if c.Name == "go-vet" {
			found = true
			if c.Command != "go vet ./..." {
				t.Errorf("expected command 'go vet ./...', got %q", c.Command)
			}
			if c.Timeout != 60*time.Second {
				t.Errorf("expected 60s timeout, got %v", c.Timeout)
			}
		}
	}
	if !found {
		t.Error("go-vet check not detected for project with go.mod")
	}
}

func TestPhase0_DetectNodeProject(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]interface{}{
		"scripts": map[string]string{
			"lint":      "eslint .",
			"typecheck": "tsc --noEmit",
		},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	checks := detectChecks(dir, "")

	names := make(map[string]bool)
	for _, c := range checks {
		names[c.Name] = true
	}
	if !names["lint"] {
		t.Error("lint check not detected for project with package.json lint script")
	}
	if !names["typecheck"] {
		t.Error("typecheck check not detected for project with package.json typecheck script")
	}
}

func TestPhase0_VerifyCmdOverride(t *testing.T) {
	dir := t.TempDir()

	checks := detectChecks(dir, "make verify")
	if len(checks) == 0 {
		t.Fatal("expected at least one check with verifyCmd")
	}
	if checks[0].Name != "verify" {
		t.Errorf("expected first check name 'verify', got %q", checks[0].Name)
	}
	if checks[0].Command != "make verify" {
		t.Errorf("expected command 'make verify', got %q", checks[0].Command)
	}
	if checks[0].Timeout != 2*time.Minute {
		t.Errorf("expected 2min timeout for verifyCmd, got %v", checks[0].Timeout)
	}
}

func TestPhase0_RunChecks_AllPass(t *testing.T) {
	checks := []Phase0Check{
		{Name: "echo-ok", Command: "echo ok", Timeout: 5 * time.Second},
		{Name: "true", Command: "true", Timeout: 5 * time.Second},
	}

	report := runPhase0Checks(context.Background(), t.TempDir(), checks)

	if !report.AllGreen {
		t.Error("expected AllGreen=true when all checks pass")
	}
	if len(report.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(report.Results))
	}
	for _, r := range report.Results {
		if !r.Passed {
			t.Errorf("check %q should have passed", r.Check.Name)
		}
	}
}

func TestPhase0_RunChecks_OneFails(t *testing.T) {
	checks := []Phase0Check{
		{Name: "pass", Command: "echo ok", Timeout: 5 * time.Second},
		{Name: "fail", Command: "exit 1", Timeout: 5 * time.Second},
	}

	report := runPhase0Checks(context.Background(), t.TempDir(), checks)

	if report.AllGreen {
		t.Error("expected AllGreen=false when one check fails")
	}

	passCount := 0
	failCount := 0
	for _, r := range report.Results {
		if r.Passed {
			passCount++
		} else {
			failCount++
		}
	}
	if passCount != 1 || failCount != 1 {
		t.Errorf("expected 1 pass and 1 fail, got %d pass and %d fail", passCount, failCount)
	}
}

func TestPhase0_Timeout(t *testing.T) {
	checks := []Phase0Check{
		{Name: "slow", Command: "sleep 30", Timeout: 100 * time.Millisecond},
	}

	report := runPhase0Checks(context.Background(), t.TempDir(), checks)

	if report.AllGreen {
		t.Error("expected AllGreen=false for timed-out check")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Passed {
		t.Error("timed-out check should not pass")
	}
}

func TestPhase0_OutputTruncation(t *testing.T) {
	checks := []Phase0Check{
		{Name: "big-output", Command: "seq 1 5000", Timeout: 10 * time.Second},
	}

	report := runPhase0Checks(context.Background(), t.TempDir(), checks)

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	output := report.Results[0].Output
	if len(output) > 2200 {
		t.Errorf("output should be truncated to ~2100 chars, got %d chars", len(output))
	}
	if !strings.Contains(output, "... (truncated) ...") {
		t.Error("truncated output should contain '... (truncated) ...' marker")
	}
}

func TestPhase0_AllCriticalFailed(t *testing.T) {
	tests := []struct {
		name     string
		results  []Phase0Result
		expected bool
	}{
		{
			name: "all failed",
			results: []Phase0Result{
				{Passed: false},
				{Passed: false},
			},
			expected: true,
		},
		{
			name: "one passed",
			results: []Phase0Result{
				{Passed: true},
				{Passed: false},
			},
			expected: false,
		},
		{
			name: "all passed",
			results: []Phase0Result{
				{Passed: true},
				{Passed: true},
			},
			expected: false,
		},
		{
			name:     "empty results",
			results:  []Phase0Result{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Phase0Report{Results: tt.results}
			if got := report.AllCriticalFailed(); got != tt.expected {
				t.Errorf("AllCriticalFailed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPhase0_ToFindings(t *testing.T) {
	report := &Phase0Report{
		Results: []Phase0Result{
			{
				Check:  Phase0Check{Name: "go-vet", Command: "go vet ./..."},
				Passed: false,
				Output: "vet: some error found",
			},
			{
				Check:  Phase0Check{Name: "lint", Command: "npm run lint"},
				Passed: true,
				Output: "all good",
			},
			{
				Check:  Phase0Check{Name: "typecheck", Command: "tsc --noEmit"},
				Passed: false,
				Output: "type error on line 42",
			},
		},
	}

	findings := report.ToFindings()

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (only failed checks), got %d", len(findings))
	}

	for _, f := range findings {
		if f.Severity != "CRITICAL" {
			t.Errorf("expected severity CRITICAL, got %q", f.Severity)
		}
		if f.Source != "phase0" {
			t.Errorf("expected source 'phase0', got %q", f.Source)
		}
	}

	// Verify specific findings
	if findings[0].Body == "" {
		t.Error("finding body should not be empty")
	}
	if !strings.Contains(findings[0].Body, "go-vet") {
		t.Error("finding body should reference the check name")
	}
}
