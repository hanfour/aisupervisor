package config

import (
	"os"
	"path/filepath"

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
		DefaultBackend: "claude-api",
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
		Backends: []BackendConfig{
			{
				Name:      "claude-api",
				Type:      "anthropic_api",
				APIKeyEnv: "ANTHROPIC_API_KEY",
				Model:     "claude-sonnet-4-6-20260301",
			},
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
	return cfg, nil
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
