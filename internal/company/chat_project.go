package company

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// ChatMessage represents a single message in the chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// ChatProjectResponse is the structured response from the AI.
type ChatProjectResponse struct {
	Status      string   `json:"status"` // "ready" or "needs_info"
	Message     string   `json:"message,omitempty"`
	Questions   []string `json:"questions,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	RepoPath    string   `json:"repoPath,omitempty"`
	BaseBranch  string   `json:"baseBranch,omitempty"`
	Goals       []string `json:"goals,omitempty"`
}

func chatProjectSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a friendly and enthusiastic project creation assistant. Chat naturally with the user to understand what they want to build. Be warm, curious, and collaborative — like a helpful colleague brainstorming together.

Your goal is to gather enough info to create a project:
- name: A short project name
- description: What the project is about
- repoPath: The filesystem path to the repository
- baseBranch: The git branch (default "main")
- goals: What they want to achieve

Guidelines:
- Chat naturally! Show genuine interest in their idea. Share brief thoughts or suggestions.
- Don't interrogate — weave questions into natural conversation. Ask 1-2 things at a time, not a checklist.
- If the user gives a vague idea, help them flesh it out. Suggest concrete goals based on what they described.
- If you can reasonably infer information (e.g., goals from context), do so — don't ask for things you can figure out.
- When you have enough info, set status to "ready" and include a brief excited summary in "message".

Always respond with valid JSON only:
{
  "status": "ready" | "needs_info",
  "message": "Your natural conversational response here",
  "name": "project name",
  "description": "project description",
  "repoPath": "/path/to/repo",
  "baseBranch": "main",
  "goals": ["goal1", "goal2"]
}`
	}
	return `你是一個友善且充滿熱情的專案建立助手。用自然的方式和使用者聊天，了解他們想做什麼。像一個好同事一樣，熱心、好奇、一起腦力激盪。

你的目標是收集足夠的資訊來建立專案：
- name：簡短的專案名稱
- description：專案描述
- repoPath：儲存庫的檔案路徑
- baseBranch：git 分支（預設 "main"）
- goals：想達成的目標

指引：
- 自然地聊天！對使用者的想法展現真誠的興趣，分享你的想法和建議。
- 不要像問卷一樣逐題問，把問題融入自然對話中。一次問 1-2 個重點就好。
- 如果使用者給了模糊的想法，幫他們發想、補充。根據他們說的內容主動建議具體目標。
- 如果你能合理推斷資訊（例如從上下文推出目標），就直接推斷，不要問已經能推測的事。
- 當你收集到足夠資訊時，將 status 設為 "ready"，在 "message" 中寫一段簡短的總結。

始終只用有效的 JSON 回應：
{
  "status": "ready" | "needs_info",
  "message": "你的自然對話回覆寫在這裡",
  "name": "專案名稱",
  "description": "專案描述",
  "repoPath": "/path/to/repo",
  "baseBranch": "main",
  "goals": ["目標1", "目標2"]
}`
}

// ChatCreateProject processes a chat conversation and returns either
// the extracted project information or follow-up questions.
func (m *Manager) ChatCreateProject(ctx context.Context, messages []ChatMessage) (*ChatProjectResponse, error) {
	if m.chatProvider == nil {
		return nil, fmt.Errorf("chat provider not configured")
	}

	// Build chat messages
	chatMessages := make([]ai.ChatMessage, 0, len(messages)+1)
	chatMessages = append(chatMessages, ai.ChatMessage{Role: "system", Content: chatProjectSystemPrompt(m.GetLanguage())})
	for _, msg := range messages {
		chatMessages = append(chatMessages, ai.ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	text, err := m.chatProvider.Chat(ctx, chatMessages)
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}

	if text == "" {
		return nil, fmt.Errorf("empty response from chat provider")
	}

	// Parse the JSON response
	var result ChatProjectResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		// Try to extract JSON from the response
		extracted := extractChatJSON(text)
		if err2 := json.Unmarshal([]byte(extracted), &result); err2 != nil {
			// Return the raw text as a question if parsing fails
			return &ChatProjectResponse{
				Status:    "needs_info",
				Questions: []string{text},
			}, nil
		}
	}

	// Set default base branch if ready but not specified
	if result.Status == "ready" && result.BaseBranch == "" {
		result.BaseBranch = "main"
	}

	return &result, nil
}

// extractChatJSON extracts a JSON object from text that might contain markdown or extra content.
func extractChatJSON(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return text
	}

	depth := 0
	inString := false
	escape := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return text[start:]
}
