package company

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

const chatProjectSystemPrompt = `You are a project creation assistant. Your job is to help the user create a software project by extracting the following information from their natural language description:

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

// ChatCreateProject processes a chat conversation and returns either
// the extracted project information or follow-up questions.
func (m *Manager) ChatCreateProject(ctx context.Context, messages []ChatMessage) (*ChatProjectResponse, error) {
	client := anthropic.NewClient(option.WithAPIKey(""))

	// Build messages for the API
	apiMessages := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			apiMessages = append(apiMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case "assistant":
			apiMessages = append(apiMessages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model("claude-sonnet-4-6"),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: chatProjectSystemPrompt},
		},
		Messages: apiMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	text := resp.Content[0].Text

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
