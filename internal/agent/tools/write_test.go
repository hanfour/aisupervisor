package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTool_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	tool := &WriteTool{}
	args, _ := json.Marshal(writeArgs{FilePath: path, Content: "hello\n"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestWriteTool_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "file.txt")

	tool := &WriteTool{}
	args, _ := json.Marshal(writeArgs{FilePath: path, Content: "nested\n"})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "nested\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestWriteTool_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("old"), 0644)

	tool := &WriteTool{}
	args, _ := json.Marshal(writeArgs{FilePath: path, Content: "new"})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Errorf("unexpected content: %q", string(data))
	}
}
