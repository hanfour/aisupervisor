package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/prompt"
	"github.com/hanfourmini/aisupervisor/internal/keychain"
)

const tokenEndpoint = "https://console.anthropic.com/api/oauth/token"

type OAuthBackend struct {
	name  string
	model string

	mu          sync.Mutex
	accessToken string
	refreshTok  string
	expiresAt   time.Time
}

func NewOAuthBackend(name, model string) (*OAuthBackend, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	b := &OAuthBackend{
		name:  name,
		model: model,
	}

	// Load initial credentials from keychain
	creds, err := keychain.ReadClaudeCredentials()
	if err != nil {
		return nil, fmt.Errorf("reading OAuth credentials: %w", err)
	}

	b.accessToken = creds.AccessToken
	b.refreshTok = creds.RefreshToken
	b.expiresAt = creds.ExpiresAt

	return b, nil
}

func (b *OAuthBackend) Name() string { return b.name }

func (b *OAuthBackend) getClient(ctx context.Context) (sdk.Client, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if time.Now().After(b.expiresAt.Add(-5 * time.Minute)) {
		if err := b.refresh(ctx); err != nil {
			return sdk.Client{}, fmt.Errorf("refreshing token: %w", err)
		}
	}

	return sdk.NewClient(
		option.WithAPIKey(b.accessToken),
	), nil
}

func (b *OAuthBackend) refresh(ctx context.Context) error {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {b.refreshTok},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	b.accessToken = result.AccessToken
	if result.RefreshToken != "" {
		b.refreshTok = result.RefreshToken
	}
	b.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return nil
}

func (b *OAuthBackend) Analyze(ctx context.Context, req ai.AnalysisRequest) (*ai.Decision, error) {
	client, err := b.getClient(ctx)
	if err != nil {
		return nil, err
	}

	userPrompt, err := prompt.BuildUserPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	systemPrompt := prompt.SystemPrompt
	if req.SystemPromptOverride != "" {
		systemPrompt = req.SystemPromptOverride
	}

	resp, err := client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(b.model),
		MaxTokens: 1024,
		System: []sdk.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	return parseDecision(resp.Content[0].Text, req)
}

func (b *OAuthBackend) Healthy(ctx context.Context) error {
	client, err := b.getClient(ctx)
	if err != nil {
		return err
	}

	_, err = client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(b.model),
		MaxTokens: 10,
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock("ping")),
		},
	})
	return err
}
