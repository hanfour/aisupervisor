# Knowledge Tiers + Debate Review Pipeline Design

**Date:** 2026-04-12
**Status:** Approved
**Origin:** Extracted from [multi-repo-agent](https://github.com/hanfour/multi-repo-agent) PKB and debate review systems

## Goal

1. **Reduce token waste** — Add 4-tier knowledge caching to `knowledge.Injector` so workers receive only the context they need (L0~L3)
2. **Improve review quality** — Replace keyword-based verdict parsing with a structured 3-round debate review pipeline using `ai.ChatProvider`

## A. Knowledge Tier System

### Data Model Changes (`internal/knowledge/types.go`)

Add `KnowledgeTier` to `Entry`:

```go
type KnowledgeTier int

const (
    TierL0Identity   KnowledgeTier = 0 // ~50 tokens: name, type, tech stack
    TierL1Essential  KnowledgeTier = 1 // ~200 tokens: conventions, patterns, decisions
    TierL2RoomRecall KnowledgeTier = 2 // ~500 tokens: architecture, module summaries
    TierL3DeepSearch KnowledgeTier = 3 // ~800+ tokens: full API surface, all modules
)
```

`Entry` gains one field:

```go
Tier KnowledgeTier `yaml:"tier" json:"tier"`
```

### Tier → KnowledgeType Mapping

| Tier | Included KnowledgeTypes |
|------|------------------------|
| L0 | (identity-tagged entries only) |
| L1 | L0 + `decision`, `feedback` |
| L2 | L1 + `architecture`, `gotcha`, `task_summary` |
| L3 | L2 + `lesson_learnt` + all remaining + `FullContent` |

### Injector Changes (`internal/knowledge/injector.go`)

`BuildContext` signature becomes:

```go
func (inj *Injector) BuildContext(workerID, projectID string, tier KnowledgeTier) (string, error)
```

Per-tier cumulative char budgets:

| Tier | Cumulative Budget |
|------|------------------|
| L0 | 250 chars |
| L0+L1 | 1,250 chars |
| L0+L1+L2 | 3,750 chars |
| L0+L1+L2+L3 | 7,500 chars |

At L3, entries include `FullContent` (previously ignored).

### Caller Tier Assignment

| Caller | Tier | Rationale |
|--------|------|-----------|
| PRD / orchestrator worker | L3 | Needs complete project understanding |
| General code task | L2 | Architecture + relevant modules |
| Debate analysis agents | L2 | Need architecture context |
| Debate vote agents | L1 | Only judge findings against conventions |
| Debate synthesizer | none | Only processes findings |

### Backward Compatibility

- Existing entries with no `Tier` field unmarshal as `0` (L0) — safe default
- Existing callers of `BuildContext(workerID, projectID)` must add tier parameter (compile-time catch)
- Default tier for `spawner.go` call site: `TierL2RoomRecall`

## B. Debate Review Pipeline

### Config (`internal/config/config.go`)

New struct in `Config`:

```go
type ReviewConfig struct {
    AnalysisModel    string `yaml:"analysis_model,omitempty"`     // default: "opus"
    VoteModel        string `yaml:"vote_model,omitempty"`         // default: "haiku"
    SynthesisModel   string `yaml:"synthesis_model,omitempty"`    // default: "sonnet"
    DebateThreshold  int    `yaml:"debate_threshold,omitempty"`   // diff lines, default: 300
    LightMaxLines    int    `yaml:"light_max_lines,omitempty"`    // default: 50
    LightMaxFiles    int    `yaml:"light_max_files,omitempty"`    // default: 3
    MaxDebateRetries int    `yaml:"max_debate_retries,omitempty"` // default: 3
    FastConverge     int    `yaml:"fast_converge,omitempty"`      // skip vote if findings <= N, default: 5
}
```

Example `config.yaml`:

```yaml
review:
  analysis_model: opus
  vote_model: haiku
  synthesis_model: sonnet
  debate_threshold: 300
  fast_converge: 5
```

### Strategy Selection

| Strategy | Condition | Behavior |
|----------|-----------|----------|
| `light` | <50 diff lines, <=3 files | Single ChatProvider call |
| `standard` | <300 diff lines | Single detailed ChatProvider call |
| `debate` | >=300 lines OR API change detected | Full 3-round pipeline |

### Data Structures (`internal/company/debate.go`)

```go
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
    Severity string `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM"
    Body     string `json:"body"`
    Source   string `json:"source"`   // "impact" or "quality"
}

type DebateResult struct {
    Status   string    `json:"status"`  // "APPROVED" or "CHANGES_REQUESTED"
    Summary  string    `json:"summary"`
    Comments []Finding `json:"comments"`
}
```

### Debate Pipeline Flow

```
Round 1: Parallel Analysis (2 goroutines, analysis_model)
+-- Agent A (Impact Analyst)
|   input: diff + PKB(L2)
|   output: []Finding
|
+-- Agent B (Quality Auditor)
    input: diff + PKB(L2)
    output: []Finding

Fast Convergence:
  0 findings -> APPROVED
  <= fast_converge -> skip Round 2

Round 2: Mailbox Voting (2 goroutines, vote_model)
+-- Voter 1: input=merged findings pool + PKB(L1), output=KEEP/DROP per finding
+-- Voter 2: same
Tally: finding survives if net score >= 1

Round 3: Synthesis (synthesis_model)
  input: surviving findings
  output: DebateResult JSON
```

### Integration with ReviewPipeline (`internal/company/review.go`)

Key change: when `ChatProvider` is available, reviews run via debate pipeline instead of tmux sub-tasks.

```
executeReview()
  +-- getDiffStats(repoPath, baseBranch, taskBranch)
  +-- selectStrategy(diffLines, fileCount) -> light/standard/debate
  |
  +-- [has ChatProvider] -> runChatReview(strategy, diff, pkbContext)
  |     +-- light/standard: single Chat call -> DebateResult
  |     +-- debate: runDebate() -> 3-round pipeline -> DebateResult
  |
  +-- [no ChatProvider] -> fallback to tmux review sub-task (existing logic)
  |
  +-- handleDebateResult(DebateResult)
        +-- APPROVED -> TaskDone + promote
        +-- CHANGES_REQUESTED (CRITICAL/HIGH) -> structured feedback -> TaskRevision
        +-- MEDIUM-only -> APPROVED with notes
        +-- parse failure -> fallback keyword -> verdictInconclusive -> human gate
```

**Manager workers are no longer occupied for reviews** — debate runs via ChatProvider async calls; the manager is immediately available for other work.

### Verdict Logic

- `CHANGES_REQUESTED` only for CRITICAL or HIGH findings
- MEDIUM-only reviews → `APPROVED` with notes appended to task
- JSON parse failure → fallback to existing `parseReviewVerdict()` keyword parsing

### Rejection Feedback

When rejected, the structured findings list replaces raw tmux output:

```
--- Review Feedback (attempt 1) ---
[CRITICAL] src/handler.go:42 — SQL injection in user input
[HIGH] src/auth.go:15 — Missing token expiry validation

Please address the above feedback and resubmit.
```

### Backward Compatibility

- `ReviewConfig` fields all optional; zero-config uses defaults
- No `ChatProvider` → existing tmux-based review unchanged
- `parseReviewVerdict()` kept as fallback
- Training data capture, personality mood/skill updates, circuit breaker — all unchanged

## Files Changed

| File | Change | Type |
|------|--------|------|
| `internal/knowledge/types.go` | Add `KnowledgeTier`, `Tier` field on Entry | Modify |
| `internal/knowledge/injector.go` | `BuildContext` tier param, layered budgets | Modify |
| `internal/config/config.go` | Add `ReviewConfig` struct + field on Config | Modify |
| `internal/config/defaults.go` | Review defaults | Modify |
| `internal/company/debate.go` | Full debate pipeline | **New** |
| `internal/company/review.go` | Strategy selection, JSON verdict, ChatProvider path | Modify |
| `internal/worker/spawner.go` | `BuildContext` call adds tier param | Modify |
| `internal/company/company.go` | Wire ReviewConfig + ChatProvider to ReviewPipeline | Modify |

## Test Plan

- `debate_test.go` — table-driven:
  - `selectStrategy()` for various diff sizes
  - `mergeFindings()` dedup by file:line
  - `tallyVotes()` vote combinations
  - `parseDebateResult()` JSON extraction + fallback
- `injector_test.go` — tier filtering, budget enforcement
- `review_test.go` — ChatProvider path vs tmux fallback
