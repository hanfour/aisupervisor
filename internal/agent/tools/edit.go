package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditTool performs exact string replacement in a file.
type EditTool struct{}

type editArgs struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (e *EditTool) Name() string        { return "Edit" }
func (e *EditTool) Description() string { return "Perform exact string replacement in a file." }
func (e *EditTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"file_path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"},"replace_all":{"type":"boolean"}},"required":["file_path","old_string","new_string"]}`)
}

func (e *EditTool) Execute(_ context.Context, rawArgs json.RawMessage) (string, error) {
	var args editArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", args.FilePath, err)
	}

	content := string(data)
	count := strings.Count(content, args.OldString)

	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", args.FilePath)
	}

	if count > 1 && !args.ReplaceAll {
		return "", fmt.Errorf("old_string appears %d times in %s; use replace_all to replace all occurrences", count, args.FilePath)
	}

	var newContent string
	if args.ReplaceAll {
		newContent = strings.ReplaceAll(content, args.OldString, args.NewString)
	} else {
		newContent = strings.Replace(content, args.OldString, args.NewString, 1)
	}

	if err := os.WriteFile(args.FilePath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", args.FilePath, err)
	}

	return fmt.Sprintf("Replaced %d occurrence(s) in %s", count, args.FilePath), nil
}
