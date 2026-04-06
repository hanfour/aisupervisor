package agent

import (
	"encoding/json"
	"testing"
)

func TestContextManager_AddAndGet(t *testing.T) {
	cm := NewContextManager("You are a helper.", 200000)

	cm.AddUser("Hello")
	cm.AddAssistant(Message{Role: "assistant", Content: "Hi there!"})
	cm.AddToolResult(ToolResult{CallID: "call_1", Content: "result"})

	msgs := cm.Messages()
	if len(msgs) != 3 {
		t.Fatalf("messages length: got %d, want 3", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("msg[0] role: got %q, want %q", msgs[0].Role, "user")
	}
	if msgs[0].Content != "Hello" {
		t.Errorf("msg[0] content: got %q, want %q", msgs[0].Content, "Hello")
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("msg[1] role: got %q, want %q", msgs[1].Role, "assistant")
	}
	if msgs[2].Role != "tool" {
		t.Errorf("msg[2] role: got %q, want %q", msgs[2].Role, "tool")
	}
	if msgs[2].ToolResult == nil {
		t.Fatal("msg[2] toolResult is nil")
	}
	if msgs[2].ToolResult.CallID != "call_1" {
		t.Errorf("msg[2] callId: got %q, want %q", msgs[2].ToolResult.CallID, "call_1")
	}
}

func TestContextManager_SystemPrompt(t *testing.T) {
	cm := NewContextManager("System prompt here.", 100000)
	if cm.SystemPrompt() != "System prompt here." {
		t.Errorf("system prompt: got %q, want %q", cm.SystemPrompt(), "System prompt here.")
	}
}

func TestContextManager_DefaultMaxTokens(t *testing.T) {
	cm := NewContextManager("test", 0)
	// With default max tokens (200000), estimate should work
	cm.AddUser("hello")
	if cm.EstimateTokens() <= 0 {
		t.Error("EstimateTokens should be > 0 after adding messages")
	}
}

func TestContextManager_EstimateTokens(t *testing.T) {
	cm := NewContextManager("system prompt", 200000)
	cm.AddUser("hello world")
	cm.AddAssistant(Message{
		Role:    "assistant",
		Content: "response text",
		ToolCalls: []ToolCall{
			{ID: "1", Name: "test", Arguments: json.RawMessage(`{"key":"value"}`)},
		},
	})
	cm.AddToolResult(ToolResult{CallID: "1", Content: "tool output"})

	tokens := cm.EstimateTokens()
	if tokens <= 0 {
		t.Errorf("EstimateTokens: got %d, want > 0", tokens)
	}
}

func TestContextManager_Truncate(t *testing.T) {
	// Use a very small max to force truncation
	cm := NewContextManager("sys", 20)

	// Add many messages to exceed the threshold
	for i := 0; i < 50; i++ {
		cm.AddUser("This is a message with some content that takes up tokens for testing purposes.")
	}

	before := len(cm.Messages())
	cm.Truncate()
	after := len(cm.Messages())

	if after >= before {
		t.Errorf("Truncate should reduce messages: before=%d, after=%d", before, after)
	}
	if after < 10 {
		t.Errorf("Truncate should keep at least 10 messages: got %d", after)
	}
}

func TestContextManager_TruncateKeepsLast(t *testing.T) {
	cm := NewContextManager("sys", 20)

	for i := 0; i < 50; i++ {
		cm.AddUser("This is a message with some content that takes up tokens for testing.")
	}
	cm.AddUser("LAST_MESSAGE")

	cm.Truncate()
	msgs := cm.Messages()
	last := msgs[len(msgs)-1]
	if last.Content != "LAST_MESSAGE" {
		t.Errorf("last message after truncate: got %q, want %q", last.Content, "LAST_MESSAGE")
	}
}

func TestContextManager_Reset(t *testing.T) {
	cm := NewContextManager("sys", 200000)
	cm.AddUser("hello")
	cm.AddAssistant(Message{Role: "assistant", Content: "hi"})
	cm.Reset()

	if len(cm.Messages()) != 0 {
		t.Errorf("messages after reset: got %d, want 0", len(cm.Messages()))
	}
	// System prompt should still be there
	if cm.SystemPrompt() != "sys" {
		t.Errorf("system prompt after reset: got %q, want %q", cm.SystemPrompt(), "sys")
	}
}
