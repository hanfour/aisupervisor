package anthropic

import (
	"context"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// Chat implements ai.ChatProvider for APIBackend.
func (b *APIBackend) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	system, sdkMessages := convertChatMessages(messages)

	resp, err := b.client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(b.model),
		MaxTokens: 4096,
		System:    system,
		Messages:  sdkMessages,
	})
	if err != nil {
		return "", fmt.Errorf("anthropic chat: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic")
	}

	return resp.Content[0].Text, nil
}

// Chat implements ai.ChatProvider for OAuthBackend.
func (b *OAuthBackend) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	client, err := b.getClient(ctx)
	if err != nil {
		return "", err
	}

	system, sdkMessages := convertChatMessages(messages)

	resp, err := client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(b.model),
		MaxTokens: 4096,
		System:    system,
		Messages:  sdkMessages,
	})
	if err != nil {
		return "", fmt.Errorf("anthropic chat: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic")
	}

	return resp.Content[0].Text, nil
}

// convertChatMessages splits ChatMessages into Anthropic system blocks and message params.
func convertChatMessages(messages []ai.ChatMessage) ([]sdk.TextBlockParam, []sdk.MessageParam) {
	var system []sdk.TextBlockParam
	var sdkMessages []sdk.MessageParam

	for _, m := range messages {
		switch m.Role {
		case "system":
			system = append(system, sdk.TextBlockParam{Text: m.Content})
		case "assistant":
			sdkMessages = append(sdkMessages, sdk.NewAssistantMessage(sdk.NewTextBlock(m.Content)))
		default:
			sdkMessages = append(sdkMessages, sdk.NewUserMessage(sdk.NewTextBlock(m.Content)))
		}
	}

	return system, sdkMessages
}
