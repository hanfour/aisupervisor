package company

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// createPRDTask builds a PRD task for the project and adds it to the task queue.
func (m *Manager) createPRDTask(p *project.Project) {
	goalsText := ""
	for _, g := range p.Goals {
		goalsText += fmt.Sprintf("- %s\n", g)
	}

	prompt := buildPRDPrompt(p.Name, p.Description, goalsText, p.RepoPath, m.GetLanguage())

	t, err := m.AddTask(p.ID, "PRD: "+p.Name, "Create PRD document with market analysis and requirements", prompt, nil, 1, "", "prd")
	if err != nil {
		log.Printf("WARNING: failed to create PRD task for project %s: %v", p.ID, err)
		return
	}

	m.emit(Event{
		Type:      EventTaskCreated,
		ProjectID: p.ID,
		TaskID:    t.ID,
		Message:   m.msgf("PRD task created for project %q", "已為專案「%s」建立 PRD 任務", p.Name),
	})
}

// buildPRDPrompt generates the prompt for a PRD task.
func buildPRDPrompt(name, description, goals, repoPath, lang string) string {
	if lang == "en" {
		return fmt.Sprintf(`You are a product manager creating a PRD (Product Requirements Document).

Project: %s
Description: %s
Goals:
%s

IMPORTANT: You are working autonomously without a human operator.
- Do NOT ask questions or use AskUserQuestion
- Do NOT invoke brainstorming, writing-plans, or any interactive skills
- Do NOT use EnterPlanMode or ExitPlanMode — execute directly without planning
- Make reasonable decisions on your own and proceed immediately

Your task:
1. **Market Analysis**: Research similar products, identify target users, competitive landscape
2. **Functional Requirements**: Define core features, user stories, acceptance criteria
3. **Non-functional Requirements**: Performance, security, scalability, accessibility
4. **Technical Constraints**: Technology stack, integration points, deployment considerations
5. **Suggested Tasks**: Break down the implementation into actionable development tasks

Output the PRD to docs/prd.md in the repository. Create the docs/ directory if it doesn't exist.

Format the document in Markdown with clear sections. Be thorough but practical.

When done:
1. Commit the docs/prd.md file with message "docs: add PRD for %s"
2. Type /stop to signal completion`, name, description, goals, name)
	}

	return fmt.Sprintf(`你是一位產品經理，負責撰寫 PRD（產品需求文件）。

專案：%s
描述：%s
目標：
%s

重要：你正在自主工作，沒有人類操作員。
- 不要詢問問題、不要使用 AskUserQuestion
- 不要使用 brainstorming、writing-plans 或其他互動式技能
- 不要使用 EnterPlanMode 或 ExitPlanMode — 直接執行，不要進入計劃模式
- 自行做出合理決策並立即開始工作

你的任務：
1. **市場分析**：調查類似產品、識別目標用戶、分析競爭格局
2. **功能需求**：定義核心功能、使用者故事、驗收標準
3. **非功能需求**：效能、安全性、可擴展性、無障礙設計
4. **技術限制**：技術棧、整合點、部署考量
5. **建議任務**：將實作拆分為可執行的開發任務

將 PRD 輸出到倉庫的 docs/prd.md。如果 docs/ 目錄不存在，請建立它。

以 Markdown 格式撰寫，結構清晰。務實且完整。

完成時：
1. 用訊息 "docs: add PRD for %s" 提交 docs/prd.md
2. 輸入 /stop 表示完成`, name, description, goals, name)
}

// handlePRDCompletion processes a completed PRD task.
// Must be called with m.mu held. Releases m.mu before returning.
func (m *Manager) handlePRDCompletion(w *worker.Worker, t *project.Task, p *project.Project) {
	// Record token usage and analytics
	m.recordCompletionMetrics(w, t, true)

	// Mark task as done
	if err := m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone); err != nil {
		log.Printf("failed to update PRD task %s status: %v", t.ID, err)
	}

	m.personalityStore.UpdateProfile(w.ID, func(prof *personality.CharacterProfile) {
		personality.ApplyEvent(prof, personality.EventTaskCompleted)
		personality.UpdateAutoMood(prof)
	})
	m.emit(Event{
		Type:     EventMoodChanged,
		WorkerID: w.ID,
		Message:  fmt.Sprintf("Mood changed for %s", w.Name),
	})

	m.emit(Event{
		Type:      EventPRDCompleted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		WorkerID:  w.ID,
		Message:   m.msgf("PRD completed for project %q — awaiting approval", "專案「%s」的 PRD 已完成，等待核准", p.Name),
	})

	// Create human gate request for PRD approval
	gateReq := m.humanGate.createRequest(HumanGateRequest{
		Reason:   "prd_approval",
		TaskID:   t.ID,
		WorkerID: w.ID,
		Message:  m.msgf("PRD for project %q is ready for review", "專案「%s」的 PRD 已準備好待審核", p.Name),
		Blocking: true,
	})

	// Save gate request ID to task
	t.GateRequestID = gateReq.ID
	if err := m.projectStore.SaveTask(t); err != nil {
		log.Printf("failed to save gate request ID to task %s: %v", t.ID, err)
	}

	// Reset worker to idle
	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	m.saveWorkers()

	m.emit(Event{
		Type:     EventWorkerIdle,
		WorkerID: w.ID,
		Message:  m.msgf("Worker %s is idle", "員工 %s 已閒置", w.Name),
	})

	shouldAutoSchedule := m.autoSchedule
	workerID := w.ID

	m.mu.Unlock()

	if shouldAutoSchedule {
		go m.tryAutoAssign(workerID)
	}
}

// handleDesignCompletion processes a completed design task.
// Must be called with m.mu held. Releases m.mu before returning.
func (m *Manager) handleDesignCompletion(w *worker.Worker, t *project.Task, p *project.Project) {
	// Record token usage and analytics
	m.recordCompletionMetrics(w, t, true)

	// Mark task as done (no gate for design)
	if err := m.projectStore.UpdateTaskStatus(t.ID, project.TaskDone); err != nil {
		log.Printf("failed to update design task %s status: %v", t.ID, err)
	}

	m.personalityStore.UpdateProfile(w.ID, func(prof *personality.CharacterProfile) {
		personality.ApplyEvent(prof, personality.EventTaskCompleted)
		personality.UpdateAutoMood(prof)
	})
	m.emit(Event{
		Type:     EventMoodChanged,
		WorkerID: w.ID,
		Message:  fmt.Sprintf("Mood changed for %s", w.Name),
	})

	m.emit(Event{
		Type:      EventDesignCompleted,
		ProjectID: p.ID,
		TaskID:    t.ID,
		WorkerID:  w.ID,
		Message:   m.msgf("Design task %q completed", "設計任務「%s」已完成", t.Title),
	})

	// Reset worker to idle
	w.Status = worker.WorkerIdle
	w.CurrentTaskID = ""
	m.saveWorkers()

	m.emit(Event{
		Type:     EventWorkerIdle,
		WorkerID: w.ID,
		Message:  m.msgf("Worker %s is idle", "員工 %s 已閒置", w.Name),
	})

	// Promote newly unblocked tasks
	promoted, _ := m.projectStore.PromoteReady(p.ID)
	for _, pt := range promoted {
		m.emit(Event{
			Type:      EventTaskCreated,
			ProjectID: p.ID,
			TaskID:    pt.ID,
			Message:   m.msgf("Task %q is now ready (dependencies resolved)", "任務 %q 已就緒（依賴已解決）", pt.Title),
		})
	}

	shouldAutoSchedule := m.autoSchedule
	workerID := w.ID
	projectID := p.ID

	m.mu.Unlock()

	if shouldAutoSchedule {
		go m.tryAutoAssign(workerID)
	}
	if len(promoted) > 0 {
		go m.engageIdleManagers(context.Background(), projectID)
		go m.drainReadyQueue(context.Background())
	}

	go m.checkProjectCompletion(projectID)
}

// advanceFromPRD reads the PRD document, updates project phase, and decomposes into tasks.
func (m *Manager) advanceFromPRD(prdTaskID string) {
	// Find the task to get project ID
	t, ok := m.projectStore.GetTask(prdTaskID)
	if !ok {
		log.Printf("WARNING: advanceFromPRD: task %s not found", prdTaskID)
		return
	}

	p, ok := m.projectStore.GetProject(t.ProjectID)
	if !ok {
		log.Printf("WARNING: advanceFromPRD: project %s not found", t.ProjectID)
		return
	}

	// Read PRD content
	prdPath := filepath.Join(p.RepoPath, "docs", "prd.md")
	prdData, err := os.ReadFile(prdPath)
	if err != nil {
		log.Printf("WARNING: advanceFromPRD: failed to read %s: %v", prdPath, err)
		// Continue anyway — decompose without PRD content
	}
	prdContent := string(prdData)

	// Update project phase
	m.mu.Lock()
	p.Phase = project.PhaseDevelopment
	m.projectStore.SaveProject(p)
	m.mu.Unlock()

	m.emit(Event{
		Type:      EventPRDApproved,
		ProjectID: p.ID,
		TaskID:    prdTaskID,
		Message:   m.msgf("PRD approved for project %q — entering development phase", "專案「%s」的 PRD 已核准，進入開發階段", p.Name),
	})

	// Decompose from PRD
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := m.DecomposeFromPRD(ctx, p.ID, prdContent); err != nil {
		log.Printf("WARNING: DecomposeFromPRD failed for project %s: %v", p.ID, err)
	}

	// Engage idle workers with newly created tasks
	go m.engageIdleManagers(context.Background(), p.ID)
	go m.drainReadyQueue(context.Background())
}

// DecomposeFromPRD uses AI to break a PRD into actionable tasks.
func (m *Manager) DecomposeFromPRD(ctx context.Context, projectID, prdContent string) error {
	if m.chatProvider == nil {
		return fmt.Errorf("chat provider not configured")
	}

	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return fmt.Errorf("project %q not found", projectID)
	}

	systemPrompt := decomposeFromPRDSystemPrompt(m.GetLanguage())

	userMsg := fmt.Sprintf("專案名稱：%s\n描述：%s\n\n--- PRD 內容 ---\n%s", p.Name, p.Description, prdContent)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMsg},
	}

	text, err := m.chatProvider.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("decompose from PRD: %w", err)
	}

	var result struct {
		Tasks []decomposedTask `json:"tasks"`
	}
	extracted := extractChatJSON(text)
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return fmt.Errorf("failed to parse decomposed tasks from PRD: %w (raw: %s)", err, text)
	}

	for _, dt := range result.Tasks {
		taskType := "code"
		switch dt.Type {
		case "research":
			taskType = "research"
		case "design":
			taskType = "design"
		case "admin":
			taskType = "admin"
		case "hr":
			taskType = "hr"
		}
		if _, err := m.AddTask(projectID, dt.Title, dt.Description, dt.Prompt, nil, dt.Priority, "", taskType); err != nil {
			return fmt.Errorf("failed to add task %q: %w", dt.Title, err)
		}
	}

	m.emit(Event{
		Type:      EventTaskCreated,
		ProjectID: projectID,
		Message:   m.msgf("Auto-generated %d tasks from PRD", "已從 PRD 自動生成 %d 個任務", len(result.Tasks)),
	})

	return nil
}

func decomposeFromPRDSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a project manager who breaks down a PRD into actionable development tasks.

Given a project name, description, and PRD content, create a list of concrete tasks.

Rules:
- Each task should be small and focused (completable in a few hours).
- Include a clear title, description, and a detailed prompt that a developer can directly use.
- Set type to "code" for implementation, "research" for investigation, "design" for UI/UX, "admin" for document/template tasks, or "hr" for hiring/workforce tasks.
- Design tasks should output to docs/design/ directory.
- Priority: 1 = highest, higher numbers = lower priority. Order tasks logically.
- The prompt should be specific enough that an AI coding assistant can execute it.
- Generate 3-15 tasks depending on the project scope.

Respond with valid JSON only:
{"tasks": [{"title": "...", "description": "...", "prompt": "...", "type": "code", "priority": 1}]}`
	}
	return `你是一位專案經理，負責將 PRD 分解為可執行的開發任務。

根據專案名稱、描述和 PRD 內容，建立一份具體任務清單。

規則：
- 每個任務應該小而專注（幾小時內可完成）。
- 包含清楚的標題、描述，以及開發者可以直接使用的詳細 prompt。
- type 設為 "code"（實作）、"research"（調查）、"design"（UI/UX 設計）、"admin"（文件/模板任務）或 "hr"（招募/人力任務）。
- 設計任務應輸出到 docs/design/ 目錄。
- 優先順序：1 = 最高，數字越大優先度越低。按邏輯順序排列任務。
- prompt 要夠具體，讓 AI 程式助手可以直接執行。
- 根據專案規模生成 3-15 個任務。

只用有效的 JSON 回應：
{"tasks": [{"title": "...", "description": "...", "prompt": "...", "type": "code", "priority": 1}]}`
}

// preferredProfiles returns the preferred skill profiles for a given task type.
func preferredProfiles(taskType project.TaskType) []string {
	switch taskType {
	case project.TaskTypePRD:
		return []string{"researcher", "analyst", "architect"}
	case project.TaskTypeDesign:
		return []string{"designer", "architect"}
	case project.TaskTypeCode:
		return []string{"coder", "hacker", "devops"}
	case project.TaskTypeResearch:
		return []string{"researcher", "analyst"}
	case project.TaskTypeAdmin:
		return []string{"assistant"}
	case project.TaskTypeHR:
		return []string{"hr", "analyst"}
	default:
		return nil
	}
}

// tierCompatible checks if a worker tier is appropriate for a task type.
// Managers should not do regular code tasks; engineers should not do management tasks.
func tierCompatible(tier worker.WorkerTier, taskType project.TaskType) bool {
	switch tier {
	case worker.TierManager:
		// Managers handle reviews (via review pipeline), PRD, admin, HR, and delegated code tasks
		// but should not be auto-assigned regular code/research tasks
		return taskType != project.TaskTypeCode && taskType != project.TaskTypeResearch
	case worker.TierConsultant:
		// Consultants can handle any task type
		return true
	default:
		// Engineers handle code, research, design tasks
		return taskType != project.TaskTypePRD && taskType != project.TaskTypeAdmin && taskType != project.TaskTypeHR
	}
}

// workerSkillScore extracts the relevant skill score for a task type from a worker's scores.
func workerSkillScore(scores personality.SkillScores, taskType project.TaskType) int {
	switch taskType {
	case project.TaskTypeCode, project.TaskTypeTraining:
		// Use composite of code quality + carefulness
		return (scores.CodeQuality + scores.Carefulness) / 2
	case project.TaskTypeResearch:
		return scores.CommunicationClarity
	default:
		// Average of all scores for generic tasks
		return (scores.Carefulness + scores.BoundaryChecking + scores.TestCoverageAware +
			scores.CommunicationClarity + scores.CodeQuality + scores.SecurityAwareness) / 6
	}
}

// matchWorker selects the best idle worker ID for a task using skill-aware matching.
// Three-layer strategy:
//  1. Best match: profile match + highest skill score
//  2. Second-best: tier-compatible + highest skill score
//  3. Fallback: any idle tier-compatible worker
//
// Filters by tier compatibility. Returns empty string if no match found.
func matchWorker(t *project.Task, idle []idleWorkerSnapshot, assigned map[string]bool) string {
	preferred := preferredProfiles(t.Type)

	// First pass: profile match + best skill score
	bestID := ""
	bestScore := -1
	for _, w := range idle {
		if assigned[w.ID] || !tierCompatible(w.Tier, t.Type) {
			continue
		}
		profileMatch := false
		for _, pref := range preferred {
			if w.SkillProfile == pref {
				profileMatch = true
				break
			}
		}
		if !profileMatch {
			continue
		}
		score := workerSkillScore(w.SkillScores, t.Type)
		if score > bestScore {
			bestScore = score
			bestID = w.ID
		}
	}
	if bestID != "" {
		return bestID
	}

	// Second pass: tier-compatible + best skill score
	bestID = ""
	bestScore = -1
	for _, w := range idle {
		if assigned[w.ID] || !tierCompatible(w.Tier, t.Type) {
			continue
		}
		score := workerSkillScore(w.SkillScores, t.Type)
		if score > bestScore {
			bestScore = score
			bestID = w.ID
		}
	}
	if bestID != "" {
		return bestID
	}

	return ""
}
