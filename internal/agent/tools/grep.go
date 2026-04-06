package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GrepTool searches file contents using rg (ripgrep) with grep fallback.
type GrepTool struct{}

type grepArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	OutputMode string `json:"output_mode,omitempty"` // "files_with_matches" (default), "content"
	Glob       string `json:"glob,omitempty"`
}

func (g *GrepTool) Name() string        { return "Grep" }
func (g *GrepTool) Description() string { return "Search file contents using ripgrep with grep fallback." }
func (g *GrepTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"output_mode":{"type":"string","enum":["files_with_matches","content"]},"glob":{"type":"string"}},"required":["pattern"]}`)
}

func (g *GrepTool) Execute(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var args grepArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	path := args.Path
	if path == "" {
		path = "."
	}

	mode := args.OutputMode
	if mode == "" {
		mode = "files_with_matches"
	}

	// Try ripgrep first
	output, err := g.tryRg(ctx, args.Pattern, path, mode, args.Glob)
	if err == nil {
		return output, nil
	}

	// Fall back to grep
	return g.tryGrep(ctx, args.Pattern, path, mode)
}

func (g *GrepTool) tryRg(ctx context.Context, pattern, path, mode, glob string) (string, error) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return "", err
	}

	cmdArgs := []string{}
	switch mode {
	case "files_with_matches":
		cmdArgs = append(cmdArgs, "--files-with-matches")
	case "content":
		cmdArgs = append(cmdArgs, "-n")
	}

	if glob != "" {
		cmdArgs = append(cmdArgs, "--glob", glob)
	}

	cmdArgs = append(cmdArgs, pattern, path)

	cmd := exec.CommandContext(ctx, rgPath, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// rg exits 1 when no matches found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}

	return strings.TrimRight(string(output), "\n"), nil
}

func (g *GrepTool) tryGrep(ctx context.Context, pattern, path, mode string) (string, error) {
	cmdArgs := []string{"-rn"}
	if mode == "files_with_matches" {
		cmdArgs = []string{"-rl"}
	}
	cmdArgs = append(cmdArgs, pattern, path)

	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// grep exits 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("grep failed: %w", err)
	}

	return strings.TrimRight(string(output), "\n"), nil
}
