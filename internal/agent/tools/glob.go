package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

type globArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (g *GlobTool) Name() string        { return "Glob" }
func (g *GlobTool) Description() string { return "Find files matching a glob pattern." }
func (g *GlobTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"}},"required":["pattern"]}`)
}

func (g *GlobTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args globArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	dir := args.Path
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
	}

	pattern := filepath.Join(dir, args.Pattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob pattern error: %w", err)
	}

	if len(matches) == 0 {
		return "", nil
	}

	return strings.Join(matches, "\n") + "\n", nil
}
