package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// BashTool executes a shell command and returns combined output.
type BashTool struct{}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // milliseconds, default 120000
}

func (b *BashTool) Name() string        { return "Bash" }
func (b *BashTool) Description() string { return "Execute a bash command and return its output." }
func (b *BashTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"}},"required":["command"]}`)
}

func (b *BashTool) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var args bashArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	timeout := 120 * time.Second
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("command timed out after %v", timeout)
		}
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}
