package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadTool reads a file and returns its contents with line numbers.
type ReadTool struct{}

type readArgs struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func (r *ReadTool) Name() string        { return "Read" }
func (r *ReadTool) Description() string { return "Read a file and return its contents with line numbers." }
func (r *ReadTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"file_path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["file_path"]}`)
}

func (r *ReadTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args readArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", args.FilePath, err)
	}
	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return "", nil
	}

	start := 0
	if args.Offset > 0 {
		start = args.Offset - 1
	}
	if start > len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if args.Limit > 0 && start+args.Limit < end {
		end = start + args.Limit
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "%d\t%s\n", i+1, lines[i])
	}
	return sb.String(), nil
}
