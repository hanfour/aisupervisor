package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool writes content to a file, creating parent directories as needed.
type WriteTool struct{}

type writeArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (w *WriteTool) Name() string        { return "Write" }
func (w *WriteTool) Description() string { return "Write content to a file, creating parent directories if needed." }
func (w *WriteTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"file_path":{"type":"string"},"content":{"type":"string"}},"required":["file_path","content"]}`)
}

func (w *WriteTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args writeArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	dir := filepath.Dir(args.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating directories for %s: %w", args.FilePath, err)
	}

	if err := os.WriteFile(args.FilePath, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", args.FilePath, err)
	}

	return fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.FilePath), nil
}
