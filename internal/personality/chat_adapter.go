package personality

import (
	"context"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// ChatAdapter adapts an ai.ChatProvider to the AIGenerator interface
// used by Narrator, so any chat backend (Claude CLI, Anthropic, OpenAI, etc.)
// can be used for personality generation.
type ChatAdapter struct {
	provider ai.ChatProvider
}

func NewChatAdapter(provider ai.ChatProvider) *ChatAdapter {
	return &ChatAdapter{provider: provider}
}

func (c *ChatAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	messages := []ai.ChatMessage{
		{Role: "user", Content: prompt},
	}
	return c.provider.Chat(ctx, messages)
}
