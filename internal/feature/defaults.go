package feature

type FeatureFlag struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	EnvVar      string `yaml:"envVar,omitempty" json:"envVar,omitempty"`
}

var DefaultFlags = []FeatureFlag{
	{ID: "growth_engine", Name: "成長引擎", Enabled: true},
	{ID: "skill_tree", Name: "技能樹", Enabled: true},
	{ID: "exp_system", Name: "經驗值系統", Enabled: true},
	{ID: "level_config_map", Name: "等級配置映射", Enabled: true},
	{ID: "boss_feedback", Name: "老闆回饋系統", Enabled: true},
	{ID: "boss_chat_learn", Name: "對話偏好學習", Enabled: true},
	{ID: "knowledge_base", Name: "專案知識庫", Enabled: true},
	{ID: "knowledge_inject", Name: "知識自動注入", Enabled: true},
	{ID: "task_auto_summary", Name: "任務自動摘要", Enabled: true},
	{ID: "auto_pair", Name: "自動配對建議", Enabled: false},
	{ID: "difficulty_gate", Name: "任務難度門檻", Enabled: false},
	{ID: "policy_engine", Name: "規則引擎", Enabled: false},
	{ID: "pixel_evolution", Name: "像素外觀進化", Enabled: false},
	{ID: "achievement_system", Name: "成就系統", Enabled: false},
}
