package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/prompt"
	oai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Backend struct {
	name   string
	client oai.Client
	model  string
}

func NewBackend(name, apiKey, model string) *Backend {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := oai.NewClient(opts...)

	if model == "" {
		model = "gpt-4o"
	}

	return &Backend{
		name:   name,
		client: client,
		model:  model,
	}
}

func (b *Backend) Name() string { return b.name }

func (b *Backend) Analyze(ctx context.Context, req ai.AnalysisRequest) (*ai.Decision, error) {
	userPrompt, err := prompt.BuildUserPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	systemPrompt := prompt.SystemPrompt
	if req.SystemPromptOverride != "" {
		systemPrompt = req.SystemPromptOverride
	}

	resp, err := b.client.Chat.Completions.New(ctx, oai.ChatCompletionNewParams{
		Model: b.model,
		Messages: []oai.ChatCompletionMessageParamUnion{
			oai.SystemMessage(systemPrompt),
			oai.UserMessage(userPrompt),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("openai API call: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	text := resp.Choices[0].Message.Content
	return parseDecision(text, req)
}

func (b *Backend) Healthy(ctx context.Context) error {
	_, err := b.client.Chat.Completions.New(ctx, oai.ChatCompletionNewParams{
		Model: b.model,
		Messages: []oai.ChatCompletionMessageParamUnion{
			oai.UserMessage("ping"),
		},
	})
	return err
}

func parseDecision(text string, req ai.AnalysisRequest) (*ai.Decision, error) {
	var jd struct {
		ChosenKey  string  `json:"chosen_key"`
		Reasoning  string  `json:"reasoning"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(text), &jd); err != nil {
		extracted := extractJSON(text)
		if err2 := json.Unmarshal([]byte(extracted), &jd); err2 != nil {
			return nil, fmt.Errorf("parsing decision: %w (raw: %s)", err, text)
		}
	}

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

	if len(req.Prompt.Options) > 0 {
		decision.ChosenOption = req.Prompt.Options[0]
		decision.Confidence = 0.3
	}
	return decision, nil
}

func extractJSON(text string) string {
	start, end, depth := -1, -1, 0
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
