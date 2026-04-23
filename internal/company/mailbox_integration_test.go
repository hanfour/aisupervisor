package company

import (
	"testing"
	"time"
)

// TestMailboxIntegration_SendReceiveReply tests the full lifecycle of sending,
// delivering, replying, threading, and marking a message as read.
func TestMailboxIntegration_SendReceiveReply(t *testing.T) {
	dir := t.TempDir()
	mb, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	// 1. Worker A sends a question to Worker B.
	question := Envelope{
		StructuredMessage: StructuredMessage{
			ID:       "q-001",
			From:     "worker-a",
			To:       "worker-b",
			Type:     MsgQuestion,
			Priority: PriorityNormal,
			Content:  "How should I handle error X?",
		},
	}
	if err := mb.Send(question); err != nil {
		t.Fatalf("Send question: %v", err)
	}

	// 2. Peek B's inbox -- verify 1 pending message.
	pending := mb.Peek("worker-b")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending for worker-b, got %d", len(pending))
	}
	if pending[0].ID != "q-001" {
		t.Errorf("expected ID q-001, got %s", pending[0].ID)
	}
	if pending[0].Status != EnvPending {
		t.Errorf("expected pending status, got %s", pending[0].Status)
	}

	// 3. Deliver B's inbox -- verify message returned and status changes.
	delivered := mb.Deliver("worker-b")
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivered, got %d", len(delivered))
	}
	if delivered[0].Content != "How should I handle error X?" {
		t.Errorf("unexpected content: %s", delivered[0].Content)
	}

	// After delivery, Peek should return nothing pending.
	if msgs := mb.Peek("worker-b"); len(msgs) != 0 {
		t.Errorf("expected 0 pending after deliver, got %d", len(msgs))
	}

	// 4. Worker B replies.
	reply := Envelope{
		StructuredMessage: StructuredMessage{
			From:     "worker-b",
			To:       "worker-a",
			Type:     MsgFeedback,
			Priority: PriorityNormal,
			Content:  "Wrap it with fmt.Errorf and add context.",
		},
	}
	if err := mb.Reply("q-001", reply); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	// 5. GetThread -- verify 2 messages in order (original + reply).
	thread := mb.GetThread("q-001")
	if len(thread) != 2 {
		t.Fatalf("expected 2 messages in thread, got %d", len(thread))
	}
	if thread[0].Content != "How should I handle error X?" {
		t.Errorf("first thread message wrong: %s", thread[0].Content)
	}
	if thread[1].Content != "Wrap it with fmt.Errorf and add context." {
		t.Errorf("second thread message wrong: %s", thread[1].Content)
	}
	if thread[1].ReplyToID != "q-001" {
		t.Errorf("reply ReplyToID should be q-001, got %s", thread[1].ReplyToID)
	}
	if thread[1].ThreadID != "q-001" {
		t.Errorf("reply ThreadID should be q-001, got %s", thread[1].ThreadID)
	}

	// 6. MarkRead on reply.
	replyID := thread[1].ID
	mb.MarkRead(replyID)

	// 7. Verify reply status=read, ReadAt not nil.
	mb.mu.RLock()
	var found bool
	for _, envs := range mb.inbox {
		for _, e := range envs {
			if e.ID == replyID {
				found = true
				if e.Status != EnvRead {
					t.Errorf("expected read status, got %s", e.Status)
				}
				if e.ReadAt == nil {
					t.Error("expected ReadAt to be set after MarkRead")
				}
			}
		}
	}
	mb.mu.RUnlock()
	if !found {
		t.Error("reply message not found after MarkRead")
	}
}

// TestMailboxIntegration_ExpireCleanup verifies that expiration removes messages
// by both TTL and maxAge.
func TestMailboxIntegration_ExpireCleanup(t *testing.T) {
	mb, err := NewMailbox(t.TempDir())
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	// 1. Send message with TTL=1ms (set timestamp in the past so it's already expired).
	ttlMsg := Envelope{
		StructuredMessage: StructuredMessage{
			ID:        "ttl-msg",
			From:      "worker-a",
			To:        "worker-b",
			Content:   "ephemeral",
			Timestamp: time.Now().Add(-5 * time.Millisecond),
		},
		TTL: 1 * time.Millisecond,
	}
	if err := mb.Send(ttlMsg); err != nil {
		t.Fatalf("Send ttl msg: %v", err)
	}

	// 2. Expire with a generous maxAge -- only TTL should trigger removal.
	removed := mb.Expire(1 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed by TTL, got %d", removed)
	}

	// Verify it's gone.
	if msgs := mb.Peek("worker-b"); len(msgs) != 0 {
		t.Errorf("expected 0 after TTL expiry, got %d", len(msgs))
	}

	// 3. Send a message without TTL.
	normalMsg := Envelope{
		StructuredMessage: StructuredMessage{
			ID:        "normal-msg",
			From:      "worker-a",
			To:        "worker-b",
			Content:   "normal",
			Timestamp: time.Now().Add(-10 * time.Millisecond),
		},
	}
	if err := mb.Send(normalMsg); err != nil {
		t.Fatalf("Send normal msg: %v", err)
	}

	// 4. Expire with a very small maxAge -- should remove by maxAge.
	removed = mb.Expire(1 * time.Millisecond)
	if removed != 1 {
		t.Errorf("expected 1 removed by maxAge, got %d", removed)
	}

	if msgs := mb.Peek("worker-b"); len(msgs) != 0 {
		t.Errorf("expected 0 after maxAge expiry, got %d", len(msgs))
	}
}

// TestMailboxIntegration_PersistReload verifies that mailbox state survives
// save-and-reload cycles.
func TestMailboxIntegration_PersistReload(t *testing.T) {
	dir := t.TempDir()

	// 1. Create mailbox, send 3 messages to different workers.
	mb1, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	workers := []struct {
		id      string
		content string
	}{
		{"worker-a", "task for A"},
		{"worker-b", "task for B"},
		{"worker-c", "task for C"},
	}

	for _, w := range workers {
		if err := mb1.Send(Envelope{
			StructuredMessage: StructuredMessage{
				From:    "supervisor",
				To:      w.id,
				Type:    MsgTaskAssignment,
				Content: w.content,
			},
		}); err != nil {
			t.Fatalf("Send to %s: %v", w.id, err)
		}
	}

	// 2. Save explicitly.
	if err := mb1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 3. Create new mailbox from same dir (reload).
	mb2, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox reload: %v", err)
	}

	// 4. Verify all 3 messages present via Peek.
	for _, w := range workers {
		msgs := mb2.Peek(w.id)
		if len(msgs) != 1 {
			t.Errorf("expected 1 message for %s, got %d", w.id, len(msgs))
			continue
		}
		if msgs[0].Content != w.content {
			t.Errorf("content mismatch for %s: got %s", w.id, msgs[0].Content)
		}
	}

	// 5. Deliver one, save, reload -- verify delivery persisted.
	mb2.Deliver("worker-a")
	if err := mb2.Save(); err != nil {
		t.Fatalf("Save after deliver: %v", err)
	}

	mb3, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox second reload: %v", err)
	}

	// worker-a should have 0 pending (delivered), B and C still pending.
	if msgs := mb3.Peek("worker-a"); len(msgs) != 0 {
		t.Errorf("expected 0 pending for worker-a after reload, got %d", len(msgs))
	}
	if msgs := mb3.Peek("worker-b"); len(msgs) != 1 {
		t.Errorf("expected 1 pending for worker-b after reload, got %d", len(msgs))
	}
	if msgs := mb3.Peek("worker-c"); len(msgs) != 1 {
		t.Errorf("expected 1 pending for worker-c after reload, got %d", len(msgs))
	}
}

// TestMailboxIntegration_ThreadedConversation verifies a multi-turn threaded
// conversation where replies chain through ReplyToID.
func TestMailboxIntegration_ThreadedConversation(t *testing.T) {
	mb, err := NewMailbox(t.TempDir())
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	// 1. A asks B a question.
	q := Envelope{
		StructuredMessage: StructuredMessage{
			ID:       "conv-001",
			From:     "worker-a",
			To:       "worker-b",
			Type:     MsgQuestion,
			Priority: PriorityHigh,
			Content:  "What API should I use for file uploads?",
		},
	}
	if err := mb.Send(q); err != nil {
		t.Fatalf("Send question: %v", err)
	}

	// Small sleep to guarantee timestamp ordering.
	time.Sleep(1 * time.Millisecond)

	// 2. B replies.
	r1 := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "worker-b",
			To:      "worker-a",
			Type:    MsgFeedback,
			Content: "Use multipart/form-data with the /upload endpoint.",
		},
	}
	if err := mb.Reply("conv-001", r1); err != nil {
		t.Fatalf("Reply 1: %v", err)
	}

	// Get reply ID for follow-up.
	aMsgs := mb.Peek("worker-a")
	if len(aMsgs) != 1 {
		t.Fatalf("expected 1 message for worker-a, got %d", len(aMsgs))
	}
	replyID := aMsgs[0].ID

	time.Sleep(1 * time.Millisecond)

	// 3. A follows up (reply to B's reply).
	r2 := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "worker-a",
			To:      "worker-b",
			Type:    MsgQuestion,
			Content: "What is the max file size?",
		},
	}
	if err := mb.Reply(replyID, r2); err != nil {
		t.Fatalf("Reply 2: %v", err)
	}

	// 4. GetThread -- verify 3 messages in chronological order, all same ThreadID.
	thread := mb.GetThread("conv-001")
	if len(thread) != 3 {
		t.Fatalf("expected 3 messages in thread, got %d", len(thread))
	}

	// Verify chronological order.
	for i := 1; i < len(thread); i++ {
		if thread[i].Timestamp.Before(thread[i-1].Timestamp) {
			t.Errorf("thread not sorted: msg[%d] (%v) before msg[%d] (%v)",
				i, thread[i].Timestamp, i-1, thread[i-1].Timestamp)
		}
	}

	// Verify content order.
	expectedContents := []string{
		"What API should I use for file uploads?",
		"Use multipart/form-data with the /upload endpoint.",
		"What is the max file size?",
	}
	for i, expected := range expectedContents {
		if thread[i].Content != expected {
			t.Errorf("thread[%d] content: got %q, want %q", i, thread[i].Content, expected)
		}
	}

	// Verify all share the same ThreadID (the original message ID).
	for i, msg := range thread {
		// The root message matches by ID (not ThreadID), replies match by ThreadID.
		if i > 0 && msg.ThreadID != "conv-001" {
			t.Errorf("thread[%d] ThreadID: got %q, want conv-001", i, msg.ThreadID)
		}
	}
}

// TestMailboxIntegration_MultipleWorkers verifies that each worker has an
// independent inbox and operations on one do not affect others.
func TestMailboxIntegration_MultipleWorkers(t *testing.T) {
	mb, err := NewMailbox(t.TempDir())
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	// 1. Send messages to workers A, B, C.
	for _, w := range []string{"worker-a", "worker-b", "worker-c"} {
		if err := mb.Send(Envelope{
			StructuredMessage: StructuredMessage{
				From:    "supervisor",
				To:      w,
				Type:    MsgTaskAssignment,
				Content: "task for " + w,
			},
		}); err != nil {
			t.Fatalf("Send to %s: %v", w, err)
		}
	}

	// Send an extra message to worker-a to test independence.
	if err := mb.Send(Envelope{
		StructuredMessage: StructuredMessage{
			From:    "supervisor",
			To:      "worker-a",
			Type:    MsgStatusUpdate,
			Content: "second message for A",
		},
	}); err != nil {
		t.Fatalf("Send extra to worker-a: %v", err)
	}

	// 2. Verify each worker's inbox is independent.
	if c := mb.Count("worker-a"); c != 2 {
		t.Errorf("worker-a count: expected 2, got %d", c)
	}
	if c := mb.Count("worker-b"); c != 1 {
		t.Errorf("worker-b count: expected 1, got %d", c)
	}
	if c := mb.Count("worker-c"); c != 1 {
		t.Errorf("worker-c count: expected 1, got %d", c)
	}

	// 3. Deliver A -- B and C should be unaffected.
	deliveredA := mb.Deliver("worker-a")
	if len(deliveredA) != 2 {
		t.Errorf("expected 2 delivered for worker-a, got %d", len(deliveredA))
	}

	// worker-a should now have 0 pending.
	if c := mb.Count("worker-a"); c != 0 {
		t.Errorf("worker-a count after deliver: expected 0, got %d", c)
	}

	// worker-b and worker-c should still have 1 pending each.
	if c := mb.Count("worker-b"); c != 1 {
		t.Errorf("worker-b count after delivering A: expected 1, got %d", c)
	}
	if c := mb.Count("worker-c"); c != 1 {
		t.Errorf("worker-c count after delivering A: expected 1, got %d", c)
	}

	// Verify B's content is intact.
	bMsgs := mb.Peek("worker-b")
	if len(bMsgs) != 1 {
		t.Fatalf("expected 1 for worker-b, got %d", len(bMsgs))
	}
	if bMsgs[0].Content != "task for worker-b" {
		t.Errorf("worker-b content: got %q, want %q", bMsgs[0].Content, "task for worker-b")
	}
}
