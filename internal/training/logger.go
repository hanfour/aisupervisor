package training

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Verdict represents the review outcome.
type Verdict string

const (
	VerdictAccepted Verdict = "accepted"
	VerdictRejected Verdict = "rejected"
	VerdictRevised  Verdict = "revised"
)

// ReviewPair captures one engineer→manager review cycle for training data.
type ReviewPair struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	TaskID         string    `json:"task_id"`
	ProjectID      string    `json:"project_id"`
	EngineerID     string    `json:"engineer_id"`
	ManagerID      string    `json:"manager_id"`
	EngineerModel  string    `json:"engineer_model"`
	ManagerModel   string    `json:"manager_model"`
	ModelVersion   string    `json:"model_version"`
	Prompt         string    `json:"prompt"`
	EngineerOutput string    `json:"engineer_output"`
	ManagerOutput  string    `json:"manager_output"`
	Verdict        Verdict   `json:"verdict"`
	Feedback       string    `json:"feedback"`
	DiffSummary    string    `json:"diff_summary"`
	DurationMs     int64     `json:"duration_ms"`
}

// Logger writes ReviewPairs as JSONL, compatible with Llama Factory SFT/DPO.
type Logger struct {
	mu      sync.Mutex
	dataDir string
	counter int64
}

func NewLogger(dataDir string) (*Logger, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating training data dir: %w", err)
	}
	return &Logger{dataDir: dataDir}, nil
}

// Log appends a ReviewPair to the JSONL file.
func (l *Logger) Log(pair ReviewPair) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if pair.ID == "" {
		l.counter++
		pair.ID = fmt.Sprintf("rp-%d-%d", time.Now().UnixMilli(), l.counter)
	}
	if pair.Timestamp.IsZero() {
		pair.Timestamp = time.Now()
	}

	data, err := json.Marshal(pair)
	if err != nil {
		return fmt.Errorf("marshaling review pair: %w", err)
	}

	path := filepath.Join(l.dataDir, "review_pairs.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening JSONL file: %w", err)
	}
	defer f.Close()

	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// DataDir returns the configured data directory.
func (l *Logger) DataDir() string {
	return l.dataDir
}
