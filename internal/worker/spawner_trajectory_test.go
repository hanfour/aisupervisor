package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/config"
)

func TestSpawner_SetTrajectoryRecorder(t *testing.T) {
	s := &Spawner{
		tierConfigs:    make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:  make(map[string]config.SkillProfile),
		skillOverrides: make(map[string]config.SkillProfileOverride),
	}

	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)
	s.SetTrajectoryRecorder(rec)

	if s.trajectoryRecorder == nil {
		t.Error("expected trajectoryRecorder to be set")
	}
}

func TestSpawner_RecordTrajectory_NilRecorderNoOp(t *testing.T) {
	s := &Spawner{
		tierConfigs:    make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:  make(map[string]config.SkillProfile),
		skillOverrides: make(map[string]config.SkillProfileOverride),
	}

	// Should not panic when recorder is nil
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: time.Now(),
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
	})
}

func TestSpawner_RecordTrajectory_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	rec := NewTrajectoryRecorder(dir)
	s := &Spawner{
		tierConfigs:        make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:      make(map[string]config.SkillProfile),
		skillOverrides:     make(map[string]config.SkillProfileOverride),
		trajectoryRecorder: rec,
	}

	now := time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	s.recordTrajectory(TrajectoryEntry{
		Timestamp: now,
		WorkerID:  "w1",
		TaskID:    "t1",
		Event:     TrajectoryEventSpawn,
		Details:   "test spawn",
	})

	// Verify file was written
	expectedFile := filepath.Join(dir, "2026-04-14.jsonl")
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var entry TrajectoryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Event != TrajectoryEventSpawn {
		t.Errorf("Event: expected spawn, got %s", entry.Event)
	}
	if entry.Details != "test spawn" {
		t.Errorf("Details: expected 'test spawn', got %s", entry.Details)
	}
}
