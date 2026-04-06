package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent/tools"
)

// mockProvider returns pre-configured responses in sequence.
type mockProvider struct {
	responses []ChatResponse
	callIdx   int
}

func (m *mockProvider) Name() string          { return "mock" }
func (m *mockProvider) MaxContextTokens() int { return 100000 }
func (m *mockProvider) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	if m.callIdx >= len(m.responses) {
		return &ChatResponse{Message: Message{Role: "assistant", Content: "done"}, StopReason: "end_turn"}, nil
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return &resp, nil
}

func TestLoop_SimpleResponse(t *testing.T) {
	provider := &mockProvider{responses: []ChatResponse{
		{Message: Message{Role: "assistant", Content: "Hello!"}, StopReason: "end_turn"},
	}}
	registry := tools.NewRegistry()
	loop := NewLoop(provider, registry, "Be helpful.", 200000)

	var output string
	loop.OnOutput = func(text string) { output = text }

	err := loop.Run(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "Hello!" {
		t.Errorf("expected Hello!, got: %s", output)
	}
}

func TestLoop_ToolUse(t *testing.T) {
	provider := &mockProvider{responses: []ChatResponse{
		{Message: Message{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "tc1", Name: "Read", Arguments: json.RawMessage(`{"file_path":"/dev/null"}`)},
		}}, StopReason: "tool_use"},
		{Message: Message{Role: "assistant", Content: "File is empty."}, StopReason: "end_turn"},
	}}
	registry := tools.NewRegistry()
	registry.Register(&tools.ReadTool{})

	loop := NewLoop(provider, registry, "Be helpful.", 200000)
	var output string
	loop.OnOutput = func(text string) { output = text }

	err := loop.Run(context.Background(), "Read /dev/null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "File is empty." {
		t.Errorf("expected 'File is empty.', got: %s", output)
	}
	if provider.callIdx != 2 {
		t.Errorf("expected 2 LLM calls, got %d", provider.callIdx)
	}
}

func TestLoop_MaxIterations(t *testing.T) {
	provider := &infiniteToolProvider{}
	registry := tools.NewRegistry()
	registry.Register(&tools.ReadTool{})

	loop := NewLoop(provider, registry, "sys", 200000)
	loop.MaxIterations = 3

	err := loop.Run(context.Background(), "loop forever")
	if err == nil {
		t.Error("should return max iterations error")
	}
}

// infiniteToolProvider always returns a tool_use response, causing an infinite loop.
type infiniteToolProvider struct{}

func (p *infiniteToolProvider) Name() string          { return "infinite" }
func (p *infiniteToolProvider) MaxContextTokens() int { return 100000 }
func (p *infiniteToolProvider) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{
		Message: Message{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "tc", Name: "Read", Arguments: json.RawMessage(`{"file_path":"/dev/null"}`)},
		}},
		StopReason: "tool_use",
	}, nil
}

func TestLoop_UnknownTool(t *testing.T) {
	provider := &mockProvider{responses: []ChatResponse{
		{Message: Message{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "tc1", Name: "UnknownTool", Arguments: json.RawMessage(`{}`)},
		}}, StopReason: "tool_use"},
		{Message: Message{Role: "assistant", Content: "Ok"}, StopReason: "end_turn"},
	}}
	registry := tools.NewRegistry()
	loop := NewLoop(provider, registry, "sys", 200000)

	var output string
	loop.OnOutput = func(text string) { output = text }

	err := loop.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "Ok" {
		t.Errorf("expected Ok, got: %s", output)
	}
}

func TestLoop_TokenAccumulation(t *testing.T) {
	provider := &mockProvider{responses: []ChatResponse{
		{Message: Message{Role: "assistant", Content: "Hello!"}, StopReason: "end_turn", InputTokens: 50, OutputTokens: 10},
	}}
	registry := tools.NewRegistry()
	loop := NewLoop(provider, registry, "sys", 200000)
	loop.OnOutput = func(string) {}

	if err := loop.Run(context.Background(), "Hi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loop.TotalInput != 50 {
		t.Errorf("expected TotalInput 50, got %d", loop.TotalInput)
	}
	if loop.TotalOutput != 10 {
		t.Errorf("expected TotalOutput 10, got %d", loop.TotalOutput)
	}
}

func TestLoop_OnToolCallCallback(t *testing.T) {
	provider := &mockProvider{responses: []ChatResponse{
		{Message: Message{Role: "assistant", ToolCalls: []ToolCall{
			{ID: "tc1", Name: "Read", Arguments: json.RawMessage(`{"file_path":"/dev/null"}`)},
		}}, StopReason: "tool_use"},
		{Message: Message{Role: "assistant", Content: "done"}, StopReason: "end_turn"},
	}}
	registry := tools.NewRegistry()
	registry.Register(&tools.ReadTool{})
	loop := NewLoop(provider, registry, "sys", 200000)

	var calledTool string
	loop.OnToolCall = func(name, args string) { calledTool = name }
	loop.OnOutput = func(string) {}

	if err := loop.Run(context.Background(), "test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calledTool != "Read" {
		t.Errorf("expected tool call to Read, got: %s", calledTool)
	}
}

func TestLoop_ResetHistory(t *testing.T) {
	provider := &mockProvider{responses: []ChatResponse{
		{Message: Message{Role: "assistant", Content: "first"}, StopReason: "end_turn"},
	}}
	registry := tools.NewRegistry()
	loop := NewLoop(provider, registry, "sys", 200000)
	loop.OnOutput = func(string) {}

	_ = loop.Run(context.Background(), "Hi")
	loop.ResetHistory()

	// After reset, context messages should be empty (only the new user msg will be added on next Run)
	msgs := loop.context.Messages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after reset, got %d", len(msgs))
	}
}
