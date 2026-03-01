package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/detector"
)

type Entry struct {
	Timestamp        time.Time           `json:"timestamp"`
	SessionID        string              `json:"session_id"`
	SessionName      string              `json:"session_name"`
	PromptType       detector.PromptType `json:"prompt_type"`
	Summary          string              `json:"summary"`
	ChosenKey        string              `json:"chosen_key"`
	ChosenLabel      string              `json:"chosen_label"`
	Reasoning        string              `json:"reasoning"`
	Confidence       float64             `json:"confidence"`
	AutoApprove      bool                `json:"auto_approve"`
	DryRun           bool                `json:"dry_run"`
	Backend          string              `json:"backend,omitempty"`
	RoleID           string              `json:"role_id,omitempty"`
	InterventionType string              `json:"intervention_type,omitempty"`
}

// DiscussionEntry records a discussion event to the audit log.
type DiscussionEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	EntryType    string    `json:"entry_type"` // always "discussion"
	DiscussionID string    `json:"discussion_id"`
	SessionID    string    `json:"session_id"`
	GroupID      string    `json:"group_id"`
	Phase        string    `json:"phase"`
	RoleID       string    `json:"role_id"`
	RoleName     string    `json:"role_name"`
	Action       string    `json:"action,omitempty"`
	Message      string    `json:"message,omitempty"`
	Confidence   float64   `json:"confidence"`
}

type Logger struct {
	mu      sync.Mutex
	file    *os.File
	enabled bool
}

func NewLogger(path string, enabled bool) (*Logger, error) {
	if !enabled {
		return &Logger{enabled: false}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating audit dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening audit log: %w", err)
	}

	return &Logger{file: f, enabled: true}, nil
}

func (l *Logger) Log(entry Entry) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(l.file, "%s\n", data)
	return err
}

func (l *Logger) LogDiscussion(entry DiscussionEntry) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	entry.EntryType = "discussion"

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(l.file, "%s\n", data)
	return err
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
