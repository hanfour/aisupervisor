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
	Questions   []string `json:"questions,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	RepoPath    string   `json:"repoPath,omitempty"`
	BaseBranch  string   `json:"baseBranch,omitempty"`
	Goals       []string `json:"goals,omitempty"`
}

func chatProjectSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a project creation assistant. Your job is to help the user create a software project by extracting the following information from their natural language description:

1. **name** (required): A short project name
2. **description** (required): A brief description of the project
3. **repoPath** (required): The filesystem path to the repository
4. **baseBranch** (optional, default "main"): The base git branch
5. **goals** (optional): A list of project goals

## Rules:
- If you have enough information to create the project, respond with status "ready" and fill in all fields.
- If critical information is missing (especially name, description, or repoPath), respond with status "needs_info" and ask specific questions.
- Be conversational and helpful. Ask one round of questions at a time.
- Always respond with valid JSON only, no markdown or extra text.

## Response format (JSON only):
{
  "status": "ready" | "needs_info",
  "questions": ["question1", "question2"],
  "name": "project name",
  "description": "project description",
  "repoPath": "/path/to/repo",
  "baseBranch": "main",
  "goals": ["goal1", "goal2"]
}`
	}
	return `你是一個專案建立助手。你的工作是從使用者的自然語言描述中提取以下資訊來建立軟體專案：

1. **name**（必填）：簡短的專案名稱
2. **description**（必填）：專案的簡要描述
3. **repoPath**（必填）：儲存庫的檔案系統路徑
4. **baseBranch**（選填，預設 "main"）：基礎 git 分支
5. **goals**（選填）：專案目標列表

## 規則：
- 如果你有足夠的資訊來建立專案，請以 status "ready" 回應並填入所有欄位。
- 如果缺少關鍵資訊（特別是 name、description 或 repoPath），請以 status "needs_info" 回應並提出具體問題。
- 保持對話式和友善的風格。一次只問一輪問題。
- 始終只用有效的 JSON 回應，不要加 markdown 或額外文字。

## 回應格式（僅限 JSON）：
{
  "status": "ready" | "needs_info",
  "questions": ["問題1", "問題2"],
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
	for i := start; i < len(text); i++ {
		switch text[i] {
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
