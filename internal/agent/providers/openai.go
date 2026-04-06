package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	oai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// OpenAIProvider implements agent.Provider using the OpenAI Chat Completions API.
type OpenAIProvider struct {
	client oai.Client
	model  string
}

// NewOpenAI creates a new OpenAI provider. If model is empty, defaults to "gpt-4o".
// If baseURL is provided, it overrides the default API endpoint (useful for proxies).
func NewOpenAI(apiKey, model, baseURL string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4o"
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	return &OpenAIProvider{
		client: oai.NewClient(opts...),
		model:  model,
	}
}

// Name returns "openai".
func (p *OpenAIProvider) Name() string { return "openai" }

// MaxContextTokens returns the context window size for the configured model.
func (p *OpenAIProvider) MaxContextTokens() int {
	return modelContextSize(p.model)
}

// Chat sends a chat completion request and returns the response.
func (p *OpenAIProvider) Chat(ctx context.Context, req agent.ChatRequest) (*agent.ChatResponse, error) {
	model := p.model
	if req.Model != "" {
		model = req.Model
	}

	messages := convertMessages(req.SystemPrompt, req.Messages)
	tools := convertTools(req.Tools)

	params := oai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model),
		Messages: messages,
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = oai.Int(int64(req.MaxTokens))
	}

	if req.Temperature > 0 {
		params.Temperature = oai.Float(req.Temperature)
	}

	completion, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("openai chat: no choices in response")
	}

	choice := completion.Choices[0]
	resp := &agent.ChatResponse{
		Message:      convertResponseMessage(choice.Message),
		StopReason:   mapFinishReason(choice.FinishReason),
		InputTokens:  int(completion.Usage.PromptTokens),
		OutputTokens: int(completion.Usage.CompletionTokens),
	}

	return resp, nil
}

// convertMessages converts agent messages to OpenAI SDK message params.
func convertMessages(systemPrompt string, messages []agent.Message) []oai.ChatCompletionMessageParamUnion {
	var out []oai.ChatCompletionMessageParamUnion

	if systemPrompt != "" {
		out = append(out, oai.SystemMessage(systemPrompt))
	}

	for _, m := range messages {
		switch m.Role {
		case "user":
			out = append(out, oai.UserMessage(m.Content))
		case "assistant":
			msg := oai.ChatCompletionAssistantMessageParam{
				Content: oai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: oai.String(m.Content),
				},
			}
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					msg.ToolCalls = append(msg.ToolCalls, oai.ChatCompletionMessageToolCallParam{
						ID: tc.ID,
						Function: oai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Arguments),
						},
					})
				}
			}
			out = append(out, oai.ChatCompletionMessageParamUnion{
				OfAssistant: &msg,
			})
		case "tool":
			if m.ToolResult != nil {
				out = append(out, oai.ToolMessage(m.ToolResult.Content, m.ToolResult.CallID))
			}
		}
	}

	return out
}

// convertTools converts agent tool definitions to OpenAI SDK tool params.
func convertTools(tools []agent.ToolDefinition) []oai.ChatCompletionToolParam {
	var out []oai.ChatCompletionToolParam
	for _, t := range tools {
		var params shared.FunctionParameters
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &params)
		}
		out = append(out, oai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: oai.String(t.Description),
				Parameters:  params,
			},
		})
	}
	return out
}

// convertResponseMessage converts an OpenAI response message to agent.Message.
func convertResponseMessage(msg oai.ChatCompletionMessage) agent.Message {
	result := agent.Message{
		Role:    "assistant",
		Content: msg.Content,
	}

	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result
}

// mapFinishReason maps OpenAI finish reasons to agent stop reasons.
func mapFinishReason(reason string) string {
	switch reason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}

// modelContextSize returns the context window size for a given model.
func modelContextSize(model string) int {
	m := strings.ToLower(model)

	switch {
	case strings.HasPrefix(m, "gpt-4.1"), strings.HasPrefix(m, "gpt-4-1"):
		return 1047576
	case strings.HasPrefix(m, "o1-mini"):
		return 128000
	case strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"):
		return 200000
	case strings.HasPrefix(m, "gpt-3.5"):
		return 16385
	case strings.HasPrefix(m, "gpt-4o"), strings.HasPrefix(m, "gpt-4-turbo"):
		return 128000
	default:
		return 128000
	}
}
