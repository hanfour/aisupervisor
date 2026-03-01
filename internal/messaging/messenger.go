package messaging

import "context"

// CommandHandler processes an incoming text command and returns a reply.
type CommandHandler func(text string) string

// Messenger is the interface for chat platform integrations (Slack, LINE, etc.).
type Messenger interface {
	Start(ctx context.Context) error
	SendNotification(msg string) error
	OnCommand(handler CommandHandler)
}
