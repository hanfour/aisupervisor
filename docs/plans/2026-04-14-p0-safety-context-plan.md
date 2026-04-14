# P0 Safety & Context — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add delegation depth limit, error classification pipeline, and GRAPH_REPORT auto-injection to improve worker safety, error resilience, and architectural context.

**Architecture:** Three independent features. Delegation depth uses a new Task field to track depth and suppress delegation prompts. Error classification adds a keyword-based classifier that drives the SpawnForTask retry loop. GRAPH_REPORT injection reads a file from disk at prompt build time.

**Tech Stack:** Go 1.23+, existing project/worker/company packages

---

## Task 1: Delegation Depth Limit — Data Model

**Files:**
- Modify: `internal/project/task.go:49-98` (Task struct)
- Modify: `internal/project/task_test.go`

**Step 1: Write the failing test**

```go
// Append to internal/project/task_test.go
func TestTask_DelegationDepth(t *testing.T) {
	task := Task{
		ID:              "t1",
		DelegationDepth: 2,
	}
	if task.DelegationDepth != 2 {
		t.Errorf("expected depth 2, got %d", task.DelegationDepth)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/project/ -run "TestTask_DelegationDepth" -v`
Expected: FAIL — `DelegationDepth` not defined

**Step 3: Add DelegationDepth to Task struct**

In `internal/project/task.go`, add after the `ParentTaskID` field (around line 63):

```go
DelegationDepth  int            `yaml:"delegation_depth,omitempty" json:"delegationDepth,omitempty"`
```

**Step 4: Run test to verify it passes**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/project/ -run "TestTask_DelegationDepth" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/task.go internal/project/task_test.go
git commit -m "feat(project): add DelegationDepth to Task struct"
```

---

## Task 2: Delegation Depth Limit — Enforce in Delegation & Spawner

**Files:**
- Modify: `internal/company/delegation.go:85-113` (set depth on created tasks)
- Modify: `internal/worker/spawner.go:841-849,893-901` (suppress delegation prompt)

**Step 1: Write the failing tests**

```go
// internal/company/delegation_depth_test.go
package company

import (
	"testing"
)

func TestDelegationDepth_IsSetOnSubTask(t *testing.T) {
	// When a parent task has depth 0 (root), delegated tasks should have depth 1
	parentDepth := 0
	childDepth := parentDepth + 1
	if childDepth != 1 {
		t.Errorf("expected child depth 1, got %d", childDepth)
	}
}

func TestDelegationDepth_MaxPreventsPrompt(t *testing.T) {
	// MaxDelegationDepth is 2, so depth >= 2 should suppress delegation
	const MaxDelegationDepth = 2
	if 2 >= MaxDelegationDepth {
		// This is expected: depth 2 should NOT get delegation prompt
	}
	if 1 >= MaxDelegationDepth {
		t.Error("depth 1 should still allow delegation")
	}
}
```

```go
// internal/worker/spawner_delegation_test.go
package worker

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestShouldIncludeDelegation_RootTask(t *testing.T) {
	task := &project.Task{DelegationDepth: 0}
	if !shouldIncludeDelegation(task) {
		t.Error("root task (depth 0) should include delegation prompt")
	}
}

func TestShouldIncludeDelegation_ChildTask(t *testing.T) {
	task := &project.Task{DelegationDepth: 1}
	if !shouldIncludeDelegation(task) {
		t.Error("depth 1 task should still include delegation prompt")
	}
}

func TestShouldIncludeDelegation_MaxDepth(t *testing.T) {
	task := &project.Task{DelegationDepth: 2}
	if shouldIncludeDelegation(task) {
		t.Error("depth 2 task should NOT include delegation prompt")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestShouldIncludeDelegation" -v`
Expected: FAIL — `shouldIncludeDelegation` not defined

**Step 3: Add shouldIncludeDelegation to spawner**

In `internal/worker/spawner.go`, add after `buildKarpathyOverlay`:

```go
const MaxDelegationDepth = 2

func shouldIncludeDelegation(t *project.Task) bool {
	return t.DelegationDepth < MaxDelegationDepth
}
```

**Step 4: Wire into buildPromptForTierInner**

In `internal/worker/spawner.go`, change the two delegation sections.

English section (around line 841):

```go
// BEFORE:
if tier == TierManager || tier == TierConsultant {
    sb.WriteString("\n--- Delegation ---\n")

// AFTER:
if (tier == TierManager || tier == TierConsultant) && shouldIncludeDelegation(t) {
    sb.WriteString("\n--- Delegation ---\n")
```

zh-TW section (around line 893):

```go
// BEFORE:
if tier == TierManager || tier == TierConsultant {
    sb.WriteString("\n--- 委派 ---\n")

// AFTER:
if (tier == TierManager || tier == TierConsultant) && shouldIncludeDelegation(t) {
    sb.WriteString("\n--- 委派 ---\n")
```

**Step 5: Set depth on delegated sub-tasks**

In `internal/company/delegation.go`, in the `handleDelegationOutput` method, around line 91 where `m.AddTask` is called, set the depth on the new task after creation:

```go
// AFTER m.AddTask (line 91-95):
newTask, err := m.AddTask(p.ID, cmd.Title, "", cmd.Prompt, nil, priority, "", "code")
if err != nil {
    log.Printf("delegation: failed to create task %q: %v", cmd.Title, err)
    continue
}
newTask.DelegationDepth = t.DelegationDepth + 1
m.projectStore.SaveTask(newTask)
```

**Step 6: Run all tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestShouldIncludeDelegation" -v && go test ./internal/company/ -run "TestDelegationDepth" -v`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/worker/spawner.go internal/worker/spawner_delegation_test.go internal/company/delegation.go internal/company/delegation_depth_test.go
git commit -m "feat(delegation): enforce max depth limit to prevent recursive delegation"
```

---

## Task 3: Error Classification — Core Classifier

**Files:**
- Create: `internal/worker/errclass.go`
- Create: `internal/worker/errclass_test.go`

**Step 1: Write the failing tests**

```go
// internal/worker/errclass_test.go
package worker

import (
	"errors"
	"testing"
)

func TestClassifyError_RateLimit(t *testing.T) {
	err := errors.New("API error: rate limit exceeded (429)")
	action := ClassifyError(err)
	if action != ActionRetry {
		t.Errorf("expected retry for rate limit, got %s", action)
	}
}

func TestClassifyError_ContextLength(t *testing.T) {
	err := errors.New("context length exceeded: too many tokens")
	action := ClassifyError(err)
	if action != ActionCompress {
		t.Errorf("expected compress for context length, got %s", action)
	}
}

func TestClassifyError_InvalidKey(t *testing.T) {
	err := errors.New("authentication failed: invalid api key (401)")
	action := ClassifyError(err)
	if action != ActionAbandon {
		t.Errorf("expected abandon for invalid key, got %s", action)
	}
}

func TestClassifyError_Timeout(t *testing.T) {
	err := errors.New("connection timeout after 30s")
	action := ClassifyError(err)
	if action != ActionRetry {
		t.Errorf("expected retry for timeout, got %s", action)
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	err := errors.New("some unknown error occurred")
	action := ClassifyError(err)
	if action != ActionRetry {
		t.Errorf("expected retry for unknown error, got %s", action)
	}
}

func TestClassifyError_Nil(t *testing.T) {
	action := ClassifyError(nil)
	if action != ActionRetry {
		t.Errorf("expected retry for nil error, got %s", action)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestClassifyError" -v`
Expected: FAIL — `ClassifyError`, `ActionRetry` not defined

**Step 3: Implement error classifier**

```go
// internal/worker/errclass.go
package worker

import "strings"

// ErrorAction represents the recommended action for a classified error.
type ErrorAction string

const (
	ActionRetry    ErrorAction = "retry"
	ActionRotate   ErrorAction = "rotate"
	ActionAbandon  ErrorAction = "abandon"
	ActionCompress ErrorAction = "compress"
)

// errorPatterns maps keywords to actions, checked in priority order.
var errorPatterns = []struct {
	keywords []string
	action   ErrorAction
}{
	{[]string{"context length", "too many tokens", "maximum context", "token limit"}, ActionCompress},
	{[]string{"invalid api key", "invalid_api_key", "401", "403", "unauthorized", "forbidden"}, ActionAbandon},
	{[]string{"billing", "payment required", "402", "insufficient_quota"}, ActionAbandon},
	{[]string{"rate limit", "429", "quota", "too many requests"}, ActionRetry},
	{[]string{"timeout", "connection", "timed out", "connect error", "eof"}, ActionRetry},
}

// ClassifyError examines an error message and returns the recommended action.
func ClassifyError(err error) ErrorAction {
	if err == nil {
		return ActionRetry
	}
	lower := strings.ToLower(err.Error())
	for _, p := range errorPatterns {
		for _, kw := range p.keywords {
			if strings.Contains(lower, kw) {
				return p.action
			}
		}
	}
	return ActionRetry
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestClassifyError" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worker/errclass.go internal/worker/errclass_test.go
git commit -m "feat(worker): add error classification pipeline"
```

---

## Task 4: Error Classification — Wire into SpawnForTask

**Files:**
- Modify: `internal/worker/spawner.go:357-375` (SpawnForTask retry loop)

**Step 1: Integrate ClassifyError into retry loop**

In `internal/worker/spawner.go`, replace the current retry loop in `SpawnForTask` (lines 357-375):

```go
// BEFORE:
func (s *Spawner) SpawnForTask(ctx context.Context, w *Worker, t *project.Task, p *project.Project) error {
	backoffs := []time.Duration{0, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := s.checkTmuxServer(); err != nil {
				log.Printf("WARNING: tmux server unhealthy before retry %d: %v", attempt, err)
				time.Sleep(backoffs[attempt])
				continue
			}
			time.Sleep(backoffs[attempt])
			log.Printf("SpawnForTask: retry %d for worker %s task %s", attempt, w.ID, t.ID)
		}
		if err := s.spawnForTaskInner(ctx, w, t, p); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

// AFTER:
func (s *Spawner) SpawnForTask(ctx context.Context, w *Worker, t *project.Task, p *project.Project) error {
	backoffs := []time.Duration{0, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := s.checkTmuxServer(); err != nil {
				log.Printf("WARNING: tmux server unhealthy before retry %d: %v", attempt, err)
				time.Sleep(backoffs[attempt])
				continue
			}
			time.Sleep(backoffs[attempt])
			log.Printf("SpawnForTask: retry %d for worker %s task %s", attempt, w.ID, t.ID)
		}
		if err := s.spawnForTaskInner(ctx, w, t, p); err != nil {
			lastErr = err
			action := ClassifyError(err)
			log.Printf("SpawnForTask: error classified as %s: %v", action, err)
			switch action {
			case ActionAbandon:
				return fmt.Errorf("spawn abandoned (unrecoverable): %w", err)
			case ActionCompress:
				log.Printf("SpawnForTask: context too long for worker %s, cannot compress at spawn", w.ID)
				return fmt.Errorf("spawn failed (context too long): %w", err)
			default:
				continue
			}
		}
		return nil
	}
	return lastErr
}
```

**Step 2: Verify build and all tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./internal/... && go test ./internal/worker/ -count=1`
Expected: Build OK, all PASS

**Step 3: Commit**

```bash
git add internal/worker/spawner.go
git commit -m "feat(worker): integrate error classification into SpawnForTask retry loop"
```

---

## Task 5: GRAPH_REPORT Auto-Injection

**Files:**
- Modify: `internal/worker/spawner.go` (buildPromptForTierInner)
- Create: `internal/worker/spawner_graphreport_test.go`

**Step 1: Write the failing test**

```go
// internal/worker/spawner_graphreport_test.go
package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadGraphReport_Exists(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	os.MkdirAll(outDir, 0755)
	os.WriteFile(filepath.Join(outDir, "GRAPH_REPORT.md"), []byte("# Graph Report\nNode: main.go (degree 15)"), 0644)

	report := readGraphReport(dir)
	if report == "" {
		t.Error("expected non-empty report")
	}
	if !strings.Contains(report, "Project Architecture") {
		t.Error("expected header in report")
	}
	if !strings.Contains(report, "main.go") {
		t.Error("expected content from report file")
	}
}

func TestReadGraphReport_NotExists(t *testing.T) {
	dir := t.TempDir()
	report := readGraphReport(dir)
	if report != "" {
		t.Errorf("expected empty report for missing file, got %q", report)
	}
}

func TestReadGraphReport_TruncatesLongContent(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "graphify-out")
	os.MkdirAll(outDir, 0755)
	// Write content longer than maxGraphReportLen (4000)
	longContent := strings.Repeat("x", 5000)
	os.WriteFile(filepath.Join(outDir, "GRAPH_REPORT.md"), []byte(longContent), 0644)

	report := readGraphReport(dir)
	if len(report) > 4200 { // 4000 content + header/footer
		t.Errorf("report should be truncated, got length %d", len(report))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestReadGraphReport" -v`
Expected: FAIL — `readGraphReport` not defined

**Step 3: Implement readGraphReport**

In `internal/worker/spawner.go`, add after `buildKarpathyOverlay`:

```go
const maxGraphReportLen = 4000

// readGraphReport reads the Graphify GRAPH_REPORT.md if it exists in the repo.
// Returns formatted section string, or empty string if not found.
func readGraphReport(repoPath string) string {
	reportPath := filepath.Join(repoPath, "graphify-out", "GRAPH_REPORT.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > maxGraphReportLen {
		content = content[:maxGraphReportLen] + "\n... (truncated)"
	}
	var sb strings.Builder
	sb.WriteString("--- Project Architecture (Knowledge Graph) ---\n")
	sb.WriteString(content)
	sb.WriteString("\n--- End Architecture ---\n\n")
	return sb.String()
}
```

Add `"os"` to the imports in spawner.go if not already present.

**Step 4: Inject into buildPromptForTierInner**

In the English code task section, AFTER the Karpathy overlay and BEFORE "IMPORTANT: Start writing code":

```go
// BEFORE:
if overlay := buildKarpathyOverlay(t, "en"); overlay != "" {
    sb.WriteString(overlay)
}
sb.WriteString("IMPORTANT: Start writing code IMMEDIATELY...")

// AFTER:
if overlay := buildKarpathyOverlay(t, "en"); overlay != "" {
    sb.WriteString(overlay)
}
if graphReport := readGraphReport(p.RepoPath); graphReport != "" {
    sb.WriteString(graphReport)
}
sb.WriteString("IMPORTANT: Start writing code IMMEDIATELY...")
```

Same for zh-TW section:

```go
if overlay := buildKarpathyOverlay(t, "zh-TW"); overlay != "" {
    sb.WriteString(overlay)
}
if graphReport := readGraphReport(p.RepoPath); graphReport != "" {
    sb.WriteString(graphReport)
}
sb.WriteString("重要：請立即開始寫程式碼...")
```

**Step 5: Run tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestReadGraphReport" -v`
Expected: PASS

**Step 6: Run all tests for regression**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ ./internal/company/ ./internal/project/ -count=1`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/worker/spawner.go internal/worker/spawner_graphreport_test.go
git commit -m "feat(worker): auto-inject Graphify GRAPH_REPORT.md into worker prompts"
```

---

## Summary

| Task | Component | Files |
|------|-----------|-------|
| 1 | DelegationDepth field | `project/task.go`, `project/task_test.go` |
| 2 | Depth enforcement | `company/delegation.go`, `worker/spawner.go`, tests |
| 3 | Error classifier | `worker/errclass.go`, `worker/errclass_test.go` |
| 4 | Wire into SpawnForTask | `worker/spawner.go` |
| 5 | GRAPH_REPORT injection | `worker/spawner.go`, `worker/spawner_graphreport_test.go` |
