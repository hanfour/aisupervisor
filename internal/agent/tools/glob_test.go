package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobTool_BasicGlob(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0644)

	tool := &GlobTool{}
	args, _ := json.Marshal(globArgs{Pattern: "*.txt", Path: dir})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a.txt") {
		t.Errorf("expected a.txt in result: %s", result)
	}
	if !strings.Contains(result, "b.txt") {
		t.Errorf("expected b.txt in result: %s", result)
	}
	if strings.Contains(result, "c.go") {
		t.Errorf("should not contain c.go: %s", result)
	}
}

func TestGlobTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	tool := &GlobTool{}
	args, _ := json.Marshal(globArgs{Pattern: "*.xyz", Path: dir})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result) != "" {
		t.Errorf("expected empty result, got: %q", result)
	}
}

func TestGlobTool_DefaultPath(t *testing.T) {
	tool := &GlobTool{}
	// Should not error when no path given (uses cwd)
	args, _ := json.Marshal(globArgs{Pattern: "*.go"})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
