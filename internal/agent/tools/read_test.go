package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_BasicRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644)

	tool := &ReadTool{}
	args, _ := json.Marshal(readArgs{FilePath: path})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "1\tline1") {
		t.Errorf("expected line-numbered output, got: %s", result)
	}
	if !strings.Contains(result, "3\tline3") {
		t.Errorf("expected line 3, got: %s", result)
	}
}

func TestReadTool_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0644)

	tool := &ReadTool{}
	args, _ := json.Marshal(readArgs{FilePath: path, Offset: 2, Limit: 2})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), result)
	}
	if !strings.Contains(lines[0], "2\tb") {
		t.Errorf("expected line 2 = b, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "3\tc") {
		t.Errorf("expected line 3 = c, got: %s", lines[1])
	}
}

func TestReadTool_FileNotFound(t *testing.T) {
	tool := &ReadTool{}
	args, _ := json.Marshal(readArgs{FilePath: "/nonexistent/file.txt"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadTool_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	os.WriteFile(path, []byte(""), 0644)

	tool := &ReadTool{}
	args, _ := json.Marshal(readArgs{FilePath: path})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got: %q", result)
	}
}
