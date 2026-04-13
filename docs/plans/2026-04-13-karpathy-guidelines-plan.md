# Karpathy Guidelines Dynamic Injection — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Dynamically inject targeted behavioral guidelines into worker prompts based on rejection history, so workers avoid repeating the same class of mistakes.

**Architecture:** Add ViolationTags to Rejection struct. Classify rejection output via keyword matching in config package. Spawner reads tags at prompt build time and prepends matching guideline text. Pure additive — no existing behavior changes.

**Tech Stack:** Go 1.23+, existing project/config/worker/company packages

---

## Task 1: Add ViolationTags to Rejection Struct

**Files:**
- Modify: `internal/project/task.go:101-106`

**Step 1: Write the failing test**

```go
// internal/project/task_test.go (append to existing or create)
package project

import (
	"testing"
)

func TestRejection_HasViolationTags(t *testing.T) {
	r := Rejection{
		Stage:         TaskReady,
		RejectorID:    "mgr-1",
		Reason:        "scope creep",
		ViolationTags: []string{"scope_creep", "no_verification"},
	}
	if len(r.ViolationTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(r.ViolationTags))
	}
	if r.ViolationTags[0] != "scope_creep" {
		t.Errorf("expected scope_creep, got %s", r.ViolationTags[0])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/project/ -run "TestRejection_HasViolationTags" -v`
Expected: FAIL — `ViolationTags` field not defined

**Step 3: Add ViolationTags field**

In `internal/project/task.go`, modify `Rejection` struct:

```go
type Rejection struct {
	Stage         TaskStatus `yaml:"stage" json:"stage"`
	RejectorID    string     `yaml:"rejector_id" json:"rejectorId"`
	Reason        string     `yaml:"reason" json:"reason"`
	ViolationTags []string   `yaml:"violation_tags,omitempty" json:"violationTags,omitempty"`
	Timestamp     time.Time  `yaml:"timestamp" json:"timestamp"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/project/ -run "TestRejection_HasViolationTags" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/project/task.go internal/project/task_test.go
git commit -m "feat(project): add ViolationTags to Rejection struct"
```

---

## Task 2: Karpathy Guidelines Map & ClassifyViolations

**Files:**
- Modify: `internal/config/defaults.go` (append after `AutonomousDisallowedTools`)
- Modify: `internal/config/config_test.go` (append tests)

**Step 1: Write the failing tests**

```go
// Append to internal/config/config_test.go

func TestKarpathyGuidelines_AllTagsHaveEntries(t *testing.T) {
	guidelines := KarpathyGuidelines()
	expectedTags := []string{"assumptions", "overengineered", "scope_creep", "no_verification"}
	for _, tag := range expectedTags {
		if _, ok := guidelines[tag]; !ok {
			t.Errorf("missing guideline for tag %q", tag)
		}
	}
}

func TestClassifyViolations_ScopeCreep(t *testing.T) {
	output := "REJECTED: The changes include unrelated reformatting and out of scope modifications to the config parser."
	tags := ClassifyViolations(output)
	found := false
	for _, tag := range tags {
		if tag == "scope_creep" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scope_creep tag, got %v", tags)
	}
}

func TestClassifyViolations_MultipleViolations(t *testing.T) {
	output := "REJECTED: Code is overengineered with unnecessary abstraction layers. Also, no test was written for the new endpoint."
	tags := ClassifyViolations(output)
	if len(tags) < 2 {
		t.Errorf("expected at least 2 tags, got %v", tags)
	}
}

func TestClassifyViolations_NoMatch(t *testing.T) {
	output := "REJECTED: The logic is incorrect, the sorting algorithm returns wrong results."
	tags := ClassifyViolations(output)
	if len(tags) != 0 {
		t.Errorf("expected 0 tags for generic rejection, got %v", tags)
	}
}

func TestClassifyViolations_CaseInsensitive(t *testing.T) {
	output := "REJECTED: Worker made an ASSUMPTION about the API format without checking."
	tags := ClassifyViolations(output)
	found := false
	for _, tag := range tags {
		if tag == "assumptions" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected assumptions tag, got %v", tags)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -run "TestKarpathyGuidelines|TestClassifyViolations" -v`
Expected: FAIL — `KarpathyGuidelines`, `ClassifyViolations` not defined

**Step 3: Implement guidelines map and classifier**

Append to `internal/config/defaults.go` after `MergeSkillProfiles`:

```go
// KarpathyGuidelines returns behavioral guidelines keyed by violation tag.
// Injected into worker prompts when prior rejections match the tag.
// Based on: https://github.com/forrestchang/andrej-karpathy-skills
func KarpathyGuidelines() map[string]string {
	return map[string]string{
		"assumptions": "IMPORTANT: Before writing any code, explicitly state your assumptions about the task requirements. " +
			"If anything is ambiguous, implement the simplest interpretation and note what you assumed. Do NOT silently guess.",
		"overengineered": "IMPORTANT: Write the minimum code that solves exactly what was asked. " +
			"No premature abstractions, no speculative features, no 'just in case' error handling. " +
			"If a simple function works, do not create a class hierarchy.",
		"scope_creep": "IMPORTANT: Only modify code directly related to this task. " +
			"Do NOT improve surrounding code, add comments to unrelated functions, reformat files, " +
			"or refactor code you weren't asked to touch. Surgical precision.",
		"no_verification": "IMPORTANT: Before committing, you MUST verify your changes work. " +
			"Run existing tests, write a quick test for new logic, and confirm the build passes. " +
			"Do NOT commit code you haven't tested.",
	}
}

// violationKeywords maps violation tags to keyword patterns found in rejection output.
var violationKeywords = map[string][]string{
	"assumptions":     {"assumption", "assumed", "misunderstand", "wrong interpretation", "not what was asked", "misread"},
	"overengineered":  {"overengineer", "unnecessary abstraction", "too complex", "bloat", "over-architected", "overkill", "unnecessary"},
	"scope_creep":     {"unrelated change", "scope", "out of scope", "didn't ask", "beyond the task", "unrelated", "not requested"},
	"no_verification": {"no test", "untested", "didn't verify", "missing test", "test fail", "not tested", "without testing"},
}

// ClassifyViolations scans rejection output for keyword patterns and returns matching violation tags.
func ClassifyViolations(output string) []string {
	lower := strings.ToLower(output)
	var tags []string
	for tag, keywords := range violationKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				tags = append(tags, tag)
				break
			}
		}
	}
	return tags
}
```

Also add `"strings"` to the import block in `defaults.go` if not already present.

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -run "TestKarpathyGuidelines|TestClassifyViolations" -v`
Expected: PASS

**Step 5: Run all config tests for regression**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -v -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/config/defaults.go internal/config/config_test.go
git commit -m "feat(config): add Karpathy guidelines map and violation classifier"
```

---

## Task 3: Wire ClassifyViolations into Review Rejection Paths

**Files:**
- Modify: `internal/company/review.go:415-421` (handleDebateResult rejection)
- Modify: `internal/company/review.go:568-574` (HandleReviewResult rejection)

**Step 1: Write the failing test**

```go
// internal/company/review_karpathy_test.go
package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/config"
)

func TestClassifyViolations_IntegrationWithReview(t *testing.T) {
	// Verify the config function works with review-like output
	output := "REJECTED\nThe code has unrelated changes to the logger module and assumed the API returns JSON without checking."
	tags := config.ClassifyViolations(output)

	hasScope := false
	hasAssumptions := false
	for _, tag := range tags {
		if tag == "scope_creep" {
			hasScope = true
		}
		if tag == "assumptions" {
			hasAssumptions = true
		}
	}
	if !hasScope {
		t.Error("expected scope_creep tag")
	}
	if !hasAssumptions {
		t.Error("expected assumptions tag")
	}
}
```

**Step 2: Run test to verify it passes** (this tests the config package integration)

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/company/ -run "TestClassifyViolations_Integration" -v`
Expected: PASS

**Step 3: Wire into handleDebateResult rejection path**

In `internal/company/review.go`, at the rejection path around line 415-421, change:

```go
	// BEFORE:
	t.RejectionCount++
	t.RejectionHistory = append(t.RejectionHistory, project.Rejection{
		Stage:      t.Status,
		RejectorID: "debate-review",
		Reason:     sanitizeForYAML(output),
		Timestamp:  time.Now(),
	})

	// AFTER:
	t.RejectionCount++
	t.RejectionHistory = append(t.RejectionHistory, project.Rejection{
		Stage:         t.Status,
		RejectorID:    "debate-review",
		Reason:        sanitizeForYAML(output),
		ViolationTags: config.ClassifyViolations(output),
		Timestamp:     time.Now(),
	})
```

**Step 4: Wire into HandleReviewResult rejection path**

In `internal/company/review.go`, at the rejection path around line 568-574, change:

```go
	// BEFORE:
	originalTask.RejectionCount++
	originalTask.RejectionHistory = append(originalTask.RejectionHistory, project.Rejection{
		Stage:      originalTask.Status,
		RejectorID: managerWorker.ID,
		Reason:     sanitizeForYAML(output),
		Timestamp:  time.Now(),
	})

	// AFTER:
	originalTask.RejectionCount++
	originalTask.RejectionHistory = append(originalTask.RejectionHistory, project.Rejection{
		Stage:         originalTask.Status,
		RejectorID:    managerWorker.ID,
		Reason:        sanitizeForYAML(output),
		ViolationTags: config.ClassifyViolations(output),
		Timestamp:     time.Now(),
	})
```

Add `"github.com/hanfourmini/aisupervisor/internal/config"` to the import block if not already present.

**Step 5: Verify compilation and tests**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./internal/... && go test ./internal/company/ -count=1`
Expected: Build OK, all tests PASS

**Step 6: Commit**

```bash
git add internal/company/review.go internal/company/review_karpathy_test.go
git commit -m "feat(review): classify violation tags on rejection"
```

---

## Task 4: Inject Guidelines into Worker Prompts

**Files:**
- Modify: `internal/worker/spawner.go:756-764` (English prompt) and `807-812` (zh-TW prompt)
- Create: `internal/worker/spawner_karpathy_test.go`

**Step 1: Write the failing test**

```go
// internal/worker/spawner_karpathy_test.go
package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestBuildKarpathyOverlay_NoHistory(t *testing.T) {
	task := &project.Task{ID: "t1"}
	overlay := buildKarpathyOverlay(task, "en")
	if overlay != "" {
		t.Errorf("expected empty overlay for no rejection history, got %q", overlay)
	}
}

func TestBuildKarpathyOverlay_WithTags(t *testing.T) {
	task := &project.Task{
		ID: "t1",
		RejectionHistory: []project.Rejection{
			{
				Stage:         project.TaskReady,
				RejectorID:    "mgr-1",
				Reason:        "scope creep",
				ViolationTags: []string{"scope_creep", "no_verification"},
				Timestamp:     time.Now(),
			},
		},
	}
	overlay := buildKarpathyOverlay(task, "en")
	if !strings.Contains(overlay, "Behavioral Guidelines") {
		t.Error("expected guidelines header in overlay")
	}
	if !strings.Contains(overlay, "Only modify code directly related") {
		t.Error("expected scope_creep guideline in overlay")
	}
	if !strings.Contains(overlay, "you MUST verify") {
		t.Error("expected no_verification guideline in overlay")
	}
}

func TestBuildKarpathyOverlay_Deduplicates(t *testing.T) {
	task := &project.Task{
		ID: "t1",
		RejectionHistory: []project.Rejection{
			{ViolationTags: []string{"scope_creep"}},
			{ViolationTags: []string{"scope_creep", "assumptions"}},
		},
	}
	overlay := buildKarpathyOverlay(task, "en")
	// "scope_creep" should appear once in the overlay, not twice
	count := strings.Count(overlay, "Only modify code directly related")
	if count != 1 {
		t.Errorf("expected scope_creep guideline once, found %d times", count)
	}
}

func TestBuildKarpathyOverlay_ZhTW(t *testing.T) {
	task := &project.Task{
		ID: "t1",
		RejectionHistory: []project.Rejection{
			{ViolationTags: []string{"assumptions"}},
		},
	}
	overlay := buildKarpathyOverlay(task, "zh-TW")
	if !strings.Contains(overlay, "行為準則") {
		t.Error("expected zh-TW header in overlay")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestBuildKarpathyOverlay" -v`
Expected: FAIL — `buildKarpathyOverlay` not defined

**Step 3: Implement buildKarpathyOverlay**

In `internal/worker/spawner.go`, add after the `worktreePath` method:

```go
// buildKarpathyOverlay generates targeted behavioral guidelines based on rejection history.
// Returns empty string if no violation tags are present.
func buildKarpathyOverlay(t *project.Task, lang string) string {
	if len(t.RejectionHistory) == 0 {
		return ""
	}

	// Collect unique tags across all rejections
	seen := make(map[string]bool)
	for _, r := range t.RejectionHistory {
		for _, tag := range r.ViolationTags {
			seen[tag] = true
		}
	}
	if len(seen) == 0 {
		return ""
	}

	guidelines := config.KarpathyGuidelines()
	var sb strings.Builder

	if lang == "zh-TW" {
		sb.WriteString("--- 行為準則（根據先前審查回饋）---\n")
	} else {
		sb.WriteString("--- Behavioral Guidelines (from prior review feedback) ---\n")
	}

	for tag := range seen {
		if g, ok := guidelines[tag]; ok {
			sb.WriteString(g)
			sb.WriteString("\n\n")
		}
	}

	if lang == "zh-TW" {
		sb.WriteString("--- 準則結束 ---\n\n")
	} else {
		sb.WriteString("--- End Guidelines ---\n\n")
	}

	return sb.String()
}
```

**Step 4: Inject into buildPromptForTierInner**

In `internal/worker/spawner.go`, in the English code task prompt section (around line 758-759), insert the overlay BEFORE the "IMPORTANT: Start writing code" line:

```go
	// BEFORE:
	if lang == "en" {
		sb.WriteString("IMPORTANT: Start writing code IMMEDIATELY...")

	// AFTER:
	if lang == "en" {
		if overlay := buildKarpathyOverlay(t, "en"); overlay != "" {
			sb.WriteString(overlay)
		}
		sb.WriteString("IMPORTANT: Start writing code IMMEDIATELY...")
```

Do the same for the zh-TW section (around line 807):

```go
	// BEFORE:
	} else {
		sb.WriteString("重要：請立即開始寫程式碼...")

	// AFTER:
	} else {
		if overlay := buildKarpathyOverlay(t, "zh-TW"); overlay != "" {
			sb.WriteString(overlay)
		}
		sb.WriteString("重要：請立即開始寫程式碼...")
```

**Step 5: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestBuildKarpathyOverlay" -v`
Expected: PASS

**Step 6: Run all worker tests for regression**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -v -count=1`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/worker/spawner.go internal/worker/spawner_karpathy_test.go
git commit -m "feat(worker): inject Karpathy guidelines based on rejection violation tags"
```

---

## Task 5: Full Integration Test

**Files:**
- Create: `internal/config/karpathy_integration_test.go`

**Step 1: Write integration test**

```go
// internal/config/karpathy_integration_test.go
package config

import (
	"testing"
)

func TestKarpathyGuidelines_FullPipeline(t *testing.T) {
	// 1. Simulate rejection output from a real review
	rejectionOutput := `REJECTED
The implementation has several issues:
1. You assumed the input would always be JSON without verifying the content-type header
2. The code includes an unnecessary abstraction layer with a Strategy pattern for a single discount type
3. Several unrelated files were reformatted and import ordering was changed
4. No tests were written for the new validation logic`

	// 2. Classify violations
	tags := ClassifyViolations(rejectionOutput)

	// 3. Verify all 4 violations detected
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	expected := []string{"assumptions", "overengineered", "scope_creep", "no_verification"}
	for _, e := range expected {
		if !tagSet[e] {
			t.Errorf("expected tag %q not found in %v", e, tags)
		}
	}

	// 4. Verify guidelines exist for all tags
	guidelines := KarpathyGuidelines()
	for _, tag := range tags {
		if g, ok := guidelines[tag]; !ok || g == "" {
			t.Errorf("missing or empty guideline for tag %q", tag)
		}
	}
}
```

**Step 2: Run integration test**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -run "TestKarpathyGuidelines_FullPipeline" -v`
Expected: PASS

**Step 3: Run all tests across affected packages**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ ./internal/project/ ./internal/worker/ ./internal/company/ -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/config/karpathy_integration_test.go
git commit -m "test(config): add Karpathy guidelines full pipeline integration test"
```

---

## Summary

| Task | Component | New/Modified Files |
|------|-----------|-------------------|
| 1 | ViolationTags on Rejection | `project/task.go`, `project/task_test.go` |
| 2 | Guidelines map + classifier | `config/defaults.go`, `config/config_test.go` |
| 3 | Wire into rejection paths | `company/review.go`, `company/review_karpathy_test.go` |
| 4 | Prompt injection | `worker/spawner.go`, `worker/spawner_karpathy_test.go` |
| 5 | Integration test | `config/karpathy_integration_test.go` |
