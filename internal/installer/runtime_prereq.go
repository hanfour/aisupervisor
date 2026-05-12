package installer

import (
	"fmt"
	"sort"
	"strings"
)

// MissingRuntime describes one agent-runtime CLI tool that workers
// referenced but the host doesn't have on PATH. Returned by
// CheckRuntimePrereqs so the GUI / wizard can render install
// guidance instead of letting users create workers that will
// silently fail to spawn at first task assignment.
type MissingRuntime struct {
	// Name is the runtime identifier as it appears in
	// WorkerConfig.CLITool (e.g. "claude", "codex", "aider",
	// "ais-agent"). When multiple workers want the same runtime
	// we report it once with the affected worker names listed.
	Name string

	// AffectedWorkers names the workers (by their display name)
	// that would fail to spawn because of this missing CLI.
	AffectedWorkers []string

	// InstallHint is a one-line shell command or URL the user
	// can act on. Empty when there isn't a single canonical
	// install path (e.g. ais-agent which ships its own binary).
	InstallHint string
}

// runtimeBinaryName maps the supervisor's internal runtime
// identifier to the executable name we expect on PATH. Kept here
// rather than in each plugin's package so the prereq check has a
// single source of truth and can be exercised without dragging
// every runtime's tmux/HTTP scaffolding into the test.
var runtimeBinaryName = map[string]string{
	"claude":    "claude",
	"codex":     "codex",
	"aider":     "aider",
	"ais-agent": "ais-agent",
}

// runtimeInstallHint is the install-this-thing guidance shown
// when a runtime is missing. Empty string means "no canonical
// install path" — the caller should fall through to a generic
// "see runtime docs" message.
var runtimeInstallHint = map[string]string{
	"claude": "npm install -g @anthropic-ai/claude-code",
	"codex":  "npm install -g @openai/codex",
	"aider":  "pipx install aider-chat   # or pip install aider-chat",
	// ais-agent ships as a Go binary; no global package manager
	// hosts it yet. Leaving the hint empty surfaces the generic
	// docs link in the caller.
	"ais-agent": "",
}

// CheckRuntimePrereqs walks the supplied worker plan (name +
// resolved CLI tool) and returns one MissingRuntime entry per
// distinct runtime that is referenced but absent from PATH.
//
// Resolution rules
//   - Empty CLITool is treated as the project default "claude"
//     to match the supervisor's spawner.resolveRuntimeName seed
//     value. If users want to opt out of the check they must set
//     the tool to a stub the registry won't try to spawn.
//   - Unknown tool names (e.g. "gemini" today, before that's
//     wired) are flagged as missing with an empty InstallHint so
//     the wizard surfaces "we don't know this runtime" instead
//     of falsely greenlighting a worker that can't spawn.
//   - Affected worker names are sorted + deduplicated per
//     runtime so the GUI's "Cannot create workers" message reads
//     deterministically across runs.
func CheckRuntimePrereqs(workers []WorkerPrereq) []MissingRuntime {
	// runtime → set(worker names)
	byRuntime := make(map[string]map[string]struct{})
	for _, w := range workers {
		tool := strings.TrimSpace(w.CLITool)
		if tool == "" {
			tool = "claude"
		}
		bin, known := runtimeBinaryName[tool]
		if !known {
			// Use the tool name verbatim as the binary so the
			// missing-runtime entry carries the unknown name
			// through to the user. findDepPath will return ""
			// for an unknown binary and the entry will be
			// flagged as missing.
			bin = tool
		}
		if findDepPath(bin) != "" {
			continue
		}
		if _, ok := byRuntime[tool]; !ok {
			byRuntime[tool] = make(map[string]struct{})
		}
		byRuntime[tool][w.Name] = struct{}{}
	}

	missing := make([]MissingRuntime, 0, len(byRuntime))
	for tool, names := range byRuntime {
		affected := make([]string, 0, len(names))
		for n := range names {
			affected = append(affected, n)
		}
		sort.Strings(affected)
		missing = append(missing, MissingRuntime{
			Name:            tool,
			AffectedWorkers: affected,
			InstallHint:     runtimeInstallHint[tool], // empty for unknown
		})
	}
	// Stable order on the outer list too.
	sort.Slice(missing, func(i, j int) bool {
		return missing[i].Name < missing[j].Name
	})
	return missing
}

// WorkerPrereq is the minimum shape CheckRuntimePrereqs needs:
// a display name (used in error messages) and the resolved CLI
// tool the worker would use at spawn time. The full Worker /
// OnboardingWorkerDTO types in higher layers convert to this
// before calling.
type WorkerPrereq struct {
	Name    string
	CLITool string
}

// FormatMissingError renders a slice of MissingRuntime entries
// into a single user-facing error string suitable for returning
// from BatchCreateWorkers. Returns the empty string when the
// slice is empty (no missing runtimes), so the caller can use it
// as `if msg := FormatMissingError(m); msg != "" { return ... }`.
func FormatMissingError(missing []MissingRuntime) string {
	if len(missing) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("cannot create workers — required CLI runtimes not on PATH:\n")
	for _, m := range missing {
		fmt.Fprintf(&b, "  - %s (affects: %s)", m.Name, strings.Join(m.AffectedWorkers, ", "))
		if m.InstallHint != "" {
			fmt.Fprintf(&b, "\n      install: %s", m.InstallHint)
		} else {
			fmt.Fprintf(&b, "\n      install: see runtime docs (no canonical package)")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
