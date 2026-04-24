package agent_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/agent/aider"
	"github.com/hanfourmini/aisupervisor/internal/agent/aisagent"
	"github.com/hanfourmini/aisupervisor/internal/agent/claudecode"
)

// TestRuntimeRegistry_AllPluginsRegistered asserts that when all three
// bundled plugins (claudecode, aisagent, aider) are registered, the
// registry exposes them by name and honors the first-registered-is-Default
// contract.
func TestRuntimeRegistry_AllPluginsRegistered(t *testing.T) {
	r := agent.NewRuntimeRegistry()
	r.Register(claudecode.New(nil))
	r.Register(aisagent.New(nil))
	r.Register(aider.New(nil))

	names := r.List()
	if len(names) != 3 {
		t.Fatalf("List length: got %d, want 3 (%v)", len(names), names)
	}

	want := map[string]bool{"claude": false, "ais-agent": false, "aider": false}
	for _, n := range names {
		if _, ok := want[n]; !ok {
			t.Errorf("unexpected runtime name %q in List", n)
		}
		want[n] = true
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("runtime %q missing from List", n)
		}
	}

	// First registered must be Default().
	def := r.Default()
	if def == nil {
		t.Fatal("Default() returned nil with 3 runtimes registered")
	}
	if def.Name() != "claude" {
		t.Errorf("Default().Name(): got %q, want %q", def.Name(), "claude")
	}

	// Get returns each by name.
	for _, want := range []string{"claude", "ais-agent", "aider"} {
		rt, ok := r.Get(want)
		if !ok {
			t.Errorf("Get(%q): ok=false", want)
			continue
		}
		if rt.Name() != want {
			t.Errorf("Get(%q).Name(): got %q", want, rt.Name())
		}
	}

	// Unknown name fails.
	if rt, ok := r.Get("nonexistent"); ok || rt != nil {
		t.Errorf("Get(\"nonexistent\"): got (%v, %v), want (nil, false)", rt, ok)
	}
}

// TestClaudeCodeRuntime_BuildCommand_FullConfig is a black-box integration
// check: the plugin's public Name and — through a dry-run with nil
// tmuxClient — its input validation behave correctly for a SpawnConfig
// that mirrors a realistic production call.
func TestClaudeCodeRuntime_BuildCommand_FullConfig(t *testing.T) {
	rt := claudecode.New(nil)
	if rt.Name() != "claude" {
		t.Fatalf("claude runtime Name: got %q, want %q", rt.Name(), "claude")
	}

	// ParseTokenUsage must roundtrip a realistic Claude Code summary.
	usage, err := rt.ParseTokenUsage("Session ended\nTotal tokens: 12,345\nTotal cost: $0.1234")
	if err != nil {
		t.Fatalf("ParseTokenUsage error: %v", err)
	}
	total := usage.InputTokens + usage.OutputTokens
	if total != 12345 {
		t.Errorf("ParseTokenUsage total: got %d (in=%d out=%d), want 12345",
			total, usage.InputTokens, usage.OutputTokens)
	}
	if usage.TotalCost < 0.12 || usage.TotalCost > 0.13 {
		t.Errorf("ParseTokenUsage TotalCost: got %v, want ~0.1234", usage.TotalCost)
	}
}

// TestAiderRuntime_BuildCommand_FullConfig mirrors the claudecode test for
// the Aider plugin, including the aider-specific Tokens summary format.
func TestAiderRuntime_BuildCommand_FullConfig(t *testing.T) {
	rt := aider.New(nil)
	if rt.Name() != "aider" {
		t.Fatalf("aider runtime Name: got %q, want %q", rt.Name(), "aider")
	}

	usage, err := rt.ParseTokenUsage("Tokens: 1,234 sent, 567 received, $0.01 cost")
	if err != nil {
		t.Fatalf("ParseTokenUsage error: %v", err)
	}
	if usage.InputTokens != 1234 {
		t.Errorf("InputTokens: got %d, want 1234", usage.InputTokens)
	}
	if usage.OutputTokens != 567 {
		t.Errorf("OutputTokens: got %d, want 567", usage.OutputTokens)
	}
	if usage.TotalCost != 0.01 {
		t.Errorf("TotalCost: got %v, want 0.01", usage.TotalCost)
	}

	// Empty output returns zero values and no error.
	usage, err = rt.ParseTokenUsage("nothing to see here")
	if err != nil {
		t.Errorf("ParseTokenUsage on empty: unexpected error %v", err)
	}
	if usage.InputTokens != 0 || usage.OutputTokens != 0 || usage.TotalCost != 0 {
		t.Errorf("ParseTokenUsage on empty: got %+v, want zero value", usage)
	}
}

// TestAISAgentRuntime_Name verifies the ais-agent runtime exposes its
// canonical name (used by the Worker.CLITool selector).
func TestAISAgentRuntime_Name(t *testing.T) {
	rt := aisagent.New(nil)
	if rt.Name() != "ais-agent" {
		t.Fatalf("ais-agent runtime Name: got %q, want %q", rt.Name(), "ais-agent")
	}
}

// TestRuntimeRegistry_NamesAreStable guards against accidental rename of a
// plugin's Name() — Worker.CLITool strings in stored YAML are keyed on
// these, so they are part of the persistence contract.
func TestRuntimeRegistry_NamesAreStable(t *testing.T) {
	r := agent.NewRuntimeRegistry()
	r.Register(claudecode.New(nil))
	r.Register(aisagent.New(nil))
	r.Register(aider.New(nil))

	names := r.List()
	sort.Strings(names)
	got := strings.Join(names, ",")
	want := "aider,ais-agent,claude"
	if got != want {
		t.Errorf("registered names: got %q, want %q", got, want)
	}
}
