package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Trajectory event constants.
const (
	TrajectoryEventSpawn              = "spawn"
	TrajectoryEventPromptSent         = "prompt_sent"
	TrajectoryEventCompletionDetected = "completion_detected"
	TrajectoryEventReviewApproved     = "review_approved"
	TrajectoryEventReviewRejected     = "review_rejected"
)

// TrajectoryEntry is a single event in a worker's execution timeline.
type TrajectoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	WorkerID   string    `json:"worker_id"`
	TaskID     string    `json:"task_id"`
	Event      string    `json:"event"`
	Details    string    `json:"details,omitempty"`
	TokensUsed int64     `json:"tokens_used,omitempty"`
}

// TrajectoryRecorder appends trajectory events to date-based JSONL files.
type TrajectoryRecorder struct {
	mu  sync.Mutex
	dir string
}

// NewTrajectoryRecorder creates a recorder that writes to the given directory.
// Default directory is ~/.local/share/aisupervisor/trajectories/ if dir is empty.
func NewTrajectoryRecorder(dir string) *TrajectoryRecorder {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.TempDir()
			log.Printf("WARNING: UserHomeDir failed, using temp dir for trajectories: %v", err)
		}
		dir = filepath.Join(home, ".local", "share", "aisupervisor", "trajectories")
	}
	return &TrajectoryRecorder{dir: dir}
}

// Record appends a single trajectory entry as a JSON line to the date-based file.
func (r *TrajectoryRecorder) Record(entry TrajectoryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("creating trajectory dir: %w", err)
	}

	// Build filename from entry timestamp: YYYY-MM-DD.jsonl
	filename := entry.Timestamp.Format("2006-01-02") + ".jsonl"
	path := filepath.Join(r.dir, filename)

	// Marshal the entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling trajectory entry: %w", err)
	}
	data = append(data, '\n')

	// Append to file (create if not exists)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening trajectory file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing trajectory entry: %w", err)
	}

	return nil
}
