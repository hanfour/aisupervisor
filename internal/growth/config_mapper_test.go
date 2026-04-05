package growth

import "testing"

func TestMapLevelToConfig_Level1(t *testing.T) {
	cfg := MapLevelToConfig(1)
	if cfg.Model != "haiku" {
		t.Errorf("level 1 model should be haiku, got %s", cfg.Model)
	}
	if cfg.PermissionMode != "plan" {
		t.Errorf("level 1 permission should be plan, got %s", cfg.PermissionMode)
	}
	if cfg.CanReview {
		t.Error("level 1 should not review")
	}
}

func TestMapLevelToConfig_Level4(t *testing.T) {
	cfg := MapLevelToConfig(4)
	if cfg.Model != "opus" {
		t.Errorf("level 4 model should be opus, got %s", cfg.Model)
	}
	if !cfg.CanReview {
		t.Error("level 4 should be able to review")
	}
}

func TestMapLevelToConfig_Level5(t *testing.T) {
	cfg := MapLevelToConfig(5)
	if !cfg.CanMentor {
		t.Error("level 5 should be able to mentor")
	}
	if cfg.MaxTokenBudget != 1000000 {
		t.Errorf("level 5 budget should be 1M, got %d", cfg.MaxTokenBudget)
	}
}

func TestEffectiveConfig_UsesHighestRelevantBranch(t *testing.T) {
	st := NewSkillTree()
	st.Branches[BranchFrontend].Level = 4
	st.Branches[BranchBackend].Level = 2

	cfg := EffectiveConfig(st, BranchFrontend)
	if cfg.Model != "opus" {
		t.Errorf("should use level 4 config for frontend task, got model %s", cfg.Model)
	}
}
