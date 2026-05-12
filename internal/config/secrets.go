package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Secrets holds API keys / OAuth tokens that must persist across
// supervisor restarts BUT must NOT live in the main config.yaml.
//
// Why a separate file:
//   - config.yaml is often the file users tweak by hand or commit to
//     dotfile repos for portability. Secrets in the same file = leak
//     by default.
//   - secrets.yaml is enforced to 0o600 on save and read into the
//     process env on boot, so resolvers that look up `os.Getenv(...)`
//     keep working without changes.
//
// Long-term v0.2+: the macOS keychain takes over; this YAML is the
// portable beta fallback so users don't have to manually edit shell
// profiles after the onboarding wizard accepts a key.
type Secrets struct {
	AnthropicAPIKey string `yaml:"anthropic_api_key,omitempty"`
	OpenAIAPIKey    string `yaml:"openai_api_key,omitempty"`
	GeminiAPIKey    string `yaml:"gemini_api_key,omitempty"`
	PixelLabAPIKey  string `yaml:"pixellab_api_key,omitempty"`
}

// DefaultSecretsPath returns ~/.config/aisupervisor/secrets.yaml.
// Lives alongside config.yaml but is a separate file so the
// 0o600-vs-0o644 distinction is unambiguous and the file can be
// excluded from any future dotfile-sync workflow.
func DefaultSecretsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aisupervisor", "secrets.yaml")
}

// LoadSecrets reads secrets.yaml from path (or the default location
// when path is empty). Missing file is NOT an error — a brand-new
// install has no secrets yet and the wizard will create one. A file
// with stale permissions (>0o600) gets a one-shot warning and is
// silently fixed by the next Save.
func LoadSecrets(path string) (*Secrets, error) {
	if path == "" {
		path = DefaultSecretsPath()
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return &Secrets{}, nil
		}
		return nil, statErr
	}

	// Permission canary: warn if anything beyond owner can read this
	// file. We don't auto-chmod here — the next Save will fix it, and
	// breaking startup over a permission discrepancy is worse than
	// the leak window for paid beta.
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		log.Printf("⚠️  secrets: %s permissions are %#o (want 0o600); next save will tighten",
			path, perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &Secrets{}
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("secrets: parse %s: %w", path, err)
	}
	return s, nil
}

// SaveSecrets writes s to path (or the default location when path is
// empty) atomically with 0o600 permissions. Uses the same
// temp-file-then-rename pattern as Config.Save so observers never
// see a partial file with looser permissions.
func SaveSecrets(path string, s *Secrets) error {
	if path == "" {
		path = DefaultSecretsPath()
	}
	if s == nil {
		s = &Secrets{}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("secrets: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("secrets: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("secrets: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("secrets: close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		cleanup()
		return fmt.Errorf("secrets: chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("secrets: rename temp to %s: %w", path, err)
	}
	return nil
}

// ApplyToEnv calls os.Setenv for each populated field so the rest of
// the supervisor's resolvers (setupBackend / setupChatProvider /
// ResolvePixelLabAPIKey / messenger token lookups) can keep using
// os.Getenv without changes.
//
// Empty fields are NOT set — preserves whatever the user explicitly
// exported in their shell. (This means shell env still wins when
// both are set, but a wizard-written secret survives restart.)
//
// Returns the names of env vars that were set, for audit logging.
func (s *Secrets) ApplyToEnv() []string {
	if s == nil {
		return nil
	}
	var applied []string
	if s.AnthropicAPIKey != "" {
		_ = os.Setenv("ANTHROPIC_API_KEY", s.AnthropicAPIKey)
		applied = append(applied, "ANTHROPIC_API_KEY")
	}
	if s.OpenAIAPIKey != "" {
		_ = os.Setenv("OPENAI_API_KEY", s.OpenAIAPIKey)
		applied = append(applied, "OPENAI_API_KEY")
	}
	if s.GeminiAPIKey != "" {
		_ = os.Setenv("GEMINI_API_KEY", s.GeminiAPIKey)
		applied = append(applied, "GEMINI_API_KEY")
	}
	if s.PixelLabAPIKey != "" {
		_ = os.Setenv("PIXELLAB_API_KEY", s.PixelLabAPIKey)
		applied = append(applied, "PIXELLAB_API_KEY")
	}
	return applied
}

// SetByEnvVar updates the field corresponding to envVarName (e.g.
// "OPENAI_API_KEY" → s.OpenAIAPIKey). Returns false if the name is
// not one of the recognised keys, in which case the caller should
// either reject or extend the struct.
//
// This indirection lets the GUI wizard call a single helper rather
// than switch over env var names twice (once for the in-memory key
// pool and once for the persistence layer).
func (s *Secrets) SetByEnvVar(envVarName, value string) bool {
	switch envVarName {
	case "ANTHROPIC_API_KEY":
		s.AnthropicAPIKey = value
		return true
	case "OPENAI_API_KEY":
		s.OpenAIAPIKey = value
		return true
	case "GEMINI_API_KEY":
		s.GeminiAPIKey = value
		return true
	case "PIXELLAB_API_KEY":
		s.PixelLabAPIKey = value
		return true
	}
	return false
}
