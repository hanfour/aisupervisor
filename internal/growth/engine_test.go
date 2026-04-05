package growth

import (
	"testing"
	"time"
)

func TestEngine_ProcessTaskCompleted(t *testing.T) {
	engine := NewEngine()
	tree := NewSkillTree()
	engine.SetSkillTree("w1", tree)

	events := engine.ProcessTaskCompleted("w1", TaskCompletedInfo{
		TaskType:               "code",
		ReviewPassedFirstTime:  true,
		ReviewAttempts:         1,
		TokenEfficiency:        0.8,
		VerifyCmdPassed:        true,
		ConsecutiveCompletions: 1,
		CompletionTime:         30 * time.Minute,
	})

	if len(events) == 0 {
		t.Fatal("should produce at least one growth event")
	}
	if events[0].Type != GrowthEXPGained {
		t.Errorf("first event should be exp_gained, got %s", events[0].Type)
	}
	if events[0].Amount <= 0 {
		t.Error("EXP amount should be positive")
	}

	if tree.Branches[BranchBackend].TotalEXP == 0 {
		t.Error("backend branch should have gained EXP")
	}
	if tree.Branches[BranchFrontend].TotalEXP == 0 {
		t.Error("frontend branch should have gained EXP")
	}
}

func TestEngine_ProcessTaskCompleted_LevelUp(t *testing.T) {
	engine := NewEngine()
	tree := NewSkillTree()
	tree.Branches[BranchBackend].CurrentEXP = 95
	tree.Branches[BranchBackend].TotalEXP = 95
	engine.SetSkillTree("w1", tree)

	events := engine.ProcessTaskCompleted("w1", TaskCompletedInfo{
		TaskType:               "code",
		ReviewPassedFirstTime:  true,
		ReviewAttempts:         1,
		TokenEfficiency:        0.9,
		VerifyCmdPassed:        true,
		ConsecutiveCompletions: 1,
		CompletionTime:         20 * time.Minute,
	})

	hasLevelUp := false
	for _, e := range events {
		if e.Type == GrowthLevelUp {
			hasLevelUp = true
		}
	}
	if !hasLevelUp {
		t.Error("should have triggered a level-up event")
	}
}
