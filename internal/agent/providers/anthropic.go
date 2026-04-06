package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/agent"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements agent.Provider using the Anthropic Messages API.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
}

// NewAnthropic creates a new Anthropic provider. If model is empty, defaults to
// "claude-sonnet-4-20250514". If baseURL is provided, it overrides the default
// API endpoint.
func NewAnthropic(apiKey, model, baseURL string) *AnthropicProvider {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	return &AnthropicProvider{
		client: anthropic.NewClient(opts...),
		model:  model,
	}
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string { return "anthropic" }

// MaxContextTokens returns the context window size for the configured model.
func (p *AnthropicProvider) MaxContextTokens() int {
	return anthropicModelContextSize(p.model)
}

// Chat sends a message request to the Anthropic Messages API and returns the response.
func (p *AnthropicProvider) Chat(ctx context.Context, req agent.ChatRequest) (*agent.ChatResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	params := anthropic.MessageNewParams{
		Model:    anthropic.Model(model),
		Messages: convertAnthropicMessages(req.Messages),
		MaxTokens: maxTokens,
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	if len(req.Tools) > 0 {
		params.Tools = convertAnthropicTools(req.Tools)
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}

	resp := &agent.ChatResponse{
		Message:      convertAnthropicResponse(msg),
		StopReason:   string(msg.StopReason),
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}

	return resp, nil
}

// convertAnthropicMessages converts agent messages to Anthropic MessageParam slices.
// Anthropic requires alternating user/assistant turns. Tool results are sent as
// user messages with tool_result content blocks.
func convertAnthropicMessages(messages []agent.Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam

	for _, m := range messages {
		switch m.Role {
		case "user":
			out = append(out, anthropic.NewUserMessage(
				anthropic.NewTextBlock(m.Content),
			))

		case "assistant":
			var blocks []anthropic.ContentBlockParamUnion
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				var input any
				if len(tc.Arguments) > 0 {
					_ = json.Unmarshal(tc.Arguments, &input)
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
			}
			if len(blocks) > 0 {
				out = append(out, anthropic.NewAssistantMessage(blocks...))
			}

		case "tool":
			if m.ToolResult != nil {
				out = append(out, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(m.ToolResult.CallID, m.ToolResult.Content, m.ToolResult.IsError),
				))
			}
		}
	}

	return out
}

// convertAnthropicTools converts agent tool definitions to Anthropic ToolUnionParam slices.
func convertAnthropicTools(tools []agent.ToolDefinition) []anthropic.ToolUnionParam {
	var out []anthropic.ToolUnionParam

	for _, t := range tools {
		schema := anthropic.ToolInputSchemaParam{}
		if len(t.Parameters) > 0 {
			var raw map[string]any
			if err := json.Unmarshal(t.Parameters, &raw); err == nil {
				if props, ok := raw["properties"]; ok {
					schema.Properties = props
				}
				if req, ok := raw["required"]; ok {
					if reqSlice, ok := req.([]any); ok {
						for _, r := range reqSlice {
							if s, ok := r.(string); ok {
								schema.Required = append(schema.Required, s)
							}
						}
					}
				}
			}
		}

		tp := anthropic.ToolUnionParamOfTool(schema, t.Name)
		tp.OfTool.Description = anthropic.String(t.Description)
		out = append(out, tp)
	}

	return out
}

// convertAnthropicResponse converts an Anthropic Message response to agent.Message.
func convertAnthropicResponse(msg *anthropic.Message) agent.Message {
	result := agent.Message{
		Role: "assistant",
	}

	var textParts []string
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			inputBytes, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: json.RawMessage(inputBytes),
			})
		}
	}

	result.Content = strings.Join(textParts, "")
	return result
}

// anthropicModelContextSize returns the context window size for a given Anthropic model.
func anthropicModelContextSize(model string) int {
	m := strings.ToLower(model)

	switch {
	case strings.Contains(m, "opus"):
		return 200000
	case strings.Contains(m, "sonnet"):
		return 200000
	case strings.Contains(m, "haiku"):
		return 200000
	default:
		return 200000
	}
}
