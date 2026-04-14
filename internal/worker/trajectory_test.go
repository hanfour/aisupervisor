package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrajectoryRecorder_WritesValidJSONL(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	entry := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 14, 10, 30, 0, 0, time.UTC),
		WorkerID:  "worker-1",
		TaskID:    "task-abc",
		Event:     TrajectoryEventSpawn,
		Details:   "spawned claude in tmux session aiworker-worker-1",
	}

	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Read the file and verify JSONL format
	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	lines := strings.TrimSpace(string(data))
	var decoded TrajectoryEntry
	if err := json.Unmarshal([]byte(lines), &decoded); err != nil {
		t.Fatalf("invalid JSON line: %v", err)
	}

	if decoded.WorkerID != "worker-1" {
		t.Errorf("WorkerID: expected worker-1, got %s", decoded.WorkerID)
	}
	if decoded.TaskID != "task-abc" {
		t.Errorf("TaskID: expected task-abc, got %s", decoded.TaskID)
	}
	if decoded.Event != TrajectoryEventSpawn {
		t.Errorf("Event: expected %s, got %s", TrajectoryEventSpawn, decoded.Event)
	}
}

func TestTrajectoryRecorder_MultipleRecordsAppend(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	entries := []TrajectoryEntry{
		{Timestamp: now, WorkerID: "w1", TaskID: "t1", Event: TrajectoryEventSpawn, Details: "first"},
		{Timestamp: now.Add(1 * time.Minute), WorkerID: "w1", TaskID: "t1", Event: TrajectoryEventPromptSent, Details: "second"},
		{Timestamp: now.Add(5 * time.Minute), WorkerID: "w1", TaskID: "t1", Event: TrajectoryEventCompletionDetected, Details: "third"},
	}

	for _, e := range entries {
		if err := rec.Record(e); err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	// Read and count lines
	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var entry TrajectoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
		if entry.Details != entries[i].Details {
			t.Errorf("line %d: expected details %q, got %q", i, entries[i].Details, entry.Details)
		}
	}
}

func TestTrajectoryRecorder_DateBasedFilename(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	// Record on April 14
	e1 := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 14, 23, 59, 0, 0, time.UTC),
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
	}
	if err := rec.Record(e1); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Record on April 15
	e2 := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 15, 0, 1, 0, 0, time.UTC),
		WorkerID:  "w2",
		TaskID:    "t2",
		Event:     TrajectoryEventSpawn,
	}
	if err := rec.Record(e2); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Verify two separate files exist
	f1 := filepath.Join(dir, "2026-04-14.jsonl")
	f2 := filepath.Join(dir, "2026-04-15.jsonl")

	if _, err := os.Stat(f1); os.IsNotExist(err) {
		t.Error("expected 2026-04-14.jsonl to exist")
	}
	if _, err := os.Stat(f2); os.IsNotExist(err) {
		t.Error("expected 2026-04-15.jsonl to exist")
	}
}

func TestTrajectoryRecorder_TokensUsedField(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)

	entry := TrajectoryEntry{
		Timestamp:  time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		WorkerID:   "w1",
		TaskID:     "t1",
		Event:      TrajectoryEventCompletionDetected,
		Details:    "task completed",
		TokensUsed: 45000,
	}
	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var decoded TrajectoryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.TokensUsed != 45000 {
		t.Errorf("TokensUsed: expected 45000, got %d", decoded.TokensUsed)
	}
}

func TestTrajectoryRecorder_CreatesDirectoryIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "trajectories")
	rec := NewTrajectoryRecorder(dir)

	entry := TrajectoryEntry{
		Timestamp: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
	}
	if err := rec.Record(entry); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Error("expected file to be created in auto-created directory")
	}
}
