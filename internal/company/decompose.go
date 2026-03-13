package company

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// decomposedTask represents a single task extracted by the AI.
type decomposedTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Type        string `json:"type"` // "code" or "research"
	Priority    int    `json:"priority"`
}

func decomposeSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a project manager who breaks down project goals into actionable development tasks.

Given a project name, description, and goals, create a list of concrete tasks that developers can work on.

Rules:
- Each task should be small and focused (completable in a few hours).
- Include a clear title, description, and a detailed prompt that a developer can directly use.
- Set type to "code" for implementation tasks or "research" for investigation/design tasks.
- Priority: 1 = highest, higher numbers = lower priority. Order tasks logically.
- The prompt should be specific enough that an AI coding assistant can execute it.
- Generate 3-8 tasks depending on the project scope.

Respond with valid JSON only:
{"tasks": [{"title": "...", "description": "...", "prompt": "...", "type": "code", "priority": 1}]}`
	}
	return `你是一位專案經理，負責將專案目標分解為可執行的開發任務。

根據專案名稱、描述和目標，建立一份開發者可以直接執行的具體任務清單。

規則：
- 每個任務應該小而專注（幾小時內可完成）。
- 包含清楚的標題、描述，以及開發者可以直接使用的詳細 prompt。
- type 設為 "code"（實作任務）或 "research"（調查/設計任務）。
- 優先順序：1 = 最高，數字越大優先度越低。按邏輯順序排列任務。
- prompt 要夠具體，讓 AI 程式助手可以直接執行。
- 根據專案規模生成 3-8 個任務。

只用有效的 JSON 回應：
{"tasks": [{"title": "...", "description": "...", "prompt": "...", "type": "code", "priority": 1}]}`
}

// decomposedProject represents a project extracted from an objective decomposition.
type decomposedProject struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Goals       []string `json:"goals"`
}

func decomposeObjectiveSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a strategic planner who breaks down company objectives into concrete projects.

Given an objective title, description, and key results, create a list of projects that will achieve the objective.

Rules:
- Each project should be a coherent unit of work with clear deliverables.
- Include a name, description, and 2-4 goals per project.
- Goals should map back to the key results of the objective.
- Generate 2-5 projects depending on the objective scope.

Respond with valid JSON only:
{"projects": [{"name": "...", "description": "...", "goals": ["...", "..."]}]}`
	}
	return `你是一位策略規劃師，負責將公司目標分解為具體專案。

根據目標標題、描述和關鍵結果，建立一份能達成目標的專案清單。

規則：
- 每個專案應該是有明確交付物的工作單元。
- 包含名稱、描述，以及每個專案 2-4 個目標。
- 目標應對應到公司目標的關鍵結果。
- 根據目標規模生成 2-5 個專案。

只用有效的 JSON 回應：
{"projects": [{"name": "...", "description": "...", "goals": ["...", "..."]}]}`
}

// DecomposeObjective uses AI to break an objective into projects and creates them.
func (m *Manager) DecomposeObjective(ctx context.Context, objectiveID string) ([]string, error) {
	if m.chatProvider == nil {
		return nil, fmt.Errorf("chat provider not configured")
	}

	obj, ok := m.GetObjective(objectiveID)
	if !ok {
		return nil, fmt.Errorf("objective %q not found", objectiveID)
	}

	userMsg := fmt.Sprintf("目標標題：%s\n描述：%s\n", obj.Title, obj.Description)
	if len(obj.KeyResults) > 0 {
		userMsg += "關鍵結果：\n"
		for _, kr := range obj.KeyResults {
			userMsg += fmt.Sprintf("- %s (目標: %.0f %s)\n", kr.Title, kr.Target, kr.Unit)
		}
	}

	messages := []ai.ChatMessage{
		{Role: "system", Content: decomposeObjectiveSystemPrompt(m.GetLanguage())},
		{Role: "user", Content: userMsg},
	}

	text, err := m.chatProvider.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("decompose objective: %w", err)
	}

	var result struct {
		Projects []decomposedProject `json:"projects"`
	}
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("failed to parse decomposed projects: %w (raw: %s)", err, text)
	}

	var projectIDs []string
	for _, dp := range result.Projects {
		p, err := m.CreateProject(dp.Name, dp.Description, "", "", dp.Goals)
		if err != nil {
			return projectIDs, fmt.Errorf("failed to create project %q: %w", dp.Name, err)
		}
		projectIDs = append(projectIDs, p.ID)
		if err := m.LinkProjectToObjective(objectiveID, p.ID); err != nil {
			return projectIDs, fmt.Errorf("failed to link project: %w", err)
		}
	}

	m.emit(Event{
		Type:    EventObjectiveCreated,
		Message: m.msgf("Decomposed objective into %d projects", "已將目標拆解為 %d 個專案", len(result.Projects)),
	})

	return projectIDs, nil
}

// DecomposeGoals uses AI to break project goals into actionable tasks and adds them to the project.
func (m *Manager) DecomposeGoals(ctx context.Context, projectID string) error {
	if m.chatProvider == nil {
		return fmt.Errorf("chat provider not configured")
	}

	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return fmt.Errorf("project %q not found", projectID)
	}

	if len(p.Goals) == 0 {
		return nil // nothing to decompose
	}

	userMsg := fmt.Sprintf("專案名稱：%s\n描述：%s\n目標：\n", p.Name, p.Description)
	for _, g := range p.Goals {
		userMsg += fmt.Sprintf("- %s\n", g)
	}

	messages := []ai.ChatMessage{
		{Role: "system", Content: decomposeSystemPrompt(m.GetLanguage())},
		{Role: "user", Content: userMsg},
	}

	text, err := m.chatProvider.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("decompose goals: %w", err)
	}

	// Parse response
	var result struct {
		Tasks []decomposedTask `json:"tasks"`
	}
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return fmt.Errorf("failed to parse decomposed tasks: %w (raw: %s)", err, text)
	}

	// Create tasks
	for _, dt := range result.Tasks {
		taskType := "code"
		if dt.Type == "research" {
			taskType = "research"
		}
		if _, err := m.AddTask(projectID, dt.Title, dt.Description, dt.Prompt, nil, dt.Priority, "", taskType); err != nil {
			return fmt.Errorf("failed to add task %q: %w", dt.Title, err)
		}
	}

	m.emit(Event{
		Type:      EventTaskCreated,
		ProjectID: projectID,
		Message:   m.msgf("Auto-generated %d tasks from project goals", "已從專案目標自動生成 %d 個任務", len(result.Tasks)),
	})

	return nil
}
