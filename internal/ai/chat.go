package ai

import "context"

// ChatMessage represents a single message in a multi-turn chat conversation.
type ChatMessage struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// ChatProvider is the interface for multi-turn chat backends (worker NPC chat, project creation, etc.).
type ChatProvider interface {
	Chat(ctx context.Context, messages []ChatMessage) (string, error)
}

// ModelOverrideChat is an optional interface that ChatProviders can implement
// to support per-call model overrides. Used by the debate review pipeline.
type ModelOverrideChat interface {
	ChatWithModel(ctx context.Context, messages []ChatMessage, model string) (string, error)
}

// ChatWithModelOrFallback calls ChatWithModel if the provider supports it,
// otherwise falls back to Chat (ignoring the model parameter).
func ChatWithModelOrFallback(ctx context.Context, cp ChatProvider, messages []ChatMessage, model string) (string, error) {
	if override, ok := cp.(ModelOverrideChat); ok && model != "" {
		return override.ChatWithModel(ctx, messages, model)
	}
	return cp.Chat(ctx, messages)
}
