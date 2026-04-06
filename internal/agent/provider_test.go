package agent

import (
	"encoding/json"
	"testing"
)

func TestMessageJSONRoundtrip(t *testing.T) {
	original := Message{
		Role:    "assistant",
		Content: "Hello, world!",
		ToolCalls: []ToolCall{
			{
				ID:        "call_123",
				Name:      "read_file",
				Arguments: json.RawMessage(`{"path":"/tmp/test.txt"}`),
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Role != original.Role {
		t.Errorf("role: got %q, want %q", decoded.Role, original.Role)
	}
	if decoded.Content != original.Content {
		t.Errorf("content: got %q, want %q", decoded.Content, original.Content)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("toolCalls length: got %d, want 1", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].ID != "call_123" {
		t.Errorf("toolCall ID: got %q, want %q", decoded.ToolCalls[0].ID, "call_123")
	}
	if decoded.ToolCalls[0].Name != "read_file" {
		t.Errorf("toolCall name: got %q, want %q", decoded.ToolCalls[0].Name, "read_file")
	}
}

func TestMessageWithToolResult_JSONRoundtrip(t *testing.T) {
	original := Message{
		Role: "tool",
		ToolResult: &ToolResult{
			CallID:  "call_456",
			Content: "file contents here",
			IsError: false,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Role != "tool" {
		t.Errorf("role: got %q, want %q", decoded.Role, "tool")
	}
	if decoded.ToolResult == nil {
		t.Fatal("toolResult is nil")
	}
	if decoded.ToolResult.CallID != "call_456" {
		t.Errorf("callId: got %q, want %q", decoded.ToolResult.CallID, "call_456")
	}
	if decoded.ToolResult.Content != "file contents here" {
		t.Errorf("content: got %q, want %q", decoded.ToolResult.Content, "file contents here")
	}
	if decoded.ToolResult.IsError {
		t.Error("isError: got true, want false")
	}
}

func TestChatResponseFields(t *testing.T) {
	resp := ChatResponse{
		Message: Message{
			Role:    "assistant",
			Content: "Done!",
		},
		StopReason:   "end_turn",
		InputTokens:  100,
		OutputTokens: 50,
	}

	if resp.StopReason != "end_turn" {
		t.Errorf("stopReason: got %q, want %q", resp.StopReason, "end_turn")
	}
	if resp.InputTokens != 100 {
		t.Errorf("inputTokens: got %d, want 100", resp.InputTokens)
	}
	if resp.OutputTokens != 50 {
		t.Errorf("outputTokens: got %d, want 50", resp.OutputTokens)
	}
	if resp.Message.Role != "assistant" {
		t.Errorf("message role: got %q, want %q", resp.Message.Role, "assistant")
	}
}

func TestToolDefinitionJSON(t *testing.T) {
	td := ToolDefinition{
		Name:        "bash",
		Description: "Execute a bash command",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}}}`),
	}

	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ToolDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Name != "bash" {
		t.Errorf("name: got %q, want %q", decoded.Name, "bash")
	}
	if decoded.Description != "Execute a bash command" {
		t.Errorf("description: got %q, want %q", decoded.Description, "Execute a bash command")
	}
}
