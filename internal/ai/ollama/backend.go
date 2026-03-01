package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/prompt"
)

type Backend struct {
	name    string
	baseURL string
	model   string
	client  *http.Client
}

func NewBackend(name, baseURL, model string) *Backend {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}

	return &Backend{
		name:    name,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

func (b *Backend) Name() string { return b.name }

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
}

func (b *Backend) Analyze(ctx context.Context, req ai.AnalysisRequest) (*ai.Decision, error) {
	userPrompt, err := prompt.BuildUserPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	systemPrompt := prompt.SystemPrompt
	if req.SystemPromptOverride != "" {
		systemPrompt = req.SystemPromptOverride
	}

	body := chatRequest{
		Model: b.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	data, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama error (%d): %s", resp.StatusCode, respBody)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parsing ollama response: %w", err)
	}

	return parseDecision(chatResp.Message.Content, req)
}

func (b *Backend) Healthy(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama not reachable: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
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
