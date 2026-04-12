package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBashTool_BasicCommand(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "echo hello"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result) != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestBashTool_CommandFailure(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "exit 1"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "sleep 10", Timeout: 500})
	start := time.Now()
	_, err := tool.Execute(context.Background(), args)
	elapsed := time.Since(start)
	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestBashTool_CombinedOutput(t *testing.T) {
	tool := &BashTool{}
	args, _ := json.Marshal(bashArgs{Command: "echo out && echo err >&2"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "out") {
		t.Errorf("expected stdout in output, got %q", result)
	}
	if !strings.Contains(result, "err") {
		t.Errorf("expected stderr in output, got %q", result)
	}
}
