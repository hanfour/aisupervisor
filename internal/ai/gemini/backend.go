package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/prompt"
	"google.golang.org/genai"
)

type Backend struct {
	name   string
	client *genai.Client
	model  string
}

func NewBackend(name, apiKey, model string) (*Backend, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	if model == "" {
		model = "gemini-2.0-flash"
	}

	return &Backend{
		name:   name,
		client: client,
		model:  model,
	}, nil
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

	resp, err := b.client.Models.GenerateContent(ctx, b.model, genai.Text(userPrompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, "user"),
	})
	if err != nil {
		return nil, fmt.Errorf("gemini API call: %w", err)
	}

	text := resp.Text()
	return parseDecision(text, req)
}

func (b *Backend) Healthy(ctx context.Context) error {
	_, err := b.client.Models.GenerateContent(ctx, b.model, genai.Text("ping"), nil)
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
