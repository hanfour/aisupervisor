# Phase 2: Structured Meeting System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a meeting framework where multiple workers collaboratively discuss, debate, and reach consensus on reviews, project planning, and incident debugging.

**Architecture:** `MeetingEngine` orchestrates meetings through rounds of parallel speech collection and consensus checking. Three scenario-specific handlers (Review/Planning/Debug) implement domain logic. Built on Phase 1's Mailbox for notifications and existing debate/council infrastructure for AI consensus. Persistent via `MeetingStore` (YAML).

**Tech Stack:** Go 1.23, `ai.ChatProvider`, `tmux.TmuxClient`, `gopkg.in/yaml.v3`

**Depends on:** Phase 1 (Mailbox for notifications + ASK/REPLY for in-meeting communication)

---

## Task 1: Events & Data Model

**Files:**
- Modify: `internal/company/events.go`
- Create: `internal/company/meeting.go` (types only, no logic yet)

**Step 1: Add event constants**

In `events.go`, add:

```go
	EventMeetingScheduled    EventType = "meeting_scheduled"
	EventMeetingStarted      EventType = "meeting_started"
	EventMeetingRoundDone    EventType = "meeting_round_done"
	EventMeetingCompleted    EventType = "meeting_completed"
	EventMeetingCancelled    EventType = "meeting_cancelled"
```

**Step 2: Create meeting.go with types**

Define all types: `MeetingType`, `MeetingStatus`, `Meeting`, `MeetingRound`, `Speech`, `MeetingStore`, `MeetingEngine`, `MeetingRequest`. No methods yet — just the structs as specified in the design doc.

Also add:

```go
type MeetingRequest struct {
    Type         MeetingType
    Title        string
    ProjectID    string
    TaskID       string      // optional, for Review/Debug meetings
    ChairID      string
    Participants []string
    Agenda       []string
    MaxRounds    int          // 0 = default (3)
    Config       interface{}  // ReviewMeetingConfig, PlanningMeetingConfig, or DebugMeetingConfig
}
```

**Step 3: Build**

Run: `go build ./internal/...`

**Step 4: Commit**

```bash
git add internal/company/events.go internal/company/meeting.go
git commit -m "feat(meeting): add meeting data model and event types"
```

---

## Task 2: MeetingStore — CRUD & Persistence

**Files:**
- Modify: `internal/company/meeting.go` (add MeetingStore methods)
- Create: `internal/company/meeting_test.go`

**Step 1: Write failing tests**

- `TestMeetingStore_NewEmpty` — empty dir, no error
- `TestMeetingStore_CreateAndGet` — create meeting, retrieve by ID
- `TestMeetingStore_List` — create 3 meetings, list returns all
- `TestMeetingStore_ListByProject` — filter by ProjectID
- `TestMeetingStore_ListByStatus` — filter by status
- `TestMeetingStore_Update` — update meeting status, verify persisted
- `TestMeetingStore_SaveAndReload` — create, save, reload, verify data intact
- `TestMeetingStore_ConcurrentAccess` — parallel reads/writes with -race

**Step 2: Implement MeetingStore**

```go
func NewMeetingStore(dataDir string) (*MeetingStore, error)
func (ms *MeetingStore) Create(req MeetingRequest) (*Meeting, error)  // assigns ID, sets scheduled
func (ms *MeetingStore) Get(id string) (*Meeting, error)
func (ms *MeetingStore) List() []*Meeting
func (ms *MeetingStore) ListByProject(projectID string) []*Meeting
func (ms *MeetingStore) ListByStatus(status MeetingStatus) []*Meeting
func (ms *MeetingStore) Update(m *Meeting) error
func (ms *MeetingStore) Save() error   // persist to meetings.yaml
```

Storage: `~/.local/share/aisupervisor/company/meetings.yaml`

**Step 3: Run tests**

Run: `go test ./internal/company/ -run TestMeetingStore -v -race`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/company/meeting.go internal/company/meeting_test.go
git commit -m "feat(meeting): add MeetingStore with YAML persistence"
```

---

## Task 3: MeetingEngine Core — Schedule, Start, Consensus

**Files:**
- Modify: `internal/company/meeting.go` (add MeetingEngine methods)
- Modify: `internal/company/meeting_test.go`

**Step 1: Write failing tests**

- `TestMeetingEngine_Schedule` — creates meeting with status=scheduled
- `TestMeetingEngine_Start_AllAvailable` — all participants idle → status=in_progress
- `TestMeetingEngine_Start_SomeBusy` — some participants busy → returns error with names
- `TestMeetingEngine_Cancel` — cancel sets status=cancelled
- `TestCheckConsensus_Unanimous` — all approve → reached=true
- `TestCheckConsensus_TwoThirds` — 2/3 approve → reached=true at 0.67 threshold
- `TestCheckConsensus_NoConsensus` — split vote → reached=false
- `TestCheckConsensus_Empty` — no votes → reached=false

**Step 2: Implement**

```go
func NewMeetingEngine(cp ai.ChatProvider, mailbox *Mailbox, tmuxClient tmux.TmuxClient, language string, store *MeetingStore) *MeetingEngine

func (me *MeetingEngine) Schedule(req MeetingRequest) (*Meeting, error) {
    // Create via store, emit EventMeetingScheduled
    // Send notification to all participants via Mailbox
}

func (me *MeetingEngine) Start(ctx context.Context, meetingID string) error {
    // Verify all participants available (check worker status)
    // If any busy, return error listing unavailable workers
    // Update status to in_progress
    // Notify via Mailbox
    // Emit EventMeetingStarted
}

func (me *MeetingEngine) Cancel(meetingID string) error {
    // Set status cancelled, notify participants, emit event
}

func checkConsensus(speeches []Speech, threshold float64) (reached bool, verdict string) {
    // Count votes, check if any vote exceeds threshold
}
```

Note: `MeetingEngine` needs access to workers map. Add a `workerChecker` interface:

```go
type workerChecker interface {
    GetWorkerStatus(id string) (worker.WorkerStatus, bool)
}
```

Manager implements this; pass it to MeetingEngine.

**Step 3: Run tests**

Run: `go test ./internal/company/ -run "TestMeetingEngine|TestCheckConsensus" -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/company/meeting.go internal/company/meeting_test.go
git commit -m "feat(meeting): add MeetingEngine core — schedule, start, consensus"
```

---

## Task 4: Speech Collection — API & CLI Modes

**Files:**
- Modify: `internal/company/meeting.go`
- Modify: `internal/company/meeting_test.go`

**Step 1: Write failing tests**

- `TestCollectSpeeches_APIMode` — mock ChatProvider returns speech content, verify Speech struct populated
- `TestCollectSpeeches_Parallel` — 3 participants, all return in parallel, verify 3 speeches
- `TestCollectSpeeches_OneFailure` — 1 of 3 fails, still get 2 speeches
- `TestCollectSpeeches_Timeout` — slow provider, timeout respected

**Step 2: Implement collectSpeeches**

```go
func (me *MeetingEngine) collectSpeeches(ctx context.Context, m *Meeting, roundNum int, prompt string, mode ExecMode) ([]Speech, error) {
    // Similar to council.dispatchExperts pattern
    // For each participant:
    //   API mode: ChatProvider.Chat() with personality system prompt
    //   CLI mode: spawn temp tmux session (reuse council's pattern)
    // Parallel via WaitGroup
    // Single failure → log, continue
    // Parse each response for Vote (look for VOTE:approve/reject/abstain)
    // Parse for Findings (JSON array, same as council expert format)
}
```

API mode system prompt per participant:

```go
func buildMeetingSpeechPrompt(m *Meeting, participant string, roundNum int, prevRound *MeetingRound) string {
    // Include: meeting type, agenda, previous round speeches (if round > 1)
    // Instruct: provide your analysis, end with VOTE:{approve|reject|abstain}
    // If review: include diff context
    // If planning: include project goals
    // If debug: include error log
}
```

**Step 3: Run tests**

Run: `go test ./internal/company/ -run TestCollectSpeeches -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/company/meeting.go internal/company/meeting_test.go
git commit -m "feat(meeting): add parallel speech collection with API/CLI modes"
```

---

## Task 5: RunRound & Synthesize

**Files:**
- Modify: `internal/company/meeting.go`
- Modify: `internal/company/meeting_test.go`

**Step 1: Write failing tests**

- `TestRunRound_CollectsAndChecksConsensus` — mock provider, verify round has speeches + consensus check
- `TestRunRound_EarlyExit` — unanimous vote → consensus reached in round 1
- `TestSynthesize_ProducesSummary` — mock provider returns summary, verify meeting verdict + summary set

**Step 2: Implement RunRound**

```go
func (me *MeetingEngine) RunRound(ctx context.Context, m *Meeting, roundNum int, mode ExecMode, threshold float64) (*MeetingRound, bool, error) {
    // 1. Build round prompt (include agenda + previous round context)
    // 2. collectSpeeches(ctx, m, roundNum, prompt, mode)
    // 3. Create MeetingRound with speeches
    // 4. checkConsensus(speeches, threshold)
    // 5. Set round.Consensus if reached
    // 6. Append round to m.Rounds
    // 7. Emit EventMeetingRoundDone
    // 8. Return round, consensusReached, nil
}
```

**Step 3: Implement Synthesize**

```go
func (me *MeetingEngine) Synthesize(ctx context.Context, m *Meeting) error {
    // If consensus was reached in last round, use that verdict
    // Otherwise, AI synthesis (single ChatProvider call):
    //   Input: all rounds' speeches + votes
    //   Output: verdict + summary
    // Set m.Verdict, m.Summary, m.Status=completed, m.CompletedAt
    // Notify participants via Mailbox
    // Emit EventMeetingCompleted
    // Save via store
}
```

**Step 4: Run tests**

Run: `go test ./internal/company/ -run "TestRunRound|TestSynthesize" -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/meeting.go internal/company/meeting_test.go
git commit -m "feat(meeting): add RunRound and Synthesize with consensus detection"
```

---

## Task 6: Review Consensus Meeting

**Files:**
- Create: `internal/company/meeting_review.go`
- Create: `internal/company/meeting_review_test.go`

**Step 1: Write failing tests**

- `TestReviewMeeting_FullFlow` — mock provider, 3 participants, unanimous approve → completed
- `TestReviewMeeting_Divergence` — split votes → goes to round 2
- `TestReviewMeeting_FinalVoteMajority` — 2/3 approve in round 3 → approved

**Step 2: Implement**

```go
type ReviewMeetingConfig struct {
    Diff  string
    Brief *ContextBrief
}

func (me *MeetingEngine) RunReviewMeeting(ctx context.Context, m *Meeting, cfg ReviewMeetingConfig) error {
    // Round 1: Independent review (CLI mode, parallel)
    //   Prompt includes diff + brief
    //   Threshold: 2/3
    round1, consensus, _ := me.RunRound(ctx, m, 1, ExecCLI, 0.67)
    if consensus { return me.Synthesize(ctx, m) }

    // Round 2: Debate (API mode, serial — each sees previous speeches)
    //   Only participants with divergent votes speak
    //   Threshold: 2/3
    round2, consensus, _ := me.RunRound(ctx, m, 2, ExecAPI, 0.67)
    if consensus { return me.Synthesize(ctx, m) }

    // Round 3: Final vote (API mode)
    //   Chair summarizes, all vote
    //   Threshold: 2/3 (majority decides)
    me.RunRound(ctx, m, 3, ExecAPI, 0.67)
    return me.Synthesize(ctx, m)
}
```

**Step 3: Run tests**

Run: `go test ./internal/company/ -run TestReviewMeeting -v`

**Step 4: Commit**

```bash
git add internal/company/meeting_review.go internal/company/meeting_review_test.go
git commit -m "feat(meeting): add Review Consensus Meeting scenario"
```

---

## Task 7: Planning Meeting

**Files:**
- Create: `internal/company/meeting_planning.go`
- Create: `internal/company/meeting_planning_test.go`

**Step 1: Write failing tests**

- `TestPlanningMeeting_ProducesTaskList` — mock returns task proposals, verify tasks in output
- `TestPlanningMeeting_MergesProposals` — round 2 merges divergent proposals

**Step 2: Implement**

```go
type PlanningMeetingConfig struct {
    Goals       []string
    Constraints []string
}

func (me *MeetingEngine) RunPlanningMeeting(ctx context.Context, m *Meeting, cfg PlanningMeetingConfig) ([]TaskProposal, error) {
    // Round 1: Independent proposals (API mode)
    // Round 2: Cross-evaluate (API mode, serial)
    // Round 3: Finalize (API mode, chair synthesizes)
    // Parse final round for task list
    // Return []TaskProposal for caller to create actual Tasks
}

type TaskProposal struct {
    Title       string
    Description string
    AssignTo    string   // suggested worker ID
    Priority    int
    Dependencies []string
}
```

**Step 3: Run tests, commit**

```bash
git commit -m "feat(meeting): add Planning Meeting scenario"
```

---

## Task 8: Debug Meeting

**Files:**
- Create: `internal/company/meeting_debug.go`
- Create: `internal/company/meeting_debug_test.go`

**Step 1: Write failing tests**

- `TestDebugMeeting_DiagnosesRootCause` — mock returns hypotheses, verify verdict contains root cause
- `TestDebugMeeting_ProducesFixTasks` — verify fix tasks in output

**Step 2: Implement**

```go
type DebugMeetingConfig struct {
    ErrorLog   string
    Rejections []string
}

func (me *MeetingEngine) RunDebugMeeting(ctx context.Context, m *Meeting, cfg DebugMeetingConfig) ([]TaskProposal, error) {
    // Round 1: Diagnosis (CLI mode — need to read code)
    // Round 2: Evaluate hypotheses (API mode)
    // Round 3: Action plan (API mode)
    // Return fix tasks
}
```

**Step 3: Run tests, commit**

```bash
git commit -m "feat(meeting): add Debug Meeting scenario"
```

---

## Task 9: Integration — Wire into Manager & Review Pipeline

**Files:**
- Modify: `internal/company/company.go` — add meetingEngine field, init
- Modify: `internal/company/review.go` — council ambiguity → trigger Review Meeting

**Step 1: Add meetingEngine to Manager**

```go
meetingEngine *MeetingEngine
```

Init in New():
```go
meetingStore, _ := NewMeetingStore(dataDir)
m.meetingEngine = NewMeetingEngine(chatProvider, m.mailbox, tmuxClient, m.language, meetingStore)
```

**Step 2: Add escalation from council to review meeting**

In `handleCouncilResult`, after the existing logic, add:

```go
// If council result is ambiguous (APPROVED with HIGH findings), offer meeting escalation
if result.Status == "APPROVED" && countHigh(result.Findings) >= 2 {
    log.Printf("council: ambiguous result for task %s, scheduling review meeting", t.ID)
    // Auto-schedule review meeting if enough idle workers
    // ... (create MeetingRequest with available reviewers)
}
```

**Step 3: Add Wails bindings**

In `internal/gui/company_app.go`, add:

```go
func (a *CompanyApp) ScheduleMeeting(req MeetingRequestDTO) (*MeetingDTO, error)
func (a *CompanyApp) GetMeeting(id string) (*MeetingDTO, error)
func (a *CompanyApp) ListMeetings(projectID string) []*MeetingDTO
func (a *CompanyApp) CancelMeeting(id string) error
```

**Step 4: Frontend i18n**

```javascript
'event.meeting_scheduled': { en: 'Meeting Scheduled', zh: '會議已排程' },
'event.meeting_started': { en: 'Meeting Started', zh: '會議已開始' },
'event.meeting_round_done': { en: 'Meeting Round Complete', zh: '會議輪次完成' },
'event.meeting_completed': { en: 'Meeting Completed', zh: '會議已結束' },
'event.meeting_cancelled': { en: 'Meeting Cancelled', zh: '會議已取消' },
```

**Step 5: Build and full test**

Run: `go build ./internal/...`
Run: `go test ./internal/... -count=1`
Expected: ALL PASS

**Step 6: Commit**

```bash
git commit -m "feat(meeting): wire MeetingEngine into Manager with Wails bindings"
```

---

## Task 10: Integration Test

**Files:**
- Create: `internal/company/meeting_integration_test.go`

Tests:
- `TestMeetingIntegration_ReviewFlow` — schedule → start → 3 rounds → completed
- `TestMeetingIntegration_PlanningProducesTasks` — planning meeting outputs task proposals
- `TestMeetingIntegration_DebugFromCircuitBreaker` — simulate bounce → debug meeting triggered
- `TestMeetingIntegration_CancelMidway` — cancel during round 2

**Final verification:**

Run: `go test ./internal/... -count=1`
Run: `go build ./internal/...`

```bash
git commit -m "test(meeting): add integration tests for all meeting scenarios"
```
