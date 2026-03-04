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
