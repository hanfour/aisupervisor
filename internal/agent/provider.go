package agent

import (
	"context"
	"encoding/json"
)

// Message represents a chat message in a conversation.
type Message struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	ToolCalls  []ToolCall   `json:"toolCalls,omitempty"`
	ToolResult *ToolResult  `json:"toolResult,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of executing a tool call.
type ToolResult struct {
	CallID  string `json:"callId"`
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// ToolDefinition describes a tool that the model can invoke.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ChatRequest contains all parameters for a chat completion request.
type ChatRequest struct {
	Model        string
	Messages     []Message
	Tools        []ToolDefinition
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	OnChunk      func(text string)
}

// ChatResponse contains the model's response and usage metadata.
type ChatResponse struct {
	Message      Message
	StopReason   string // "end_turn", "tool_use", "max_tokens"
	InputTokens  int
	OutputTokens int
}

// Provider is the interface that LLM backends must implement.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	Name() string
	MaxContextTokens() int
}
