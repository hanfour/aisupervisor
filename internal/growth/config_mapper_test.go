package growth

import "testing"

func TestMapLevelToConfig_Level1(t *testing.T) {
	cfg := MapLevelToConfig(1)
	if cfg.CLITool != "ais-agent" {
		t.Errorf("level 1 CLITool should be ais-agent, got %s", cfg.CLITool)
	}
	if cfg.Provider != "ollama" {
		t.Errorf("level 1 provider should be ollama, got %s", cfg.Provider)
	}
	if cfg.Model != "llama3" {
		t.Errorf("level 1 model should be llama3, got %s", cfg.Model)
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
	if cfg.CLITool != "claude" {
		t.Errorf("level 4 CLITool should be claude, got %s", cfg.CLITool)
	}
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("level 4 model should be claude-sonnet-4-20250514, got %s", cfg.Model)
	}
	if !cfg.CanReview {
		t.Error("level 4 should be able to review")
	}
}

func TestMapLevelToConfig_Level5(t *testing.T) {
	cfg := MapLevelToConfig(5)
	if cfg.CLITool != "claude" {
		t.Errorf("level 5 CLITool should be claude, got %s", cfg.CLITool)
	}
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
	if cfg.CLITool != "claude" {
		t.Errorf("level 4 should use claude, got %s", cfg.CLITool)
	}
}

func TestMapLevelToConfig_AISAgentLevels(t *testing.T) {
	for level := 1; level <= 3; level++ {
		cfg := MapLevelToConfig(level)
		if cfg.CLITool != "ais-agent" {
			t.Errorf("level %d should use ais-agent, got %s", level, cfg.CLITool)
		}
	}
	for level := 4; level <= 5; level++ {
		cfg := MapLevelToConfig(level)
		if cfg.CLITool != "claude" {
			t.Errorf("level %d should use claude, got %s", level, cfg.CLITool)
		}
	}
}
