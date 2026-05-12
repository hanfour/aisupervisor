package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSave_WritesMode0600 confirms a fresh config save lands on disk
// with owner-only read/write perms. Required for credentials storage:
// the previous os.WriteFile(..., 0o644) left config readable by any
// process on the machine, which is unacceptable when the file holds
// API keys.
func TestSave_WritesMode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o600 {
		t.Errorf("permissions = %#o, want %#o", got, 0o600)
	}
}

// TestSave_OverwriteForcesMode0600 covers the scenario where a stale
// 0o644 file already exists on disk (e.g., from before this fix).
// The new Save path uses temp-file-then-rename, so the destination's
// pre-existing perms are irrelevant — the rename replaces the inode
// with one that was created at 0o600.
func TestSave_OverwriteForcesMode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Pre-seed a file with 0o644 permissions, simulating the broken
	// state from old installs.
	if err := os.WriteFile(path, []byte("# stale\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Fatalf("seed perms = %#o, want %#o", info.Mode().Perm(), 0o644)
	}

	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("permissions after overwrite = %#o, want %#o", got, 0o600)
	}
}

// TestSave_NoStaleTempFiles confirms the temp file used during
// atomic write doesn't survive a successful Save. A stale .config-*.tmp
// in the user's config dir would be noisy at best and confusing at
// worst.
func TestSave_NoStaleTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

// TestWarnPlaintextSecrets_PixelLabFlagged confirms a YAML-stored
// PixelLab key produces a warning and suggests the env var.
func TestWarnPlaintextSecrets_PixelLabFlagged(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PixelLab.APIKey = "secret-uuid-value"

	warnings := cfg.WarnPlaintextSecrets()
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning, got none")
	}
	found := false
	for _, w := range warnings {
		if w.Field == "pixellab.api_key" {
			found = true
			if w.EnvVar != "PIXELLAB_API_KEY" {
				t.Errorf("EnvVar = %q, want PIXELLAB_API_KEY", w.EnvVar)
			}
			if w.Comment == "" {
				t.Error("Comment is empty")
			}
		}
	}
	if !found {
		t.Errorf("no warning for pixellab.api_key; got %+v", warnings)
	}
}

// TestWarnPlaintextSecrets_SkillsMPFlagged confirms the SkillsMP key
// is flagged even without a current env-var alternative.
func TestWarnPlaintextSecrets_SkillsMPFlagged(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SkillsMPAPIKey = "secret-token"

	warnings := cfg.WarnPlaintextSecrets()
	found := false
	for _, w := range warnings {
		if w.Field == "skillsmp_api_key" {
			found = true
			// No env var alternative yet — explicit empty string
			// guards against silently introducing one.
			if w.EnvVar != "" {
				t.Errorf("EnvVar = %q, want empty (no env alternative shipped yet)", w.EnvVar)
			}
		}
	}
	if !found {
		t.Errorf("no warning for skillsmp_api_key; got %+v", warnings)
	}
}

// TestWarnPlaintextSecrets_Empty confirms a clean config (no secrets
// in YAML, all going through env vars) produces zero warnings.
func TestWarnPlaintextSecrets_Empty(t *testing.T) {
	cfg := DefaultConfig() // no key fields populated by default
	warnings := cfg.WarnPlaintextSecrets()
	if len(warnings) != 0 {
		t.Errorf("DefaultConfig should produce zero warnings, got %+v", warnings)
	}
}
