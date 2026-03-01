package role

// RoleConfig defines a role in YAML configuration.
type RoleConfig struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description,omitempty"`
	Mode            Mode     `yaml:"mode"`
	SystemPrompt    string   `yaml:"system_prompt"`
	Backend         string   `yaml:"backend,omitempty"`
	Priority        int      `yaml:"priority"`
	Enabled         bool     `yaml:"enabled"`
	TriggerPatterns []string `yaml:"trigger_patterns,omitempty"`
	CooldownSec     int      `yaml:"cooldown_sec,omitempty"`
	ResponseFormat  string   `yaml:"response_format,omitempty"` // "option" | "freetext" | "either"
}
