package company

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/hanfourmini/aisupervisor/internal/project"
)

// WorkerChatMessage represents a single message in a worker chat conversation.
type WorkerChatMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// WorkerChatResponse is the response from the worker NPC chat.
type WorkerChatResponse struct {
	Content string `json:"content"`
}

// ChatWithWorker sends a conversation to the Claude API with a system prompt
// that reflects the worker's personality and knowledge (including research reports).
func (m *Manager) ChatWithWorker(ctx context.Context, workerID string, messages []WorkerChatMessage) (*WorkerChatResponse, error) {
	w, ok := m.GetWorker(workerID)
	if !ok {
		return nil, fmt.Errorf("worker %q not found", workerID)
	}

	// Build system prompt from personality
	systemPrompt := m.buildWorkerSystemPrompt(workerID, w.Name, string(w.EffectiveTier()))

	// Convert messages
	apiMessages := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			apiMessages = append(apiMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case "assistant":
			apiMessages = append(apiMessages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}

	client := anthropic.NewClient(option.WithAPIKey(""))
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model("claude-sonnet-4-6"),
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: apiMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	return &WorkerChatResponse{
		Content: resp.Content[0].Text,
	}, nil
}

// buildWorkerSystemPrompt constructs the NPC system prompt from personality data
// and any research reports the worker has completed.
func (m *Manager) buildWorkerSystemPrompt(workerID, name, tier string) string {
	lang := m.GetLanguage()
	var sb strings.Builder

	if lang == "en" {
		sb.WriteString(fmt.Sprintf("You are %s, a %s-tier AI worker.\n", name, tier))
	} else {
		sb.WriteString(fmt.Sprintf("你是 %s，一位 %s 級 AI 工作者。\n", name, tier))
	}

	// Add personality info if available
	store := m.GetPersonalityStore()
	if store != nil {
		profile := store.GetProfile(workerID)
		if profile != nil {
			if lang == "en" {
				if profile.Narrative.Description != "" {
					sb.WriteString(fmt.Sprintf("Personality: %s\n", profile.Narrative.Description))
				}
				if len(profile.Narrative.Catchphrases) > 0 {
					sb.WriteString(fmt.Sprintf("Catchphrases: %s\n", strings.Join(profile.Narrative.Catchphrases, ", ")))
				}
				if profile.Narrative.Backstory != "" {
					sb.WriteString(fmt.Sprintf("Background: %s\n", profile.Narrative.Backstory))
				}
				sb.WriteString(fmt.Sprintf("Current mood: %s, energy %d/100\n", profile.Mood.Current, profile.Mood.Energy))
			} else {
				if profile.Narrative.Description != "" {
					sb.WriteString(fmt.Sprintf("性格：%s\n", profile.Narrative.Description))
				}
				if len(profile.Narrative.Catchphrases) > 0 {
					sb.WriteString(fmt.Sprintf("口頭禪：%s\n", strings.Join(profile.Narrative.Catchphrases, "、")))
				}
				if profile.Narrative.Backstory != "" {
					sb.WriteString(fmt.Sprintf("背景：%s\n", profile.Narrative.Backstory))
				}
				sb.WriteString(fmt.Sprintf("目前心情：%s，精力 %d/100\n", profile.Mood.Current, profile.Mood.Energy))
			}
		}
	}

	// Add research reports as knowledge context (projectStore has its own lock)
	reports := m.findWorkerReports(workerID)
	if len(reports) > 0 {
		if lang == "en" {
			sb.WriteString("\nYou recently completed the following research:\n")
			for _, r := range reports {
				sb.WriteString(fmt.Sprintf("- Research summary: %s\n", r.Summary))
				if r.RawContent != "" {
					content := r.RawContent
					if len(content) > 2000 {
						content = content[:2000] + "..."
					}
					sb.WriteString(fmt.Sprintf("  Details: %s\n", content))
				}
			}
		} else {
			sb.WriteString("\n你最近完成了以下研究：\n")
			for _, r := range reports {
				sb.WriteString(fmt.Sprintf("- 研究摘要：%s\n", r.Summary))
				if r.RawContent != "" {
					content := r.RawContent
					if len(content) > 2000 {
						content = content[:2000] + "..."
					}
					sb.WriteString(fmt.Sprintf("  詳細內容：%s\n", content))
				}
			}
		}
	}

	if lang == "en" {
		sb.WriteString("\nRespond in character. Use English. Keep it concise (under 200 words per reply).")
	} else {
		sb.WriteString("\n請以符合你性格的方式回覆用戶。使用繁體中文。保持簡潔（每次回覆不超過 200 字）。")
	}

	return sb.String()
}

// findWorkerReports returns all research reports completed by a worker.
// Safe to call with or without m.mu held — projectStore uses its own lock.
func (m *Manager) findWorkerReports(workerID string) []*project.ResearchReport {
	var results []*project.ResearchReport
	for _, p := range m.projectStore.ListProjects() {
		for _, r := range m.projectStore.ListReports(p.ID) {
			if r.WorkerID == workerID {
				results = append(results, r)
			}
		}
	}
	return results
}
