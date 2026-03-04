package openai

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	oai "github.com/openai/openai-go"
)

// Chat implements ai.ChatProvider using the OpenAI Chat Completions API.
func (b *Backend) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	oaiMessages := make([]oai.ChatCompletionMessageParamUnion, len(messages))
	for i, m := range messages {
		switch m.Role {
		case "system":
			oaiMessages[i] = oai.SystemMessage(m.Content)
		case "assistant":
			oaiMessages[i] = oai.AssistantMessage(m.Content)
		default:
			oaiMessages[i] = oai.UserMessage(m.Content)
		}
	}

	resp, err := b.client.Chat.Completions.New(ctx, oai.ChatCompletionNewParams{
		Model:    b.model,
		Messages: oaiMessages,
	})
	if err != nil {
		return "", fmt.Errorf("openai chat: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
