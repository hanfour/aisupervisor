# Phase 1: Inter-Worker Communication Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable direct messaging between workers via a persistent Mailbox with synchronous Q&A and automatic message injection.

**Architecture:** New `Mailbox` struct with YAML persistence provides the storage layer. `CompletionMonitor` extended with ASK/REPLY pattern detection for sync Q&A. Message injection at three points: task spawn, task completion, and idle polling. Existing `CommunicationMatrix` validates routing.

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, `sync.RWMutex`

---

## Task 1: Events Extension

**Files:**
- Modify: `internal/company/events.go:66` (add constants before closing paren)

**Step 1: Add event constants**

Add before the closing `)` of the EventType const block:

```go
	EventMessageSent      EventType = "message_sent"
	EventMessageDelivered EventType = "message_delivered"
	EventMessageRead      EventType = "message_read"
```

**Step 2: Build**

Run: `go build ./internal/...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add internal/company/events.go
git commit -m "feat(comm): add message event types"
```

---

## Task 2: Mailbox Core — Envelope, Store, CRUD

**Files:**
- Create: `internal/company/mailbox.go`
- Create: `internal/company/mailbox_test.go`

**Step 1: Write failing tests**

Tests to cover:
- `TestMailbox_NewEmpty` — new mailbox from empty dir, no error
- `TestMailbox_SendAndPeek` — send envelope, peek returns it, status=pending
- `TestMailbox_SendValidatesRouting` — send to invalid target (CommunicationMatrix mock) returns error
- `TestMailbox_Deliver` — deliver changes status to delivered, peek no longer returns it
- `TestMailbox_MarkRead` — mark read sets ReadAt and status
- `TestMailbox_Reply` — reply sets ReplyToID and ThreadID
- `TestMailbox_GetThread` — multiple messages in thread returned in order
- `TestMailbox_Expire` — old messages with TTL are cleaned up
- `TestMailbox_SaveAndLoad` — save to YAML, reload, data persists
- `TestMailbox_ConcurrentSendSafe` — parallel sends don't race (use -race flag)

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/company/ -run TestMailbox -v`
Expected: FAIL

**Step 3: Write implementation**

`internal/company/mailbox.go`:

Types:
- `EnvelopeStatus` — pending/delivered/read/expired
- `Envelope` — embeds `StructuredMessage`, adds Status, ReadAt, ReplyToID, ThreadID, TTL
- `Mailbox` — mu, inbox map[string][]Envelope, filePath string
- `mailboxFile` — YAML wrapper struct

Methods:
- `NewMailbox(dataDir string) (*Mailbox, error)` — create dir, load mailbox.yaml if exists
- `Send(env Envelope) error` — validate From/To non-empty, assign ID (uuid-like), set Timestamp, append to inbox[To], auto-save
- `Peek(workerID string) []Envelope` — return pending envelopes (status == EnvPending)
- `Deliver(workerID string) []Envelope` — return pending, mark as EnvDelivered, save
- `MarkRead(messageID string)` — find by ID across all inboxes, set status=EnvRead, ReadAt=now, save
- `Reply(originalID string, reply Envelope) error` — find original, set reply.ReplyToID=originalID, reply.ThreadID = original.ThreadID or original.ID, send
- `GetThread(threadID string) []Envelope` — collect all envelopes where ThreadID==threadID or ID==threadID, sort by timestamp
- `Expire(maxAge time.Duration) int` — remove envelopes where TTL > 0 and age > TTL, or age > maxAge. Return count removed.
- `Save() error` — marshal inbox to YAML, write to filePath

ID generation: use `fmt.Sprintf("msg-%d-%04d", time.Now().Unix(), rand.Intn(10000))`

Note: For `Send()`, do NOT validate via CommunicationMatrix in the Mailbox itself — the caller (Manager) handles routing validation. Mailbox is a dumb storage layer.

**Step 4: Run tests**

Run: `go test ./internal/company/ -run TestMailbox -v -race`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/mailbox.go internal/company/mailbox_test.go
git commit -m "feat(comm): add Mailbox persistent storage with YAML persistence"
```

---

## Task 3: ASK/REPLY Pattern Detection in Monitor

**Files:**
- Modify: `internal/worker/monitor.go`
- Create: `internal/worker/monitor_comm_test.go`

**Step 1: Write failing tests**

```go
// internal/worker/monitor_comm_test.go
func TestDetectAskPattern(t *testing.T) // "ASK:worker-2:How should I handle errors?" → targetID="worker-2", question="How should I handle errors?"
func TestDetectAskPattern_NoMatch(t *testing.T) // "normal output" → found=false
func TestDetectAskPattern_MultiLine(t *testing.T) // ASK: in middle of output
func TestDetectReplyPattern(t *testing.T) // "REPLY:msg-123:Use fmt.Errorf" → messageID="msg-123", reply="Use fmt.Errorf"
func TestDetectReplyPattern_NoMatch(t *testing.T)
```

**Step 2: Implement detection functions**

Add to `monitor.go`:

```go
func detectAskPattern(content string) (targetID, question string, found bool)
func detectReplyPattern(content string) (messageID, reply string, found bool)
```

Parse using `strings.SplitN` on `:` delimiter. ASK format: `ASK:{workerID}:{question}`. REPLY format: `REPLY:{messageID}:{content}`.

Scan from end of content backwards (most recent output first). Only match the LAST occurrence.

**Step 3: Integrate into WatchForCompletion**

In the poll loop (after HELP_NEEDED detection at ~line 130), add:

```go
if targetID, question, found := detectAskPattern(newContent); found {
    return CompletionResult{
        Success: false,
        Reason:  "ask_request",
        HelpRequest: fmt.Sprintf("ASK:%s:%s", targetID, question),
    }, nil
}
if msgID, reply, found := detectReplyPattern(newContent); found {
    return CompletionResult{
        Success: false,
        Reason:  "reply_sent",
        HelpRequest: fmt.Sprintf("REPLY:%s:%s", msgID, reply),
    }, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/worker/ -run TestDetect -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/worker/monitor.go internal/worker/monitor_comm_test.go
git commit -m "feat(comm): add ASK/REPLY pattern detection in completion monitor"
```

---

## Task 4: Manager — handleSyncAsk & handleReply

**Files:**
- Modify: `internal/company/company.go`

**Step 1: Add mailbox field to Manager**

After `expertReg` field (~line 83), add:

```go
	mailbox *Mailbox
```

In `New()`, after council initialization, add:

```go
	mailbox, mbErr := NewMailbox(dataDir)
	if mbErr != nil {
		log.Printf("warning: mailbox init failed: %v", mbErr)
		mailbox, _ = NewMailbox(os.TempDir())
	}
	m.mailbox = mailbox
```

**Step 2: Implement handleSyncAsk**

```go
func (m *Manager) handleSyncAsk(sender *worker.Worker, targetID, question string) {
    m.mu.RLock()
    target, ok := m.workers[targetID]
    m.mu.RUnlock()

    if !ok || target.Status != worker.WorkerIdle {
        // Fallback: async mailbox
        m.mailbox.Send(Envelope{
            StructuredMessage: StructuredMessage{
                ID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
                From: sender.ID, To: targetID,
                Type: MsgQuestion, Priority: PriorityHigh,
                Content: question, Timestamp: time.Now(),
            },
            TTL: 10 * time.Minute,
        })
        m.spawner.SendPromptToExisting(sender,
            fmt.Sprintf("[System] %s is busy. Question queued to mailbox. Continue your work.", targetID))
        m.emit(Event{Type: EventMessageSent, WorkerID: sender.ID,
            Message: fmt.Sprintf("async question to %s", targetID)})
        return
    }

    // Sync path
    m.spawner.SendPromptToExisting(target,
        fmt.Sprintf("[Question from %s] %s\nReply with REPLY:%s-sync:{your answer}",
            sender.Name, question, sender.ID))
    m.emit(Event{Type: EventMessageSent, WorkerID: sender.ID,
        Message: fmt.Sprintf("sync question to %s", target.Name)})
    go m.watchForSyncReply(sender, target, question, 3*time.Minute)
}
```

**Step 3: Implement watchForSyncReply**

```go
func (m *Manager) watchForSyncReply(sender, target *worker.Worker, question string, timeout time.Duration) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // Timeout — notify sender, convert to async
            m.spawner.SendPromptToExisting(sender,
                fmt.Sprintf("[System] No reply from %s within timeout. Question moved to mailbox.", target.Name))
            m.mailbox.Send(Envelope{
                StructuredMessage: StructuredMessage{
                    From: sender.ID, To: target.ID,
                    Type: MsgQuestion, Priority: PriorityHigh,
                    Content: question, Timestamp: time.Now(),
                },
                TTL: 30 * time.Minute,
            })
            return
        case <-ticker.C:
            content, err := m.tmuxClient.CapturePane(target.TmuxSession, target.Window, target.Pane, 50)
            if err != nil {
                continue
            }
            if _, reply, found := detectReplyPattern(content); found {
                m.spawner.SendPromptToExisting(sender,
                    fmt.Sprintf("[Reply from %s] %s", target.Name, reply))
                m.emit(Event{Type: EventMessageDelivered, WorkerID: target.ID,
                    Message: fmt.Sprintf("reply to %s", sender.Name)})
                return
            }
        }
    }
}
```

Note: Import `detectReplyPattern` — it's in the `worker` package. Either export it or duplicate the simple parsing logic in `company` package.

**Step 4: Wire into watchCompletion**

In `watchCompletion()` (the goroutine that monitors worker completion), add handling for the new `ask_request` and `reply_sent` reasons from CompletionResult:

```go
case "ask_request":
    // Parse ASK:targetID:question from result.HelpRequest
    parts := strings.SplitN(strings.TrimPrefix(result.HelpRequest, "ASK:"), ":", 2)
    if len(parts) == 2 {
        m.handleSyncAsk(w, parts[0], parts[1])
    }
    // Continue monitoring — don't complete the task
    continue
case "reply_sent":
    // Reply was sent, continue monitoring
    continue
```

**Step 5: Build and test**

Run: `go build ./internal/...`
Expected: SUCCESS

**Step 6: Commit**

```bash
git add internal/company/company.go
git commit -m "feat(comm): add handleSyncAsk and watchForSyncReply in Manager"
```

---

## Task 5: Message Injection into Worker Prompts

**Files:**
- Modify: `internal/worker/spawner.go`
- Modify: `internal/company/company.go`

**Step 1: Add message injection to spawner**

In `spawner.go`, add a `Mailbox` interface to avoid circular imports:

```go
type mailboxPeeker interface {
    Peek(workerID string) []Envelope
}
```

Wait — `Envelope` is in `company` package. To avoid circular import, pass the messages as a string. Add method:

```go
func (s *Spawner) SetPendingMessages(fn func(workerID string) string) {
    s.pendingMessagesFn = fn
}
```

Add field `pendingMessagesFn func(string) string` to Spawner struct.

In `buildPromptForTier()`, after knowledge injection, append:

```go
if s.pendingMessagesFn != nil {
    if msgs := s.pendingMessagesFn(w.ID); msgs != "" {
        prompt += "\n\n## Pending Messages\n\n" + msgs
    }
}
```

**Step 2: Wire in company.go New()**

After mailbox initialization:

```go
spawner.SetPendingMessages(func(workerID string) string {
    messages := m.mailbox.Peek(workerID)
    if len(messages) == 0 {
        return ""
    }
    var sb strings.Builder
    for _, env := range messages {
        sb.WriteString(fmt.Sprintf("From: %s [%s] %s\n> %s\n\n",
            env.From, env.Type, env.Timestamp.Format("15:04"), env.Content))
    }
    sb.WriteString("Reply with REPLY:{messageID}:{your response} if needed.\n")
    return sb.String()
})
```

**Step 3: Add idle mailbox processing and background poller**

In `company.go`, add `processIdleMailbox()` and the background ticker in the bgCtx goroutine section:

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-bgCtx.Done():
            return
        case <-ticker.C:
            m.mu.RLock()
            for _, w := range m.workers {
                if w.Status == worker.WorkerIdle {
                    m.processIdleMailbox(w)
                }
            }
            m.mu.RUnlock()
            m.mailbox.Expire(1 * time.Hour)
        }
    }
}()
```

```go
func (m *Manager) processIdleMailbox(w *worker.Worker) {
    pending := m.mailbox.Deliver(w.ID)
    if len(pending) == 0 {
        return
    }
    for _, env := range pending {
        switch env.Type {
        case MsgQuestion:
            prompt := fmt.Sprintf("[Question from %s]\n%s\n\nReply with REPLY:%s:{your answer}",
                env.From, env.Content, env.ID)
            m.spawner.SendPromptToExisting(w, prompt)
        default:
            prompt := fmt.Sprintf("[%s from %s] %s", env.Type, env.From, env.Content)
            m.spawner.SendPromptToExisting(w, prompt)
            m.mailbox.MarkRead(env.ID)
        }
        m.emit(Event{Type: EventMessageDelivered, WorkerID: w.ID,
            Message: fmt.Sprintf("delivered %s from %s", env.Type, env.From)})
    }
}
```

**Step 4: Build and run full tests**

Run: `go build ./internal/...`
Run: `go test ./internal/company/ -count=1`
Run: `go test ./internal/worker/ -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/worker/spawner.go internal/company/company.go
git commit -m "feat(comm): add message injection at spawn and idle polling"
```

---

## Task 6: Frontend i18n

**Files:**
- Modify: `frontend/src/lib/stores/i18n.js`

**Step 1: Add translations**

```javascript
// --- Communication Events ---
'event.message_sent': { en: 'Message Sent', zh: '訊息已發送' },
'event.message_delivered': { en: 'Message Delivered', zh: '訊息已送達' },
'event.message_read': { en: 'Message Read', zh: '訊息已讀' },
```

**Step 2: Commit**

```bash
git add frontend/src/lib/stores/i18n.js
git commit -m "feat(comm): add zh-TW translations for communication events"
```

---

## Task 7: Integration Test

**Files:**
- Create: `internal/company/mailbox_integration_test.go`

Tests:
- `TestMailboxIntegration_SendReceiveReply` — full send → deliver → reply → getThread cycle
- `TestMailboxIntegration_ExpireCleanup` — send with short TTL, expire, verify removed
- `TestMailboxIntegration_PersistReload` — send messages, save, create new store, verify messages survive

**Step 1: Write and run**

Run: `go test ./internal/company/ -run TestMailboxIntegration -v`
Expected: ALL PASS

**Step 2: Final build verification**

Run: `go build ./internal/...`
Run: `go test ./internal/... -count=1`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add internal/company/mailbox_integration_test.go
git commit -m "test(comm): add integration tests for mailbox lifecycle"
```
