package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEditTool_BasicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world\n"), 0644)

	tool := &EditTool{}
	args, _ := json.Marshal(editArgs{
		FilePath:  path,
		OldString: "hello",
		NewString: "goodbye",
	})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "goodbye world\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestEditTool_NotUnique(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("aaa bbb aaa\n"), 0644)

	tool := &EditTool{}
	args, _ := json.Marshal(editArgs{
		FilePath:  path,
		OldString: "aaa",
		NewString: "ccc",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for non-unique old_string")
	}
}

func TestEditTool_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("aaa bbb aaa\n"), 0644)

	tool := &EditTool{}
	args, _ := json.Marshal(editArgs{
		FilePath:   path,
		OldString:  "aaa",
		NewString:  "ccc",
		ReplaceAll: true,
	})
	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "ccc bbb ccc\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestEditTool_OldStringNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world\n"), 0644)

	tool := &EditTool{}
	args, _ := json.Marshal(editArgs{
		FilePath:  path,
		OldString: "nonexistent",
		NewString: "replacement",
	})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for old_string not found")
	}
}
