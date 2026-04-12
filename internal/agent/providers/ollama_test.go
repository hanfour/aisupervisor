package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent"
)

func TestNewOllama_Defaults(t *testing.T) {
	p := NewOllama("", "")

	if p.Name() != "ollama" {
		t.Errorf("Name: got %q, want %q", p.Name(), "ollama")
	}
	if p.MaxContextTokens() != 128000 {
		t.Errorf("MaxContextTokens: got %d, want %d", p.MaxContextTokens(), 128000)
	}
	if p.model != "llama3" {
		t.Errorf("default model: got %q, want %q", p.model, "llama3")
	}
	if p.baseURL != "http://localhost:11434" {
		t.Errorf("default baseURL: got %q, want %q", p.baseURL, "http://localhost:11434")
	}
}

func TestNewOllama_CustomValues(t *testing.T) {
	p := NewOllama("mistral", "http://myhost:1234/")

	if p.model != "mistral" {
		t.Errorf("model: got %q, want %q", p.model, "mistral")
	}
	// Trailing slash should be trimmed.
	if p.baseURL != "http://myhost:1234" {
		t.Errorf("baseURL: got %q, want %q", p.baseURL, "http://myhost:1234")
	}
}

func TestOllama_ParseToolCallTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		first   string // expected first tool name
	}{
		{
			name:    "single tool call",
			content: `Sure, I'll read that file. <tool_call>{"name": "Read", "arguments": {"file_path": "/tmp/x"}}</tool_call>`,
			want:    1,
			first:   "Read",
		},
		{
			name: "multiple tool calls",
			content: `Let me check both files.
<tool_call>{"name": "Read", "arguments": {"file_path": "/a"}}</tool_call>
<tool_call>{"name": "Glob", "arguments": {"pattern": "*.go"}}</tool_call>`,
			want:  2,
			first: "Read",
		},
		{
			name:    "no tool calls",
			content: "Just a regular response with no tools.",
			want:    0,
		},
		{
			name:    "malformed JSON inside tag",
			content: `<tool_call>not valid json</tool_call>`,
			want:    0,
		},
		{
			name:    "empty name ignored",
			content: `<tool_call>{"name": "", "arguments": {}}</tool_call>`,
			want:    0,
		},
		{
			name:    "tool call with whitespace in tag",
			content: "<tool_call> {\"name\": \"Bash\", \"arguments\": {\"command\": \"ls\"}} </tool_call>",
			want:    1,
			first:   "Bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := parseToolCallTags(tt.content)
			if len(calls) != tt.want {
				t.Fatalf("got %d calls, want %d", len(calls), tt.want)
			}
			if tt.want > 0 && calls[0].Name != tt.first {
				t.Errorf("first call name: got %q, want %q", calls[0].Name, tt.first)
			}
		})
	}
}

func TestOllama_Chat_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify request body.
		var reqBody ollamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reqBody.Model != "llama3" {
			t.Errorf("model: got %q, want %q", reqBody.Model, "llama3")
		}
		if reqBody.Stream != false {
			t.Error("stream should be false")
		}

		resp := ollamaChatResponse{
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "Hello! How can I help you?",
			},
			Done:            true,
			EvalCount:       42,
			PromptEvalCount: 10,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllama("llama3", server.URL)
	resp, err := p.Chat(context.Background(), agent.ChatRequest{
		Messages: []agent.Message{
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	if resp.Message.Role != "assistant" {
		t.Errorf("role: got %q, want %q", resp.Message.Role, "assistant")
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("content: got %q", resp.Message.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop reason: got %q, want %q", resp.StopReason, "end_turn")
	}
	if resp.InputTokens != 10 {
		t.Errorf("input tokens: got %d, want %d", resp.InputTokens, 10)
	}
	if resp.OutputTokens != 42 {
		t.Errorf("output tokens: got %d, want %d", resp.OutputTokens, 42)
	}
}

func TestOllama_Chat_NativeToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaChatResponse{
			Message: ollamaMessage{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{
						Function: ollamaFunctionCall{
							Name:      "Read",
							Arguments: json.RawMessage(`{"file_path":"/tmp/test.go"}`),
						},
					},
				},
			},
			Done:            true,
			EvalCount:       15,
			PromptEvalCount: 50,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllama("llama3", server.URL)
	resp, err := p.Chat(context.Background(), agent.ChatRequest{
		Messages: []agent.Message{
			{Role: "user", Content: "Read the file"},
		},
		Tools: []agent.ToolDefinition{
			{Name: "Read", Description: "Read a file", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("stop reason: got %q, want %q", resp.StopReason, "tool_use")
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls: got %d, want %d", len(resp.Message.ToolCalls), 1)
	}
	tc := resp.Message.ToolCalls[0]
	if tc.Name != "Read" {
		t.Errorf("tool name: got %q, want %q", tc.Name, "Read")
	}
	if tc.ID != "ollama_0" {
		t.Errorf("tool ID: got %q, want %q", tc.ID, "ollama_0")
	}
}

func TestOllama_Chat_FallbackToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Model returns tool call tags in plain text (no native tool_calls).
		resp := ollamaChatResponse{
			Message: ollamaMessage{
				Role:    "assistant",
				Content: `I'll read that file for you. <tool_call>{"name": "Read", "arguments": {"file_path": "/tmp/x"}}</tool_call>`,
			},
			Done:            true,
			EvalCount:       20,
			PromptEvalCount: 30,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOllama("llama3", server.URL)
	resp, err := p.Chat(context.Background(), agent.ChatRequest{
		Messages: []agent.Message{
			{Role: "user", Content: "Read /tmp/x"},
		},
		Tools: []agent.ToolDefinition{
			{Name: "Read", Description: "Read a file", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("stop reason: got %q, want %q", resp.StopReason, "tool_use")
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls: got %d, want %d", len(resp.Message.ToolCalls), 1)
	}
	if resp.Message.ToolCalls[0].Name != "Read" {
		t.Errorf("tool name: got %q, want %q", resp.Message.ToolCalls[0].Name, "Read")
	}
	if resp.Message.ToolCalls[0].ID != "fallback_0" {
		t.Errorf("tool ID: got %q, want %q", resp.Message.ToolCalls[0].ID, "fallback_0")
	}
}

func TestOllama_Chat_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	p := NewOllama("llama3", server.URL)
	_, err := p.Chat(context.Background(), agent.ChatRequest{
		Messages: []agent.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !contains(got, "status 500") {
		t.Errorf("error should mention status 500: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
