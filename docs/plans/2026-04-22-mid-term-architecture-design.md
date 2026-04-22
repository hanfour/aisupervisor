# Mid-Term Architecture Design — Communication, Meetings, Pluggable Backends

> Date: 2026-04-22
> Status: Approved
> Delivery: Three independent branches, sequential: Phase 1 → Phase 2 → Phase 3

## Overview

Three architectural upgrades designed together for consistency, delivered sequentially by dependency order:

1. **Phase 1: Inter-Worker Communication** — Mailbox + sync Q&A + message injection
2. **Phase 2: Structured Meeting System** — Meeting framework + Review/Planning/Debug scenarios
3. **Phase 3: Pluggable Agent Backends** — AgentRuntime interface + Claude CLI / ais-agent / Aider plugins

---

## Phase 1: Inter-Worker Communication

### Problem

Workers are fully isolated. The only "communication" is help requests that create sub-tasks. No direct messaging, no Q&A, no persistent message history.

### Existing Foundation

- `StructuredMessage` in `events.go` — From/To/Type/Priority/Content
- `CommunicationMatrix` in `comm_matrix.go` — hierarchical routing rules
- `EventHelpRequested` handler — creates research sub-tasks

### 1.1 Mailbox Persistent Layer

New file: `internal/company/mailbox.go`

```go
type Envelope struct {
    StructuredMessage
    Status    EnvelopeStatus `yaml:"status" json:"status"`
    ReadAt    *time.Time     `yaml:"read_at,omitempty" json:"readAt,omitempty"`
    ReplyToID string         `yaml:"reply_to_id,omitempty" json:"replyToId,omitempty"`
    ThreadID  string         `yaml:"thread_id,omitempty" json:"threadId,omitempty"`
    TTL       time.Duration  `yaml:"ttl,omitempty" json:"ttl,omitempty"`
}

type EnvelopeStatus string
const (
    EnvPending   EnvelopeStatus = "pending"
    EnvDelivered EnvelopeStatus = "delivered"
    EnvRead      EnvelopeStatus = "read"
    EnvExpired   EnvelopeStatus = "expired"
)

type Mailbox struct {
    mu       sync.RWMutex
    inbox    map[string][]Envelope  // workerID → messages
    filePath string                 // mailbox.yaml
}
```

Methods:
- `NewMailbox(dataDir) (*Mailbox, error)` — load/create mailbox.yaml
- `Send(env Envelope) error` — validate via CommunicationMatrix, deliver to inbox
- `Peek(workerID) []Envelope` — view pending without status change
- `Deliver(workerID) []Envelope` — take pending, mark delivered
- `MarkRead(messageID)` — mark read
- `Reply(originalID, reply) error` — set ReplyToID + ThreadID
- `GetThread(threadID) []Envelope` — full conversation thread
- `Expire(maxAge) int` — cleanup
- `Save() error` — persist to YAML

Routing: `Send()` calls `CommunicationMatrix.CanCommunicate()`. Cross-tier messages auto-route through manager.

Storage: `~/.local/share/aisupervisor/company/mailbox.yaml`

### 1.2 Synchronous Q&A

Workers use keyword patterns detected by `CompletionMonitor`:

- `ASK:{workerID}:{question}` — request sync answer
- `REPLY:{messageID}:{answer}` — respond to question

Flow:
1. Monitor detects `ASK:` → calls `Manager.handleSyncAsk(sender, targetID, question)`
2. Target idle? → inject question to target pane, watch for `REPLY:`
3. Target busy? → fallback to async Mailbox, notify sender
4. Reply detected → inject back to sender pane

```go
func (m *Manager) handleSyncAsk(sender *worker.Worker, targetID, question string)
func (m *Manager) watchForReply(sender, target *worker.Worker, timeout time.Duration)
```

### 1.3 Message Injection

Three injection points:

| Trigger | When | What |
|---------|------|------|
| Task Spawn | `SpawnForTask()` | Append pending messages to prompt |
| Task Completion | `handleTaskCompletion()` | Idle worker processes inbox |
| Idle Poll | Background goroutine, 30s | Inject new messages to idle workers |

```go
func (s *Spawner) injectPendingMessages(w *worker.Worker, mb *Mailbox) string
func (m *Manager) processIdleMailbox(w *worker.Worker)
```

Background poller in Manager also runs `Expire(1*time.Hour)` every 10 minutes.

### Phase 1 Files

New:
- `internal/company/mailbox.go` (~300 lines)
- `internal/company/mailbox_test.go` (~400 lines)

Modified:
- `internal/company/company.go` — mailbox field, processIdleMailbox, handleSyncAsk, handleReply, background poller
- `internal/company/events.go` — EventMessageSent, EventMessageDelivered, EventMessageRead
- `internal/worker/monitor.go` — ASK/REPLY pattern detection
- `internal/worker/spawner.go` — injectPendingMessages
- `frontend/src/lib/stores/i18n.js` — message event translations

---

## Phase 2: Structured Meeting System

### Problem

No mechanism for multiple workers to collaboratively discuss, debate, and reach consensus on projects, reviews, or incidents. Debate is 2-agent only; council is non-interactive.

### Existing Foundation

- `debate.go` — 3-round consensus (analysis → voting → synthesis)
- `council.go` — parallel expert dispatch + Carmack Filter
- `Mailbox` from Phase 1 — meeting notifications
- `CommunicationMatrix` — participant eligibility

### 2.1 Meeting Framework

New file: `internal/company/meeting.go`

```go
type MeetingType string
const (
    MeetingReview   MeetingType = "review"
    MeetingPlanning MeetingType = "planning"
    MeetingDebug    MeetingType = "debug"
)

type MeetingStatus string
const (
    MeetingScheduled  MeetingStatus = "scheduled"
    MeetingInProgress MeetingStatus = "in_progress"
    MeetingCompleted  MeetingStatus = "completed"
    MeetingCancelled  MeetingStatus = "cancelled"
)

type Meeting struct {
    ID            string         `yaml:"id" json:"id"`
    Type          MeetingType    `yaml:"type" json:"type"`
    Title         string         `yaml:"title" json:"title"`
    Status        MeetingStatus  `yaml:"status" json:"status"`
    ProjectID     string         `yaml:"project_id" json:"projectId"`
    TaskID        string         `yaml:"task_id,omitempty" json:"taskId,omitempty"`
    ChairID       string         `yaml:"chair_id" json:"chairId"`
    Participants  []string       `yaml:"participants" json:"participants"`
    Rounds        []MeetingRound `yaml:"rounds" json:"rounds"`
    Verdict       string         `yaml:"verdict,omitempty" json:"verdict,omitempty"`
    Summary       string         `yaml:"summary,omitempty" json:"summary,omitempty"`
    Agenda        []string       `yaml:"agenda,omitempty" json:"agenda,omitempty"`
    MaxRounds     int            `yaml:"max_rounds" json:"maxRounds"`
    CreatedAt     time.Time      `yaml:"created_at" json:"createdAt"`
    CompletedAt   *time.Time     `yaml:"completed_at,omitempty" json:"completedAt,omitempty"`
}

type MeetingRound struct {
    Number    int      `yaml:"number" json:"number"`
    Speeches  []Speech `yaml:"speeches" json:"speeches"`
    Consensus string   `yaml:"consensus,omitempty" json:"consensus,omitempty"`
}

type Speech struct {
    WorkerID  string    `yaml:"worker_id" json:"workerId"`
    Role      string    `yaml:"role" json:"role"`
    Content   string    `yaml:"content" json:"content"`
    Vote      string    `yaml:"vote,omitempty" json:"vote,omitempty"`
    Findings  []Finding `yaml:"findings,omitempty" json:"findings,omitempty"`
    Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

type MeetingEngine struct {
    chatProvider ai.ChatProvider
    mailbox      *Mailbox
    spawner      *worker.Spawner
    tmuxClient   tmux.TmuxClient
    language     string
    store        *MeetingStore
}

type MeetingStore struct {
    mu       sync.RWMutex
    meetings map[string]*Meeting
    filePath string
}
```

Core methods:
- `Schedule(req) (*Meeting, error)` — create scheduled meeting
- `Start(ctx, meetingID) error` — verify participants, notify via Mailbox
- `RunRound(ctx, meeting) (*MeetingRound, error)` — collect speeches, check consensus
- `Synthesize(ctx, meeting) (verdict, summary, error)` — AI-powered final synthesis
- `Cancel(meetingID) error`

Generic flow: Schedule → Start → RunRound × MaxRounds (exit early on consensus) → Synthesize → Complete

Speech collection modes:
- **API mode**: `ChatProvider.Chat()` with worker personality as system prompt (Planning meetings)
- **CLI mode**: spawn temp tmux session with tool access (Review and Debug meetings)

Consensus: `checkConsensus(speeches, threshold)` — Review: 2/3, Planning: 1/2, Debug: 1/2

### 2.2 Three Meeting Scenarios

#### Review Consensus Meeting

Trigger: council review with ambiguous result, or manual scheduling.

- Round 1: Independent review (parallel, CLI mode) → findings + vote
- Round 2: Debate on divergent votes (serial, API mode) → revised votes
- Round 3: Final vote → majority decision (2/3 threshold)

Complements council (automated, non-interactive) with human-like deliberation for edge cases.

#### Planning Meeting

Trigger: new project creation, or manual scheduling.

- Round 1: Independent proposals (parallel, API mode) → task lists + estimates
- Round 2: Cross-evaluate proposals (serial, API mode) → merge ideas
- Round 3: Finalize plan → output `[]project.Task` written to ProjectStore

#### Debug Meeting

Trigger: CircuitBreaker bounce detection, or manual scheduling.

- Round 1: Independent diagnosis (parallel, CLI mode) → root cause hypotheses
- Round 2: Evaluate hypotheses (serial, API mode) → vote on most likely cause
- Round 3: Action plan → output fix Tasks written to ProjectStore

CircuitBreaker integration: bounce detected → try Debug Meeting first → still failing → human gate.

### Phase 2 Files

New:
- `internal/company/meeting.go` (~500 lines)
- `internal/company/meeting_review.go` (~200 lines)
- `internal/company/meeting_planning.go` (~200 lines)
- `internal/company/meeting_debug.go` (~200 lines)
- `internal/company/meeting_test.go` (~500 lines)

Modified:
- `internal/company/company.go` — meetingEngine field + init
- `internal/company/events.go` — EventMeetingScheduled/Started/RoundCompleted/Completed
- `internal/company/review.go` — council ambiguity → trigger Review Meeting
- `internal/gui/company_app.go` — Wails bindings: ScheduleMeeting, GetMeeting, ListMeetings
- `frontend/src/lib/stores/i18n.js` — meeting event translations

---

## Phase 3: Pluggable Agent Backends

### Problem

`spawner.go` hard-codes Claude Code CLI launch, ready detection, prompt sending. Worker struct has `CLITool` and `BackendID` fields but only "claude" is actually implemented. No way to use Aider, ais-agent, or other runtimes without forking spawner logic.

### Existing Foundation

- `agent/provider.go` — `Provider` interface (Chat-level)
- `ai/chat.go` — `ChatProvider` interface
- `ai/backend.go` — `Backend` interface (decision-making)
- `worker.Worker.CLITool` — already stores runtime name per worker
- `config.WorkerTierConfig.CLITool` — configurable per tier
- `spawner.TierSpawnConfig` — resolved CLI args per tier

### 3.1 AgentRuntime Interface

New file: `internal/agent/runtime.go`

```go
type AgentRuntime interface {
    Name() string
    Spawn(ctx context.Context, cfg SpawnConfig) (*AgentSession, error)
    SendPrompt(session *AgentSession, prompt string) error
    CaptureOutput(session *AgentSession, lines int) (string, error)
    DetectReady(ctx context.Context, session *AgentSession, timeout time.Duration) error
    DetectCompletion(ctx context.Context, session *AgentSession) (bool, error)
    ParseTokenUsage(output string) (TokenUsage, error)
    Cleanup(session *AgentSession) error
}

type SpawnConfig struct {
    WorkDir         string
    Branch          string
    SystemPrompt    string
    AllowedTools    []string
    DisallowedTools []string
    Model           string
    PermissionMode  string
    ExtraCLIArgs    string
    EnvVars         map[string]string
}

type AgentSession struct {
    ID          string
    RuntimeName string
    TmuxSession string
    Window      int
    Pane        int
    WorkDir     string
    StartedAt   time.Time
    Metadata    map[string]string
}

type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalCost    float64
}
```

### 3.2 RuntimeRegistry

New file: `internal/agent/registry.go`

```go
type RuntimeRegistry struct {
    mu       sync.RWMutex
    runtimes map[string]AgentRuntime
}

func NewRuntimeRegistry() *RuntimeRegistry
func (r *RuntimeRegistry) Register(rt AgentRuntime)
func (r *RuntimeRegistry) Get(name string) (AgentRuntime, bool)
func (r *RuntimeRegistry) List() []string
func (r *RuntimeRegistry) Default() AgentRuntime
```

### 3.3 Three Plugins

#### Claude Code CLI (`internal/agent/claudecode/runtime.go`)

Extracted from `spawner.go`:
- `Spawn()` ← tmux session creation + `claude` CLI launch
- `SendPrompt()` ← SendLiteralKeys + Enter + promptRenderDelay
- `DetectReady()` ← waitForReady (❯, >, Welcome detection + auto-accept dialogs)
- `DetectCompletion()` ← idle prompt + changeCount logic from monitor
- `Cleanup()` ← KillSession

#### ais-agent (`internal/agent/aisagent/runtime.go`)

Extracted from `spawner.go` `buildAISAgentArgs`:
- `Spawn()` ← tmux session + `ais-agent` CLI with multi-provider flags
- `DetectReady()` ← ais-agent specific prompt detection

#### Aider (`internal/agent/aider/runtime.go`)

New implementation:
- `Spawn()` ← tmux session + `aider --model {model} --no-auto-commits --yes`
- `DetectReady()` ← detect `aider>` prompt
- `DetectCompletion()` ← detect return to `aider>` prompt
- `ParseTokenUsage()` ← parse Aider's `Tokens:` output
- `Cleanup()` ← send `/exit` + KillSession

### 3.4 Spawner Refactoring

Before: spawner contains all CLI launch logic inline.
After: spawner delegates to AgentRuntime via registry.

```go
type Spawner struct {
    // ... existing ...
    runtimeRegistry *agent.RuntimeRegistry
}

func (s *Spawner) spawnForTaskInner(...) error {
    // 1. Git branch (stays in spawner)
    // 2. rt := registry.Get(w.CLITool) or Default()
    // 3. cfg := s.buildSpawnConfig(w, t, p)
    // 4. session := rt.Spawn(ctx, cfg)
    // 5. rt.DetectReady(ctx, session, 120s)
    // 6. prompt := s.buildPromptForTier(...) (stays — business logic)
    // 7. rt.SendPrompt(session, prompt)
    // 8. Update worker state from session
}
```

CompletionMonitor also uses `rt.DetectCompletion()` with fallback to existing logic.

Council and Meeting engines use RuntimeRegistry for CLI-mode expert/participant sessions.

### Phase 3 Files

New:
- `internal/agent/runtime.go` (~100 lines)
- `internal/agent/registry.go` (~80 lines)
- `internal/agent/claudecode/runtime.go` (~350 lines)
- `internal/agent/claudecode/runtime_test.go` (~200 lines)
- `internal/agent/aisagent/runtime.go` (~250 lines)
- `internal/agent/aider/runtime.go` (~250 lines)
- `internal/agent/aider/runtime_test.go` (~150 lines)

Modified:
- `internal/worker/spawner.go` — delegate to AgentRuntime
- `internal/worker/monitor.go` — use DetectCompletion from runtime
- `internal/company/council.go` — spawnExpertAgent uses RuntimeRegistry
- `internal/company/meeting.go` — CLI mode uses RuntimeRegistry
- `internal/company/company.go` — RuntimeRegistry init + register 3 plugins
- `internal/config/config.go` — possible runtime-specific config sections

---

## Summary

| Phase | Feature | New Files | Modified | Est. Lines | Dependency |
|-------|---------|-----------|----------|------------|------------|
| 1 | Inter-Worker Communication | 2 | 5 | ~700 | None |
| 2 | Structured Meeting System | 5 | 5 | ~1600 | Phase 1 (Mailbox) |
| 3 | Pluggable Agent Backends | 7 | 6 | ~1380 | None (but benefits from Phase 1+2 experience) |
| **Total** | | **14** | **16** | **~3680** | |

Delivery order: Phase 1 → Phase 2 → Phase 3 (three independent branches, merged sequentially).
