package growth

type LevelConfig struct {
	CLITool         string   `json:"cliTool"`
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	AllowedTools    []string `json:"allowedTools"`
	DisallowedTools []string `json:"disallowedTools"`
	PermissionMode  string   `json:"permissionMode"`
	MaxTokenBudget  int      `json:"maxTokenBudget"`
	ExtraPrompt     string   `json:"extraPrompt"`
	CanMentor       bool     `json:"canMentor"`
	CanReview       bool     `json:"canReview"`
}

var levelDefaults = map[int]LevelConfig{
	// Level 1 originally targeted ais-agent + ollama for cheap-junior
	// learning, but ais-agent v0.1.0's ollama provider was never wired
	// end-to-end (validate rejects it — see internal/agent/aisagent),
	// and the claude fallback path hits its own ~120s DetectReady
	// timeout when spawned cold. The combination meant every level-1
	// worker (default for new hires) got stuck in a 2-min health-check
	// retry loop on first task assignment — observed live 2026-05-07
	// when the smoke task hit Michael (a default-level-1 devops).
	//
	// Switching to claude+haiku puts juniors on the same battle-tested
	// runtime path as the rest of the tiers. Permission mode stays
	// "plan" so juniors still operate read-mostly. Ollama support can
	// come back via a future PR once ais-agent actually implements it.
	1: {
		CLITool:        "claude",
		Provider:       "",
		Model:          "claude-haiku-4-5",
		PermissionMode: "plan",
		AllowedTools:   []string{"Read", "Glob", "Grep"},
		MaxTokenBudget: 50000,
		ExtraPrompt:    "You are a junior developer. Ask for help when unsure. Focus on learning.",
	},
	2: {
		CLITool:        "ais-agent",
		Provider:       "openai",
		Model:          "gpt-4o-mini",
		PermissionMode: "acceptEdits",
		AllowedTools:   []string{"Read", "Glob", "Grep", "Edit", "Write"},
		MaxTokenBudget: 100000,
		ExtraPrompt:    "You are developing your skills. Write tests for your changes.",
	},
	3: {
		CLITool:        "ais-agent",
		Provider:       "openai",
		Model:          "gpt-4o",
		PermissionMode: "acceptEdits",
		AllowedTools:   nil,
		MaxTokenBudget: 200000,
		ExtraPrompt:    "You are an experienced developer. Follow best practices consistently.",
	},
	4: {
		CLITool:        "claude",
		Provider:       "",
		Model:          "claude-sonnet-4-20250514",
		PermissionMode: "bypassPermissions",
		AllowedTools:   nil,
		MaxTokenBudget: 500000,
		CanReview:      true,
		ExtraPrompt:    "You are a senior developer. Consider architecture impacts and mentor others.",
	},
	5: {
		CLITool:        "claude",
		Provider:       "",
		Model:          "claude-opus-4-20250514",
		PermissionMode: "bypassPermissions",
		AllowedTools:   nil,
		MaxTokenBudget: 1000000,
		CanMentor:      true,
		CanReview:      true,
		ExtraPrompt:    "You are an expert. Drive technical excellence and guide the team.",
	},
}

func MapLevelToConfig(level int) LevelConfig {
	if level < 1 {
		level = 1
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	return levelDefaults[level]
}

func EffectiveConfig(st *SkillTree, branch SkillBranch) LevelConfig {
	node, ok := st.Branches[branch]
	if !ok {
		return MapLevelToConfig(1)
	}
	return MapLevelToConfig(node.Level)
}
