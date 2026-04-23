package agent

import (
	"context"
	"time"
)

// AgentRuntime is the contract every pluggable agent backend must satisfy.
// Concrete runtimes (claudecode, aisagent, aider, ...) wrap a specific CLI
// driven inside a tmux pane and expose a uniform lifecycle to the spawner.
type AgentRuntime interface {
	// Name returns the unique identifier for this runtime (e.g. "claudecode").
	Name() string

	// Spawn launches a new agent process according to cfg and returns a
	// session handle that other methods can operate on.
	Spawn(ctx context.Context, cfg SpawnConfig) (*AgentSession, error)

	// SendPrompt delivers a user prompt to an already-spawned session.
	SendPrompt(session *AgentSession, prompt string) error

	// CaptureOutput returns up to the last `lines` lines of visible pane
	// output (including scrollback) for the session.
	CaptureOutput(session *AgentSession, lines int) (string, error)

	// DetectReady blocks until the agent's prompt is ready for input or
	// timeout elapses, whichever comes first.
	DetectReady(ctx context.Context, session *AgentSession, timeout time.Duration) error

	// DetectCompletion reports whether the agent has finished its current
	// turn (idle prompt restored) and is awaiting new input.
	DetectCompletion(ctx context.Context, session *AgentSession) (bool, error)

	// ParseTokenUsage extracts token/cost telemetry from raw pane output.
	ParseTokenUsage(output string) (TokenUsage, error)

	// Cleanup tears down any runtime-owned resources attached to the session.
	Cleanup(session *AgentSession) error
}

// SpawnConfig carries all parameters needed to launch an agent session.
// Runtimes translate these fields into runtime-specific CLI flags.
type SpawnConfig struct {
	WorkDir         string
	Branch          string
	SystemPrompt    string
	AllowedTools    []string
	DisallowedTools []string
	Model           string
	PermissionMode  string
	ExtraCLIArgs    string
	EnvVars         map[string]string
}

// AgentSession is an opaque handle identifying a spawned agent instance.
// Runtimes may stash runtime-specific state under Metadata.
type AgentSession struct {
	ID          string
	RuntimeName string
	TmuxSession string
	Window      int
	Pane        int
	WorkDir     string
	StartedAt   time.Time
	Metadata    map[string]string
}

// TokenUsage summarises token consumption and cost for a single turn.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalCost    float64
}
