# Council Review Engine Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the dual-agent debate review pipeline with a Carmack-Council-inspired multi-expert council system featuring Phase 0 automated checks, unified Context Brief, dynamic expert selection, parallel dispatch, Carmack Filter synthesis, and cross-session convention learning.

**Architecture:** New review pipeline runs as 6 sequential phases inside `internal/company/`. Each phase is a standalone `.go` file with its own test file. The council engine replaces `runChatReview` as the primary review path, with the existing debate system preserved as fallback. `CouncilResult` is converted to `DebateResult` format so `handleDebateResult` is reused unchanged.

**Tech Stack:** Go 1.23, `sync/errgroup` for parallel dispatch, `ai.ChatProvider` for API-mode experts, `tmux.TmuxClient` for CLI-mode experts, `gopkg.in/yaml.v3` for conventions persistence.

---

## Task 1: Events & Config Extension

**Files:**
- Modify: `internal/company/events.go:66` (add new constants before closing paren)
- Modify: `internal/config/config.go:48-56` (extend ReviewConfig)
- Test: `internal/company/events_test.go` (new)
- Test: `internal/config/config_test.go` (if exists, else new)

**Step 1: Add event type constants**

In `internal/company/events.go`, add before line 66 (before the closing `)`):

```go
	EventPhase0Completed       EventType = "phase0_completed"
	EventCouncilStarted        EventType = "council_started"
	EventExpertCompleted       EventType = "expert_completed"
	EventCouncilSynthesized    EventType = "council_synthesized"
	EventConventionProposed    EventType = "convention_proposed"
	EventConventionAccepted    EventType = "convention_accepted"
	EventConventionDecayed     EventType = "convention_decayed"
```

**Step 2: Extend ReviewConfig**

In `internal/config/config.go`, replace the `ReviewConfig` struct (lines 48-56) with:

```go
type ReviewConfig struct {
	AnalysisModel       string `yaml:"analysis_model,omitempty"`
	VoteModel           string `yaml:"vote_model,omitempty"`
	SynthesisModel      string `yaml:"synthesis_model,omitempty"`
	DebateThreshold     int    `yaml:"debate_threshold,omitempty"`
	LightMaxLines       int    `yaml:"light_max_lines,omitempty"`
	LightMaxFiles       int    `yaml:"light_max_files,omitempty"`
	FastConverge        int    `yaml:"fast_converge,omitempty"`
	CouncilEnabled      bool   `yaml:"council_enabled,omitempty"`
	MaxExperts          int    `yaml:"max_experts,omitempty"`
	Phase0Enabled       bool   `yaml:"phase0_enabled,omitempty"`
	CarmackFilterScale  string `yaml:"carmack_filter_scale,omitempty"`
	CLIExpertTimeoutS   int    `yaml:"cli_expert_timeout_s,omitempty"`
	APIExpertTimeoutS   int    `yaml:"api_expert_timeout_s,omitempty"`
	ConventionDecayDays int    `yaml:"convention_decay_days,omitempty"`
}
```

Update `reviewConfigWithDefaults` in `review.go` (find the existing function) to add defaults:

```go
if cfg.CouncilEnabled == false && cfg.MaxExperts == 0 {
	cfg.CouncilEnabled = true // default on
}
if cfg.MaxExperts == 0 {
	cfg.MaxExperts = 5
}
if cfg.Phase0Enabled == false && cfg.CLIExpertTimeoutS == 0 {
	cfg.Phase0Enabled = true
}
if cfg.CLIExpertTimeoutS == 0 {
	cfg.CLIExpertTimeoutS = 300
}
if cfg.APIExpertTimeoutS == 0 {
	cfg.APIExpertTimeoutS = 60
}
if cfg.ConventionDecayDays == 0 {
	cfg.ConventionDecayDays = 30
}
if cfg.CarmackFilterScale == "" {
	cfg.CarmackFilterScale = "auto"
}
```

**Step 3: Write test for config defaults**

```go
// internal/config/council_config_test.go
func TestReviewConfigDefaults(t *testing.T) {
	cfg := ReviewConfig{}
	// verify zero values are as expected
	assert CouncilEnabled is false (will be defaulted at runtime)
	assert MaxExperts is 0 (will be defaulted at runtime)
}
```

**Step 4: Run tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./...`
Expected: BUILD SUCCESS

**Step 5: Commit**

```bash
git add internal/company/events.go internal/config/config.go
git commit -m "feat(council): add event types and extend ReviewConfig for council pipeline"
```

---

## Task 2: Phase 0 — Automated Pre-Review Checks

**Files:**
- Create: `internal/company/phase0.go`
- Create: `internal/company/phase0_test.go`

**Step 1: Write failing tests**

```go
// internal/company/phase0_test.go
package company

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPhase0_DetectGoProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	checks := detectChecks(dir, "")
	found := false
	for _, c := range checks {
		if c.Name == "go-vet" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected go-vet check for Go project")
	}
}

func TestPhase0_DetectNodeProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"lint":"eslint ."}}`), 0644)
	checks := detectChecks(dir, "")
	found := false
	for _, c := range checks {
		if c.Name == "lint" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected lint check for Node project")
	}
}

func TestPhase0_VerifyCmdOverride(t *testing.T) {
	dir := t.TempDir()
	checks := detectChecks(dir, "make test")
	if len(checks) == 0 {
		t.Fatal("expected at least one check from VerifyCmd")
	}
	if checks[0].Command != "make test" {
		t.Fatalf("expected 'make test', got %q", checks[0].Command)
	}
}

func TestPhase0_RunChecks_AllPass(t *testing.T) {
	checks := []Phase0Check{
		{Name: "echo-pass", Command: "echo ok", Timeout: 5 * time.Second},
	}
	report := runPhase0Checks(context.Background(), t.TempDir(), checks)
	if !report.AllGreen {
		t.Fatal("expected all green")
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
}

func TestPhase0_RunChecks_OneFails(t *testing.T) {
	checks := []Phase0Check{
		{Name: "pass", Command: "echo ok", Timeout: 5 * time.Second},
		{Name: "fail", Command: "exit 1", Timeout: 5 * time.Second},
	}
	report := runPhase0Checks(context.Background(), t.TempDir(), checks)
	if report.AllGreen {
		t.Fatal("expected not all green")
	}
}

func TestPhase0_Timeout(t *testing.T) {
	checks := []Phase0Check{
		{Name: "slow", Command: "sleep 30", Timeout: 100 * time.Millisecond},
	}
	report := runPhase0Checks(context.Background(), t.TempDir(), checks)
	if report.AllGreen {
		t.Fatal("expected timeout failure")
	}
}

func TestPhase0_OutputTruncation(t *testing.T) {
	// Generate output > 2000 chars
	checks := []Phase0Check{
		{Name: "verbose", Command: "seq 1 5000", Timeout: 5 * time.Second},
	}
	report := runPhase0Checks(context.Background(), t.TempDir(), checks)
	for _, r := range report.Results {
		if len(r.Output) > 2100 {
			t.Fatalf("output should be truncated to ~2000 chars, got %d", len(r.Output))
		}
	}
}

func TestPhase0_AllCriticalFailed(t *testing.T) {
	report := &Phase0Report{
		Results: []Phase0Result{
			{Check: Phase0Check{Name: "a"}, Passed: false},
			{Check: Phase0Check{Name: "b"}, Passed: false},
		},
	}
	if !report.AllCriticalFailed() {
		t.Fatal("all checks failed, should return true")
	}
	report.Results[0].Passed = true
	if report.AllCriticalFailed() {
		t.Fatal("one check passed, should return false")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run TestPhase0 -v`
Expected: FAIL — `Phase0Check` type not defined

**Step 3: Write implementation**

```go
// internal/company/phase0.go
package company

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Phase0Check represents a single mechanical verification check.
type Phase0Check struct {
	Name    string
	Command string
	Timeout time.Duration
}

// Phase0Result holds the outcome of a single check.
type Phase0Result struct {
	Check   Phase0Check
	Passed  bool
	Output  string
	Elapsed time.Duration
}

// Phase0Report aggregates all check results.
type Phase0Report struct {
	Results  []Phase0Result
	AllGreen bool
	Summary  string
}

// AllCriticalFailed returns true if every check in the report failed.
func (r *Phase0Report) AllCriticalFailed() bool {
	if len(r.Results) == 0 {
		return false
	}
	for _, res := range r.Results {
		if res.Passed {
			return false
		}
	}
	return true
}

// ToFindings converts failed checks into Finding structs for injection into the review pipeline.
func (r *Phase0Report) ToFindings() []Finding {
	var findings []Finding
	for _, res := range r.Results {
		if !res.Passed {
			findings = append(findings, Finding{
				File:     "",
				Severity: "CRITICAL",
				Body:     fmt.Sprintf("[%s] failed: %s", res.Check.Name, truncateOutput(res.Output, 500)),
				Source:   "phase0",
			})
		}
	}
	return findings
}

// detectChecks auto-detects which checks to run based on project files.
// If verifyCmd is non-empty, it is added as the primary check.
func detectChecks(dir, verifyCmd string) []Phase0Check {
	var checks []Phase0Check
	defaultTimeout := 60 * time.Second

	if verifyCmd != "" {
		checks = append(checks, Phase0Check{
			Name:    "verify",
			Command: verifyCmd,
			Timeout: 2 * time.Minute,
		})
	}

	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		checks = append(checks, Phase0Check{
			Name:    "go-vet",
			Command: "go vet ./...",
			Timeout: defaultTimeout,
		})
	}

	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		data, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err == nil {
			var pkg map[string]json.RawMessage
			if json.Unmarshal(data, &pkg) == nil {
				if scripts, ok := pkg["scripts"]; ok {
					var scriptMap map[string]string
					if json.Unmarshal(scripts, &scriptMap) == nil {
						if _, ok := scriptMap["lint"]; ok {
							checks = append(checks, Phase0Check{
								Name:    "lint",
								Command: "npm run lint",
								Timeout: defaultTimeout,
							})
						}
						if _, ok := scriptMap["typecheck"]; ok {
							checks = append(checks, Phase0Check{
								Name:    "typecheck",
								Command: "npm run typecheck",
								Timeout: defaultTimeout,
							})
						}
					}
				}
			}
		}
	}

	return checks
}

// runPhase0Checks executes all checks in parallel and returns a report.
func runPhase0Checks(ctx context.Context, workDir string, checks []Phase0Check) *Phase0Report {
	if len(checks) == 0 {
		return &Phase0Report{AllGreen: true, Summary: "no checks configured"}
	}

	results := make([]Phase0Result, len(checks))
	var wg sync.WaitGroup

	for i, check := range checks {
		wg.Add(1)
		go func(idx int, c Phase0Check) {
			defer wg.Done()
			results[idx] = executeCheck(ctx, workDir, c)
		}(i, check)
	}
	wg.Wait()

	allGreen := true
	var summaryParts []string
	for _, r := range results {
		if !r.Passed {
			allGreen = false
			summaryParts = append(summaryParts, fmt.Sprintf("%s: FAIL (%s)", r.Check.Name, r.Elapsed.Round(time.Millisecond)))
		} else {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: PASS (%s)", r.Check.Name, r.Elapsed.Round(time.Millisecond)))
		}
	}

	return &Phase0Report{
		Results:  results,
		AllGreen: allGreen,
		Summary:  strings.Join(summaryParts, "; "),
	}
}

func executeCheck(ctx context.Context, workDir string, check Phase0Check) Phase0Result {
	start := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, "sh", "-c", check.Command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	output := truncateOutput(string(out), 2000)
	passed := err == nil

	return Phase0Result{
		Check:   check,
		Passed:  passed,
		Output:  output,
		Elapsed: elapsed,
	}
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	half := maxLen / 2
	return s[:half] + "\n... (truncated) ...\n" + s[len(s)-half:]
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/company/ -run TestPhase0 -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/phase0.go internal/company/phase0_test.go
git commit -m "feat(council): add Phase 0 automated pre-review checks"
```

---

## Task 3: Context Brief System

**Files:**
- Create: `internal/company/context_brief.go`
- Create: `internal/company/context_brief_test.go`

**Step 1: Write failing tests**

```go
// internal/company/context_brief_test.go
package company

import (
	"testing"
	"time"
)

func TestBriefSection_Truncate(t *testing.T) {
	s := BriefSection{Name: "test", Content: strings.Repeat("x", 10000), TokenBudget: 100, Priority: 1}
	s.truncate()
	// ~100 tokens ≈ 400 chars
	if len(s.Content) > 500 {
		t.Fatalf("expected truncation to ~400 chars, got %d", len(s.Content))
	}
}

func TestContextBriefBuilder_Build_BasicSections(t *testing.T) {
	b := &ContextBriefBuilder{
		totalBudget: 6000,
		taskID:      "task-1",
		projectID:   "proj-1",
		projectName: "test-project",
		techStack:   "Go",
		baseBranch:  "main",
		diffStats:   "10 files, 200 lines",
		changedFiles: []string{"main.go", "handler.go"},
	}
	brief, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if brief.TaskID != "task-1" {
		t.Fatalf("expected task-1, got %s", brief.TaskID)
	}
	if len(brief.Sections) == 0 {
		t.Fatal("expected at least one section")
	}
}

func TestContextBriefBuilder_Build_PriorityTruncation(t *testing.T) {
	b := &ContextBriefBuilder{
		totalBudget: 500, // very small budget
		taskID:      "task-1",
		projectID:   "proj-1",
		projectName: "test",
		techStack:   "Go",
		baseBranch:  "main",
		karpathyOverlay:  strings.Repeat("guideline\n", 500),
		rejectionHistory: strings.Repeat("rejection\n", 500),
	}
	brief, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	rendered := brief.Render()
	// Total should be within budget (500 tokens ≈ 2000 chars)
	if len(rendered) > 2500 {
		t.Fatalf("rendered brief too long: %d chars", len(rendered))
	}
}

func TestContextBrief_Render(t *testing.T) {
	brief := &ContextBrief{
		TaskID:    "t1",
		ProjectID: "p1",
		Sections: []BriefSection{
			{Name: "project_summary", Content: "Test project"},
			{Name: "diff_stats", Content: "5 files changed"},
		},
	}
	rendered := brief.Render()
	if !strings.Contains(rendered, "Test project") {
		t.Fatal("rendered brief should contain project summary")
	}
	if !strings.Contains(rendered, "5 files changed") {
		t.Fatal("rendered brief should contain diff stats")
	}
}

func TestContextBrief_WriteFile(t *testing.T) {
	dir := t.TempDir()
	brief := &ContextBrief{
		TaskID:    "t1",
		ProjectID: "p1",
		Sections: []BriefSection{
			{Name: "test", Content: "hello"},
		},
	}
	path, err := brief.WriteToDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatal("file should contain brief content")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run TestBrief -v && go test ./internal/company/ -run TestContextBrief -v`
Expected: FAIL

**Step 3: Write implementation**

```go
// internal/company/context_brief.go
package company

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const charsPerToken = 4

type ContextBrief struct {
	TaskID      string
	ProjectID   string
	GeneratedAt time.Time
	Sections    []BriefSection
	FilePath    string
}

type BriefSection struct {
	Name        string
	Content     string
	TokenBudget int
	Priority    int // 1 = never truncate, 4 = truncate first
}

func (s *BriefSection) truncate() {
	maxChars := s.TokenBudget * charsPerToken
	if len(s.Content) <= maxChars {
		return
	}
	half := maxChars / 2
	s.Content = s.Content[:half] + "\n... (truncated) ...\n" + s.Content[len(s.Content)-half:]
}

func (b *ContextBrief) Render() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Context Brief\n\nTask: %s | Project: %s | Generated: %s\n\n",
		b.TaskID, b.ProjectID, b.GeneratedAt.Format(time.RFC3339)))
	for _, sec := range b.Sections {
		if sec.Content == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", sectionTitle(sec.Name), sec.Content))
	}
	return sb.String()
}

func (b *ContextBrief) WriteToDir(dir string) (string, error) {
	reviewDir := filepath.Join(dir, ".ais-review")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(reviewDir, "context-brief.md")
	content := b.Render()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	b.FilePath = path
	return path, nil
}

func sectionTitle(name string) string {
	titles := map[string]string{
		"project_summary":    "Project Summary",
		"diff_stats":         "Diff Statistics",
		"architecture":       "Architecture Context",
		"phase0_results":     "Phase 0 Results",
		"conventions":        "Project Conventions",
		"graph_context":      "Knowledge Graph Context",
		"rejection_history":  "Rejection History",
		"karpathy_overlay":   "Behavioral Guidelines",
	}
	if t, ok := titles[name]; ok {
		return t
	}
	return name
}

type ContextBriefBuilder struct {
	totalBudget      int
	taskID           string
	projectID        string
	projectName      string
	techStack        string
	baseBranch       string
	diffStats        string
	changedFiles     []string
	architectureCtx  string
	phase0Summary    string
	conventions      string
	graphContext      string
	rejectionHistory string
	karpathyOverlay  string
}

func (b *ContextBriefBuilder) Build() (*ContextBrief, error) {
	if b.totalBudget <= 0 {
		b.totalBudget = 6000
	}

	sections := []BriefSection{
		{Name: "project_summary", Priority: 1, TokenBudget: 500,
			Content: b.buildProjectSummary()},
		{Name: "diff_stats", Priority: 1, TokenBudget: 200,
			Content: b.diffStats},
		{Name: "architecture", Priority: 2, TokenBudget: 1500,
			Content: b.architectureCtx},
		{Name: "phase0_results", Priority: 1, TokenBudget: 800,
			Content: b.phase0Summary},
		{Name: "conventions", Priority: 3, TokenBudget: 1000,
			Content: b.conventions},
		{Name: "rejection_history", Priority: 4, TokenBudget: 500,
			Content: b.rejectionHistory},
		{Name: "karpathy_overlay", Priority: 4, TokenBudget: 500,
			Content: b.karpathyOverlay},
	}

	// Step 1: truncate each section to its own budget
	for i := range sections {
		sections[i].truncate()
	}

	// Step 2: if total exceeds budget, compress low-priority sections
	b.enforceGlobalBudget(sections)

	brief := &ContextBrief{
		TaskID:      b.taskID,
		ProjectID:   b.projectID,
		GeneratedAt: time.Now(),
		Sections:    sections,
	}
	return brief, nil
}

func (b *ContextBriefBuilder) buildProjectSummary() string {
	parts := []string{}
	if b.projectName != "" {
		parts = append(parts, fmt.Sprintf("Project: %s", b.projectName))
	}
	if b.techStack != "" {
		parts = append(parts, fmt.Sprintf("Stack: %s", b.techStack))
	}
	if b.baseBranch != "" {
		parts = append(parts, fmt.Sprintf("Base: %s", b.baseBranch))
	}
	if len(b.changedFiles) > 0 {
		parts = append(parts, fmt.Sprintf("Changed files: %s", strings.Join(b.changedFiles, ", ")))
	}
	return strings.Join(parts, "\n")
}

func (b *ContextBriefBuilder) enforceGlobalBudget(sections []BriefSection) {
	totalChars := 0
	for _, s := range sections {
		totalChars += len(s.Content)
	}
	maxChars := b.totalBudget * charsPerToken
	if totalChars <= maxChars {
		return
	}

	// Compress from highest priority number (lowest importance) first
	for pri := 4; pri >= 2; pri-- {
		if totalChars <= maxChars {
			break
		}
		for i := range sections {
			if sections[i].Priority != pri || sections[i].Content == "" {
				continue
			}
			excess := totalChars - maxChars
			currentLen := len(sections[i].Content)
			if currentLen <= 100 {
				continue
			}
			targetLen := currentLen - excess
			if targetLen < 100 {
				targetLen = 100
			}
			half := targetLen / 2
			old := sections[i].Content
			sections[i].Content = old[:half] + "\n... (budget truncated) ...\n" + old[len(old)-half:]
			totalChars -= (currentLen - len(sections[i].Content))
		}
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/company/ -run "TestBrief|TestContextBrief" -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/context_brief.go internal/company/context_brief_test.go
git commit -m "feat(council): add Context Brief system with token budget management"
```

---

## Task 4: Expert Registry & Dynamic Selection

**Files:**
- Create: `internal/company/expert.go`
- Create: `internal/company/expert_test.go`

**Step 1: Write failing tests**

```go
// internal/company/expert_test.go
package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

func TestExpertRegistry_DefaultExperts(t *testing.T) {
	reg := NewExpertRegistry()
	if len(reg.experts) != 10 {
		t.Fatalf("expected 10 default experts, got %d", len(reg.experts))
	}
}

func TestSelectExperts_GoFiles(t *testing.T) {
	reg := NewExpertRegistry()
	selected := reg.SelectExperts(
		[]string{"internal/company/review.go", "internal/company/debate.go"},
		"func handleReview() { sync.Mutex }",
		nil, nil,
	)
	if len(selected) < 2 {
		t.Fatalf("expected at least 2 experts, got %d", len(selected))
	}
	hasConcurrency := false
	for _, s := range selected {
		if s.Domain == DomainConcurrency {
			hasConcurrency = true
		}
	}
	if !hasConcurrency {
		t.Fatal("expected concurrency expert due to sync.Mutex keyword")
	}
}

func TestSelectExperts_FrontendFiles(t *testing.T) {
	reg := NewExpertRegistry()
	selected := reg.SelectExperts(
		[]string{"frontend/src/App.svelte", "frontend/src/stores/i18n.js"},
		"<script> bind:value dispatch </script>",
		nil, nil,
	)
	hasFrontend := false
	for _, s := range selected {
		if s.Domain == DomainFrontend {
			hasFrontend = true
		}
	}
	if !hasFrontend {
		t.Fatal("expected frontend expert for .svelte files")
	}
}

func TestSelectExperts_MinimumTwo(t *testing.T) {
	reg := NewExpertRegistry()
	selected := reg.SelectExperts(
		[]string{"README.md"},
		"just a readme change",
		nil, nil,
	)
	if len(selected) < 2 {
		t.Fatalf("expected minimum 2 experts, got %d", len(selected))
	}
}

func TestSelectExperts_MaxFive(t *testing.T) {
	reg := NewExpertRegistry()
	// Trigger many domains
	selected := reg.SelectExperts(
		[]string{"main.go", "handler.go", "db.sql", "app.svelte", "Dockerfile", "test_test.go"},
		"sync.Mutex SELECT password goroutine bind:value mock assert crypto exec",
		nil, nil,
	)
	if len(selected) > 5 {
		t.Fatalf("expected max 5 experts, got %d", len(selected))
	}
}

func TestSelectExperts_GodNodeForceArchitecture(t *testing.T) {
	reg := NewExpertRegistry()
	graph := &knowledge.CodeGraph{
		GodNodes: []string{"internal/company/company.go"},
	}
	selected := reg.SelectExperts(
		[]string{"internal/company/company.go"},
		"minor change",
		graph, nil,
	)
	hasArch := false
	for _, s := range selected {
		if s.Domain == DomainArchitecture {
			hasArch = true
		}
	}
	if !hasArch {
		t.Fatal("expected architecture expert when god node is touched")
	}
}

func TestSelectExperts_CrossCommunityForceArchitecture(t *testing.T) {
	reg := NewExpertRegistry()
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 1, Files: []string{"a.go"}},
			{ID: 2, Files: []string{"b.go"}},
			{ID: 3, Files: []string{"c.go"}},
		},
	}
	selected := reg.SelectExperts(
		[]string{"a.go", "b.go", "c.go"},
		"cross-cutting change",
		graph, nil,
	)
	hasArch := false
	for _, s := range selected {
		if s.Domain == DomainArchitecture {
			hasArch = true
		}
	}
	if !hasArch {
		t.Fatal("expected architecture expert for 3+ communities")
	}
}

func TestSelectExperts_Phase0FailureForcesExpert(t *testing.T) {
	reg := NewExpertRegistry()
	phase0 := &Phase0Report{
		Results: []Phase0Result{
			{Check: Phase0Check{Name: "go-vet"}, Passed: false},
		},
	}
	selected := reg.SelectExperts(
		[]string{"main.go"},
		"some code",
		nil, phase0,
	)
	hasBackend := false
	for _, s := range selected {
		if s.Domain == DomainBackend || s.Domain == DomainRefactoring {
			hasBackend = true
		}
	}
	if !hasBackend {
		t.Fatal("expected backend/refactoring expert when go-vet fails")
	}
}

func TestSelectExperts_ExecMode(t *testing.T) {
	reg := NewExpertRegistry()
	selected := reg.SelectExperts(
		[]string{"a.go", "b.go"},
		"small change",
		nil, nil,
	)
	for _, s := range selected {
		// With only 2 files and no large diff indicator, should be API mode
		if s.Mode != ExecAPI {
			t.Fatalf("expected API mode for small change, got %s for %s", s.Mode, s.Domain)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run TestExpert -v && go test ./internal/company/ -run TestSelect -v`
Expected: FAIL

**Step 3: Write implementation**

Create `internal/company/expert.go` with:
- `ExpertDomain` type and 10 domain constants
- `Expert` struct with `Domain, Name, SystemPrompt, Severity, FilePatterns, Keywords, Model`
- `ExecMode` type with `ExecAPI`, `ExecCLI` constants
- `SelectedExpert` struct embedding `Expert` + `AssignedFiles, Reason, Mode`
- `ExpertRegistry` struct with `experts []Expert`
- `NewExpertRegistry()` — populates 10 default experts with system prompts, patterns, keywords
- `SelectExperts(changedFiles, diff, graph, phase0)` — implements selection rules:
  1. Score each expert by file pattern matches + keyword matches
  2. Force-add experts for Phase 0 failures, god nodes, cross-community
  3. Sort by score descending, take min 2 / max 5
  4. Assign `ExecMode` based on file count per expert and total diff size
  5. Assign `AssignedFiles` per expert (files matching their patterns)

Each expert's `SystemPrompt` should be a focused review instruction (10-20 lines) telling the expert what to look for in their domain and how to format findings as JSON.

**Step 4: Run tests**

Run: `go test ./internal/company/ -run "TestExpert|TestSelect" -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/expert.go internal/company/expert_test.go
git commit -m "feat(council): add Expert Registry with dynamic selection algorithm"
```

---

## Task 5: Conventions Learning System

**Files:**
- Create: `internal/company/conventions.go`
- Create: `internal/company/conventions_test.go`

**Step 1: Write failing tests**

```go
// internal/company/conventions_test.go
package company

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConventionStore_NewEmpty(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.conventions) != 0 {
		t.Fatal("expected empty store")
	}
}

func TestConventionStore_ProposeAndSave(t *testing.T) {
	dir := t.TempDir()
	cs, err := NewConventionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml.Unmarshal error ignored",
		Description: "Config loading allows silent YAML errors",
		FileGlob:    "internal/config/*.go",
		Source:      "review:task-123",
	})
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	if err := cs.Save(); err != nil {
		t.Fatal(err)
	}
	// Verify files written
	if _, err := os.Stat(filepath.Join(dir, "conventions", "index.yaml")); err != nil {
		t.Fatal("index.yaml not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "conventions", "conventions.md")); err != nil {
		t.Fatal("conventions.md not created")
	}
}

func TestConventionStore_LoadExisting(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{
		Domain:  DomainSecurity,
		Pattern: "test pattern",
	})
	cs.Save()

	cs2, err := NewConventionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs2.conventions) != 1 {
		t.Fatalf("expected 1 convention after reload, got %d", len(cs2.conventions))
	}
}

func TestConventionStore_Accept(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	id := cs.Propose(Convention{Domain: DomainBackend, Pattern: "test"})
	cs.Accept(id)
	for _, c := range cs.conventions {
		if c.ID == id && c.AcceptCount != 1 {
			t.Fatalf("expected AcceptCount 1, got %d", c.AcceptCount)
		}
	}
}

func TestConventionStore_FindRelevant(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{Domain: DomainBackend, Pattern: "error ignored", FileGlob: "*.go"})
	cs.Propose(Convention{Domain: DomainFrontend, Pattern: "no typescript", FileGlob: "*.svelte"})

	relevant := cs.FindRelevant(DomainBackend, "main.go")
	if len(relevant) != 1 {
		t.Fatalf("expected 1 relevant, got %d", len(relevant))
	}
}

func TestConventionStore_MatchesFinding(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml.Unmarshal error ignored",
		AcceptCount: 2, // already accepted
	})

	match := cs.MatchesFinding(ExpertFinding{
		Finding: Finding{Body: "yaml.Unmarshal return value is discarded"},
		Expert:  DomainBackend,
	})
	if match == nil {
		t.Fatal("expected match for similar pattern")
	}
}

func TestConventionStore_MatchesFinding_NoMatch(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{Domain: DomainBackend, Pattern: "yaml error", AcceptCount: 2})

	match := cs.MatchesFinding(ExpertFinding{
		Finding: Finding{Body: "SQL injection vulnerability"},
		Expert:  DomainSecurity,
	})
	if match != nil {
		t.Fatal("expected no match for unrelated finding")
	}
}

func TestConventionStore_Decay(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "old pattern",
		AcceptCount: 1,
	})
	// Manually set LastUsed to old date
	cs.conventions[0].LastUsed = time.Now().Add(-60 * 24 * time.Hour)

	removed := cs.Decay(30*24*time.Hour, 5)
	if removed != 1 {
		t.Fatalf("expected 1 decayed, got %d", removed)
	}
	if len(cs.conventions) != 0 {
		t.Fatal("expected empty after decay")
	}
}

func TestConventionStore_Decay_KeepsFrequentlyUsed(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "frequently used",
		AcceptCount: 10,
	})
	cs.conventions[0].LastUsed = time.Now().Add(-60 * 24 * time.Hour)

	removed := cs.Decay(30*24*time.Hour, 5)
	if removed != 0 {
		t.Fatalf("expected 0 decayed (high accept count), got %d", removed)
	}
}

func TestConventionStore_MarkdownRender(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml error ignored",
		Description: "Config allows silent YAML errors",
		FileGlob:    "internal/config/*.go",
		Source:      "review:task-123",
	})
	cs.Save()

	data, _ := os.ReadFile(filepath.Join(dir, "conventions", "conventions.md"))
	content := string(data)
	if !strings.Contains(content, "yaml error ignored") {
		t.Fatal("markdown should contain pattern")
	}
	if !strings.Contains(content, "backend") {
		t.Fatal("markdown should contain domain")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run TestConvention -v`
Expected: FAIL

**Step 3: Write implementation**

Create `internal/company/conventions.go` with:
- `Convention` struct (ID, Domain, Pattern, Description, FileGlob, Source, AcceptedAt, AcceptCount, LastUsed)
- `ConventionStore` struct (mu sync.RWMutex, conventions, indexPath, mdPath, idSeq)
- `NewConventionStore(dataDir)` — creates `conventions/` subdir, loads `index.yaml` if exists
- `Save()` — writes both `index.yaml` (via yaml.Marshal) and `conventions.md` (rendered markdown grouped by domain)
- `Propose(conv)` — assigns ID like `conv-NNN`, sets AcceptedAt=now, AcceptCount=0, appends
- `Accept(id)` — increments AcceptCount, updates LastUsed
- `FindRelevant(domain, filePath)` — filters by domain match and FileGlob match (using `filepath.Match`)
- `MatchesFinding(f ExpertFinding)` — checks if any convention with AcceptCount >= 2 has Pattern words overlapping with finding Body (simple word-overlap similarity > 50%)
- `Decay(maxAge, minUses)` — removes conventions where LastUsed older than maxAge AND AcceptCount < minUses

The `index.yaml` is a simple YAML struct wrapper:
```go
type conventionIndex struct {
    Conventions []Convention `yaml:"conventions"`
}
```

**Step 4: Run tests**

Run: `go test ./internal/company/ -run TestConvention -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/conventions.go internal/company/conventions_test.go
git commit -m "feat(council): add Conventions learning system with Markdown + YAML persistence"
```

---

## Task 6: Synthesis Engine — Merge, Carmack Filter, AI Synthesis

**Files:**
- Create: `internal/company/synthesis.go`
- Create: `internal/company/synthesis_test.go`

**Step 1: Write failing tests**

```go
// internal/company/synthesis_test.go
package company

import (
	"context"
	"testing"
)

func TestMergeCouncilFindings_DeduplicateSameFileLine(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{File: "a.go", Line: 10, Severity: "HIGH", Body: "missing error check"}, Expert: DomainBackend},
		{Finding: Finding{File: "a.go", Line: 10, Severity: "MEDIUM", Body: "missing error check"}, Expert: DomainRefactoring},
	}
	merged := mergeCouncilFindings(findings)
	if len(merged) != 1 {
		t.Fatalf("expected 1 after dedup, got %d", len(merged))
	}
	if merged[0].Severity != "HIGH" {
		t.Fatal("should keep highest severity")
	}
}

func TestMergeCouncilFindings_KeepDifferentFiles(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{File: "a.go", Line: 10, Severity: "HIGH", Body: "issue"}, Expert: DomainBackend},
		{Finding: Finding{File: "b.go", Line: 10, Severity: "HIGH", Body: "issue"}, Expert: DomainBackend},
	}
	merged := mergeCouncilFindings(findings)
	if len(merged) != 2 {
		t.Fatalf("expected 2 (different files), got %d", len(merged))
	}
}

func TestMergeCouncilFindings_DomainPriority(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{File: "a.go", Line: 10, Severity: "HIGH", Body: "unsafe input"}, Expert: DomainBackend, Confidence: 0.9},
		{Finding: Finding{File: "a.go", Line: 10, Severity: "MEDIUM", Body: "unsafe user input"}, Expert: DomainSecurity, Confidence: 0.8},
	}
	merged := mergeCouncilFindings(findings)
	if len(merged) != 1 {
		t.Fatalf("expected 1 after dedup, got %d", len(merged))
	}
	// Security takes priority over backend
	if merged[0].Expert != DomainSecurity {
		t.Fatalf("expected security to win over backend, got %s", merged[0].Expert)
	}
}

func TestCarmackFilter_ConventionFiltering(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)
	cs.Propose(Convention{
		Domain:      DomainBackend,
		Pattern:     "yaml error ignored",
		AcceptCount: 3,
	})

	findings := []ExpertFinding{
		{Finding: Finding{File: "config.go", Severity: "MEDIUM", Body: "yaml.Unmarshal error is ignored"}, Expert: DomainBackend},
		{Finding: Finding{File: "handler.go", Severity: "HIGH", Body: "SQL injection risk"}, Expert: DomainSecurity},
	}

	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		MaxFindings:     15,
		ProjectScale:    ScaleMedium,
		ConventionStore: cs,
	})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 after convention filter, got %d", len(filtered))
	}
	if filtered[0].Expert != DomainSecurity {
		t.Fatal("SQL injection should survive filter")
	}
}

func TestCarmackFilter_ScaleFiltering(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "MEDIUM", Body: "minor perf issue"}, Expert: DomainPerformance},
		{Finding: Finding{Severity: "HIGH", Body: "security hole"}, Expert: DomainSecurity},
	}
	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		MaxFindings:  15,
		ProjectScale: ScaleSmall,
	})
	// Small scale should remove MEDIUM performance findings
	if len(filtered) != 1 {
		t.Fatalf("expected 1 (MEDIUM perf removed for small scale), got %d", len(filtered))
	}
}

func TestCarmackFilter_PatternCompression(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{File: "a.go", Line: 10, Severity: "MEDIUM", Body: "missing error check"}, Expert: DomainBackend},
		{Finding: Finding{File: "b.go", Line: 20, Severity: "MEDIUM", Body: "missing error check"}, Expert: DomainBackend},
		{Finding: Finding{File: "c.go", Line: 30, Severity: "MEDIUM", Body: "missing error check"}, Expert: DomainBackend},
	}
	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		MaxFindings:  15,
		ProjectScale: ScaleLarge,
	})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 compressed finding, got %d", len(filtered))
	}
	if !strings.Contains(filtered[0].Body, "3") {
		t.Fatal("compressed finding should mention count")
	}
}

func TestCarmackFilter_MaxFindings(t *testing.T) {
	var findings []ExpertFinding
	for i := 0; i < 20; i++ {
		findings = append(findings, ExpertFinding{
			Finding: Finding{File: fmt.Sprintf("f%d.go", i), Severity: "HIGH", Body: fmt.Sprintf("issue %d", i)},
			Expert:  DomainBackend,
		})
	}
	filtered := applyCarmackFilter(findings, CarmackFilterConfig{
		MaxFindings:  15,
		ProjectScale: ScaleLarge,
	})
	if len(filtered) > 15 {
		t.Fatalf("expected max 15, got %d", len(filtered))
	}
}

func TestDetermineVerdict_NoFindings(t *testing.T) {
	status := determineVerdict(nil)
	if status != "APPROVED" {
		t.Fatalf("expected APPROVED for no findings, got %s", status)
	}
}

func TestDetermineVerdict_OnlyCritical(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "CRITICAL"}},
	}
	status := determineVerdict(findings)
	if status != "CHANGES_REQUESTED" {
		t.Fatalf("expected CHANGES_REQUESTED for CRITICAL, got %s", status)
	}
}

func TestDetermineVerdict_ThreeHigh(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "HIGH"}},
		{Finding: Finding{Severity: "HIGH"}},
		{Finding: Finding{Severity: "HIGH"}},
	}
	status := determineVerdict(findings)
	if status != "CHANGES_REQUESTED" {
		t.Fatalf("expected CHANGES_REQUESTED for 3 HIGH, got %s", status)
	}
}

func TestDetermineVerdict_OnlyMedium(t *testing.T) {
	findings := []ExpertFinding{
		{Finding: Finding{Severity: "MEDIUM"}},
		{Finding: Finding{Severity: "MEDIUM"}},
	}
	status := determineVerdict(findings)
	if status != "APPROVED" {
		t.Fatalf("expected APPROVED for only MEDIUM, got %s", status)
	}
}

func TestProjectScale_Auto(t *testing.T) {
	if detectProjectScale(10) != ScaleSmall {
		t.Fatal("10 files = small")
	}
	if detectProjectScale(100) != ScaleMedium {
		t.Fatal("100 files = medium")
	}
	if detectProjectScale(1000) != ScaleLarge {
		t.Fatal("1000 files = large")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run "TestMerge|TestCarmack|TestDetermine|TestProject" -v`
Expected: FAIL

**Step 3: Write implementation**

Create `internal/company/synthesis.go` with:

- `ExpertFinding` struct (embeds `Finding`, adds `Expert ExpertDomain`, `Principle string`, `Confidence float64`)
- `CouncilResult` struct (Status, Summary, Findings []ExpertFinding, ExpertCount, Phase0, Duration, TokensUsed)
- `ProjectScale` type with `ScaleSmall/Medium/Large` constants
- `CarmackFilterConfig` struct
- `mergeCouncilFindings(findings)` — dedup by file:line, domain priority map, keep highest severity
- `applyCarmackFilter(findings, cfg)` — convention filter, scale filter, pattern compression, max cap
- `determineVerdict(findings)` — mechanical verdict rules (no AI needed for clear cases)
- `detectProjectScale(fileCount)` — <50 small, 50-500 medium, >500 large
- `domainPriority` map for conflict resolution
- `bodySimilarity(a, b string) float64` — word-overlap Jaccard similarity

For AI synthesis (used only when verdict is ambiguous — 1-2 HIGH findings):
- `synthesizeWithAI(ctx, cp, findings, brief, model, lang)` — single ChatProvider call to produce Summary

**Step 4: Run tests**

Run: `go test ./internal/company/ -run "TestMerge|TestCarmack|TestDetermine|TestProject" -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/synthesis.go internal/company/synthesis_test.go
git commit -m "feat(council): add synthesis engine with Carmack Filter and mechanical verdict"
```

---

## Task 7: Council Engine — Orchestration & Dispatch

**Files:**
- Create: `internal/company/council.go`
- Create: `internal/company/council_test.go`

**Step 1: Write failing tests**

```go
// internal/company/council_test.go
package company

import (
	"context"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// mockChatProvider returns canned responses for testing
type mockChatProvider struct {
	response string
	err      error
}

func (m *mockChatProvider) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	return m.response, m.err
}

func TestCouncilEngine_RunCouncil_AllGreenPhase0(t *testing.T) {
	cp := &mockChatProvider{
		response: `[{"file":"a.go","line":1,"severity":"MEDIUM","body":"minor issue"}]`,
	}
	engine := &CouncilEngine{
		chatProvider: cp,
		registry:     NewExpertRegistry(),
		language:     "en",
	}

	brief, _ := (&ContextBriefBuilder{
		taskID: "t1", projectID: "p1", projectName: "test",
	}).Build()

	result, err := engine.RunCouncil(context.Background(), CouncilRequest{
		Diff:      "+++ b/a.go\n+func hello() {}",
		DiffLines: 5,
		FileCount: 1,
		Brief:     brief,
		Phase0:    &Phase0Report{AllGreen: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "APPROVED" {
		t.Fatalf("expected APPROVED for minor issue, got %s", result.Status)
	}
}

func TestCouncilEngine_RunCouncil_Phase0CriticalFail(t *testing.T) {
	engine := &CouncilEngine{
		registry: NewExpertRegistry(),
		language: "en",
	}
	brief, _ := (&ContextBriefBuilder{taskID: "t1", projectID: "p1"}).Build()

	result, err := engine.RunCouncil(context.Background(), CouncilRequest{
		Diff:      "+++ b/a.go\n+broken code",
		DiffLines: 5,
		FileCount: 1,
		Brief:     brief,
		Phase0: &Phase0Report{
			Results: []Phase0Result{
				{Check: Phase0Check{Name: "go-vet"}, Passed: false, Output: "compilation error"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "CHANGES_REQUESTED" {
		t.Fatal("expected CHANGES_REQUESTED when all phase0 checks fail")
	}
}

func TestCouncilEngine_ParseExpertResponse(t *testing.T) {
	raw := `[{"file":"handler.go","line":42,"severity":"HIGH","body":"SQL injection risk"}]`
	findings, err := parseExpertFindings(raw, DomainSecurity)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Expert != DomainSecurity {
		t.Fatal("expected security domain tag")
	}
}

func TestCouncilEngine_ParseExpertResponse_Markdown(t *testing.T) {
	raw := "Here are my findings:\n```json\n[{\"file\":\"a.go\",\"severity\":\"MEDIUM\",\"body\":\"issue\"}]\n```"
	findings, err := parseExpertFindings(raw, DomainBackend)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from markdown-wrapped JSON, got %d", len(findings))
	}
}

func TestCouncilEngine_ParseExpertResponse_NoFindings(t *testing.T) {
	raw := "Everything looks good. No issues found.\n```json\n[]\n```"
	findings, err := parseExpertFindings(raw, DomainBackend)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestCouncilEngine_FilterDiffForExpert(t *testing.T) {
	fullDiff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {}
diff --git a/frontend/app.svelte b/frontend/app.svelte
--- a/frontend/app.svelte
+++ b/frontend/app.svelte
@@ -1,2 +1,3 @@
 <script>
+  let x = 1
 </script>`

	goDiff := filterDiffForFiles(fullDiff, []string{"main.go"})
	if !strings.Contains(goDiff, "main.go") {
		t.Fatal("should contain main.go diff")
	}
	if strings.Contains(goDiff, "app.svelte") {
		t.Fatal("should not contain svelte diff")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run TestCouncil -v`
Expected: FAIL

**Step 3: Write implementation**

Create `internal/company/council.go` with:

- `CouncilEngine` struct (chatProvider, spawner, tmuxClient, registry, conventions, graph, language, reviewCfg)
- `CouncilRequest` struct
- `RunCouncil(ctx, req)` — main orchestration:
  1. Early reject if `phase0.AllCriticalFailed()`
  2. Extract changed files from diff
  3. `registry.SelectExperts(files, diff, graph, phase0)`
  4. `dispatchExperts(ctx, experts, brief, diff, worktreePath)` — parallel execution
  5. `mergeCouncilFindings` → `applyCarmackFilter` → `determineVerdict` or `synthesizeWithAI`
  6. Return `CouncilResult`
- `dispatchExperts(ctx, experts, brief, diff, worktreePath)` — `errgroup.WithContext`, `SetLimit(5)`
- `runExpertAPI(ctx, expert, brief, diff)` — `ChatWithModelOrFallback` with timeout
- `parseExpertFindings(raw, domain)` — JSON extraction (direct or markdown-wrapped)
- `filterDiffForFiles(fullDiff, files)` — extract relevant hunks from unified diff
- `spawnExpertAgent` — placeholder that returns error (CLI mode implemented in Task 9)

**Step 4: Run tests**

Run: `go test ./internal/company/ -run TestCouncil -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/council.go internal/company/council_test.go
git commit -m "feat(council): add Council Engine with parallel expert dispatch"
```

---

## Task 8: Integration — Wire into Review Pipeline

**Files:**
- Modify: `internal/company/review.go` (add `runCouncilReview`, modify `executeReview`)
- Modify: `internal/company/company.go` (add council/conventions/registry fields to Manager, init in New())
- Modify: `internal/worker/spawner.go` (add conventions field, update `buildPromptForTier`)

**Step 1: Modify Manager struct in company.go**

Add fields after the existing `reviewCfg` field:

```go
council      *CouncilEngine
conventions  *ConventionStore
expertReg    *ExpertRegistry
```

In `New()`, after `review` pipeline initialization, add:

```go
conventions, err := NewConventionStore(dataDir)
if err != nil {
	log.Printf("warning: conventions store init failed: %v", err)
	conventions = &ConventionStore{} // empty fallback
}
expertReg := NewExpertRegistry()
council := &CouncilEngine{
	chatProvider: chatProvider,
	spawner:      spawner,
	tmuxClient:   tmuxClient,
	registry:     expertReg,
	conventions:  conventions,
	language:     m.language,
	reviewCfg:    m.reviewCfg,
}
if m.graphProvider != nil {
	council.graph = m.graphProvider.LightGraph()
}
m.council = council
m.conventions = conventions
m.expertReg = expertReg
```

Also add conventions to spawner:

```go
spawner.SetConventions(conventions)
```

**Step 2: Add `runCouncilReview` to review.go**

Add new method to `ReviewPipeline`:

```go
func (rp *ReviewPipeline) runCouncilReview(req ReviewRequest, t *project.Task, p *project.Project, cfg config.ReviewConfig) {
	cfg = reviewConfigWithDefaults(cfg)
	ctx := context.Background()

	repoPath := p.RepoPath
	baseBranch := p.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	diffLines, fileCount, diff, err := getDiffStats(ctx, repoPath, baseBranch, t.Branch)
	if err != nil {
		log.Printf("council: diff failed, falling back to debate: %v", err)
		rp.runChatReview(req, t, p, cfg)
		return
	}

	// Phase 0
	var phase0 *Phase0Report
	if cfg.Phase0Enabled {
		workDir := repoPath
		if t.WorktreePath != "" {
			workDir = t.WorktreePath
		}
		checks := detectChecks(workDir, p.VerifyCmd)
		phase0 = runPhase0Checks(ctx, workDir, checks)
		rp.mgr.emit(Event{
			Type:      EventPhase0Completed,
			ProjectID: p.ID,
			TaskID:    t.ID,
			Message:   phase0.Summary,
		})
	} else {
		phase0 = &Phase0Report{AllGreen: true}
	}

	// Build Context Brief
	briefBuilder := &ContextBriefBuilder{
		totalBudget: 6000,
		taskID:      t.ID,
		projectID:   p.ID,
		projectName: p.Name,
		techStack:   detectTechStack(repoPath),
		baseBranch:  baseBranch,
		diffStats:   fmt.Sprintf("%d lines, %d files", diffLines, fileCount),
		changedFiles: extractChangedFiles(diff),
		phase0Summary: phase0.Summary,
	}
	// Inject knowledge context
	if inj := rp.mgr.spawner.KnowledgeInjector(); inj != nil {
		archCtx, _ := inj.BuildContext("", p.ID, knowledge.TierL2RoomRecall)
		briefBuilder.architectureCtx = archCtx
	}
	// Inject conventions
	if rp.mgr.conventions != nil {
		var convParts []string
		for _, f := range extractChangedFiles(diff) {
			for _, c := range rp.mgr.conventions.FindRelevant("", f) {
				convParts = append(convParts, fmt.Sprintf("- [%s] %s (%s)", c.Domain, c.Description, c.FileGlob))
			}
		}
		if len(convParts) > 0 {
			briefBuilder.conventions = strings.Join(uniqueStrings(convParts), "\n")
		}
	}
	// Inject rejection history and karpathy overlay
	if len(t.RejectionHistory) > 0 {
		briefBuilder.rejectionHistory = compressRejectionHistory(t, 1500)
		briefBuilder.karpathyOverlay = buildKarpathyOverlayContent(t, rp.mgr.language)
	}

	brief, err := briefBuilder.Build()
	if err != nil {
		log.Printf("council: brief build failed: %v", err)
		rp.runChatReview(req, t, p, cfg)
		return
	}

	// Set graph on council engine
	if rp.mgr.graphProvider != nil {
		rp.mgr.council.graph, _ = rp.mgr.graphProvider.GetGraph(repoPath)
	}

	// Run Council
	result, err := rp.mgr.council.RunCouncil(ctx, CouncilRequest{
		Task:      t,
		Project:   p,
		Diff:      diff,
		DiffLines: diffLines,
		FileCount: fileCount,
		Brief:     brief,
		Phase0:    phase0,
	})
	if err != nil {
		log.Printf("council: failed, falling back to debate: %v", err)
		rp.runChatReview(req, t, p, cfg)
		return
	}

	rp.handleCouncilResult(result, req, t, p)
}

func (rp *ReviewPipeline) handleCouncilResult(result *CouncilResult, req ReviewRequest, t *project.Task, p *project.Project) {
	// Convert to DebateResult for reuse of existing handleDebateResult
	var comments []Finding
	for _, ef := range result.Findings {
		f := ef.Finding
		f.Source = string(ef.Expert)
		comments = append(comments, f)
	}

	debateResult := &DebateResult{
		Status:   result.Status,
		Summary:  fmt.Sprintf("[%d experts, %d findings] %s", result.ExpertCount, len(result.Findings), result.Summary),
		Comments: comments,
	}

	rp.handleDebateResult(debateResult, req, t, p)

	// Learn from approved reviews
	if result.Status == "APPROVED" && rp.mgr.conventions != nil {
		go rp.learnFromReview(result, t)
	}
}

func (rp *ReviewPipeline) learnFromReview(result *CouncilResult, t *project.Task) {
	// Extract potential conventions from findings that were filtered or approved
	// Simple heuristic: MEDIUM findings that got approved become convention candidates
	for _, f := range result.Findings {
		if f.Severity == "MEDIUM" {
			existing := rp.mgr.conventions.MatchesFinding(f)
			if existing != nil {
				rp.mgr.conventions.Accept(existing.ID)
				rp.mgr.emit(Event{Type: EventConventionAccepted, TaskID: t.ID,
					Message: fmt.Sprintf("convention %s reinforced", existing.ID)})
			} else {
				rp.mgr.conventions.Propose(Convention{
					Domain:      f.Expert,
					Pattern:     f.Body,
					Description: f.Body,
					Source:      fmt.Sprintf("review:%s", t.ID),
				})
				rp.mgr.emit(Event{Type: EventConventionProposed, TaskID: t.ID,
					Message: fmt.Sprintf("new convention proposed: %s", f.Body[:min(50, len(f.Body))])})
			}
		}
	}
	rp.mgr.conventions.Save()
}
```

**Step 3: Modify `executeReview` routing**

In `review.go`, modify `executeReview` to route to council first:

```go
func (rp *ReviewPipeline) executeReview(ctx context.Context, req ReviewRequest, managerWorker *worker.Worker, t *project.Task, p *project.Project) error {
	if rp.mgr.chatProvider != nil {
		if rp.mgr.reviewCfg.CouncilEnabled && rp.mgr.council != nil {
			go rp.runCouncilReview(req, t, p, rp.mgr.reviewCfg)
		} else {
			go rp.runChatReview(req, t, p, rp.mgr.reviewCfg)
		}
		return nil
	}
	return rp.executeReviewTmux(ctx, req, managerWorker, t, p)
}
```

**Step 4: Update spawner for conventions**

In `internal/worker/spawner.go`, add:

```go
func (s *Spawner) SetConventions(cs interface{ FindRelevant(domain, filePath string) []Convention }) {
	s.conventions = cs
}
```

Note: Use an interface to avoid circular imports. The actual type assertion happens at call site.

**Step 5: Build and run existing tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./...`
Expected: BUILD SUCCESS

Run: `go test ./internal/company/ -v -count=1`
Expected: ALL PASS (existing + new tests)

**Step 6: Commit**

```bash
git add internal/company/review.go internal/company/company.go internal/worker/spawner.go
git commit -m "feat(council): wire council engine into review pipeline with debate fallback"
```

---

## Task 9: CLI Expert Agent Mode

**Files:**
- Modify: `internal/company/council.go` (implement `spawnExpertAgent`)

**Step 1: Implement CLI expert spawn**

Replace the placeholder `spawnExpertAgent` in `council.go`:

```go
func (c *CouncilEngine) spawnExpertAgent(
	ctx context.Context,
	expert SelectedExpert,
	brief *ContextBrief,
	worktreePath string,
) ([]ExpertFinding, error) {
	if c.tmuxClient == nil || c.spawner == nil {
		return nil, fmt.Errorf("CLI mode requires tmux client and spawner")
	}

	sessionName := fmt.Sprintf("ais-review-%s-%d", expert.Domain, time.Now().Unix())
	timeout := time.Duration(c.reviewCfg.CLIExpertTimeoutS) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	// Write brief to worktree for agent to read
	brief.WriteToDir(worktreePath)

	// Create tmux session
	if err := c.tmuxClient.NewSession(sessionName, worktreePath); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer c.tmuxClient.KillSession(sessionName)

	// Build CLI command with read-only restrictions
	cliArgs := fmt.Sprintf(
		"--append-system-prompt %q --model %s --dangerously-skip-permissions --disallowedTools Edit,Write,NotebookEdit,Bash",
		expert.SystemPrompt,
		expertModel(expert),
	)

	// Launch Claude Code
	c.tmuxClient.SendKeys(sessionName, fmt.Sprintf("claude %s", cliArgs), true)

	// Wait for ready (reuse spawner's pattern)
	readyCtx, readyCancel := context.WithTimeout(ctx, 60*time.Second)
	defer readyCancel()
	if err := waitForCLIReady(readyCtx, c.tmuxClient, sessionName); err != nil {
		return nil, fmt.Errorf("CLI not ready: %w", err)
	}

	// Send review prompt
	prompt := buildExpertCLIPrompt(expert, brief.FilePath)
	c.tmuxClient.SendLiteralKeys(sessionName, prompt)
	time.Sleep(1 * time.Second)
	c.tmuxClient.SendKeys(sessionName, "", true) // Enter

	// Wait for completion with timeout
	completionCtx, completionCancel := context.WithTimeout(ctx, timeout)
	defer completionCancel()
	if err := waitForCLICompletion(completionCtx, c.tmuxClient, sessionName); err != nil {
		return nil, fmt.Errorf("expert timeout: %w", err)
	}

	// Capture output
	output, err := c.tmuxClient.CapturePane(sessionName, 0, 0, "-S", "-500")
	if err != nil {
		return nil, fmt.Errorf("capture output: %w", err)
	}

	return parseExpertFindings(output, expert.Domain)
}
```

Helper functions:
- `expertModel(expert)` — returns expert.Model or "sonnet" default
- `buildExpertCLIPrompt(expert, briefPath)` — instructs agent to read brief, review assigned files, output JSON findings
- `waitForCLIReady` / `waitForCLICompletion` — polling loops on pane content (similar to existing `waitForReady` in spawner)

**Step 2: Build**

Run: `go build ./...`
Expected: BUILD SUCCESS

**Step 3: Commit**

```bash
git add internal/company/council.go
git commit -m "feat(council): implement CLI expert agent spawn with read-only restrictions"
```

---

## Task 10: Frontend i18n & Event Translations

**Files:**
- Modify: `frontend/src/lib/stores/i18n.js`

**Step 1: Add translations**

Add to the translations object alongside existing review events:

```javascript
// Council review events
'event.phase0_completed': { en: 'Phase 0 Checks Completed', zh: 'Phase 0 檢查完成' },
'event.council_started': { en: 'Council Review Started', zh: '委員會審查已開始' },
'event.expert_completed': { en: 'Expert Review Completed', zh: '專家審查已完成' },
'event.council_synthesized': { en: 'Council Review Synthesized', zh: '委員會審查已合成' },
'event.convention_proposed': { en: 'Convention Proposed', zh: '慣例已提議' },
'event.convention_accepted': { en: 'Convention Accepted', zh: '慣例已確認' },
'event.convention_decayed': { en: 'Convention Expired', zh: '慣例已過期' },
```

**Step 2: Verify frontend builds**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor/frontend" && npm run build`
Expected: BUILD SUCCESS

**Step 3: Commit**

```bash
git add frontend/src/lib/stores/i18n.js
git commit -m "feat(council): add zh-TW translations for council review events"
```

---

## Task 11: Integration Test

**Files:**
- Create: `internal/company/council_integration_test.go`

**Step 1: Write integration test**

```go
// internal/company/council_integration_test.go
package company

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// TestCouncilPipeline_EndToEnd tests the full pipeline from Phase 0 through synthesis.
func TestCouncilPipeline_EndToEnd(t *testing.T) {
	// Mock chat provider that returns findings
	cp := &mockChatProvider{
		response: `[{"file":"handler.go","line":42,"severity":"HIGH","body":"Missing input validation on user-supplied ID parameter"}]`,
	}

	dir := t.TempDir()
	conventions, _ := NewConventionStore(dir)
	registry := NewExpertRegistry()

	engine := &CouncilEngine{
		chatProvider: cp,
		registry:     registry,
		conventions:  conventions,
		language:     "zh-TW",
	}

	brief, _ := (&ContextBriefBuilder{
		totalBudget: 6000,
		taskID:      "task-001",
		projectID:   "proj-001",
		projectName: "aisupervisor",
		techStack:   "Go, Svelte",
		baseBranch:  "main",
		diffStats:   "150 lines, 5 files",
		changedFiles: []string{"handler.go", "router.go", "model.go"},
	}).Build()

	phase0 := &Phase0Report{
		AllGreen: true,
		Summary:  "go-vet: PASS; golint: PASS",
		Results: []Phase0Result{
			{Check: Phase0Check{Name: "go-vet"}, Passed: true, Elapsed: 2 * time.Second},
		},
	}

	result, err := engine.RunCouncil(context.Background(), CouncilRequest{
		Diff:      "+++ b/handler.go\n+func HandleUser(w http.ResponseWriter, r *http.Request) {\n+  id := r.URL.Query().Get(\"id\")\n+  user, _ := db.GetUser(id)\n+}",
		DiffLines: 150,
		FileCount: 5,
		Brief:     brief,
		Phase0:    phase0,
	})
	if err != nil {
		t.Fatalf("RunCouncil failed: %v", err)
	}

	// Verify result structure
	if result.Status == "" {
		t.Fatal("status should not be empty")
	}
	if result.ExpertCount == 0 {
		t.Fatal("at least one expert should have run")
	}
	if result.Duration == 0 {
		t.Fatal("duration should be recorded")
	}

	t.Logf("Council result: status=%s experts=%d findings=%d summary=%s",
		result.Status, result.ExpertCount, len(result.Findings), result.Summary)
}

// TestCouncilPipeline_Phase0Rejection tests early rejection on Phase 0 failure.
func TestCouncilPipeline_Phase0Rejection(t *testing.T) {
	engine := &CouncilEngine{
		registry: NewExpertRegistry(),
		language: "en",
	}
	brief, _ := (&ContextBriefBuilder{taskID: "t1", projectID: "p1"}).Build()

	result, err := engine.RunCouncil(context.Background(), CouncilRequest{
		Diff:      "+++ b/main.go\n+broken",
		DiffLines: 5,
		FileCount: 1,
		Brief:     brief,
		Phase0: &Phase0Report{
			Results: []Phase0Result{
				{Check: Phase0Check{Name: "build"}, Passed: false, Output: "compilation failed"},
				{Check: Phase0Check{Name: "lint"}, Passed: false, Output: "lint errors"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "CHANGES_REQUESTED" {
		t.Fatalf("expected rejection on phase0 failure, got %s", result.Status)
	}
	// Should have phase0 findings
	hasPhase0 := false
	for _, f := range result.Findings {
		if f.Source == "phase0" {
			hasPhase0 = true
		}
	}
	if !hasPhase0 {
		t.Fatal("expected phase0 findings in result")
	}
}

// TestConventionLearningLoop tests that approved reviews create conventions.
func TestConventionLearningLoop(t *testing.T) {
	dir := t.TempDir()
	cs, _ := NewConventionStore(dir)

	// Simulate: first review proposes a convention
	cs.Propose(Convention{
		Domain:  DomainBackend,
		Pattern: "error return ignored in config loader",
		Source:  "review:task-001",
	})

	// Simulate: second review matches and accepts
	finding := ExpertFinding{
		Finding: Finding{Body: "error return value ignored in config loader function"},
		Expert:  DomainBackend,
	}
	match := cs.MatchesFinding(finding)
	if match == nil {
		t.Fatal("should match similar pattern")
	}
	cs.Accept(match.ID)
	cs.Save()

	// Verify convention is persisted
	cs2, _ := NewConventionStore(dir)
	if len(cs2.conventions) != 1 {
		t.Fatal("convention should persist")
	}
	if cs2.conventions[0].AcceptCount != 1 {
		t.Fatalf("expected AcceptCount 1, got %d", cs2.conventions[0].AcceptCount)
	}
}
```

**Step 2: Run full test suite**

Run: `go test ./internal/company/ -v -count=1`
Expected: ALL PASS

Run: `go test ./internal/... -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add internal/company/council_integration_test.go
git commit -m "test(council): add integration tests for full council pipeline"
```

---

## Task 12: Final Build Verification & Cleanup

**Step 1: Full build**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./...`
Expected: BUILD SUCCESS

**Step 2: Full test suite**

Run: `go test ./internal/... -v -count=1`
Expected: ALL PASS

**Step 3: Verify no import cycles**

Run: `go vet ./internal/...`
Expected: No errors

**Step 4: Review all new files exist**

```
internal/company/phase0.go          ✓
internal/company/phase0_test.go     ✓
internal/company/context_brief.go   ✓
internal/company/context_brief_test.go ✓
internal/company/expert.go          ✓
internal/company/expert_test.go     ✓
internal/company/conventions.go     ✓
internal/company/conventions_test.go ✓
internal/company/synthesis.go       ✓
internal/company/synthesis_test.go  ✓
internal/company/council.go         ✓
internal/company/council_test.go    ✓
internal/company/council_integration_test.go ✓
```

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat(council): Council Review Engine — complete implementation

Carmack-Council inspired multi-expert review pipeline:
- Phase 0 automated pre-review checks
- Unified Context Brief with token budget management
- Dynamic expert selection (10 domains, 2-5 per review)
- Parallel dispatch (API + CLI mixed mode)
- Carmack Filter synthesis with domain priority dedup
- Cross-session conventions learning (Markdown + YAML)
- Existing debate system preserved as fallback"
```
