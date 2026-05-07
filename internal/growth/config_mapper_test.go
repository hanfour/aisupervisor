package growth

import "testing"

func TestMapLevelToConfig_Level1(t *testing.T) {
	// Level 1 used to target ais-agent + ollama but that runtime path
	// is broken (see config_mapper.go comment); juniors now run on
	// claude+haiku with plan-mode permissions.
	cfg := MapLevelToConfig(1)
	if cfg.CLITool != "claude" {
		t.Errorf("level 1 CLITool should be claude, got %s", cfg.CLITool)
	}
	if cfg.Provider != "" {
		t.Errorf("level 1 provider should be empty (claude doesn't take a Provider), got %s", cfg.Provider)
	}
	if cfg.Model != "claude-haiku-4-5" {
		t.Errorf("level 1 model should be claude-haiku-4-5, got %s", cfg.Model)
	}
	if cfg.PermissionMode != "plan" {
		t.Errorf("level 1 permission should be plan (read-mostly for juniors), got %s", cfg.PermissionMode)
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
	// Levels 2-3 still use ais-agent (with openai providers — those
	// work). Level 1 dropped ais-agent entirely (ollama path is
	// broken). Levels 4-5 use claude direct.
	if MapLevelToConfig(1).CLITool != "claude" {
		t.Errorf("level 1 should use claude (was ais-agent+ollama, runtime broken)")
	}
	for level := 2; level <= 3; level++ {
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
