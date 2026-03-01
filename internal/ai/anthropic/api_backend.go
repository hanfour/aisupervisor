package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/prompt"
)

type APIBackend struct {
	name   string
	client anthropic.Client
	model  string
}

func NewAPIBackend(name, apiKey, model string) *APIBackend {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := anthropic.NewClient(opts...)

	if model == "" {
		model = "claude-sonnet-4-6"
	}

	return &APIBackend{
		name:   name,
		client: client,
		model:  model,
	}
}

func (b *APIBackend) Name() string { return b.name }

func (b *APIBackend) Analyze(ctx context.Context, req ai.AnalysisRequest) (*ai.Decision, error) {
	userPrompt, err := prompt.BuildUserPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	systemPrompt := prompt.SystemPrompt
	if req.SystemPromptOverride != "" {
		systemPrompt = req.SystemPromptOverride
	}

	resp, err := b.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(b.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	text := resp.Content[0].Text
	return parseDecision(text, req)
}

func (b *APIBackend) Healthy(ctx context.Context) error {
	_, err := b.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(b.model),
		MaxTokens: 10,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ping")),
		},
	})
	return err
}

type jsonDecision struct {
	ChosenKey  string  `json:"chosen_key"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

func parseDecision(text string, req ai.AnalysisRequest) (*ai.Decision, error) {
	var jd jsonDecision
	if err := json.Unmarshal([]byte(text), &jd); err != nil {
		// Try to extract JSON from markdown code blocks
		extracted := extractJSON(text)
		if err2 := json.Unmarshal([]byte(extracted), &jd); err2 != nil {
			return nil, fmt.Errorf("parsing decision JSON: %w (raw: %s)", err, text)
		}
	}

	// Find the matching option
	decision := &ai.Decision{
		Reasoning:  jd.Reasoning,
		Confidence: jd.Confidence,
	}

	for _, opt := range req.Prompt.Options {
		if opt.Key == jd.ChosenKey {
			decision.ChosenOption = opt
			return decision, nil
		}
	}

	// If no exact match, use the first option as fallback with lower confidence
	if len(req.Prompt.Options) > 0 {
		decision.ChosenOption = req.Prompt.Options[0]
		decision.Confidence = 0.3
	}

	return decision, nil
}

func extractJSON(text string) string {
	// Look for JSON between ```json and ``` or { and }
	start := -1
	end := -1
	depth := 0

	for i, ch := range text {
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		return text[start:end]
	}
	return text
}
