package company

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func tempMailbox(t *testing.T) *Mailbox {
	t.Helper()
	dir := t.TempDir()
	mb, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}
	return mb
}

func TestMailbox_NewEmpty(t *testing.T) {
	dir := t.TempDir()
	mb, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}
	if mb == nil {
		t.Fatal("expected non-nil mailbox")
	}
	if len(mb.inbox) != 0 {
		t.Errorf("expected empty inbox, got %d entries", len(mb.inbox))
	}
	// File should exist after creation
	if _, err := os.Stat(filepath.Join(dir, "mailbox.yaml")); err != nil {
		t.Errorf("expected mailbox.yaml to exist: %v", err)
	}
}

func TestMailbox_SendAndPeek(t *testing.T) {
	mb := tempMailbox(t)

	env := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "supervisor",
			To:      "worker-1",
			Type:    MsgTaskAssignment,
			Content: "implement feature X",
		},
	}
	if err := mb.Send(env); err != nil {
		t.Fatalf("Send: %v", err)
	}

	msgs := mb.Peek("worker-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "implement feature X" {
		t.Errorf("unexpected content: %s", msgs[0].Content)
	}
	if msgs[0].Status != EnvPending {
		t.Errorf("expected pending status, got %s", msgs[0].Status)
	}

	// Peek should not change status
	msgs2 := mb.Peek("worker-1")
	if len(msgs2) != 1 {
		t.Errorf("peek should still return 1 pending message, got %d", len(msgs2))
	}
}

func TestMailbox_SendAutoAssignsID(t *testing.T) {
	mb := tempMailbox(t)

	env := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "supervisor",
			To:      "worker-1",
			Content: "test",
		},
	}
	if err := mb.Send(env); err != nil {
		t.Fatalf("Send: %v", err)
	}

	msgs := mb.Peek("worker-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	if msgs[0].ID == "" {
		t.Error("expected auto-assigned ID, got empty")
	}
	if len(msgs[0].ID) < 5 {
		t.Errorf("ID too short: %s", msgs[0].ID)
	}
}

func TestMailbox_SendValidation(t *testing.T) {
	mb := tempMailbox(t)

	// Empty From
	err := mb.Send(Envelope{
		StructuredMessage: StructuredMessage{
			To:      "worker-1",
			Content: "test",
		},
	})
	if err == nil {
		t.Error("expected error for empty From")
	}

	// Empty To
	err = mb.Send(Envelope{
		StructuredMessage: StructuredMessage{
			From:    "supervisor",
			Content: "test",
		},
	})
	if err == nil {
		t.Error("expected error for empty To")
	}
}

func TestMailbox_Deliver(t *testing.T) {
	mb := tempMailbox(t)

	env := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "supervisor",
			To:      "worker-1",
			Content: "do work",
		},
	}
	if err := mb.Send(env); err != nil {
		t.Fatalf("Send: %v", err)
	}

	delivered := mb.Deliver("worker-1")
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivered, got %d", len(delivered))
	}
	if delivered[0].Content != "do work" {
		t.Errorf("unexpected content: %s", delivered[0].Content)
	}

	// After deliver, Peek should return nothing (no more pending)
	pending := mb.Peek("worker-1")
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after deliver, got %d", len(pending))
	}

	// Delivering again should return empty
	again := mb.Deliver("worker-1")
	if len(again) != 0 {
		t.Errorf("expected 0 on second deliver, got %d", len(again))
	}
}

func TestMailbox_DeliverEmpty(t *testing.T) {
	mb := tempMailbox(t)

	delivered := mb.Deliver("nonexistent")
	if delivered != nil {
		t.Errorf("expected nil for empty deliver, got %v", delivered)
	}
}

func TestMailbox_MarkRead(t *testing.T) {
	mb := tempMailbox(t)

	env := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "msg-test-001",
			From:    "supervisor",
			To:      "worker-1",
			Content: "read me",
		},
	}
	if err := mb.Send(env); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Deliver first, then mark read
	mb.Deliver("worker-1")
	mb.MarkRead("msg-test-001")

	// Verify via internal state
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	found := false
	for _, envs := range mb.inbox {
		for _, e := range envs {
			if e.ID == "msg-test-001" {
				found = true
				if e.Status != EnvRead {
					t.Errorf("expected read status, got %s", e.Status)
				}
				if e.ReadAt == nil {
					t.Error("expected ReadAt to be set")
				}
			}
		}
	}
	if !found {
		t.Error("message not found in inbox")
	}
}

func TestMailbox_Reply(t *testing.T) {
	mb := tempMailbox(t)

	// Send original with a ThreadID already set
	original := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "msg-orig-001",
			From:    "supervisor",
			To:      "worker-1",
			Content: "please review",
		},
		ThreadID: "thread-abc",
	}
	if err := mb.Send(original); err != nil {
		t.Fatalf("Send: %v", err)
	}

	reply := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "worker-1",
			To:      "supervisor",
			Content: "review done",
		},
	}
	if err := mb.Reply("msg-orig-001", reply); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	msgs := mb.Peek("supervisor")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(msgs))
	}
	if msgs[0].ReplyToID != "msg-orig-001" {
		t.Errorf("expected ReplyToID=msg-orig-001, got %s", msgs[0].ReplyToID)
	}
	if msgs[0].ThreadID != "thread-abc" {
		t.Errorf("expected ThreadID=thread-abc, got %s", msgs[0].ThreadID)
	}
}

func TestMailbox_ReplyStartsThread(t *testing.T) {
	mb := tempMailbox(t)

	// Original has NO ThreadID
	original := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "msg-orig-002",
			From:    "supervisor",
			To:      "worker-1",
			Content: "first message",
		},
	}
	if err := mb.Send(original); err != nil {
		t.Fatalf("Send: %v", err)
	}

	reply := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "worker-1",
			To:      "supervisor",
			Content: "reply to first",
		},
	}
	if err := mb.Reply("msg-orig-002", reply); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	msgs := mb.Peek("supervisor")
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	// When original has no thread, thread should be created from original ID
	if msgs[0].ThreadID != "msg-orig-002" {
		t.Errorf("expected ThreadID=msg-orig-002, got %s", msgs[0].ThreadID)
	}
}

func TestMailbox_GetThread(t *testing.T) {
	mb := tempMailbox(t)

	// Create a thread: original + 2 replies
	original := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "thread-root",
			From:    "supervisor",
			To:      "worker-1",
			Content: "start",
		},
	}
	if err := mb.Send(original); err != nil {
		t.Fatalf("Send: %v", err)
	}

	r1 := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "worker-1",
			To:      "supervisor",
			Content: "reply 1",
		},
	}
	if err := mb.Reply("thread-root", r1); err != nil {
		t.Fatalf("Reply 1: %v", err)
	}

	r2 := Envelope{
		StructuredMessage: StructuredMessage{
			From:    "supervisor",
			To:      "worker-1",
			Content: "reply 2",
		},
	}
	if err := mb.Reply("thread-root", r2); err != nil {
		t.Fatalf("Reply 2: %v", err)
	}

	thread := mb.GetThread("thread-root")
	if len(thread) != 3 {
		t.Fatalf("expected 3 in thread, got %d", len(thread))
	}
	// Should be sorted by timestamp
	if thread[0].Content != "start" {
		t.Errorf("expected first message to be 'start', got %s", thread[0].Content)
	}
	if thread[1].Content != "reply 1" {
		t.Errorf("expected second to be 'reply 1', got %s", thread[1].Content)
	}
	if thread[2].Content != "reply 2" {
		t.Errorf("expected third to be 'reply 2', got %s", thread[2].Content)
	}
}

func TestMailbox_Expire(t *testing.T) {
	mb := tempMailbox(t)

	// Send an old message (manually set timestamp)
	old := Envelope{
		StructuredMessage: StructuredMessage{
			ID:        "msg-old",
			From:      "supervisor",
			To:        "worker-1",
			Content:   "ancient",
			Timestamp: time.Now().Add(-2 * time.Hour),
		},
	}
	if err := mb.Send(old); err != nil {
		t.Fatalf("Send old: %v", err)
	}

	fresh := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "msg-fresh",
			From:    "supervisor",
			To:      "worker-1",
			Content: "new",
		},
	}
	if err := mb.Send(fresh); err != nil {
		t.Fatalf("Send fresh: %v", err)
	}

	removed := mb.Expire(1 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Fresh should still be there
	msgs := mb.Peek("worker-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-fresh" {
		t.Errorf("wrong message remaining: %s", msgs[0].ID)
	}
}

func TestMailbox_ExpireWithTTL(t *testing.T) {
	mb := tempMailbox(t)

	// Message with short TTL that's already expired
	shortTTL := Envelope{
		StructuredMessage: StructuredMessage{
			ID:        "msg-ttl-expired",
			From:      "supervisor",
			To:        "worker-1",
			Content:   "ephemeral",
			Timestamp: time.Now().Add(-10 * time.Minute),
		},
		TTL: 5 * time.Minute,
	}
	if err := mb.Send(shortTTL); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Message with long TTL that's still valid
	longTTL := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "msg-ttl-valid",
			From:    "supervisor",
			To:      "worker-1",
			Content: "long-lived",
		},
		TTL: 24 * time.Hour,
	}
	if err := mb.Send(longTTL); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Use a very long maxAge so only TTL-based expiry triggers
	removed := mb.Expire(100 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed (TTL-expired), got %d", removed)
	}

	msgs := mb.Peek("worker-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-ttl-valid" {
		t.Errorf("wrong message: %s", msgs[0].ID)
	}
}

func TestMailbox_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	// Create and populate
	mb1, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	env := Envelope{
		StructuredMessage: StructuredMessage{
			ID:      "msg-persist",
			From:    "supervisor",
			To:      "worker-1",
			Type:    MsgTaskAssignment,
			Content: "persist me",
		},
		ThreadID: "thread-1",
	}
	if err := mb1.Send(env); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Reload from same dir
	mb2, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox reload: %v", err)
	}

	msgs := mb2.Peek("worker-1")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 after reload, got %d", len(msgs))
	}
	if msgs[0].ID != "msg-persist" {
		t.Errorf("expected msg-persist, got %s", msgs[0].ID)
	}
	if msgs[0].Content != "persist me" {
		t.Errorf("content mismatch: %s", msgs[0].Content)
	}
	if msgs[0].ThreadID != "thread-1" {
		t.Errorf("thread ID mismatch: %s", msgs[0].ThreadID)
	}
}

func TestMailbox_Count(t *testing.T) {
	mb := tempMailbox(t)

	// Send 3 messages
	for i := 0; i < 3; i++ {
		if err := mb.Send(Envelope{
			StructuredMessage: StructuredMessage{
				From:    "supervisor",
				To:      "worker-1",
				Content: "msg",
			},
		}); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	if c := mb.Count("worker-1"); c != 3 {
		t.Errorf("expected count 3, got %d", c)
	}

	// Deliver one batch (marks all as delivered)
	mb.Deliver("worker-1")
	if c := mb.Count("worker-1"); c != 0 {
		t.Errorf("expected count 0 after deliver, got %d", c)
	}

	// Non-existent worker
	if c := mb.Count("nobody"); c != 0 {
		t.Errorf("expected 0 for unknown worker, got %d", c)
	}
}

func TestMailbox_ConcurrentSend(t *testing.T) {
	mb := tempMailbox(t)

	var wg sync.WaitGroup
	n := 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = mb.Send(Envelope{
				StructuredMessage: StructuredMessage{
					From:    "supervisor",
					To:      "worker-1",
					Content: "concurrent",
				},
			})
		}()
	}
	wg.Wait()

	msgs := mb.Peek("worker-1")
	if len(msgs) != n {
		t.Errorf("expected %d messages, got %d", n, len(msgs))
	}
}
