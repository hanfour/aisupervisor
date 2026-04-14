# Karpathy Guidelines — Dynamic Injection Design

> **Source:** [andrej-karpathy-skills](https://github.com/forrestchang/andrej-karpathy-skills) — behavioral guidelines for AI coding agents based on Andrej Karpathy's observations.

**Goal:** Dynamically inject targeted behavioral guidelines into worker prompts based on rejection history, reducing repeat mistakes without wasting tokens on first-run tasks.

**Approach:** Rejection Classifier + Prompt Overlay (Option A — dynamic injection)

---

## Violation Tag System

4 tags derived from Karpathy's 4 principles:

| Tag | Karpathy Principle | Trigger |
|-----|-------------------|---------|
| `assumptions` | Think Before Coding | Worker made wrong assumptions, misunderstood requirements |
| `overengineered` | Simplicity First | Unnecessary abstractions, speculative features, bloated code |
| `scope_creep` | Surgical Changes | Modified unrelated code, reformatted files, drive-by improvements |
| `no_verification` | Goal-Driven Execution | Didn't run tests, submitted untested code, missed verification |

### Classification Logic

Keyword-based scanning of rejection output (no AI call needed):

```go
var violationKeywords = map[string][]string{
    "assumptions":     {"assumption", "assumed", "misunderstand", "wrong interpretation", "not what was asked"},
    "overengineered":  {"overengineer", "unnecessary abstraction", "too complex", "bloat", "over-architected", "overkill"},
    "scope_creep":     {"unrelated change", "scope", "out of scope", "didn't ask", "beyond the task", "unrelated"},
    "no_verification": {"no test", "untested", "didn't verify", "missing test", "test fail", "not tested"},
}
```

### Guideline Templates

Injected into prompt when corresponding tag is present in rejection history:

- **assumptions:** "IMPORTANT: Before writing any code, explicitly state your assumptions about the task requirements. If anything is ambiguous, implement the simplest interpretation and note what you assumed. Do NOT silently guess."
- **overengineered:** "IMPORTANT: Write the minimum code that solves exactly what was asked. No premature abstractions, no speculative features, no 'just in case' error handling. If a simple function works, do not create a class hierarchy."
- **scope_creep:** "IMPORTANT: Only modify code directly related to this task. Do NOT improve surrounding code, add comments to unrelated functions, reformat files, or refactor code you weren't asked to touch. Surgical precision."
- **no_verification:** "IMPORTANT: Before committing, you MUST verify your changes work. Run existing tests, write a quick test for new logic, and confirm the build passes. Do NOT commit code you haven't tested."

---

## Injection Flow

```
1. Worker completes task → review rejection
2. handleDebateResult / HandleReviewResult:
   - Call ClassifyViolations(output) → []string tags
   - Store in task.RejectionHistory[n].ViolationTags
   - SaveTask(t)
3. Task re-assigned → spawner.SpawnForTask()
4. buildPromptForTier():
   - Read all ViolationTags from t.RejectionHistory
   - Deduplicate, lookup KarpathyGuidelines map
   - Inject matching guidelines at prompt top, before task description
```

### Prompt Format

```
--- Behavioral Guidelines (from prior review feedback) ---
⚠ SCOPE: Only modify code directly related to this task...
⚠ VERIFY: Before committing, you MUST verify...
--- End Guidelines ---

IMPORTANT: Start writing code IMMEDIATELY...
[original prompt]
```

### Edge Cases

- First spawn (no rejection history) → no extra text injected
- Multiple rejections accumulate different tags → all injected (deduplicated)
- Approval → next new task starts with clean slate (tags are per-task, not per-worker)

---

## Files Modified

| File | Change |
|------|--------|
| `internal/project/task.go` | Add `ViolationTags []string` to `Rejection` struct |
| `internal/config/defaults.go` | Add `KarpathyGuidelines()` map + `ClassifyViolations()` |
| `internal/company/review.go` | Call `ClassifyViolations` in both rejection paths |
| `internal/worker/spawner.go` | Read `RejectionHistory` tags, inject guidelines in `buildPromptForTier` |
| `internal/config/defaults_test.go` | Test classify logic |
| `internal/worker/spawner_karpathy_test.go` | Test injection logic |

## Not Changed

- personality package — skill scores untouched
- config.yaml — no new config fields needed
- frontend — ViolationTags exposed via existing JSON tags automatically
