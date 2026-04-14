# Hermes + Graphify Enhancements Design

> **Source research:** [hermes-agent](https://github.com/NousResearch/hermes-agent), [Graphify](https://graphify.net/)

**Goal:** Extract high-value patterns from Hermes Agent and Graphify to strengthen aisupervisor's delegation safety, error resilience, context efficiency, and architectural awareness.

**Delivery:** 3 PRs by priority tier (P0 → P1 → P2).

---

## P0: Immediate Safety & Context (PR #1)

### P0-1: Delegation Depth Limit

**Problem:** Manager/Consultant workers can delegate sub-tasks, which could recursively delegate infinitely.

**Design:**
- Add `DelegationDepth int` field to `project.Task`
- When creating sub-tasks in `AssignTask`, set `child.DelegationDepth = parent.DelegationDepth + 1`
- In `buildPromptForTier`, if `DelegationDepth >= MaxDelegationDepth (2)`, omit the "--- Delegation ---" prompt section
- Constant `MaxDelegationDepth = 2` in `company/company.go`

**Files:** `project/task.go`, `company/company.go`, `worker/spawner.go`
**Scope:** ~15 lines modified

---

### P0-2: Error Classification Pipeline

**Problem:** Spawn and review failures are retried uniformly without distinguishing transient vs permanent errors.

**Design:**
- New file `internal/worker/errclass.go`:
  - `ErrorAction` type: `retry`, `rotate`, `abandon`, `compress`
  - `ClassifyError(err error) ErrorAction` — keyword + exit code matching:
    - `"rate limit"`, `"429"`, `"quota"` → retry with backoff
    - `"context length"`, `"too many tokens"` → compress
    - `"invalid api key"`, `"401"`, `"403"` → abandon
    - `"timeout"`, `"connection"`, tmux failure → retry
    - Other → retry (max 3 then abandon)
- Integrate into `SpawnForTask` retry loop and `HandleReviewResult` error path

**Files:** `worker/errclass.go` (new), `worker/spawner.go`, `company/review.go`
**Scope:** ~80 lines new, ~20 lines modified

---

### P0-3: GRAPH_REPORT Auto-Injection

**Problem:** Workers lack architectural context for large projects. Graphify users already have this data on disk.

**Design:**
- In `buildPromptForTier`, before task prompt, check for `<repoPath>/graphify-out/GRAPH_REPORT.md`
- If exists, read content (cap at 4000 chars), inject as `--- Project Architecture (Knowledge Graph) ---` section
- If not exists, skip silently (zero overhead)
- No config needed — pure auto-detection

**Files:** `worker/spawner.go`
**Scope:** ~25 lines new

---

## P1: Resilience & Observability (PR #2)

### P1-4: Credential Pool

**Problem:** Single API key — rate limit blocks all workers, one key failure = total stop.

**Design:**
- New file `internal/ai/credential_pool.go`:
  - `CredentialPool` struct with mutex, credentials slice, selection strategy
  - `Credential` struct: ID, APIKey, Provider, UsageCount, CooldownUntil
  - `Select()` — skips cooled-down keys, selects by strategy (round-robin or least-used)
  - `MarkRateLimited(id)` — sets 1hr cooldown
- `config.yaml` gains `api_keys` array (backward compatible — single key still works)
- Integrates with P0-2: `ActionRotate` triggers `MarkRateLimited` + retry with next key

**Files:** `ai/credential_pool.go` (new), `ai/credential_pool_test.go` (new), `config/config.go`, `company/company.go`
**Scope:** ~120 lines new, ~30 lines modified

---

### P1-5: Rejection History Compression

**Problem:** Workers rejected many times accumulate verbose rejection reasons in prompt, wasting tokens.

**Design:**
- In `buildPromptForTier`, if total `RejectionHistory.Reason` length exceeds 3000 chars:
  - Keep last 2 rejections' full reason text
  - Compress earlier rejections to one line: `"Previously rejected N times for: [violation tags]"`
- Pure rule-based truncation — no AI call needed

**Files:** `worker/spawner.go`
**Scope:** ~40 lines new

---

### P1-6: Trajectory Recording

**Problem:** Cannot analyze worker success/failure patterns or replay decision paths.

**Design:**
- New file `internal/worker/trajectory.go`:
  - `TrajectoryEntry` struct: Timestamp, WorkerID, TaskID, Event, Details, TokensUsed
  - Events: `spawn`, `prompt_sent`, `completion_detected`, `review_approved`, `review_rejected`
  - `TrajectoryRecorder` writes append-only JSONL to `~/.local/share/aisupervisor/trajectories/YYYY-MM-DD.jsonl`
- Call `Record()` in `SpawnForTask`, `WatchForCompletion`, `HandleReviewResult`
- No frontend integration yet (P2 scope)

**Files:** `worker/trajectory.go` (new), `worker/trajectory_test.go` (new), `worker/spawner.go`, `worker/monitor.go`, `company/review.go`
**Scope:** ~80 lines new, ~15 lines modified

---

## P2: Knowledge Graph Intelligence (PR #3)

### P2-7: Knowledge Graph Integration

**Problem:** Task assignment and review escalation lack architectural awareness. Workers don't know which files are central or which modules relate.

**Design — Two-tier approach (Plan C):**

**Tier 1: Built-in Lightweight Graph (zero dependencies)**
- New file `internal/knowledge/codegraph.go`:
  - `BuildLightGraph(repoPath string) (*CodeGraph, error)` — uses `go/ast` to parse import graphs + `git log` co-change frequency
  - `CodeGraph` struct: Nodes (files), Edges (imports + co-changes), Communities, GodNodes
  - Community detection: simple connected-component grouping by import clusters
  - God nodes: files with highest in-degree (most imported)
  - Cached to `<repoPath>/.aisupervisor/codegraph.json`, invalidated on new commits

**Tier 2: Graphify Enhancement (optional, auto-detected)**
- New file `internal/knowledge/graphify.go`:
  - `GraphifyIntegration` struct with `IsAvailable()`, `HasGraph()`, `RunAnalysis()`, `GetReport()`, `GetGodNodes()`, `GetCommunities()`
  - If Graphify CLI installed and `graphify-out/` exists, its richer data overrides the light graph
  - If not installed, light graph is used seamlessly

**Task Assignment Integration:**
- In `AssignTask`, query graph for task's affected files → find community → prefer worker who last handled same community (locality)

**Review Escalation Integration:**
- In review pipeline, if PR touches files across 3+ communities or any god node → auto-escalate to debate review

**GUI Integration:**
- Wails binding `GetProjectGraph()` returns graph data for frontend visualization

**Files:** `knowledge/codegraph.go` (new), `knowledge/graphify.go` (new), `knowledge/codegraph_test.go` (new), `company/company.go`, `company/review.go`, `gui/company_app.go`
**Scope:** ~200 lines new, ~40 lines modified

---

## Summary

| PR | Items | Key Benefit | Est. Scope |
|----|-------|-------------|------------|
| P0 | Delegation depth, Error classification, GRAPH_REPORT inject | Safety + context | ~140 lines |
| P1 | Credential pool, Rejection compression, Trajectory recording | Resilience + observability | ~280 lines |
| P2 | Two-tier knowledge graph | Intelligent delegation + review | ~240 lines |

## Not Changed

- Frontend (except P2-7 Wails binding)
- Existing review pipeline logic (only additions)
- config.yaml schema (backward compatible additions only)
- Personality/skill score system
