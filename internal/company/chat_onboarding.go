package company

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/claudecli"
)

// debugLog writes to ~/aisupervisor-debug.log for troubleshooting.
func debugLog(format string, args ...interface{}) {
	home, _ := os.UserHomeDir()
	if home == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(home, "aisupervisor-debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "%s\n", msg)
}

// ChatOnboardingResponse is the structured response from the onboarding AI assistant.
type ChatOnboardingResponse struct {
	Status  string             `json:"status"`  // "chatting" | "hire_hr" | "ready" | "need_api_key"
	Message string             `json:"message"`
	HRName  string             `json:"hrName,omitempty"`  // populated when status="hire_hr"
	Workers []OnboardingWorker `json:"workers,omitempty"` // populated when status="ready"
}

// OnboardingWorker describes a recommended worker from the onboarding chat.
type OnboardingWorker struct {
	Name         string `json:"name"`
	SkillProfile string `json:"skillProfile"`
	Tier         string `json:"tier"`
	Gender       string `json:"gender"`
}

func chatOnboardingSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a friendly onboarding assistant for AI Supervisor, a virtual office app that manages AI workers.

Your job: chat with the user to understand what kind of AI team they need, then help them set it up.

## Flow
1. Introduce yourself warmly. Ask what they want their AI team to do (software project, research, etc.) and how big a team they want.
2. After 1-2 exchanges, when you understand their needs, recommend hiring an HR specialist. Set status to "hire_hr" and include a suggested HR name and gender.
3. After HR is "hired", switch to speaking as the HR specialist. Recommend a complete team composition based on the user's needs. Always include the HR worker in the team list.
4. When the team is finalized, set status to "ready" with the full workers array.

## Available skill profiles
- coder: Software engineer (writes code)
- architect: System architect (designs systems, reviews code)
- qa: QA engineer (testing, quality assurance)
- security: Security specialist
- devops: DevOps engineer (CI/CD, infrastructure)
- designer: UI/UX designer
- analyst: Business/data analyst
- reviewer: Code reviewer / HR specialist

## Available tiers
- engineer: Regular worker
- manager: Team lead / manager
- consultant: Senior advisor

## Response format
Always respond with valid JSON only:
{
  "status": "chatting" | "hire_hr" | "ready",
  "message": "Your natural conversational response",
  "hrName": "Name (only when status=hire_hr)",
  "workers": [
    {"name": "Name", "skillProfile": "profile_id", "tier": "tier", "gender": "male|female"}
  ]
}

Notes:
- workers array only needed when status="ready"
- hrName only needed when status="hire_hr"
- "hire_hr" can only be used ONCE! After that, use "chatting" or "ready"
- Keep messages warm, brief, and encouraging
- Suggest 2-6 workers depending on user needs (including the HR)
- Give workers natural first names
- Mix genders naturally`
	}
	return `你是 AI Supervisor 的友善引導助理。AI Supervisor 是一個管理 AI 員工的虛擬辦公室 App。

你的任務：透過聊天了解使用者需要什麼樣的 AI 團隊，然後幫他們建立。

## 流程
1. 熱情地自我介紹。問使用者想讓 AI 團隊做什麼（軟體專案、研究等）以及希望多大的團隊。
2. 經過 1-2 輪對話，了解需求後，建議招募一位 HR 專員。將 status 設為 "hire_hr"，附上建議的 HR 名字和性別。
3. HR「到任」後，以 HR 專員的身份說話。根據使用者需求推薦完整的團隊配置。團隊列表中一定要包含 HR 自己。
4. 團隊確定後，將 status 設為 "ready"，附上完整的 workers 陣列。

## 可用的技能配置
- coder：軟體工程師（寫程式）
- architect：系統架構師（設計系統、審查程式碼）
- qa：QA 工程師（測試、品質保證）
- security：安全專家
- devops：DevOps 工程師（CI/CD、基礎設施）
- designer：UI/UX 設計師
- analyst：商業/資料分析師
- reviewer：程式碼審查員 / HR 專員

## 可用的等級
- engineer：一般員工
- manager：團隊主管 / 管理員
- consultant：資深顧問

## 回應格式
始終只用有效的 JSON 回應：
{
  "status": "chatting" | "hire_hr" | "ready",
  "message": "你的自然對話回覆",
  "hrName": "名字（僅在 status=hire_hr 時）",
  "workers": [
    {"name": "名字", "skillProfile": "profile_id", "tier": "tier", "gender": "male|female"}
  ]
}

注意：
- workers 陣列只在 status="ready" 時需要
- hrName 只在 status="hire_hr" 時需要
- "hire_hr" 只能使用一次！之後的回覆請用 "chatting" 或 "ready"
- 訊息要溫暖、簡短、有鼓勵性
- 根據使用者需求建議 2-6 位員工（包含 HR）
- 給員工取自然的名字
- 自然地混合性別`
}

// ChatOnboarding processes an onboarding conversation and returns the assistant's response.
// Priority: 1) configured chatProvider, 2) Claude CLI, 3) return "need_api_key" status.
func (m *Manager) ChatOnboarding(ctx context.Context, messages []ChatMessage) (*ChatOnboardingResponse, error) {
	debugLog("ChatOnboarding called with %d messages", len(messages))

	provider := m.resolveOnboardingProvider()
	if provider == nil {
		debugLog("no provider found, returning need_api_key")
		return m.needAPIKeyResponse(), nil
	}

	chatMessages := make([]ai.ChatMessage, 0, len(messages)+1)
	chatMessages = append(chatMessages, ai.ChatMessage{Role: "system", Content: chatOnboardingSystemPrompt(m.GetLanguage())})
	for _, msg := range messages {
		chatMessages = append(chatMessages, ai.ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	debugLog("calling provider.Chat with %d messages", len(chatMessages))
	text, err := provider.Chat(ctx, chatMessages)

	// If the configured provider fails, try Claude CLI as fallback
	if err != nil || text == "" {
		debugLog("provider.Chat failed (err=%v, empty=%v), trying Claude CLI fallback", err, text == "")
		cli := claudecli.New()
		if cli != nil {
			debugLog("fallback: found Claude CLI at %s", cli.Path())
			text, err = cli.Chat(ctx, chatMessages)
			if err != nil {
				debugLog("fallback Claude CLI also failed: %v", err)
				return m.needAPIKeyResponse(), nil
			}
		} else {
			debugLog("fallback: Claude CLI not found either")
			return m.needAPIKeyResponse(), nil
		}
	}

	debugLog("chat response: %d bytes", len(text))
	if text == "" {
		return m.needAPIKeyResponse(), nil
	}

	var result ChatOnboardingResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		extracted := extractChatJSON(text)
		if err2 := json.Unmarshal([]byte(extracted), &result); err2 != nil {
			return &ChatOnboardingResponse{
				Status:  "chatting",
				Message: text,
			}, nil
		}
	}

	return &result, nil
}

// resolveOnboardingProvider returns the best available ChatProvider:
// 1) configured chatProvider, 2) Claude CLI if available.
func (m *Manager) resolveOnboardingProvider() ai.ChatProvider {
	if m.chatProvider != nil {
		debugLog("resolveOnboardingProvider: using configured chatProvider")
		return m.chatProvider
	}
	debugLog("resolveOnboardingProvider: no chatProvider, trying Claude CLI")
	debugLog("resolveOnboardingProvider: HOME=%s PATH=%s", os.Getenv("HOME"), os.Getenv("PATH"))
	cli := claudecli.New()
	if cli != nil {
		debugLog("resolveOnboardingProvider: found Claude CLI at %s", cli.Path())
		return cli
	}
	debugLog("resolveOnboardingProvider: Claude CLI not found")
	return nil
}

func (m *Manager) needAPIKeyResponse() *ChatOnboardingResponse {
	lang := m.GetLanguage()
	if lang == "en" {
		return &ChatOnboardingResponse{
			Status:  "need_api_key",
			Message: "To get started, I need an AI backend. Please enter an API key below.",
		}
	}
	return &ChatOnboardingResponse{
		Status:  "need_api_key",
		Message: "要開始使用，我需要一個 AI 後端。請在下方輸入 API 金鑰。",
	}
}
