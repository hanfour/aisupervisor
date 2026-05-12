package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSecrets_MissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	s, err := LoadSecrets(path)
	if err != nil {
		t.Fatalf("LoadSecrets on missing file: %v", err)
	}
	if s == nil {
		t.Fatal("LoadSecrets returned nil Secrets on missing file (want zero-value pointer)")
	}
	if s.AnthropicAPIKey != "" || s.OpenAIAPIKey != "" || s.GeminiAPIKey != "" || s.PixelLabAPIKey != "" {
		t.Errorf("expected zero-value Secrets, got %+v", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	in := &Secrets{
		AnthropicAPIKey: "sk-ant-test-anthropic",
		OpenAIAPIKey:    "sk-test-openai",
		GeminiAPIKey:    "AIza-test-gemini",
		PixelLabAPIKey:  "uuid-test-pixellab",
	}
	if err := SaveSecrets(path, in); err != nil {
		t.Fatalf("SaveSecrets: %v", err)
	}
	out, err := LoadSecrets(path)
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if *out != *in {
		t.Errorf("round-trip mismatch\n in:  %+v\n out: %+v", in, out)
	}
}

func TestSaveSecrets_Mode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := SaveSecrets(path, &Secrets{OpenAIAPIKey: "leak-detector"}); err != nil {
		t.Fatalf("SaveSecrets: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %#o, want %#o", perm, 0o600)
	}
}

func TestSaveSecrets_TightensExistingFile(t *testing.T) {
	// Simulate a pre-existing secrets.yaml with looser perms (e.g.
	// from a hand-edited file). After Save the new inode should have
	// 0o600 regardless.
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte("openai_api_key: stale\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := SaveSecrets(path, &Secrets{OpenAIAPIKey: "fresh"}); err != nil {
		t.Fatalf("SaveSecrets: %v", err)
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions after overwrite = %#o, want %#o", perm, 0o600)
	}
}

func TestApplyToEnv_SetsAllPopulatedFields(t *testing.T) {
	envVars := []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "PIXELLAB_API_KEY"}
	// Save + restore so the test doesn't leak side-effects into the
	// rest of the suite.
	saved := make(map[string]string, len(envVars))
	for _, v := range envVars {
		saved[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	}()

	s := &Secrets{
		AnthropicAPIKey: "ant",
		OpenAIAPIKey:    "oai",
		GeminiAPIKey:    "gem",
		PixelLabAPIKey:  "pix",
	}
	applied := s.ApplyToEnv()
	if len(applied) != len(envVars) {
		t.Errorf("applied = %v (len %d), want %d entries", applied, len(applied), len(envVars))
	}
	for _, v := range envVars {
		if os.Getenv(v) == "" {
			t.Errorf("env var %s was not set", v)
		}
	}
}

func TestApplyToEnv_LeavesShellEnvUntouchedForEmptyFields(t *testing.T) {
	// User already exported OPENAI_API_KEY in their shell; secrets.yaml
	// has no OpenAI entry. ApplyToEnv must NOT overwrite the shell value
	// with an empty string.
	const shellValue = "shell-set-value"
	const envName = "OPENAI_API_KEY"
	old := os.Getenv(envName)
	defer os.Setenv(envName, old)

	_ = os.Setenv(envName, shellValue)
	s := &Secrets{} // empty
	applied := s.ApplyToEnv()
	if len(applied) != 0 {
		t.Errorf("ApplyToEnv on empty Secrets reported %v applied, want none", applied)
	}
	if got := os.Getenv(envName); got != shellValue {
		t.Errorf("shell env %s changed to %q, want %q", envName, got, shellValue)
	}
}

func TestSetByEnvVar(t *testing.T) {
	cases := []struct {
		env      string
		value    string
		check    func(s *Secrets) string
		expected string
	}{
		{"ANTHROPIC_API_KEY", "ant-v", func(s *Secrets) string { return s.AnthropicAPIKey }, "ant-v"},
		{"OPENAI_API_KEY", "oai-v", func(s *Secrets) string { return s.OpenAIAPIKey }, "oai-v"},
		{"GEMINI_API_KEY", "gem-v", func(s *Secrets) string { return s.GeminiAPIKey }, "gem-v"},
		{"PIXELLAB_API_KEY", "pix-v", func(s *Secrets) string { return s.PixelLabAPIKey }, "pix-v"},
	}
	for _, c := range cases {
		s := &Secrets{}
		ok := s.SetByEnvVar(c.env, c.value)
		if !ok {
			t.Errorf("SetByEnvVar(%q) = false, want true", c.env)
			continue
		}
		if got := c.check(s); got != c.expected {
			t.Errorf("after SetByEnvVar(%q): field = %q, want %q", c.env, got, c.expected)
		}
	}
}

func TestSetByEnvVar_RejectsUnknown(t *testing.T) {
	s := &Secrets{}
	if ok := s.SetByEnvVar("RANDOM_KEY", "x"); ok {
		t.Error("SetByEnvVar(RANDOM_KEY) = true, want false (struct should refuse unknown keys)")
	}
}
