package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withStubPATH points exec.LookPath at a directory containing
// only the binaries the caller wants to look "installed". Returns
// a cleanup function that restores the original PATH.
func withStubPATH(t *testing.T, installed []string) func() {
	t.Helper()
	dir := t.TempDir()
	for _, name := range installed {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("create stub %q: %v", name, err)
		}
		_ = f.Close()
		if err := os.Chmod(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("chmod stub %q: %v", name, err)
		}
	}
	original := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	return func() { _ = os.Setenv("PATH", original) }
}

func TestCheckRuntimePrereqs_AllPresent(t *testing.T) {
	defer withStubPATH(t, []string{"claude", "codex", "aider", "ais-agent"})()
	missing := CheckRuntimePrereqs([]WorkerPrereq{
		{Name: "Alice", CLITool: "claude"},
		{Name: "Bob", CLITool: "codex"},
		{Name: "Carol", CLITool: "aider"},
		{Name: "Dave", CLITool: "ais-agent"},
	})
	if len(missing) != 0 {
		t.Errorf("expected zero missing, got %+v", missing)
	}
}

func TestCheckRuntimePrereqs_OneMissingFlagged(t *testing.T) {
	defer withStubPATH(t, []string{"claude"})()
	missing := CheckRuntimePrereqs([]WorkerPrereq{
		{Name: "Alice", CLITool: "claude"},
		{Name: "Bob", CLITool: "codex"},
	})
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing, got %d: %+v", len(missing), missing)
	}
	if missing[0].Name != "codex" {
		t.Errorf("missing[0].Name = %q, want codex", missing[0].Name)
	}
	if len(missing[0].AffectedWorkers) != 1 || missing[0].AffectedWorkers[0] != "Bob" {
		t.Errorf("AffectedWorkers = %v, want [Bob]", missing[0].AffectedWorkers)
	}
	if !strings.Contains(missing[0].InstallHint, "@openai/codex") {
		t.Errorf("InstallHint = %q, want it to contain @openai/codex", missing[0].InstallHint)
	}
}

func TestCheckRuntimePrereqs_MergesAffectedWorkers(t *testing.T) {
	// Multiple workers wanting the same missing runtime should
	// appear as ONE entry with both names listed, sorted.
	defer withStubPATH(t, []string{"claude"})() // only claude installed
	missing := CheckRuntimePrereqs([]WorkerPrereq{
		{Name: "Zack", CLITool: "aider"},
		{Name: "Alice", CLITool: "aider"},
		{Name: "Mary", CLITool: "aider"},
	})
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing entry (aider), got %d", len(missing))
	}
	want := []string{"Alice", "Mary", "Zack"}
	got := missing[0].AffectedWorkers
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("AffectedWorkers = %v, want %v (sorted)", got, want)
	}
}

func TestCheckRuntimePrereqs_EmptyCLIToolDefaultsToClaude(t *testing.T) {
	// Empty CLITool → treated as "claude" (matches spawner.resolveRuntimeName seed).
	defer withStubPATH(t, []string{})() // nothing installed
	missing := CheckRuntimePrereqs([]WorkerPrereq{
		{Name: "Alice", CLITool: ""},
	})
	if len(missing) != 1 || missing[0].Name != "claude" {
		t.Fatalf("expected empty CLITool to map to claude, got %+v", missing)
	}
}

func TestCheckRuntimePrereqs_UnknownToolStillFlagged(t *testing.T) {
	// An unrecognised tool name (e.g. "gemini" before it's
	// wired) should be flagged as missing — silently allowing
	// it would let workers slip through that can't spawn.
	defer withStubPATH(t, []string{"claude"})()
	missing := CheckRuntimePrereqs([]WorkerPrereq{
		{Name: "Eve", CLITool: "gemini"},
	})
	if len(missing) != 1 || missing[0].Name != "gemini" {
		t.Fatalf("unknown tool not flagged: %+v", missing)
	}
	if missing[0].InstallHint != "" {
		t.Errorf("expected empty InstallHint for unknown tool, got %q", missing[0].InstallHint)
	}
}

func TestCheckRuntimePrereqs_StableOutputOrder(t *testing.T) {
	// Iteration over the byRuntime map is random; the function
	// must sort before returning so callers (and tests, and
	// users reading the error message) see deterministic order.
	defer withStubPATH(t, []string{})()
	plan := []WorkerPrereq{
		{Name: "W1", CLITool: "codex"},
		{Name: "W2", CLITool: "claude"},
		{Name: "W3", CLITool: "aider"},
	}
	// Call repeatedly; output must be identical each time.
	first := CheckRuntimePrereqs(plan)
	for i := 0; i < 5; i++ {
		again := CheckRuntimePrereqs(plan)
		if len(first) != len(again) {
			t.Fatalf("len drift iter %d: %d vs %d", i, len(first), len(again))
		}
		for j := range first {
			if first[j].Name != again[j].Name {
				t.Errorf("order drift iter %d at idx %d: %q vs %q", i, j, first[j].Name, again[j].Name)
			}
		}
	}
}

func TestFormatMissingError_EmptyReturnsEmptyString(t *testing.T) {
	if got := FormatMissingError(nil); got != "" {
		t.Errorf("empty slice → %q, want empty string", got)
	}
	if got := FormatMissingError([]MissingRuntime{}); got != "" {
		t.Errorf("empty slice → %q, want empty string", got)
	}
}

func TestFormatMissingError_RendersHints(t *testing.T) {
	msg := FormatMissingError([]MissingRuntime{
		{Name: "codex", AffectedWorkers: []string{"Alice", "Bob"}, InstallHint: "npm install -g @openai/codex"},
		{Name: "ais-agent", AffectedWorkers: []string{"Charlie"}, InstallHint: ""},
	})
	wantSubstrings := []string{
		"cannot create workers",
		"codex (affects: Alice, Bob)",
		"install: npm install -g @openai/codex",
		"ais-agent (affects: Charlie)",
		"install: see runtime docs",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(msg, want) {
			t.Errorf("FormatMissingError output missing %q\nfull:\n%s", want, msg)
		}
	}
}
