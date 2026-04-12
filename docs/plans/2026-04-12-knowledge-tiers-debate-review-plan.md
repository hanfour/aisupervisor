# Knowledge Tiers + Debate Review Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 4-tier knowledge caching to reduce token waste, and replace keyword-based review verdicts with a structured 3-round debate pipeline.

**Architecture:** Extend existing `knowledge.Injector` with tier-based filtering. Add `ai.ModelOverrideChat` interface for per-round model selection. New `debate.go` implements the 3-round pipeline using `ai.ChatProvider`. Review pipeline gains strategy selection (light/standard/debate) based on diff size.

**Tech Stack:** Go, `ai.ChatProvider` interface, `sync.WaitGroup` for parallel rounds, existing `extractChatJSON()`.

---

### Task 1: Add KnowledgeTier to types.go

**Files:**
- Modify: `internal/knowledge/types.go`
- Test: `internal/knowledge/injector_test.go`

**Step 1: Write the failing test**

```go
// internal/knowledge/injector_test.go
package knowledge

import "testing"

func TestTierConstants(t *testing.T) {
	tests := []struct {
		tier KnowledgeTier
		want int
	}{
		{TierL0Identity, 0},
		{TierL1Essential, 1},
		{TierL2RoomRecall, 2},
		{TierL3DeepSearch, 3},
	}
	for _, tt := range tests {
		if int(tt.tier) != tt.want {
			t.Errorf("tier %d != %d", tt.tier, tt.want)
		}
	}
}

func TestTierForType(t *testing.T) {
	tests := []struct {
		kt   KnowledgeType
		want KnowledgeTier
	}{
		{KnowledgeDecision, TierL1Essential},
		{KnowledgeFeedback, TierL1Essential},
		{KnowledgeArchitecture, TierL2RoomRecall},
		{KnowledgeGotcha, TierL2RoomRecall},
		{KnowledgeTaskSummary, TierL2RoomRecall},
		{KnowledgeLessonLearnt, TierL3DeepSearch},
	}
	for _, tt := range tests {
		if got := TierForType(tt.kt); got != tt.want {
			t.Errorf("TierForType(%s) = %d, want %d", tt.kt, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/knowledge/ -run TestTier -v`
Expected: FAIL — `TierL0Identity`, `TierForType` undefined

**Step 3: Write implementation**

Add to `internal/knowledge/types.go`:

```go
type KnowledgeTier int

const (
	TierL0Identity   KnowledgeTier = 0
	TierL1Essential  KnowledgeTier = 1
	TierL2RoomRecall KnowledgeTier = 2
	TierL3DeepSearch KnowledgeTier = 3
)

// TierForType returns the minimum tier that includes a given knowledge type.
func TierForType(kt KnowledgeType) KnowledgeTier {
	switch kt {
	case KnowledgeDecision, KnowledgeFeedback:
		return TierL1Essential
	case KnowledgeArchitecture, KnowledgeGotcha, KnowledgeTaskSummary:
		return TierL2RoomRecall
	case KnowledgeLessonLearnt:
		return TierL3DeepSearch
	default:
		return TierL0Identity
	}
}
```

Add `Tier` field to `Entry`:

```go
type Entry struct {
	// ... existing fields unchanged ...
	Tier KnowledgeTier `yaml:"tier" json:"tier"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/knowledge/ -run TestTier -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/knowledge/types.go internal/knowledge/injector_test.go
git commit -m "feat(knowledge): add KnowledgeTier constants and TierForType mapping"
```

---

### Task 2: Add tier-based BuildContext to Injector

**Files:**
- Modify: `internal/knowledge/injector.go`
- Test: `internal/knowledge/injector_test.go`

**Step 1: Write the failing test**

```go
// internal/knowledge/injector_test.go

func TestBuildContextTierFiltering(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Seed entries at various tiers
	entries := []Entry{
		{ProjectID: "p1", Type: KnowledgeArchitecture, Summary: "Uses hexagonal arch", Relevance: 0.9},
		{ProjectID: "p1", Type: KnowledgeDecision, Summary: "Use YAML for config", Relevance: 0.8},
		{ProjectID: "p1", Type: KnowledgeLessonLearnt, Summary: "Avoid global state", Relevance: 0.7},
	}
	for _, e := range entries {
		store.Add(e)
	}

	inj := NewInjector(store, 2000)

	tests := []struct {
		tier         KnowledgeTier
		wantContains []string
		wantMissing  []string
	}{
		{TierL1Essential, []string{"YAML for config"}, []string{"hexagonal", "global state"}},
		{TierL2RoomRecall, []string{"YAML for config", "hexagonal"}, []string{"global state"}},
		{TierL3DeepSearch, []string{"YAML for config", "hexagonal", "global state"}, nil},
	}

	for _, tt := range tests {
		ctx, _ := inj.BuildContext("", "p1", tt.tier)
		for _, s := range tt.wantContains {
			if !strings.Contains(ctx, s) {
				t.Errorf("tier %d: missing %q in:\n%s", tt.tier, s, ctx)
			}
		}
		for _, s := range tt.wantMissing {
			if strings.Contains(ctx, s) {
				t.Errorf("tier %d: should not contain %q", tt.tier, s)
			}
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/knowledge/ -run TestBuildContextTier -v`
Expected: FAIL — `BuildContext` signature mismatch (missing tier param)

**Step 3: Write implementation**

Update `internal/knowledge/injector.go`:

```go
// tierCharBudget returns cumulative char budget for a given tier.
func tierCharBudget(tier KnowledgeTier) int {
	switch tier {
	case TierL0Identity:
		return 250
	case TierL1Essential:
		return 1250
	case TierL2RoomRecall:
		return 3750
	case TierL3DeepSearch:
		return 7500
	default:
		return 3750
	}
}

func (inj *Injector) BuildContext(workerID, projectID string, tier KnowledgeTier) (string, error) {
	all, err := inj.store.GetAll(workerID, projectID)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", nil
	}

	// Filter by tier: only include entries whose type maps to <= requested tier
	var filtered []Entry
	for _, e := range all {
		if TierForType(e.Type) <= tier {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return "", nil
	}

	type scored struct {
		entry Entry
		score float64
	}
	now := time.Now()
	var items []scored
	for _, e := range filtered {
		recency := recencyScore(e.CreatedAt, now)
		access := math.Log2(float64(e.AccessCount + 1))
		score := e.Relevance*0.5 + recency*0.3 + access*0.2
		items = append(items, scored{e, score})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	charBudget := tierCharBudget(tier)
	// Override with injector max if smaller
	if inj.maxTokenBudget*5 < charBudget {
		charBudget = inj.maxTokenBudget * 5
	}

	var parts []string
	used := 0
	header := "## Project Knowledge\n"
	used += len(header)

	for _, item := range items {
		line := fmt.Sprintf("- [%s] %s", item.entry.Type, item.entry.Summary)
		// At L3, include FullContent if available
		if tier >= TierL3DeepSearch && item.entry.FullContent != "" {
			line += "\n  " + item.entry.FullContent
		}
		if used+len(line)+1 > charBudget {
			break
		}
		parts = append(parts, line)
		used += len(line) + 1
	}

	if len(parts) == 0 {
		return "", nil
	}

	return header + strings.Join(parts, "\n"), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/knowledge/ -run TestBuildContext -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/knowledge/injector.go internal/knowledge/injector_test.go
git commit -m "feat(knowledge): tier-based BuildContext with layered token budgets"
```

---

### Task 3: Update all BuildContext callers

**Files:**
- Modify: `internal/worker/spawner.go` (search for `BuildContext`)
- Modify: any other callers

**Step 1: Find all callers**

Run: `grep -rn 'BuildContext' internal/`

**Step 2: Update each call site to add tier parameter**

`spawner.go` line ~657:
```go
// Before:
knowledgeCtx, err := s.knowledgeInjector.BuildContext(t.AssigneeID, t.ProjectID)
// After:
knowledgeCtx, err := s.knowledgeInjector.BuildContext(t.AssigneeID, t.ProjectID, knowledge.TierL2RoomRecall)
```

For PRD tasks in spawner, use `TierL3DeepSearch`.
For research tasks, use `TierL2RoomRecall`.

**Step 3: Build to verify compilation**

Run: `go build ./internal/...`
Expected: success

**Step 4: Run all tests**

Run: `go test ./internal/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worker/spawner.go
git commit -m "refactor: update BuildContext callers with tier parameter"
```

---

### Task 4: Add ModelOverrideChat interface to ai package

**Files:**
- Modify: `internal/ai/chat.go`
- Modify: `internal/ai/anthropic/chat.go`
- Test: `internal/ai/anthropic/chat_test.go` (if exists, or create)

**Step 1: Write the interface**

Add to `internal/ai/chat.go`:

```go
// ModelOverrideChat is an optional interface that ChatProviders can implement
// to support per-call model overrides. Used by the debate review pipeline
// to use different models per round (analysis=opus, vote=haiku, synthesis=sonnet).
type ModelOverrideChat interface {
	ChatWithModel(ctx context.Context, messages []ChatMessage, model string) (string, error)
}
```

**Step 2: Implement for Anthropic backends**

Add to `internal/ai/anthropic/chat.go`:

```go
func (b *APIBackend) ChatWithModel(ctx context.Context, messages []ai.ChatMessage, model string) (string, error) {
	system, sdkMessages := convertChatMessages(messages)
	resp, err := b.client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(model),
		MaxTokens: 4096,
		System:    system,
		Messages:  sdkMessages,
	})
	if err != nil {
		return "", fmt.Errorf("anthropic chat: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic")
	}
	return resp.Content[0].Text, nil
}

func (b *OAuthBackend) ChatWithModel(ctx context.Context, messages []ai.ChatMessage, model string) (string, error) {
	client, err := b.getClient(ctx)
	if err != nil {
		return "", err
	}
	system, sdkMessages := convertChatMessages(messages)
	resp, err := client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(model),
		MaxTokens: 4096,
		System:    system,
		Messages:  sdkMessages,
	})
	if err != nil {
		return "", fmt.Errorf("anthropic chat: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic")
	}
	return resp.Content[0].Text, nil
}
```

**Step 3: Add helper function for callers**

Add to `internal/ai/chat.go`:

```go
// ChatWithModel calls ChatWithModel if the provider supports it, otherwise
// falls back to Chat (ignoring the model parameter).
func ChatWithModelOrFallback(ctx context.Context, cp ChatProvider, messages []ChatMessage, model string) (string, error) {
	if override, ok := cp.(ModelOverrideChat); ok && model != "" {
		return override.ChatWithModel(ctx, messages, model)
	}
	return cp.Chat(ctx, messages)
}
```

**Step 4: Build and test**

Run: `go build ./internal/ai/...`
Expected: success

**Step 5: Commit**

```bash
git add internal/ai/chat.go internal/ai/anthropic/chat.go
git commit -m "feat(ai): add ModelOverrideChat interface for per-call model selection"
```

---

### Task 5: Add ReviewConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (if exists)

**Step 1: Write the failing test**

```go
// In config test file
func TestReviewConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Review.AnalysisModel != "opus" {
		t.Errorf("default analysis_model = %q, want opus", cfg.Review.AnalysisModel)
	}
	if cfg.Review.VoteModel != "haiku" {
		t.Errorf("default vote_model = %q, want haiku", cfg.Review.VoteModel)
	}
	if cfg.Review.SynthesisModel != "sonnet" {
		t.Errorf("default synthesis_model = %q, want sonnet", cfg.Review.SynthesisModel)
	}
	if cfg.Review.DebateThreshold != 300 {
		t.Errorf("default debate_threshold = %d, want 300", cfg.Review.DebateThreshold)
	}
	if cfg.Review.FastConverge != 5 {
		t.Errorf("default fast_converge = %d, want 5", cfg.Review.FastConverge)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestReviewConfig -v`
Expected: FAIL — `cfg.Review` undefined

**Step 3: Write implementation**

Add struct to `internal/config/config.go`:

```go
type ReviewConfig struct {
	AnalysisModel    string `yaml:"analysis_model,omitempty"`
	VoteModel        string `yaml:"vote_model,omitempty"`
	SynthesisModel   string `yaml:"synthesis_model,omitempty"`
	DebateThreshold  int    `yaml:"debate_threshold,omitempty"`
	LightMaxLines    int    `yaml:"light_max_lines,omitempty"`
	LightMaxFiles    int    `yaml:"light_max_files,omitempty"`
	MaxDebateRetries int    `yaml:"max_debate_retries,omitempty"`
	FastConverge     int    `yaml:"fast_converge,omitempty"`
}
```

Add field to `Config`:
```go
Review ReviewConfig `yaml:"review,omitempty"`
```

Add defaults in `DefaultConfig()`:
```go
Review: ReviewConfig{
	AnalysisModel:    "opus",
	VoteModel:        "haiku",
	SynthesisModel:   "sonnet",
	DebateThreshold:  300,
	LightMaxLines:    50,
	LightMaxFiles:    3,
	MaxDebateRetries: 3,
	FastConverge:     5,
},
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestReviewConfig -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add ReviewConfig with model tiering and debate thresholds"
```

---

### Task 6: Implement debate.go — data structures and strategy selection

**Files:**
- Create: `internal/company/debate.go`
- Test: `internal/company/debate_test.go`

**Step 1: Write failing tests**

```go
// internal/company/debate_test.go
package company

import "testing"

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
	cfg := defaultReviewConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectStrategy(tt.diffLines, tt.fileCount, cfg)
			if got != tt.want {
				t.Errorf("selectStrategy(%d, %d) = %s, want %s", tt.diffLines, tt.fileCount, got, tt.want)
			}
		})
	}
}

func TestMergeFindings(t *testing.T) {
	a := []Finding{
		{File: "main.go", Line: 10, Severity: "HIGH", Body: "issue A", Source: "impact"},
		{File: "main.go", Line: 10, Severity: "MEDIUM", Body: "issue A dup", Source: "quality"},
		{File: "util.go", Line: 5, Severity: "CRITICAL", Body: "issue B", Source: "impact"},
	}
	merged := mergeFindings(a)
	if len(merged) != 2 {
		t.Fatalf("mergeFindings: got %d, want 2 (dedup by file:line)", len(merged))
	}
	// Higher severity wins on dedup
	for _, f := range merged {
		if f.File == "main.go" && f.Severity != "HIGH" {
			t.Errorf("dedup should keep higher severity: got %s", f.Severity)
		}
	}
}

func TestTallyVotes(t *testing.T) {
	findings := []Finding{
		{ID: "#1"}, {ID: "#2"}, {ID: "#3"},
	}
	votes1 := map[string]string{"#1": "KEEP", "#2": "DROP", "#3": "KEEP"}
	votes2 := map[string]string{"#1": "KEEP", "#2": "DROP", "#3": "DROP"}

	survived := tallyVotes(findings, votes1, votes2)
	// #1: 2 KEEP = survives, #2: 2 DROP = dies, #3: 1 KEEP 1 DROP = net 0 = dies
	if len(survived) != 1 || survived[0].ID != "#1" {
		t.Errorf("tallyVotes: got %v, want only #1", survived)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/company/ -run 'TestSelectStrategy|TestMergeFindings|TestTallyVotes' -v`
Expected: FAIL — types undefined

**Step 3: Write implementation**

Create `internal/company/debate.go`:

```go
package company

import (
	"fmt"
	"sort"

	"github.com/hanfourmini/aisupervisor/internal/config"
)

type ReviewStrategy string

const (
	ReviewLight    ReviewStrategy = "light"
	ReviewStandard ReviewStrategy = "standard"
	ReviewDebate   ReviewStrategy = "debate"
)

type Finding struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"`
	Body     string `json:"body"`
	Source   string `json:"source"`
}

type DebateResult struct {
	Status   string    `json:"status"`
	Summary  string    `json:"summary"`
	Comments []Finding `json:"comments"`
}

func defaultReviewConfig() config.ReviewConfig {
	return config.ReviewConfig{
		AnalysisModel:    "opus",
		VoteModel:        "haiku",
		SynthesisModel:   "sonnet",
		DebateThreshold:  300,
		LightMaxLines:    50,
		LightMaxFiles:    3,
		MaxDebateRetries: 3,
		FastConverge:     5,
	}
}

func selectStrategy(diffLines, fileCount int, cfg config.ReviewConfig) ReviewStrategy {
	if diffLines >= cfg.DebateThreshold {
		return ReviewDebate
	}
	if diffLines < cfg.LightMaxLines && fileCount <= cfg.LightMaxFiles {
		return ReviewLight
	}
	return ReviewStandard
}

// severityRank returns a numeric rank for severity comparison (higher = more severe).
func severityRank(s string) int {
	switch s {
	case "CRITICAL":
		return 3
	case "HIGH":
		return 2
	case "MEDIUM":
		return 1
	default:
		return 0
	}
}

// mergeFindings deduplicates findings by file:line, keeping the higher severity.
func mergeFindings(all []Finding) []Finding {
	type key struct {
		file string
		line int
	}
	best := make(map[key]Finding)
	for _, f := range all {
		k := key{f.File, f.Line}
		if existing, ok := best[k]; !ok || severityRank(f.Severity) > severityRank(existing.Severity) {
			best[k] = f
		}
	}

	result := make([]Finding, 0, len(best))
	for _, f := range best {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		return severityRank(result[i].Severity) > severityRank(result[j].Severity)
	})

	// Assign sequential IDs
	for i := range result {
		result[i].ID = fmt.Sprintf("#%d", i+1)
	}
	return result
}

// tallyVotes returns findings that survive the voting round.
// A finding survives if its net score (KEEP=+1, DROP=-1) >= 1.
func tallyVotes(findings []Finding, votes ...map[string]string) []Finding {
	scores := make(map[string]int)
	for _, v := range votes {
		for id, decision := range v {
			switch decision {
			case "KEEP":
				scores[id]++
			case "DROP":
				scores[id]--
			}
		}
	}

	var survived []Finding
	for _, f := range findings {
		if scores[f.ID] >= 1 {
			survived = append(survived, f)
		}
	}
	return survived
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/company/ -run 'TestSelectStrategy|TestMergeFindings|TestTallyVotes' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/company/debate.go internal/company/debate_test.go
git commit -m "feat(debate): add data structures, strategy selection, merge and tally logic"
```

---

### Task 7: Implement debate pipeline — runDebate with ChatProvider

**Files:**
- Modify: `internal/company/debate.go`
- Test: `internal/company/debate_test.go`

**Step 1: Write failing test**

```go
func TestRunDebateFastConverge(t *testing.T) {
	// Mock ChatProvider that returns zero findings
	mock := &mockChatProvider{
		responses: []string{
			`{"findings": []}`, // Agent A
			`{"findings": []}`, // Agent B
		},
	}
	cfg := defaultReviewConfig()
	result, err := runDebate(context.Background(), mock, "diff content", "", cfg, "en")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "APPROVED" {
		t.Errorf("0 findings should APPROVED, got %s", result.Status)
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/company/ -run TestRunDebate -v`
Expected: FAIL — `runDebate` undefined

**Step 3: Write implementation**

Add to `internal/company/debate.go`:

```go
import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
)

// runDebate executes the full 3-round debate review pipeline.
func runDebate(ctx context.Context, cp ai.ChatProvider, diff, pkbContext string, cfg config.ReviewConfig, lang string) (*DebateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Round 1: Parallel Analysis
	var wg sync.WaitGroup
	var findingsA, findingsB []Finding
	var errA, errB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		findingsA, errA = runAnalysisAgent(ctx, cp, diff, pkbContext, "impact", cfg.AnalysisModel, lang)
	}()
	go func() {
		defer wg.Done()
		findingsB, errB = runAnalysisAgent(ctx, cp, diff, pkbContext, "quality", cfg.AnalysisModel, lang)
	}()
	wg.Wait()

	if errA != nil && errB != nil {
		return nil, fmt.Errorf("both analysis agents failed: %w; %w", errA, errB)
	}

	// Merge and dedup
	var allFindings []Finding
	allFindings = append(allFindings, findingsA...)
	allFindings = append(allFindings, findingsB...)
	merged := mergeFindings(allFindings)

	// Fast convergence
	if len(merged) == 0 {
		return &DebateResult{Status: "APPROVED", Summary: "No issues found"}, nil
	}
	if len(merged) <= cfg.FastConverge {
		return runSynthesis(ctx, cp, merged, cfg.SynthesisModel, lang)
	}

	// Round 2: Mailbox Voting
	var votes1, votes2 map[string]string
	var errV1, errV2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		votes1, errV1 = runVoteAgent(ctx, cp, merged, cfg.VoteModel, lang)
	}()
	go func() {
		defer wg.Done()
		votes2, errV2 = runVoteAgent(ctx, cp, merged, cfg.VoteModel, lang)
	}()
	wg.Wait()

	// If voting fails, skip and pass all findings to synthesis
	survived := merged
	if errV1 == nil && errV2 == nil {
		survived = tallyVotes(merged, votes1, votes2)
	}

	if len(survived) == 0 {
		return &DebateResult{Status: "APPROVED", Summary: "All findings dropped by vote"}, nil
	}

	// Round 3: Synthesis
	return runSynthesis(ctx, cp, survived, cfg.SynthesisModel, lang)
}

func runAnalysisAgent(ctx context.Context, cp ai.ChatProvider, diff, pkbContext, role, model, lang string) ([]Finding, error) {
	var systemPrompt, userPrompt string
	if role == "impact" {
		systemPrompt = impactAnalystPrompt(lang)
	} else {
		systemPrompt = qualityAuditorPrompt(lang)
	}
	userPrompt = fmt.Sprintf("Diff:\n```\n%s\n```", diff)
	if pkbContext != "" {
		userPrompt = pkbContext + "\n\n" + userPrompt
	}

	msgs := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result struct {
		Findings []Finding `json:"findings"`
	}
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse analysis: %w", err)
	}
	for i := range result.Findings {
		result.Findings[i].Source = role
	}
	return result.Findings, nil
}

func runVoteAgent(ctx context.Context, cp ai.ChatProvider, findings []Finding, model, lang string) (map[string]string, error) {
	findingsJSON, _ := json.Marshal(findings)
	msgs := []ai.ChatMessage{
		{Role: "system", Content: voteAgentPrompt(lang)},
		{Role: "user", Content: string(findingsJSON)},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result map[string]string
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse votes: %w", err)
	}
	return result, nil
}

func runSynthesis(ctx context.Context, cp ai.ChatProvider, findings []Finding, model, lang string) (*DebateResult, error) {
	findingsJSON, _ := json.Marshal(findings)
	msgs := []ai.ChatMessage{
		{Role: "system", Content: synthesisPrompt(lang)},
		{Role: "user", Content: string(findingsJSON)},
	}
	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	var result DebateResult
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("parse synthesis: %w", err)
	}
	return &result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/company/ -run TestRunDebate -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/company/debate.go internal/company/debate_test.go
git commit -m "feat(debate): implement 3-round debate pipeline with ChatProvider"
```

---

### Task 8: Add debate prompts (bilingual)

**Files:**
- Modify: `internal/company/debate.go`

**Step 1: Write prompt functions**

Add to `internal/company/debate.go`:

```go
func impactAnalystPrompt(lang string) string {
	if lang == "en" {
		return `You are an Impact Analyst reviewing code changes.
Focus on: broken references, deleted API consumers, missing migrations, contract violations.
Search the diff for removed exports, renamed functions, or changed signatures.

Respond with JSON only:
{"findings": [{"file": "path", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "description"}]}`
	}
	return `你是一位影響分析師，負責審查程式碼變更。
重點：斷裂的引用、被刪除的 API 使用者、缺失的 migration、合約違反。
搜尋 diff 中被移除的 export、重新命名的函式、或變更的簽名。

只用 JSON 回應：
{"findings": [{"file": "路徑", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "描述"}]}`
}

func qualityAuditorPrompt(lang string) string {
	if lang == "en" {
		return `You are a Quality Auditor reviewing code changes.
Focus on: code quality, security vulnerabilities, missing tests, error handling, OWASP issues.
Check naming conventions, edge cases, and adherence to project patterns.

Respond with JSON only:
{"findings": [{"file": "path", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "description"}]}`
	}
	return `你是一位品質稽核師，負責審查程式碼變更。
重點：程式碼品質、安全漏洞、缺失的測試、錯誤處理、OWASP 問題。
檢查命名規範、邊界情況、以及是否符合專案模式。

只用 JSON 回應：
{"findings": [{"file": "路徑", "line": 42, "severity": "CRITICAL|HIGH|MEDIUM", "body": "描述"}]}`
}

func voteAgentPrompt(lang string) string {
	if lang == "en" {
		return `You are a review finding validator. For each finding, vote KEEP or DROP.
KEEP = real issue worth fixing. DROP = false positive, nitpick, or already handled.
Respond with JSON only: {"#1": "KEEP", "#2": "DROP", ...}`
	}
	return `你是一位審查結果驗證者。對每個 finding 投票 KEEP 或 DROP。
KEEP = 真正需要修復的問題。DROP = 誤報、吹毛求疵、或已處理。
只用 JSON 回應：{"#1": "KEEP", "#2": "DROP", ...}`
}

func synthesisPrompt(lang string) string {
	if lang == "en" {
		return `Synthesize the following code review findings into a final verdict.
If any CRITICAL or HIGH findings exist, status is "CHANGES_REQUESTED".
If only MEDIUM findings, status is "APPROVED".
Respond with JSON only:
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "one line", "comments": [{"file": "...", "line": 0, "severity": "...", "body": "..."}]}`
	}
	return `將以下程式碼審查結果合成為最終判決。
如果有 CRITICAL 或 HIGH 的 finding，status 為 "CHANGES_REQUESTED"。
如果只有 MEDIUM，status 為 "APPROVED"。
只用 JSON 回應：
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "一行摘要", "comments": [{"file": "...", "line": 0, "severity": "...", "body": "..."}]}`
}
```

**Step 2: Build**

Run: `go build ./internal/company/...`
Expected: success

**Step 3: Commit**

```bash
git add internal/company/debate.go
git commit -m "feat(debate): add bilingual prompts for all debate agents"
```

---

### Task 9: Integrate debate into ReviewPipeline

**Files:**
- Modify: `internal/company/review.go`
- Test: `internal/company/review_test.go` (add test for strategy routing)

**Step 1: Write failing test**

```go
func TestParseDebateResult(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{
			"valid approved",
			`{"status":"APPROVED","summary":"looks good","comments":[]}`,
			"APPROVED", true,
		},
		{
			"wrapped in markdown",
			"```json\n{\"status\":\"CHANGES_REQUESTED\",\"summary\":\"issues\",\"comments\":[{\"file\":\"x.go\",\"severity\":\"HIGH\",\"body\":\"bad\"}]}\n```",
			"CHANGES_REQUESTED", true,
		},
		{
			"garbage", "no json here", "", false,
		},
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
			} else {
				if err == nil {
					t.Error("expected error for garbage input")
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/company/ -run TestParseDebateResult -v`
Expected: FAIL — `parseDebateResult` undefined

**Step 3: Write implementation**

Add to `internal/company/review.go`:

```go
// parseDebateResult extracts a DebateResult from LLM output.
func parseDebateResult(output string) (*DebateResult, error) {
	var result DebateResult
	if err := json.Unmarshal([]byte(output), &result); err == nil && result.Status != "" {
		return &result, nil
	}
	extracted := extractChatJSON(output)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("no valid DebateResult JSON found")
	}
	if result.Status == "" {
		return nil, fmt.Errorf("DebateResult has empty status")
	}
	return &result, nil
}
```

Modify `ReviewPipeline` to accept config and ChatProvider:

```go
type ReviewPipeline struct {
	mu              sync.Mutex
	reviewQueue     []ReviewRequest
	mgr             *Manager
	reviewStartMeta map[string]reviewMeta
	reviewCfg       config.ReviewConfig  // NEW
}
```

Update `newReviewPipeline`:

```go
func newReviewPipeline(mgr *Manager) *ReviewPipeline {
	return &ReviewPipeline{
		mgr:             mgr,
		reviewStartMeta: make(map[string]reviewMeta),
		reviewCfg:       mgr.reviewCfg,  // loaded from config
	}
}
```

Add `getDiffStats` helper:

```go
// getDiffStats returns diff line count and file count for a branch.
func getDiffStats(repoPath, baseBranch, taskBranch string) (diffLines int, fileCount int, diff string, err error) {
	// Get full diff
	cmd := exec.CommandContext(context.Background(), "git", "diff", baseBranch+"..."+taskBranch)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, "", fmt.Errorf("git diff: %w", err)
	}
	diff = string(out)
	diffLines = strings.Count(diff, "\n")

	// Get file count
	cmd2 := exec.CommandContext(context.Background(), "git", "diff", "--name-only", baseBranch+"..."+taskBranch)
	cmd2.Dir = repoPath
	out2, _ := cmd2.Output()
	fileCount = len(strings.Split(strings.TrimSpace(string(out2)), "\n"))
	if strings.TrimSpace(string(out2)) == "" {
		fileCount = 0
	}

	return diffLines, fileCount, diff, nil
}
```

Add ChatProvider-based review path to `executeReview`:

```go
// In executeReview, before creating tmux review sub-task:
if rp.mgr.chatProvider != nil {
	go rp.runChatReview(req, t, p)
	return nil
}
// ... existing tmux sub-task creation below ...
```

Add `runChatReview`:

```go
func (rp *ReviewPipeline) runChatReview(req ReviewRequest, t *project.Task, p *project.Project) {
	baseBranch := p.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	diffLines, fileCount, diff, err := getDiffStats(p.RepoPath, baseBranch, t.BranchName)
	if err != nil {
		log.Printf("debate: getDiffStats failed: %v, falling back to tmux review", err)
		rp.fallbackTmuxReview(req, t, p)
		return
	}

	strategy := selectStrategy(diffLines, fileCount, rp.reviewCfg)
	log.Printf("debate: task=%s strategy=%s (lines=%d files=%d)", t.ID, strategy, diffLines, fileCount)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Build PKB context for review
	var pkbContext string
	if rp.mgr.spawner != nil {
		// Use L2 for analysis, will be downgraded for vote agents inside runDebate
		pkbCtx, _ := rp.mgr.knowledgeInjector.BuildContext("", p.ID, knowledge.TierL2RoomRecall)
		pkbContext = pkbCtx
	}

	var result *DebateResult
	switch strategy {
	case ReviewLight, ReviewStandard:
		result, err = runSinglePassReview(ctx, rp.mgr.chatProvider, diff, pkbContext, rp.reviewCfg, rp.mgr.GetLanguage(), strategy)
	case ReviewDebate:
		result, err = runDebate(ctx, rp.mgr.chatProvider, diff, pkbContext, rp.reviewCfg, rp.mgr.GetLanguage())
	}

	if err != nil {
		log.Printf("debate: failed: %v, falling back to tmux review", err)
		rp.fallbackTmuxReview(req, t, p)
		return
	}

	rp.handleDebateResult(result, req, t, p)
}
```

**Step 4: Run tests**

Run: `go test ./internal/company/ -run 'TestParseDebateResult' -v`
Expected: PASS

Run: `go build ./internal/...`
Expected: success

**Step 5: Commit**

```bash
git add internal/company/review.go internal/company/debate.go
git commit -m "feat(review): integrate debate pipeline into ReviewPipeline with strategy selection"
```

---

### Task 10: Wire ReviewConfig into Manager and add handleDebateResult

**Files:**
- Modify: `internal/company/company.go`
- Modify: `internal/company/review.go`

**Step 1: Add reviewCfg to Manager**

In `company.go` Manager struct, add:
```go
reviewCfg config.ReviewConfig
```

In `New()`, after manager creation:
```go
// reviewCfg is set later via SetReviewConfig or from GUI wiring
```

Add setter:
```go
func (m *Manager) SetReviewConfig(cfg config.ReviewConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reviewCfg = cfg
	m.review.reviewCfg = cfg
}
```

**Step 2: Add handleDebateResult to review.go**

```go
func (rp *ReviewPipeline) handleDebateResult(result *DebateResult, req ReviewRequest, t *project.Task, p *project.Project) {
	approved := result.Status == "APPROVED"

	// Convert to verdict for existing personality/training flow
	var output string
	if approved {
		output = "APPROVED\n" + result.Summary
	} else {
		var sb strings.Builder
		sb.WriteString("REJECTED\n")
		sb.WriteString(result.Summary + "\n\n")
		for _, c := range result.Comments {
			sb.WriteString(fmt.Sprintf("[%s] %s:%d — %s\n", c.Severity, c.File, c.Line, c.Body))
		}
		output = sb.String()
	}

	// Emit review event
	rp.mgr.emit(Event{
		Type:      EventReviewStarted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		Message:   rp.mgr.msgf("Debate review for task %q: %s (%d findings)", "任務 %q 的辯論審查：%s（%d 個發現）", t.Title, result.Status, len(result.Comments)),
	})

	// Reuse existing HandleReviewResult flow by constructing a fake worker.CompletionResult
	// Instead, directly invoke the approved/rejected logic from HandleReviewResult:

	// Capture training data
	rp.captureTrainingData(t, nil, p, output, approved)

	// Update personality
	if rp.mgr.personalityStore != nil && t.AssigneeID != "" {
		rp.mgr.personalityStore.UpdateProfile(t.AssigneeID, func(profile *personality.CharacterProfile) {
			if approved {
				personality.ApplyEvent(profile, personality.EventReviewApproved)
				personality.ApplySkillEvent(&profile.SkillScores, personality.SkillEventReviewApproved)
				profile.TasksCompleted++
				if profile.TasksCompleted%10 == 0 {
					personality.DecayTowardBaseline(&profile.SkillScores)
				}
			} else {
				personality.ApplyEvent(profile, personality.EventReviewRejected)
				skillEvent := personality.ClassifyRejectionType(output)
				personality.ApplySkillEvent(&profile.SkillScores, skillEvent)
			}
			personality.UpdateAutoMood(profile)
		})
	}

	if approved {
		rp.mgr.projectStore.UpdateTaskStatus(t.ID, project.TaskDone)
		rp.mgr.emit(Event{
			Type:      EventReviewApproved,
			ProjectID: p.ID,
			TaskID:    t.ID,
			Message:   rp.mgr.msgf("Task %q approved by debate review", "任務 %q 已由辯論審查核准", t.Title),
		})
		promoted, _ := rp.mgr.projectStore.PromoteReady(p.ID)
		for _, pt := range promoted {
			rp.mgr.emit(Event{
				Type:      EventTaskCreated,
				ProjectID: p.ID,
				TaskID:    pt.ID,
				Message:   rp.mgr.msgf("Task %q is now ready", "任務 %q 已就緒", pt.Title),
			})
		}
		if len(promoted) > 0 {
			go rp.mgr.engageIdleManagers(context.Background(), p.ID)
			go rp.mgr.drainReadyQueue(context.Background())
		}
		go rp.mgr.checkProjectCompletion(p.ID)
	} else {
		t.RejectionCount++
		t.RejectionHistory = append(t.RejectionHistory, project.Rejection{
			Stage:      t.Status,
			RejectorID: "debate-review",
			Reason:     sanitizeForYAML(output),
			Timestamp:  time.Now(),
		})

		cb := rp.mgr.circuitBreaker
		if cb.CheckBounceLoop(t, "debate-review", t.AssigneeID) || project.ShouldEscalate(t) {
			cb.RecordBounce(t, "debate-review", t.AssigneeID, t.Status, "debate bounce loop")
			cb.Escalate(t, fmt.Sprintf("debate: %d rejections", t.RejectionCount))
			rp.mgr.projectStore.SaveTask(t)
			return
		}
		cb.RecordBounce(t, "debate-review", t.AssigneeID, t.Status, sanitizeForYAML(output))

		rp.mgr.projectStore.UpdateTaskStatus(t.ID, project.TaskRevision)

		// Structured feedback
		basePrompt := t.Prompt
		if idx := strings.Index(basePrompt, "\n\n--- Review Feedback ---\n"); idx != -1 {
			basePrompt = basePrompt[:idx]
		}
		if idx := strings.Index(basePrompt, "\n\n--- 審查回饋 ---\n"); idx != -1 {
			basePrompt = basePrompt[:idx]
		}
		if rp.mgr.GetLanguage() == "en" {
			t.Prompt = fmt.Sprintf("%s\n\n--- Review Feedback (attempt %d) ---\n%s\n\nPlease address the above feedback and resubmit.", basePrompt, t.RejectionCount, output)
		} else {
			t.Prompt = fmt.Sprintf("%s\n\n--- 審查回饋（第 %d 次）---\n%s\n\n請針對以上回饋進行修改後重新提交。", basePrompt, t.RejectionCount, output)
		}
		t.Status = project.TaskReady
		rp.mgr.projectStore.SaveTask(t)

		rp.mgr.emit(Event{
			Type:      EventReviewRejected,
			ProjectID: p.ID,
			TaskID:    t.ID,
			Message:   rp.mgr.msgf("Task %q rejected by debate (%d/%d)", "任務 %q 被辯論審查退回（%d/%d）", t.Title, t.RejectionCount, project.MaxRejectionsBeforeEscalation),
		})

		// Auto-assign back
		if t.AssigneeID != "" {
			rp.mgr.mu.RLock()
			eng, ok := rp.mgr.workers[t.AssigneeID]
			rp.mgr.mu.RUnlock()
			if ok && eng.Status == worker.WorkerIdle {
				go func() {
					rp.mgr.AssignTask(context.Background(), eng.ID, t.ID)
				}()
			}
		}
	}
}
```

**Step 2: Add fallbackTmuxReview**

```go
func (rp *ReviewPipeline) fallbackTmuxReview(req ReviewRequest, t *project.Task, p *project.Project) {
	managerWorker, ok := rp.mgr.GetManager(req.EngineerID)
	if !ok {
		return
	}
	// Original tmux-based review path
	rp.executeReviewTmux(context.Background(), req, managerWorker, t, p)
}
```

Rename current `executeReview` body (tmux path) to `executeReviewTmux`.

**Step 3: Build and run all tests**

Run: `go build ./internal/...`
Run: `go test ./internal/company/ ./internal/knowledge/ ./internal/config/`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/company/company.go internal/company/review.go internal/company/debate.go
git commit -m "feat(review): wire debate pipeline into Manager with handleDebateResult and fallback"
```

---

### Task 11: Add runSinglePassReview for light/standard strategies

**Files:**
- Modify: `internal/company/debate.go`
- Test: `internal/company/debate_test.go`

**Step 1: Write failing test**

```go
func TestRunSinglePassReview(t *testing.T) {
	mock := &mockChatProvider{
		responses: []string{
			`{"status":"APPROVED","summary":"clean code","comments":[]}`,
		},
	}
	cfg := defaultReviewConfig()
	result, err := runSinglePassReview(context.Background(), mock, "diff", "", cfg, "en", ReviewLight)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "APPROVED" {
		t.Errorf("got %s, want APPROVED", result.Status)
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Write implementation**

```go
func runSinglePassReview(ctx context.Context, cp ai.ChatProvider, diff, pkbContext string, cfg config.ReviewConfig, lang string, strategy ReviewStrategy) (*DebateResult, error) {
	model := cfg.AnalysisModel
	if strategy == ReviewLight {
		model = cfg.SynthesisModel // lighter model for light reviews
	}

	systemPrompt := singlePassReviewPrompt(lang, strategy)
	userPrompt := fmt.Sprintf("Diff:\n```\n%s\n```", diff)
	if pkbContext != "" {
		userPrompt = pkbContext + "\n\n" + userPrompt
	}

	msgs := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return nil, err
	}

	return parseDebateResult(text)
}

func singlePassReviewPrompt(lang string, strategy ReviewStrategy) string {
	detail := "Be thorough."
	if strategy == ReviewLight {
		detail = "Be concise — this is a small change."
	}
	if lang == "en" {
		return fmt.Sprintf(`Review the following code diff. %s
Check: correctness, security, error handling, test coverage.
Only flag CRITICAL or HIGH issues for rejection. MEDIUM issues get APPROVED with notes.

Respond with JSON only:
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "one line", "comments": [{"file": "...", "line": 0, "severity": "CRITICAL|HIGH|MEDIUM", "body": "..."}]}`, detail)
	}
	if strategy == ReviewLight {
		detail = "請簡潔 — 這是小幅變更。"
	} else {
		detail = "請仔細審查。"
	}
	return fmt.Sprintf(`審查以下程式碼 diff。%s
檢查：正確性、安全性、錯誤處理、測試覆蓋率。
只有 CRITICAL 或 HIGH 問題才退回。MEDIUM 問題給予 APPROVED 並附註。

只用 JSON 回應：
{"status": "APPROVED|CHANGES_REQUESTED", "summary": "一行摘要", "comments": [{"file": "...", "line": 0, "severity": "CRITICAL|HIGH|MEDIUM", "body": "..."}]}`, detail)
}
```

**Step 4: Run test — PASS**

**Step 5: Commit**

```bash
git add internal/company/debate.go internal/company/debate_test.go
git commit -m "feat(debate): add single-pass review for light/standard strategies"
```

---

### Task 12: Wire ReviewConfig from GUI startup

**Files:**
- Modify: `internal/gui/company_app.go` (or wherever config is loaded and passed to Manager)

**Step 1: Find wiring point**

Run: `grep -rn 'SetReviewConfig\|reviewCfg\|Review.*Config' internal/gui/`

**Step 2: Add wiring**

After `Manager.New()`, call:
```go
m.SetReviewConfig(cfg.Review)
```

**Step 3: Build**

Run: `go build ./cmd/aisupervisor-gui/...`
Expected: success

**Step 4: Commit**

```bash
git add internal/gui/company_app.go
git commit -m "feat(gui): wire ReviewConfig from config.yaml into Manager"
```

---

### Task 13: Final integration test and cleanup

**Files:**
- Test: `internal/company/debate_test.go`

**Step 1: Write full round-trip test**

```go
func TestDebateFullPipeline(t *testing.T) {
	mock := &mockChatProvider{
		responses: []string{
			// Agent A: impact
			`{"findings": [{"file": "main.go", "line": 10, "severity": "HIGH", "body": "SQL injection"}]}`,
			// Agent B: quality
			`{"findings": [{"file": "main.go", "line": 10, "severity": "HIGH", "body": "SQL injection dup"}, {"file": "util.go", "line": 5, "severity": "MEDIUM", "body": "naming"}]}`,
			// Voter 1
			`{"#1": "KEEP", "#2": "DROP"}`,
			// Voter 2
			`{"#1": "KEEP", "#2": "DROP"}`,
			// Synthesizer
			`{"status": "CHANGES_REQUESTED", "summary": "SQL injection found", "comments": [{"file": "main.go", "line": 10, "severity": "HIGH", "body": "SQL injection"}]}`,
		},
	}
	cfg := defaultReviewConfig()
	cfg.FastConverge = 1 // force voting round

	result, err := runDebate(context.Background(), mock, "big diff...", "", cfg, "en")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "CHANGES_REQUESTED" {
		t.Errorf("status = %q, want CHANGES_REQUESTED", result.Status)
	}
	if len(result.Comments) == 0 {
		t.Error("expected comments")
	}
}
```

**Step 2: Run full test suite**

Run: `go test ./internal/... -v -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add internal/company/debate_test.go
git commit -m "test(debate): add full round-trip integration test"
```

---

Plan complete and saved. Two execution options:

**1. Subagent-Driven (this session)** — I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open new session with executing-plans, batch execution with checkpoints

Which approach?