# P1 Resilience & Observability — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add credential pooling for API key resilience, rejection history compression for token efficiency, and trajectory recording for worker observability.

**Architecture:** Three independent features. Credential pool manages multiple API keys with round-robin selection and cooldown. Rejection compression truncates old rejection reasons in prompts. Trajectory recorder appends events to JSONL files.

**Tech Stack:** Go 1.23+, existing ai/config/worker/company packages

---

## Task 1: Credential Pool — Core Struct and Tests

**Files:**
- Create: `internal/ai/credential_pool.go`
- Create: `internal/ai/credential_pool_test.go`

**Step 1: Write the failing tests**

```go
// internal/ai/credential_pool_test.go
package ai

import (
	"testing"
	"time"
)

func TestNewCredentialPool_RoundRobin(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
		{ID: "key2", APIKey: "sk-bbb", Provider: "anthropic"},
		{ID: "key3", APIKey: "sk-ccc", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}

	// Should cycle through keys in order
	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		cred, err := pool.Select()
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		seen[cred.ID]++
		pool.Release(cred.ID)
	}
	for _, id := range []string{"key1", "key2", "key3"} {
		if seen[id] != 2 {
			t.Errorf("expected key %s selected 2 times, got %d", id, seen[id])
		}
	}
}

func TestNewCredentialPool_LeastUsed(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic", UsageCount: 10},
		{ID: "key2", APIKey: "sk-bbb", Provider: "anthropic", UsageCount: 2},
		{ID: "key3", APIKey: "sk-ccc", Provider: "anthropic", UsageCount: 5},
	}
	pool := NewCredentialPool(creds, StrategyLeastUsed)

	cred, err := pool.Select()
	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if cred.ID != "key2" {
		t.Errorf("expected least-used key2, got %s", cred.ID)
	}
	pool.Release(cred.ID)
}

func TestCredentialPool_SkipsCooledDown(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
		{ID: "key2", APIKey: "sk-bbb", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	// Mark key1 as rate-limited for 1 hour
	pool.MarkRateLimited("key1", 1*time.Hour)

	// Should only return key2 now
	for i := 0; i < 3; i++ {
		cred, err := pool.Select()
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		if cred.ID != "key2" {
			t.Errorf("expected key2 (key1 is cooled down), got %s", cred.ID)
		}
		pool.Release(cred.ID)
	}
}

func TestCredentialPool_MarkRateLimited(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	pool.MarkRateLimited("key1", 50*time.Millisecond)

	// Immediately after marking, should get error (all cooled down)
	_, err := pool.Select()
	if err == nil {
		t.Error("expected error when all credentials are cooled down")
	}

	// Wait for cooldown to expire
	time.Sleep(60 * time.Millisecond)

	cred, err := pool.Select()
	if err != nil {
		t.Fatalf("Select() after cooldown expired: %v", err)
	}
	if cred.ID != "key1" {
		t.Errorf("expected key1 after cooldown, got %s", cred.ID)
	}
	pool.Release(cred.ID)
}

func TestCredentialPool_EmptyReturnsError(t *testing.T) {
	pool := NewCredentialPool(nil, StrategyRoundRobin)
	_, err := pool.Select()
	if err == nil {
		t.Error("expected error for empty pool")
	}
}

func TestCredentialPool_SingleCredential(t *testing.T) {
	creds := []Credential{
		{ID: "only", APIKey: "sk-only", Provider: "openai"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	for i := 0; i < 5; i++ {
		cred, err := pool.Select()
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		if cred.ID != "only" {
			t.Errorf("expected 'only', got %s", cred.ID)
		}
		if cred.APIKey != "sk-only" {
			t.Errorf("expected APIKey 'sk-only', got %s", cred.APIKey)
		}
		pool.Release(cred.ID)
	}
}

func TestCredentialPool_UsageCountIncremented(t *testing.T) {
	creds := []Credential{
		{ID: "key1", APIKey: "sk-aaa", Provider: "anthropic"},
	}
	pool := NewCredentialPool(creds, StrategyRoundRobin)

	cred, _ := pool.Select()
	if cred.UsageCount != 1 {
		t.Errorf("expected UsageCount 1 after Select, got %d", cred.UsageCount)
	}
	pool.Release(cred.ID)

	cred2, _ := pool.Select()
	if cred2.UsageCount != 2 {
		t.Errorf("expected UsageCount 2 after second Select, got %d", cred2.UsageCount)
	}
	pool.Release(cred2.ID)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/ai/ -run "TestNewCredentialPool|TestCredentialPool" -v`
Expected: FAIL — types `Credential`, `CredentialPool`, `StrategyRoundRobin`, etc. not defined

**Step 3: Implement the credential pool**

```go
// internal/ai/credential_pool.go
package ai

import (
	"fmt"
	"sync"
	"time"
)

const (
	StrategyRoundRobin = "round-robin"
	StrategyLeastUsed  = "least-used"
)

// Credential represents a single API key with usage tracking.
type Credential struct {
	ID            string
	APIKey        string
	Provider      string
	UsageCount    int64
	CooldownUntil time.Time
}

// CredentialPool manages a set of API credentials with selection strategies
// and rate-limit cooldown support.
type CredentialPool struct {
	mu       sync.Mutex
	creds    []Credential
	strategy string
	nextIdx  int // for round-robin
}

// NewCredentialPool creates a pool from the given credentials and selection strategy.
// strategy must be StrategyRoundRobin or StrategyLeastUsed.
func NewCredentialPool(creds []Credential, strategy string) *CredentialPool {
	// Defensive copy so callers cannot mutate the internal slice.
	copied := make([]Credential, len(creds))
	copy(copied, creds)
	return &CredentialPool{
		creds:    copied,
		strategy: strategy,
	}
}

// Select picks the next available credential based on the pool's strategy.
// It increments UsageCount on the selected credential.
// Returns an error if the pool is empty or all credentials are in cooldown.
func (p *CredentialPool) Select() (*Credential, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.creds) == 0 {
		return nil, fmt.Errorf("credential pool is empty")
	}

	now := time.Now()

	switch p.strategy {
	case StrategyLeastUsed:
		return p.selectLeastUsed(now)
	default: // round-robin
		return p.selectRoundRobin(now)
	}
}

func (p *CredentialPool) selectRoundRobin(now time.Time) (*Credential, error) {
	n := len(p.creds)
	for i := 0; i < n; i++ {
		idx := (p.nextIdx + i) % n
		if p.creds[idx].CooldownUntil.After(now) {
			continue
		}
		p.nextIdx = (idx + 1) % n
		p.creds[idx].UsageCount++
		cred := p.creds[idx] // return a copy
		return &cred, nil
	}
	return nil, fmt.Errorf("all credentials are in cooldown")
}

func (p *CredentialPool) selectLeastUsed(now time.Time) (*Credential, error) {
	bestIdx := -1
	var bestCount int64
	for i := range p.creds {
		if p.creds[i].CooldownUntil.After(now) {
			continue
		}
		if bestIdx == -1 || p.creds[i].UsageCount < bestCount {
			bestIdx = i
			bestCount = p.creds[i].UsageCount
		}
	}
	if bestIdx == -1 {
		return nil, fmt.Errorf("all credentials are in cooldown")
	}
	p.creds[bestIdx].UsageCount++
	cred := p.creds[bestIdx] // return a copy
	return &cred, nil
}

// MarkRateLimited sets a cooldown period on the credential with the given ID.
// During cooldown, Select() will skip this credential.
func (p *CredentialPool) MarkRateLimited(id string, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.creds {
		if p.creds[i].ID == id {
			p.creds[i].CooldownUntil = time.Now().Add(duration)
			return
		}
	}
}

// Release is called after a request completes. Currently a no-op placeholder
// for future connection-tracking or concurrency limiting.
func (p *CredentialPool) Release(id string) {
	// no-op — placeholder for future concurrency tracking
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/ai/ -run "TestNewCredentialPool|TestCredentialPool" -v`
Expected: PASS — all 7 tests green

**Step 5: Commit**

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor"
git add internal/ai/credential_pool.go internal/ai/credential_pool_test.go
git commit -m "$(cat <<'EOF'
feat(ai): add CredentialPool with round-robin and least-used strategies

Manages multiple API keys with cooldown support for rate-limit resilience.
Select() skips credentials in cooldown, picks by strategy, and increments usage.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Credential Pool — Config Integration and Wiring

**Files:**
- Modify: `internal/config/config.go` (add APIKeys field)
- Create: `internal/config/config_apikeys_test.go`

**Step 1: Write the failing test**

```go
// internal/config/config_apikeys_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_APIKeysArrayParsing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
default_backend: claude-api
api_keys:
  - id: key1
    key: sk-aaa111
    provider: anthropic
  - id: key2
    key: sk-bbb222
    provider: anthropic
  - id: key3
    key: sk-ccc333
    provider: openai
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.APIKeys) != 3 {
		t.Fatalf("expected 3 api_keys, got %d", len(cfg.APIKeys))
	}

	tests := []struct {
		idx      int
		id       string
		key      string
		provider string
	}{
		{0, "key1", "sk-aaa111", "anthropic"},
		{1, "key2", "sk-bbb222", "anthropic"},
		{2, "key3", "sk-ccc333", "openai"},
	}
	for _, tc := range tests {
		ak := cfg.APIKeys[tc.idx]
		if ak.ID != tc.id {
			t.Errorf("[%d] ID: expected %q, got %q", tc.idx, tc.id, ak.ID)
		}
		if ak.Key != tc.key {
			t.Errorf("[%d] Key: expected %q, got %q", tc.idx, tc.key, ak.Key)
		}
		if ak.Provider != tc.provider {
			t.Errorf("[%d] Provider: expected %q, got %q", tc.idx, tc.provider, ak.Provider)
		}
	}
}

func TestConfig_APIKeysEmptyBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
default_backend: claude-api
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.APIKeys) != 0 {
		t.Errorf("expected empty api_keys for backward compat, got %d", len(cfg.APIKeys))
	}
}

func TestConfig_DefaultAPIKeysEmpty(t *testing.T) {
	keys := DefaultAPIKeys()
	if len(keys) != 0 {
		t.Errorf("DefaultAPIKeys should return empty slice, got %d", len(keys))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -run "TestConfig_APIKeys|TestConfig_DefaultAPIKeys" -v`
Expected: FAIL — `APIKeys` field not defined, `DefaultAPIKeys` function not defined

**Step 3: Add APIKeyConfig and APIKeys to Config**

In `internal/config/config.go`, add the `APIKeyConfig` struct after the existing `HumanGateConfig` struct (around line 225):

```go
// APIKeyConfig defines a single API key for credential pooling.
type APIKeyConfig struct {
	ID       string `yaml:"id" json:"id"`
	Key      string `yaml:"key" json:"key"`
	Provider string `yaml:"provider" json:"provider"`
}

// DefaultAPIKeys returns an empty slice — single-key mode by default.
func DefaultAPIKeys() []APIKeyConfig {
	return nil
}
```

In the `Config` struct, add after the `Review` field (around line 37):

```go
	APIKeys              []APIKeyConfig                 `yaml:"api_keys,omitempty" json:"apiKeys,omitempty"`
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -run "TestConfig_APIKeys|TestConfig_DefaultAPIKeys" -v`
Expected: PASS

**Step 5: Run all config tests to confirm no regressions**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/config/ -v`
Expected: PASS — all existing tests still green

**Step 6: Commit**

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor"
git add internal/config/config.go internal/config/config_apikeys_test.go
git commit -m "$(cat <<'EOF'
feat(config): add APIKeys array for credential pooling

Adds APIKeyConfig struct and api_keys field to Config.
Backward compatible: empty api_keys uses existing single-key mode.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Rejection History Compression

**Files:**
- Modify: `internal/worker/spawner.go` (add `compressRejectionHistory` function, wire into `buildPromptForTierInner`)
- Create: `internal/worker/spawner_rejection_compress_test.go`

**Step 1: Write the failing tests**

```go
// internal/worker/spawner_rejection_compress_test.go
package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestCompressRejectionHistory_ShortHistory(t *testing.T) {
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        "Missing error handling in handler.go",
				ViolationTags: []string{"error-handling"},
				Timestamp:     time.Now().Add(-2 * time.Hour),
			},
			{
				Reason:        "Tests not covering edge case",
				ViolationTags: []string{"test-coverage"},
				Timestamp:     time.Now().Add(-1 * time.Hour),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)

	// Under limit, should contain both full reasons
	if !strings.Contains(result, "Missing error handling in handler.go") {
		t.Error("expected first full reason in output")
	}
	if !strings.Contains(result, "Tests not covering edge case") {
		t.Error("expected second full reason in output")
	}
}

func TestCompressRejectionHistory_LongHistoryCompressesOld(t *testing.T) {
	// Create 5 rejections where total text exceeds 3000 chars
	longReason := strings.Repeat("This is a verbose rejection reason with detailed feedback. ", 30) // ~1740 chars each
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        longReason,
				ViolationTags: []string{"style", "naming"},
				Timestamp:     time.Now().Add(-5 * time.Hour),
			},
			{
				Reason:        longReason,
				ViolationTags: []string{"error-handling"},
				Timestamp:     time.Now().Add(-4 * time.Hour),
			},
			{
				Reason:        longReason,
				ViolationTags: []string{"test-coverage"},
				Timestamp:     time.Now().Add(-3 * time.Hour),
			},
			{
				Reason:        "Recent rejection: fix the nil pointer",
				ViolationTags: []string{"bug"},
				Timestamp:     time.Now().Add(-2 * time.Hour),
			},
			{
				Reason:        "Latest rejection: add unit tests",
				ViolationTags: []string{"test-coverage"},
				Timestamp:     time.Now().Add(-1 * time.Hour),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)

	// Last 2 rejections should be present in full
	if !strings.Contains(result, "Recent rejection: fix the nil pointer") {
		t.Error("expected second-to-last rejection in full")
	}
	if !strings.Contains(result, "Latest rejection: add unit tests") {
		t.Error("expected last rejection in full")
	}

	// Old rejections should be compressed
	if !strings.Contains(result, "Previously rejected 3 times") {
		t.Error("expected compressed summary of old rejections")
	}

	// Compressed summary should include violation tags
	if !strings.Contains(result, "style") || !strings.Contains(result, "naming") {
		t.Error("expected violation tags in compressed summary")
	}

	// Full text of old rejections should NOT appear
	if strings.Contains(result, "This is a verbose rejection reason") {
		t.Error("old rejection full text should be compressed away")
	}
}

func TestCompressRejectionHistory_Empty(t *testing.T) {
	task := &project.Task{}
	result := compressRejectionHistory(task, 3000)
	if result != "" {
		t.Errorf("expected empty string for no rejections, got %q", result)
	}
}

func TestCompressRejectionHistory_ExactlyTwoLong(t *testing.T) {
	// With only 2 rejections that exceed the limit, both should still be kept in full
	// because we always keep the last 2
	longReason := strings.Repeat("X", 2000)
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        longReason,
				ViolationTags: []string{"a"},
				Timestamp:     time.Now().Add(-2 * time.Hour),
			},
			{
				Reason:        longReason,
				ViolationTags: []string{"b"},
				Timestamp:     time.Now().Add(-1 * time.Hour),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)

	// Both should be present (they are the last 2)
	if !strings.Contains(result, longReason) {
		t.Error("with only 2 rejections, both should be kept in full")
	}
	// Should NOT contain "Previously rejected"
	if strings.Contains(result, "Previously rejected") {
		t.Error("should not compress when there are only 2 rejections")
	}
}

func TestCompressRejectionHistory_SingleRejection(t *testing.T) {
	task := &project.Task{
		RejectionHistory: []project.Rejection{
			{
				Reason:        "Fix the bug",
				ViolationTags: []string{"bug"},
				Timestamp:     time.Now(),
			},
		},
	}

	result := compressRejectionHistory(task, 3000)
	if !strings.Contains(result, "Fix the bug") {
		t.Error("single rejection should appear in full")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestCompressRejectionHistory" -v`
Expected: FAIL — `compressRejectionHistory` not defined

**Step 3: Implement compressRejectionHistory**

Add the following to `internal/worker/spawner.go`, after the `promptRenderDelay` function (around line 542):

```go
// compressRejectionHistory builds a text summary of a task's rejection history.
// If total Reason text across all rejections fits within maxLen, all reasons are
// returned verbatim. Otherwise, only the last 2 rejections are kept in full and
// older ones are compressed to a one-line summary with violation tags.
func compressRejectionHistory(t *project.Task, maxLen int) string {
	if len(t.RejectionHistory) == 0 {
		return ""
	}

	// Calculate total length of all reason text
	totalLen := 0
	for _, r := range t.RejectionHistory {
		totalLen += len(r.Reason)
	}

	var sb strings.Builder

	// If under limit or 2 or fewer rejections, return all in full
	if totalLen <= maxLen || len(t.RejectionHistory) <= 2 {
		for i, r := range t.RejectionHistory {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(fmt.Sprintf("Rejection %d", i+1))
			if len(r.ViolationTags) > 0 {
				sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(r.ViolationTags, ", ")))
			}
			sb.WriteString(":\n")
			sb.WriteString(r.Reason)
		}
		return sb.String()
	}

	// Over limit with >2 rejections: compress old ones, keep last 2 in full
	oldCount := len(t.RejectionHistory) - 2
	allTags := make(map[string]bool)
	for _, r := range t.RejectionHistory[:oldCount] {
		for _, tag := range r.ViolationTags {
			allTags[tag] = true
		}
	}

	tagList := make([]string, 0, len(allTags))
	for tag := range allTags {
		tagList = append(tagList, tag)
	}
	// Sort for deterministic output
	sort.Strings(tagList)

	sb.WriteString(fmt.Sprintf("Previously rejected %d times for: [%s]", oldCount, strings.Join(tagList, ", ")))

	// Append last 2 in full
	recent := t.RejectionHistory[oldCount:]
	for i, r := range recent {
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Rejection %d", oldCount+i+1))
		if len(r.ViolationTags) > 0 {
			sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(r.ViolationTags, ", ")))
		}
		sb.WriteString(":\n")
		sb.WriteString(r.Reason)
	}

	return sb.String()
}
```

Also add `"sort"` to the imports in `spawner.go` if not already present.

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestCompressRejectionHistory" -v`
Expected: PASS — all 5 tests green

**Step 5: Wire into buildPromptForTierInner**

In `internal/worker/spawner.go`, inside `buildPromptForTierInner`, right before the `var sb strings.Builder` line (line 798), add:

```go
	// Inject compressed rejection history if the task has been rejected before
	rejectionContext := compressRejectionHistory(t, 3000)
```

Then after each `sb.WriteString(t.Prompt)` line in both the English block (around line 809) and the Chinese block (around line 860), add the rejection context injection:

In the English block, after `sb.WriteString(t.Prompt)` (line 809):

```go
		if rejectionContext != "" {
			sb.WriteString("\n\n--- Previous Rejection History ---\n")
			sb.WriteString(rejectionContext)
			sb.WriteString("\n--- End Rejection History ---\n")
		}
```

In the Chinese block, after `sb.WriteString(t.Prompt)` (line 860):

```go
		if rejectionContext != "" {
			sb.WriteString("\n\n--- 過往退回記錄 ---\n")
			sb.WriteString(rejectionContext)
			sb.WriteString("\n--- 退回記錄結束 ---\n")
		}
```

**Step 6: Run full worker tests to verify no regressions**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -v`
Expected: PASS — all existing tests still green

**Step 7: Commit**

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor"
git add internal/worker/spawner.go internal/worker/spawner_rejection_compress_test.go
git commit -m "$(cat <<'EOF'
feat(worker): compress rejection history in prompts for token efficiency

When a task has >2 rejections and total reason text exceeds 3000 chars,
older rejections are compressed to a one-line summary with violation tags.
Last 2 rejections are always kept in full. Injected into task prompts
in both English and Chinese.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Trajectory Recorder — Core Implementation

**Files:**
- Create: `internal/worker/trajectory.go`
- Create: `internal/worker/trajectory_test.go`

**Step 1: Write the failing tests**

```go
// internal/worker/trajectory_test.go
package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrajectoryRecorder_WritesValidJSONL(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	entry := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 14, 10, 30, 0, 0, time.UTC),
		WorkerID:  "worker-1",
		TaskID:    "task-abc",
		Event:     TrajectoryEventSpawn,
		Details:   "spawned claude in tmux session aiworker-worker-1",
	}

	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Read the file and verify JSONL format
	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	lines := strings.TrimSpace(string(data))
	var decoded TrajectoryEntry
	if err := json.Unmarshal([]byte(lines), &decoded); err != nil {
		t.Fatalf("invalid JSON line: %v", err)
	}

	if decoded.WorkerID != "worker-1" {
		t.Errorf("WorkerID: expected worker-1, got %s", decoded.WorkerID)
	}
	if decoded.TaskID != "task-abc" {
		t.Errorf("TaskID: expected task-abc, got %s", decoded.TaskID)
	}
	if decoded.Event != TrajectoryEventSpawn {
		t.Errorf("Event: expected %s, got %s", TrajectoryEventSpawn, decoded.Event)
	}
}

func TestTrajectoryRecorder_MultipleRecordsAppend(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	entries := []TrajectoryEntry{
		{Timestamp: now, WorkerID: "w1", TaskID: "t1", Event: TrajectoryEventSpawn, Details: "first"},
		{Timestamp: now.Add(1 * time.Minute), WorkerID: "w1", TaskID: "t1", Event: TrajectoryEventPromptSent, Details: "second"},
		{Timestamp: now.Add(5 * time.Minute), WorkerID: "w1", TaskID: "t1", Event: TrajectoryEventCompletionDetected, Details: "third"},
	}

	for _, e := range entries {
		if err := rec.Record(e); err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	// Read and count lines
	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var entry TrajectoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
		if entry.Details != entries[i].Details {
			t.Errorf("line %d: expected details %q, got %q", i, entries[i].Details, entry.Details)
		}
	}
}

func TestTrajectoryRecorder_DateBasedFilename(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	// Record on April 14
	e1 := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 14, 23, 59, 0, 0, time.UTC),
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
	}
	if err := rec.Record(e1); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Record on April 15
	e2 := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 15, 0, 1, 0, 0, time.UTC),
		WorkerID:  "w2",
		TaskID:    "t2",
		Event:     TrajectoryEventSpawn,
	}
	if err := rec.Record(e2); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Verify two separate files exist
	f1 := filepath.Join(dir, "2026-04-14.jsonl")
	f2 := filepath.Join(dir, "2026-04-15.jsonl")

	if _, err := os.Stat(f1); os.IsNotExist(err) {
		t.Error("expected 2026-04-14.jsonl to exist")
	}
	if _, err := os.Stat(f2); os.IsNotExist(err) {
		t.Error("expected 2026-04-15.jsonl to exist")
	}
}

func TestTrajectoryRecorder_TokensUsedField(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	entry := TrajectoryEntry{
		Timestamp:  time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		WorkerID:   "w1",
		TaskID:     "t1",
		Event:      TrajectoryEventCompletionDetected,
		Details:    "task completed",
		TokensUsed: 45000,
	}
	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var decoded TrajectoryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.TokensUsed != 45000 {
		t.Errorf("TokensUsed: expected 45000, got %d", decoded.TokensUsed)
	}
}

func TestTrajectoryRecorder_CreatesDirectoryIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "trajectories")
	rec := NewTrajectoryRecorder(dir)

	entry := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
	}
	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Error("expected file to be created in auto-created directory")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestTrajectoryRecorder" -v`
Expected: FAIL — `TrajectoryEntry`, `TrajectoryRecorder`, `NewTrajectoryRecorder`, etc. not defined

**Step 3: Implement the trajectory recorder**

```go
// internal/worker/trajectory.go
package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Trajectory event constants.
const (
	TrajectoryEventSpawn              = "spawn"
	TrajectoryEventPromptSent         = "prompt_sent"
	TrajectoryEventCompletionDetected = "completion_detected"
	TrajectoryEventReviewApproved     = "review_approved"
	TrajectoryEventReviewRejected     = "review_rejected"
)

// TrajectoryEntry is a single event in a worker's execution timeline.
type TrajectoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	WorkerID   string    `json:"worker_id"`
	TaskID     string    `json:"task_id"`
	Event      string    `json:"event"`
	Details    string    `json:"details,omitempty"`
	TokensUsed int64     `json:"tokens_used,omitempty"`
}

// TrajectoryRecorder appends trajectory events to date-based JSONL files.
type TrajectoryRecorder struct {
	mu  sync.Mutex
	dir string
}

// NewTrajectoryRecorder creates a recorder that writes to the given directory.
// Default directory is ~/.local/share/aisupervisor/trajectories/ if dir is empty.
func NewTrajectoryRecorder(dir string) *TrajectoryRecorder {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share", "aisupervisor", "trajectories")
	}
	return &TrajectoryRecorder{dir: dir}
}

// Record appends a single trajectory entry as a JSON line to the date-based file.
func (r *TrajectoryRecorder) Record(entry TrajectoryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("creating trajectory dir: %w", err)
	}

	// Build filename from entry timestamp: YYYY-MM-DD.jsonl
	filename := entry.Timestamp.Format("2006-01-02") + ".jsonl"
	path := filepath.Join(r.dir, filename)

	// Marshal the entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling trajectory entry: %w", err)
	}
	data = append(data, '\n')

	// Append to file (create if not exists)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening trajectory file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing trajectory entry: %w", err)
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestTrajectoryRecorder" -v`
Expected: PASS — all 5 tests green

**Step 5: Commit**

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor"
git add internal/worker/trajectory.go internal/worker/trajectory_test.go
git commit -m "$(cat <<'EOF'
feat(worker): add TrajectoryRecorder for worker observability

Appends worker lifecycle events (spawn, prompt_sent, completion_detected,
review_approved, review_rejected) as JSONL to date-based files in
~/.local/share/aisupervisor/trajectories/. Thread-safe with mutex.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Trajectory Recorder — Wire into Spawner, Monitor, and Review

**Files:**
- Modify: `internal/worker/spawner.go` (add recorder field, setter, record events)
- Modify: `internal/company/review.go` (record review events)
- Create: `internal/worker/spawner_trajectory_test.go`

**Step 1: Write the failing tests**

```go
// internal/worker/spawner_trajectory_test.go
package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpawner_SetTrajectoryRecorder(t *testing.T) {
	s := &Spawner{
		tierConfigs:    make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:  make(map[string]config.SkillProfile),
		skillOverrides: make(map[string]config.SkillProfileOverride),
	}

	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)
	s.SetTrajectoryRecorder(rec)

	if s.trajectoryRecorder == nil {
		t.Error("expected trajectoryRecorder to be set")
	}
}

func TestSpawner_RecordTrajectory_NilRecorderNoOp(t *testing.T) {
	s := &Spawner{
		tierConfigs:    make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:  make(map[string]config.SkillProfile),
		skillOverrides: make(map[string]config.SkillProfileOverride),
	}

	// Should not panic when recorder is nil
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
	})
}

func TestSpawner_RecordTrajectory_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)
	s := &Spawner{
		tierConfigs:        make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:      make(map[string]config.SkillProfile),
		skillOverrides:     make(map[string]config.SkillProfileOverride),
		trajectoryRecorder: rec,
	}

	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: now,
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
		Details:   "test spawn",
	})

	// Verify file was written
	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var entry TrajectoryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Event != TrajectoryEventSpawn {
		t.Errorf("Event: expected spawn, got %s", entry.Event)
	}
	if entry.Details != "test spawn" {
		t.Errorf("Details: expected 'test spawn', got %s", entry.Details)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestSpawner_SetTrajectoryRecorder|TestSpawner_RecordTrajectory" -v`
Expected: FAIL — `trajectoryRecorder` field, `SetTrajectoryRecorder`, `recordTrajectory` not defined

**Step 3: Add trajectory recorder to Spawner**

In `internal/worker/spawner.go`, add the field to the `Spawner` struct (after `useWorktrees`):

```go
	trajectoryRecorder *TrajectoryRecorder
```

Add the setter method after `NewSpawner`:

```go
// SetTrajectoryRecorder sets the trajectory recorder for this spawner.
func (s *Spawner) SetTrajectoryRecorder(rec *TrajectoryRecorder) {
	s.trajectoryRecorder = rec
}

// recordTrajectory is a convenience method that records a trajectory entry if the
// recorder is configured. Logs but does not propagate errors.
func (s *Spawner) recordTrajectory(entry TrajectoryEntry) {
	if s.trajectoryRecorder == nil {
		return
	}
	if err := s.trajectoryRecorder.Record(entry); err != nil {
		log.Printf("WARNING: trajectory record failed: %v", err)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go test ./internal/worker/ -run "TestSpawner_SetTrajectoryRecorder|TestSpawner_RecordTrajectory" -v`
Expected: PASS

**Step 5: Wire recording into spawnForTaskInner**

In `internal/worker/spawner.go`, inside `spawnForTaskInner`, add a "spawn" event after the worker state update block (after line 499 `w.CurrentTaskID = t.ID`):

```go
	// Record trajectory: spawn
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  w.ID,
		TaskID:    t.ID,
		Event:     TrajectoryEventSpawn,
		Details:   fmt.Sprintf("spawned %s in tmux session %s", cliTool, tmuxName),
	})
```

Add a "prompt_sent" event after the `SendKeys` Enter call (after line 493 `s.tmuxClient.SendKeys(tmuxName, 0, 0, "Enter")`):

```go
	// Record trajectory: prompt_sent
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  w.ID,
		TaskID:    t.ID,
		Event:     TrajectoryEventPromptSent,
		Details:   fmt.Sprintf("prompt sent (%d chars)", len(prompt)),
	})
```

**Step 6: Wire review events into HandleReviewResult**

In `internal/company/review.go`, the `ReviewPipeline` needs access to a trajectory recorder. Add a `trajectoryRecorder` field to `ReviewPipeline` (or access it through the manager).

First, add a getter to `Spawner` so the manager can pass the recorder:

In `internal/worker/spawner.go`, add:

```go
// TrajectoryRecorder returns the trajectory recorder, if set.
func (s *Spawner) TrajectoryRecorder() *TrajectoryRecorder {
	return s.trajectoryRecorder
}
```

In `internal/company/review.go`, inside `HandleReviewResult`, add trajectory recording after the "approved" block (after the emit for EventReviewApproved, around line 543):

```go
		// Record trajectory: review_approved
		if rp.mgr.spawner != nil {
			if rec := rp.mgr.spawner.TrajectoryRecorder(); rec != nil {
				_ = rec.Record(worker.TrajectoryEntry{
					Timestamp: time.Now(),
					WorkerID:  managerWorker.ID,
					TaskID:    originalTask.ID,
					Event:     worker.TrajectoryEventReviewApproved,
					Details:   fmt.Sprintf("approved by %s", managerWorker.Name),
				})
			}
		}
```

In the "rejected" block, after the emit for EventReviewRejected (around line 596):

```go
		// Record trajectory: review_rejected
		if rp.mgr.spawner != nil {
			if rec := rp.mgr.spawner.TrajectoryRecorder(); rec != nil {
				_ = rec.Record(worker.TrajectoryEntry{
					Timestamp: time.Now(),
					WorkerID:  managerWorker.ID,
					TaskID:    originalTask.ID,
					Event:     worker.TrajectoryEventReviewRejected,
					Details:   fmt.Sprintf("rejected by %s (attempt %d)", managerWorker.Name, originalTask.RejectionCount),
				})
			}
		}
```

**Step 7: Verify build compiles and all tests pass**

Run: `cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor" && go build ./... && go test ./internal/worker/ -v && go test ./internal/company/ -v`
Expected: PASS — build succeeds, all tests green

**Step 8: Commit**

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor"
git add internal/worker/spawner.go internal/worker/spawner_trajectory_test.go internal/company/review.go
git commit -m "$(cat <<'EOF'
feat(worker): wire TrajectoryRecorder into spawner and review pipeline

Records spawn/prompt_sent events in spawnForTaskInner and
review_approved/review_rejected events in HandleReviewResult.
Nil-safe: no-op when recorder is not configured.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Verification Checklist

After all 5 tasks are complete, run the full verification:

```bash
cd "/Volumes/SATECHI DISK Media/UserFolders/Projects/library/aisupervisor"

# 1. Full build
go build ./...

# 2. All tests
go test ./internal/ai/ -v
go test ./internal/config/ -v
go test ./internal/worker/ -v
go test ./internal/company/ -v

# 3. Full test suite
go test ./...
```

All commands should produce PASS with zero failures.

**New files created:**
- `internal/ai/credential_pool.go` — CredentialPool with round-robin and least-used strategies
- `internal/ai/credential_pool_test.go` — 7 tests for pool behavior
- `internal/config/config_apikeys_test.go` — 3 tests for APIKeys config parsing
- `internal/worker/spawner_rejection_compress_test.go` — 5 tests for compression logic
- `internal/worker/trajectory.go` — TrajectoryRecorder with JSONL output
- `internal/worker/trajectory_test.go` — 5 tests for recording behavior
- `internal/worker/spawner_trajectory_test.go` — 3 tests for spawner integration

**Files modified:**
- `internal/config/config.go` — added `APIKeyConfig` struct, `APIKeys` field, `DefaultAPIKeys()`
- `internal/worker/spawner.go` — added `compressRejectionHistory()`, `trajectoryRecorder` field, recording calls, rejection injection in `buildPromptForTierInner`
- `internal/company/review.go` — added trajectory recording in `HandleReviewResult` for approved/rejected events
