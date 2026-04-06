package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepTool_BasicSearch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\nfoo bar\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("baz qux\nhello again\n"), 0644)

	tool := &GrepTool{}
	args, _ := json.Marshal(grepArgs{Pattern: "hello", Path: dir})
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
}

func TestGrepTool_ContentMode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("line one\nline two\nline three\n"), 0644)

	tool := &GrepTool{}
	args, _ := json.Marshal(grepArgs{Pattern: "two", Path: dir, OutputMode: "content"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "line two") {
		t.Errorf("expected matching line in output: %s", result)
	}
}

func TestGrepTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\n"), 0644)

	tool := &GrepTool{}
	args, _ := json.Marshal(grepArgs{Pattern: "nonexistent", Path: dir})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result) != "" {
		t.Errorf("expected empty result for no matches, got: %q", result)
	}
}
