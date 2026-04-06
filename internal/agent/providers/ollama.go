package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/agent"
)

const (
	defaultOllamaURL   = "http://localhost:11434"
	defaultOllamaModel = "llama3"
)

// OllamaProvider implements agent.Provider using the Ollama local API.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllama creates a new Ollama provider. If model is empty, defaults to "llama3".
// If baseURL is empty, defaults to "http://localhost:11434".
func NewOllama(model, baseURL string) *OllamaProvider {
	if model == "" {
		model = defaultOllamaModel
	}
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	// Trim trailing slash for consistent URL construction.
	baseURL = strings.TrimRight(baseURL, "/")

	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

// Name returns "ollama".
func (p *OllamaProvider) Name() string { return "ollama" }

// MaxContextTokens returns a conservative default context window size.
func (p *OllamaProvider) MaxContextTokens() int { return 128000 }

// Chat sends a chat completion request to the Ollama API and returns the response.
func (p *OllamaProvider) Chat(ctx context.Context, req agent.ChatRequest) (*agent.ChatResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	messages := ollamaConvertMessages(req.SystemPrompt, req.Messages)
	ollamaTools := ollamaConvertTools(req.Tools)

	// If tools are provided but model may not support native tool calling,
	// we send them both natively and inject a prompt-based fallback.
	useFallback := len(req.Tools) > 0
	if useFallback {
		messages = injectToolFallbackPrompt(messages, req.Tools)
	}

	body := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}
	if len(ollamaTools) > 0 {
		body.Tools = ollamaTools
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ollama create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama http call: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama api error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama unmarshal response: %w", err)
	}

	resp := &agent.ChatResponse{
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
	}

	// Check for native tool calls first.
	if len(ollamaResp.Message.ToolCalls) > 0 {
		resp.Message = ollamaConvertResponseMessage(ollamaResp.Message)
		resp.StopReason = "tool_use"
		return resp, nil
	}

	// Fallback: parse <tool_call> tags from the response content.
	if useFallback {
		toolCalls := parseToolCallTags(ollamaResp.Message.Content)
		if len(toolCalls) > 0 {
			resp.Message = agent.Message{
				Role:      "assistant",
				Content:   ollamaResp.Message.Content,
				ToolCalls: toolCalls,
			}
			resp.StopReason = "tool_use"
			return resp, nil
		}
	}

	// No tool calls — plain text response.
	resp.Message = agent.Message{
		Role:    "assistant",
		Content: ollamaResp.Message.Content,
	}
	resp.StopReason = "end_turn"
	return resp, nil
}

// --- Ollama API types ---

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	ToolCalls []ollamaToolCall   `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ollamaChatResponse struct {
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	EvalCount       int           `json:"eval_count"`
	PromptEvalCount int           `json:"prompt_eval_count"`
}

// --- Conversion helpers ---

// ollamaConvertMessages converts agent messages to Ollama message format.
func ollamaConvertMessages(systemPrompt string, messages []agent.Message) []ollamaMessage {
	var out []ollamaMessage

	if systemPrompt != "" {
		out = append(out, ollamaMessage{Role: "system", Content: systemPrompt})
	}

	for _, m := range messages {
		switch m.Role {
		case "user":
			out = append(out, ollamaMessage{Role: "user", Content: m.Content})
		case "assistant":
			msg := ollamaMessage{Role: "assistant", Content: m.Content}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
					Function: ollamaFunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			out = append(out, msg)
		case "tool":
			if m.ToolResult != nil {
				// Ollama expects tool results as a "tool" role message.
				out = append(out, ollamaMessage{
					Role:    "tool",
					Content: m.ToolResult.Content,
				})
			}
		}
	}

	return out
}

// ollamaConvertTools converts agent tool definitions to Ollama tool format
// (OpenAI-compatible function calling format).
func ollamaConvertTools(tools []agent.ToolDefinition) []ollamaTool {
	var out []ollamaTool
	for _, t := range tools {
		out = append(out, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

// ollamaConvertResponseMessage converts an Ollama response message to agent.Message.
func ollamaConvertResponseMessage(msg ollamaMessage) agent.Message {
	result := agent.Message{
		Role:    "assistant",
		Content: msg.Content,
	}

	for i, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
			ID:        fmt.Sprintf("ollama_%d", i),
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return result
}

// --- Prompt-based tool call fallback ---

// injectToolFallbackPrompt appends instructions to the system message telling
// the model to emit <tool_call> tags when it wants to invoke a tool.
func injectToolFallbackPrompt(messages []ollamaMessage, tools []agent.ToolDefinition) []ollamaMessage {
	var sb strings.Builder
	sb.WriteString("If you want to use a tool, respond with a <tool_call> tag in your output. Format:\n")
	sb.WriteString(`<tool_call>{"name": "ToolName", "arguments": {"param": "value"}}</tool_call>`)
	sb.WriteString("\n\nAvailable tools:\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}

	// Append to existing system message or prepend a new one.
	if len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content += "\n\n" + sb.String()
	} else {
		messages = append([]ollamaMessage{{Role: "system", Content: sb.String()}}, messages...)
	}

	return messages
}

// toolCallTagRegex matches <tool_call>{...}</tool_call> patterns.
// Captures everything between the tags, then validates as JSON.
var toolCallTagRegex = regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*</tool_call>`)

// parseToolCallTags extracts tool calls from <tool_call> tags in content.
func parseToolCallTags(content string) []agent.ToolCall {
	matches := toolCallTagRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var calls []agent.ToolCall
	for i, m := range matches {
		var parsed struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
			continue
		}
		if parsed.Name == "" {
			continue
		}
		calls = append(calls, agent.ToolCall{
			ID:        fmt.Sprintf("fallback_%d", i),
			Name:      parsed.Name,
			Arguments: parsed.Arguments,
		})
	}

	return calls
}
