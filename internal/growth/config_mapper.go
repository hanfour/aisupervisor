package growth

type LevelConfig struct {
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
	1: {
		Model:          "haiku",
		PermissionMode: "plan",
		AllowedTools:   []string{"Read", "Glob", "Grep"},
		MaxTokenBudget: 50000,
		ExtraPrompt:    "You are a junior developer. Ask for help when unsure. Focus on learning.",
	},
	2: {
		Model:          "sonnet",
		PermissionMode: "acceptEdits",
		AllowedTools:   []string{"Read", "Glob", "Grep", "Edit", "Write"},
		MaxTokenBudget: 100000,
		ExtraPrompt:    "You are developing your skills. Write tests for your changes.",
	},
	3: {
		Model:          "sonnet",
		PermissionMode: "acceptEdits",
		AllowedTools:   nil,
		MaxTokenBudget: 200000,
		ExtraPrompt:    "You are an experienced developer. Follow best practices consistently.",
	},
	4: {
		Model:          "opus",
		PermissionMode: "bypassPermissions",
		AllowedTools:   nil,
		MaxTokenBudget: 500000,
		CanReview:      true,
		ExtraPrompt:    "You are a senior developer. Consider architecture impacts and mentor others.",
	},
	5: {
		Model:          "opus",
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
