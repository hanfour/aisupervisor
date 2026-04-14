// internal/company/graph_assign_test.go
package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/knowledge"
	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func TestFindBestWorkerForCommunity_PrefersSameCommunity(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go", "internal/worker/monitor.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
			{ID: 2, Name: "internal/tmux", Files: []string{"internal/tmux/client.go"}},
		},
	}

	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeCode,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 1}, // company community
		{ID: "w2", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0}, // worker community (match!)
		{ID: "w3", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 2}, // tmux community
	}

	bestID := findBestWorkerForCommunity(task, idle, graph, map[string]bool{})
	if bestID != "w2" {
		t.Errorf("expected w2 (same community), got %q", bestID)
	}
}

func TestFindBestWorkerForCommunity_FallsBackWhenNoMatch(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
			{ID: 1, Name: "internal/company", Files: []string{"internal/company/company.go"}},
		},
	}

	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeCode,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 1}, // company — no match
		{ID: "w2", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 1}, // company — no match
	}

	bestID := findBestWorkerForCommunity(task, idle, graph, map[string]bool{})
	if bestID != "" {
		t.Errorf("expected empty (no community match), got %q", bestID)
	}
}

func TestFindBestWorkerForCommunity_SkipsAssigned(t *testing.T) {
	graph := &knowledge.CodeGraph{
		Communities: []knowledge.Community{
			{ID: 0, Name: "internal/worker", Files: []string{"internal/worker/spawner.go"}},
		},
	}

	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeCode,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0},
		{ID: "w2", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0},
	}

	assigned := map[string]bool{"w1": true}
	bestID := findBestWorkerForCommunity(task, idle, graph, assigned)
	if bestID != "w2" {
		t.Errorf("expected w2 (w1 is assigned), got %q", bestID)
	}
}

func TestFindBestWorkerForCommunity_NilGraph(t *testing.T) {
	task := &project.Task{
		ID:    "t1",
		Type:  project.TaskTypeCode,
		Files: []string{"internal/worker/spawner.go"},
	}

	idle := []idleWorkerSnapshot{
		{ID: "w1", SkillProfile: "coder", Tier: worker.TierEngineer, LastCommunityID: 0},
	}

	bestID := findBestWorkerForCommunity(task, idle, nil, map[string]bool{})
	if bestID != "" {
		t.Errorf("expected empty for nil graph, got %q", bestID)
	}
}

// Ensure idleWorkerSnapshot was extended with LastCommunityID.
func TestIdleWorkerSnapshot_HasLastCommunityID(t *testing.T) {
	snap := idleWorkerSnapshot{
		ID:              "w1",
		SkillProfile:    "coder",
		Tier:            worker.TierEngineer,
		SkillScores:     personality.SkillScores{},
		LastCommunityID: 42,
	}
	if snap.LastCommunityID != 42 {
		t.Errorf("expected LastCommunityID 42, got %d", snap.LastCommunityID)
	}
}
