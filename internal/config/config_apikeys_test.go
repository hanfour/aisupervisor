package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_APIKeysArrayParsing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
default_backend: claude-api
api_keys:
  - id: key1
    key: sk-aaa111
    provider: anthropic
  - id: key2
    key: sk-bbb222
    provider: anthropic
  - id: key3
    key: sk-ccc333
    provider: openai
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.APIKeys) != 3 {
		t.Fatalf("expected 3 api_keys, got %d", len(cfg.APIKeys))
	}

	tests := []struct {
		idx      int
		id       string
		key      string
		provider string
	}{
		{0, "key1", "sk-aaa111", "anthropic"},
		{1, "key2", "sk-bbb222", "anthropic"},
		{2, "key3", "sk-ccc333", "openai"},
	}
	for _, tc := range tests {
		ak := cfg.APIKeys[tc.idx]
		if ak.ID != tc.id {
			t.Errorf("[%d] ID: expected %q, got %q", tc.idx, tc.id, ak.ID)
		}
		if ak.Key != tc.key {
			t.Errorf("[%d] Key: expected %q, got %q", tc.idx, tc.key, ak.Key)
		}
		if ak.Provider != tc.provider {
			t.Errorf("[%d] Provider: expected %q, got %q", tc.idx, tc.provider, ak.Provider)
		}
	}
}

func TestConfig_APIKeysEmptyBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
default_backend: claude-api
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.APIKeys) != 0 {
		t.Errorf("expected empty api_keys for backward compat, got %d", len(cfg.APIKeys))
	}
}

func TestConfig_DefaultAPIKeysEmpty(t *testing.T) {
	keys := DefaultAPIKeys()
	if len(keys) != 0 {
		t.Errorf("DefaultAPIKeys should return empty slice, got %d", len(keys))
	}
}
