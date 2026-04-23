package company

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// EnvelopeStatus tracks the delivery lifecycle of a message.
type EnvelopeStatus string

const (
	EnvPending   EnvelopeStatus = "pending"
	EnvDelivered EnvelopeStatus = "delivered"
	EnvRead      EnvelopeStatus = "read"
	EnvExpired   EnvelopeStatus = "expired"
)

// Envelope wraps a StructuredMessage with delivery metadata.
type Envelope struct {
	StructuredMessage `yaml:",inline"`
	Status            EnvelopeStatus `yaml:"status" json:"status"`
	ReadAt            *time.Time     `yaml:"read_at,omitempty" json:"readAt,omitempty"`
	ReplyToID         string         `yaml:"reply_to_id,omitempty" json:"replyToId,omitempty"`
	ThreadID          string         `yaml:"thread_id,omitempty" json:"threadId,omitempty"`
	TTL               time.Duration  `yaml:"ttl,omitempty" json:"ttl,omitempty"`
}

// Mailbox provides persistent per-worker message storage backed by YAML.
type Mailbox struct {
	mu       sync.RWMutex
	inbox    map[string][]Envelope // workerID → messages
	filePath string
}

// mailboxFile is the on-disk YAML representation.
type mailboxFile struct {
	Inbox map[string][]Envelope `yaml:"inbox"`
}

// NewMailbox creates or loads a Mailbox from the given data directory.
// It creates the directory and an initial empty YAML file if they do not exist.
func NewMailbox(dataDir string) (*Mailbox, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create mailbox dir: %w", err)
	}

	fp := filepath.Join(dataDir, "mailbox.yaml")
	mb := &Mailbox{
		inbox:    make(map[string][]Envelope),
		filePath: fp,
	}

	data, err := os.ReadFile(fp)
	if err == nil && len(data) > 0 {
		var mf mailboxFile
		if err := yaml.Unmarshal(data, &mf); err != nil {
			return nil, fmt.Errorf("parse mailbox.yaml: %w", err)
		}
		if mf.Inbox != nil {
			mb.inbox = mf.Inbox
		}
	}

	// Save initial file so it always exists on disk.
	if err := mb.Save(); err != nil {
		return nil, fmt.Errorf("save initial mailbox: %w", err)
	}
	return mb, nil
}

// Send validates and stores an envelope in the recipient's inbox.
func (mb *Mailbox) Send(env Envelope) error {
	if env.From == "" {
		return fmt.Errorf("mailbox send: From is required")
	}
	if env.To == "" {
		return fmt.Errorf("mailbox send: To is required")
	}

	mb.mu.Lock()
	defer mb.mu.Unlock()

	if env.ID == "" {
		env.ID = fmt.Sprintf("msg-%d-%04d", time.Now().UnixNano()/1e6, rand.Intn(10000))
	}
	if env.Timestamp.IsZero() {
		env.Timestamp = time.Now()
	}
	env.Status = EnvPending

	mb.inbox[env.To] = append(mb.inbox[env.To], env)
	return mb.saveLocked()
}

// Peek returns copies of all pending envelopes for the worker without changing status.
func (mb *Mailbox) Peek(workerID string) []Envelope {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var result []Envelope
	for _, env := range mb.inbox[workerID] {
		if env.Status == EnvPending {
			result = append(result, env)
		}
	}
	return result
}

// Deliver returns all pending envelopes for the worker and marks them as delivered.
func (mb *Mailbox) Deliver(workerID string) []Envelope {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	envs := mb.inbox[workerID]
	if len(envs) == 0 {
		return nil
	}

	var delivered []Envelope
	for i := range envs {
		if envs[i].Status == EnvPending {
			delivered = append(delivered, envs[i])
			envs[i].Status = EnvDelivered
		}
	}
	mb.inbox[workerID] = envs

	if len(delivered) > 0 {
		_ = mb.saveLocked()
	}
	return delivered
}

// MarkRead sets the status to read and records the read timestamp.
func (mb *Mailbox) MarkRead(messageID string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	now := time.Now()
	for wid := range mb.inbox {
		for i := range mb.inbox[wid] {
			if mb.inbox[wid][i].ID == messageID {
				mb.inbox[wid][i].Status = EnvRead
				mb.inbox[wid][i].ReadAt = &now
				_ = mb.saveLocked()
				return
			}
		}
	}
}

// Reply sends a reply to an existing message, linking it via ReplyToID and ThreadID.
func (mb *Mailbox) Reply(originalID string, reply Envelope) error {
	mb.mu.RLock()
	var original *Envelope
	for wid := range mb.inbox {
		for i := range mb.inbox[wid] {
			if mb.inbox[wid][i].ID == originalID {
				orig := mb.inbox[wid][i]
				original = &orig
				break
			}
		}
		if original != nil {
			break
		}
	}
	mb.mu.RUnlock()

	if original == nil {
		return fmt.Errorf("mailbox reply: original message %q not found", originalID)
	}

	reply.ReplyToID = originalID
	if original.ThreadID != "" {
		reply.ThreadID = original.ThreadID
	} else {
		reply.ThreadID = original.ID
	}

	return mb.Send(reply)
}

// GetThread collects all envelopes belonging to a thread, sorted by timestamp.
func (mb *Mailbox) GetThread(threadID string) []Envelope {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var thread []Envelope
	for _, envs := range mb.inbox {
		for _, env := range envs {
			if env.ThreadID == threadID || env.ID == threadID {
				thread = append(thread, env)
			}
		}
	}

	sort.Slice(thread, func(i, j int) bool {
		return thread[i].Timestamp.Before(thread[j].Timestamp)
	})
	return thread
}

// Expire removes messages older than maxAge or whose TTL has elapsed.
// Returns the number of removed messages.
func (mb *Mailbox) Expire(maxAge time.Duration) int {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	now := time.Now()
	removed := 0

	for wid := range mb.inbox {
		var kept []Envelope
		for _, env := range mb.inbox[wid] {
			expired := false
			if env.TTL > 0 && now.Sub(env.Timestamp) > env.TTL {
				expired = true
			} else if now.Sub(env.Timestamp) > maxAge {
				expired = true
			}

			if expired {
				removed++
			} else {
				kept = append(kept, env)
			}
		}
		mb.inbox[wid] = kept
	}

	if removed > 0 {
		_ = mb.saveLocked()
	}
	return removed
}

// Save marshals the mailbox to YAML and writes it to disk.
func (mb *Mailbox) Save() error {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return mb.saveLocked()
}

// saveLocked writes to disk; caller must hold at least a read lock.
func (mb *Mailbox) saveLocked() error {
	mf := mailboxFile{Inbox: mb.inbox}
	data, err := yaml.Marshal(&mf)
	if err != nil {
		return fmt.Errorf("marshal mailbox: %w", err)
	}
	if err := os.WriteFile(mb.filePath, data, 0644); err != nil {
		return fmt.Errorf("write mailbox.yaml: %w", err)
	}
	return nil
}

// Count returns the number of pending messages for a worker.
func (mb *Mailbox) Count(workerID string) int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	n := 0
	for _, env := range mb.inbox[workerID] {
		if env.Status == EnvPending {
			n++
		}
	}
	return n
}
