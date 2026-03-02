package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultBackend string               `yaml:"default_backend"`
	Polling        PollingConfig        `yaml:"polling"`
	Decision       DecisionConfig       `yaml:"decision"`
	Context        ContextConfig        `yaml:"context"`
	Backends       []BackendConfig      `yaml:"backends"`
	AutoApprove    []AutoApproveRule    `yaml:"auto_approve_rules"`
	Audit          AuditConfig          `yaml:"audit"`
	Roles          []RoleConfig         `yaml:"roles,omitempty"`
	RolesDir       string               `yaml:"roles_dir,omitempty"`
	Groups         []GroupConfig        `yaml:"groups,omitempty"`
	SessionRoles   []SessionRoleBinding `yaml:"session_roles,omitempty"`
	Messaging      MessagingConfig      `yaml:"messaging,omitempty"`
	WorkerTiers    []WorkerTierConfig   `yaml:"worker_tiers,omitempty"`
	SkillProfiles  []SkillProfile       `yaml:"skill_profiles,omitempty"`
	Training       TrainingConfig       `yaml:"training,omitempty"`
}

type WorkerTierConfig struct {
	Tier       string `yaml:"tier"`
	CLITool    string `yaml:"cli_tool"`
	CLIArgs    string `yaml:"cli_args,omitempty"`
	BackendID  string `yaml:"backend_id,omitempty"`
	MaxWorkers int    `yaml:"max_workers,omitempty"`
	ReadyCheck string `yaml:"ready_check,omitempty"`
}

type SkillProfile struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description,omitempty"`
	Icon            string   `yaml:"icon,omitempty"`
	SystemPrompt    string   `yaml:"system_prompt,omitempty"`
	AllowedTools    []string `yaml:"allowed_tools,omitempty"`
	DisallowedTools []string `yaml:"disallowed_tools,omitempty"`
	Model           string   `yaml:"model,omitempty"`
	PermissionMode  string   `yaml:"permission_mode,omitempty"`
	ExtraCLIArgs    string   `yaml:"extra_cli_args,omitempty"`
}

type TrainingConfig struct {
	Enabled      bool            `yaml:"enabled"`
	DataDir      string          `yaml:"data_dir,omitempty"`
	CaptureDiffs bool            `yaml:"capture_diffs,omitempty"`
	Finetune     FinetuneConfig  `yaml:"finetune,omitempty"`
	Promotion    PromotionConfig `yaml:"promotion,omitempty"`
}

type FinetuneConfig struct {
	Method       string  `yaml:"method,omitempty"`        // "sft" or "dpo"
	BaseModel    string  `yaml:"base_model,omitempty"`
	OutputModel  string  `yaml:"output_model,omitempty"`
	ScriptPath   string  `yaml:"script_path,omitempty"`
	AutoTrigger  int     `yaml:"auto_trigger,omitempty"`  // min pairs before auto-trigger (0=disabled)
	ValRatio     float64 `yaml:"val_ratio,omitempty"`
}

type PromotionConfig struct {
	Enabled            bool    `yaml:"enabled,omitempty"`
	MinTrainingPairs   int     `yaml:"min_training_pairs,omitempty"`
	MinBenchmarkScore  float64 `yaml:"min_benchmark_score,omitempty"`
	ConsecutivePasses  int     `yaml:"consecutive_passes,omitempty"`
	MinApprovalRate    float64 `yaml:"min_approval_rate,omitempty"`
}

type MessagingConfig struct {
	Slack       SlackConfig `yaml:"slack,omitempty"`
	Line        LineConfig  `yaml:"line,omitempty"`
	NotifyEvents []string   `yaml:"notify_events,omitempty"` // global filter; empty = all events
}

type SlackConfig struct {
	Enabled      bool     `yaml:"enabled"`
	BotTokenEnv  string   `yaml:"bot_token_env"`
	AppTokenEnv  string   `yaml:"app_token_env"`
	ChannelID    string   `yaml:"channel_id"`
	NotifyEvents []string `yaml:"notify_events,omitempty"` // per-messenger override
}

type LineConfig struct {
	Enabled          bool     `yaml:"enabled"`
	ChannelSecretEnv string   `yaml:"channel_secret_env"`
	ChannelTokenEnv  string   `yaml:"channel_token_env"`
	NotifyUserID     string   `yaml:"notify_user_id"`
	Port             int      `yaml:"port"`
	NotifyEvents     []string `yaml:"notify_events,omitempty"` // per-messenger override
}

type GroupConfig struct {
	ID                  string   `yaml:"id"`
	Name                string   `yaml:"name"`
	LeaderID            string   `yaml:"leader_id"`
	RoleIDs             []string `yaml:"role_ids"`
	DivergenceThreshold float64  `yaml:"divergence_threshold"`
}

type SessionRoleBinding struct {
	SessionID string   `yaml:"session_id"`
	RoleIDs   []string `yaml:"role_ids"`
}

type RoleConfig struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description,omitempty"`
	Mode            string   `yaml:"mode"`
	SystemPrompt    string   `yaml:"system_prompt,omitempty"`
	Backend         string   `yaml:"backend,omitempty"`
	Priority        int      `yaml:"priority"`
	Enabled         bool     `yaml:"enabled"`
	TriggerPatterns []string `yaml:"trigger_patterns,omitempty"`
	CooldownSec     int      `yaml:"cooldown_sec,omitempty"`
	ResponseFormat  string   `yaml:"response_format,omitempty"`
	Avatar          string   `yaml:"avatar,omitempty"`
}

type ContextConfig struct {
	Enabled            bool `yaml:"enabled"`
	MaxDecisions       int  `yaml:"max_decisions"`
	MaxActivities      int  `yaml:"max_activities"`
	TokenBudget        int  `yaml:"token_budget"`
	ActivityIntervalSec int  `yaml:"activity_interval_sec"`
}

type PollingConfig struct {
	IntervalMs   int `yaml:"interval_ms"`
	ContextLines int `yaml:"context_lines"`
}

type DecisionConfig struct {
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
	TimeoutSeconds      int     `yaml:"timeout_seconds"`
}

type BackendConfig struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
	Model     string `yaml:"model,omitempty"`
	BaseURL   string `yaml:"base_url,omitempty"`
}

type AutoApproveRule struct {
	Label           string `yaml:"label"`
	PatternContains string `yaml:"pattern_contains"`
	Response        string `yaml:"response"`
}

type AuditConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Polling: PollingConfig{
			IntervalMs:   500,
			ContextLines: 100,
		},
		Decision: DecisionConfig{
			ConfidenceThreshold: 0.7,
			TimeoutSeconds:      30,
		},
		Context: ContextConfig{
			Enabled:            true,
			MaxDecisions:       20,
			MaxActivities:      10,
			TokenBudget:        2000,
			ActivityIntervalSec: 60,
		},
		DefaultBackend: "claude-oauth",
		Backends: []BackendConfig{
			{
				Name:  "claude-oauth",
				Type:  "anthropic_oauth",
				Model: "claude-sonnet-4-6-20260301",
			},
			{
				Name:      "claude-api",
				Type:      "anthropic_api",
				APIKeyEnv: "ANTHROPIC_API_KEY",
				Model:     "claude-sonnet-4-6-20260301",
			},
		},
		AutoApprove: []AutoApproveRule{
			{Label: "bash", PatternContains: "Bash", Response: "y"},
			{Label: "edit", PatternContains: "Edit", Response: "y"},
			{Label: "write", PatternContains: "Write", Response: "y"},
			{Label: "git", PatternContains: "git", Response: "y"},
		},
		Audit: AuditConfig{
			Enabled: true,
			Path:    filepath.Join(home, ".local", "share", "aisupervisor", "audit.jsonl"),
		},
	}
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aisupervisor", "config.yaml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

// Validate checks the config for common errors.
func (c *Config) Validate() error {
	validTiers := map[string]bool{"consultant": true, "manager": true, "engineer": true}
	validCLITools := map[string]bool{"claude": true, "aider": true, "": true}
	for _, wt := range c.WorkerTiers {
		if !validTiers[wt.Tier] {
			return fmt.Errorf("invalid worker tier %q (must be consultant, manager, or engineer)", wt.Tier)
		}
		if !validCLITools[wt.CLITool] {
			return fmt.Errorf("invalid cli_tool %q for tier %q (must be claude or aider)", wt.CLITool, wt.Tier)
		}
		if wt.MaxWorkers < 0 {
			return fmt.Errorf("max_workers for tier %q must be >= 0", wt.Tier)
		}
		if wt.ReadyCheck != "" {
			if _, err := regexp.Compile(wt.ReadyCheck); err != nil {
				return fmt.Errorf("invalid ready_check regex for tier %q: %w", wt.Tier, err)
			}
		}
	}

	if c.Training.Promotion.MinBenchmarkScore < 0 || c.Training.Promotion.MinBenchmarkScore > 1 {
		return fmt.Errorf("min_benchmark_score must be between 0.0 and 1.0")
	}
	if c.Training.Promotion.MinApprovalRate < 0 || c.Training.Promotion.MinApprovalRate > 1 {
		return fmt.Errorf("min_approval_rate must be between 0.0 and 1.0")
	}

	if c.Training.Finetune.AutoTrigger < 0 {
		return fmt.Errorf("auto_trigger must be >= 0")
	}

	if c.Decision.ConfidenceThreshold < 0 || c.Decision.ConfidenceThreshold > 1 {
		return fmt.Errorf("confidence_threshold must be between 0.0 and 1.0")
	}

	return nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
