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
	"github.com/hanfourmini/aisupervisor/internal/knowledge"
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

When done, commit the docs/prd.md file with message "docs: add PRD for %s". The system will detect completion automatically once your turn finishes — do NOT type /stop.`, name, description, goals, name)
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

完成時，用訊息 "docs: add PRD for %s" 提交 docs/prd.md。回合結束後系統會自動偵測完成，**不要**輸入 /stop。`, name, description, goals, name)
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
		return `You are a senior project manager breaking down a PRD into actionable development tasks for an autonomous AI worker swarm.

Your output must produce a FULL-STACK task list — not just the easy slice. Workers cannot deliver a working product if entire layers are missing.

# MANDATORY COVERAGE

For every layer the PRD mentions (explicitly or implicitly), produce at least one task. If the PRD does not mention a layer, do NOT skip it silently — instead include exactly one task with title prefixed "out-of-scope: <layer>" and a prompt that says "PRD does not require this layer; no work needed". This makes the omission auditable.

The layers (apply each test against the PRD):

  1. Database / data model — DDL, indexes, RLS, GDPR, audit log if multi-tenant. Output: schema/*.sql
  2. Backend API / services — handlers, models, middleware (auth, rate-limit). Output: backend/, src/api/, internal/
  3. Frontend — pages, components, routing, state, API client; LIFF/SPA build infra. Output: frontend/, web/
  4. Third-party integration — webhook endpoints, OAuth flow, SDK setup, retry/idempotency. Output: integration/, webhooks/
  5. Background jobs / workers / queues — cron, queue consumers, async pipelines. Output: jobs/, workers/
  6. Tests — unit, integration, e2e, fixtures. Output: tests/, *_test.go, *.spec.ts
  7. Documentation — API reference, ops runbook. Output: docs/
  8. Infrastructure / deployment — Dockerfile, compose, CI/CD, env config. Output: infra/, .github/, Dockerfile

# TASK SHAPE

Each task must have:
  - title: imperative, scope-narrow ("Implement LINE webhook signature verification middleware" — not "Handle LINE integration")
  - description: one paragraph stating what is delivered and how it is verified
  - prompt: concrete enough that an AI coding assistant can run it without further questions. Include file paths, key APIs, validation criteria.
  - type: "code", "research", "design", "admin", "hr"
  - priority: 1 (highest) — order topologically: data layer → API → frontend → tests → infra → docs

# QUANTITY

Generate as many tasks as the PRD genuinely requires (typical full-stack SaaS: 25-50). Do NOT cap at 15. Do NOT pad: one file = one task; ten endpoints = ten tasks.

# OUTPUT FORMAT

Respond with valid JSON only — no markdown fences, no commentary:

{"tasks": [{"title": "...", "description": "...", "prompt": "...", "type": "code", "priority": 1}]}`
	}
	return `你是一位資深專案經理，負責將 PRD 分解為可執行的開發任務，給自主 AI worker swarm 執行。

輸出**必須是全棧任務清單** — 不能只挑容易的一層做。整層沒人做的話 worker 永遠交不出可用產品。

# 強制涵蓋

PRD 提到（明示或暗示）的每一層，至少產出一個任務。若 PRD 沒提到某層，**不要無聲跳過** — 改用標題前綴 "out-of-scope: <層>" 並在 prompt 中註明「PRD 未要求此層，本任務無需執行」。這樣遺漏可被稽核。

各層（逐項對照 PRD）：

  1. 資料庫 / 資料模型 — DDL、索引、RLS、GDPR、若多租戶必含 audit log。輸出：schema/*.sql
  2. 後端 API / services — handler、model、middleware（auth、rate-limit）。輸出：backend/、src/api/、internal/
  3. 前端 — 頁面、component、routing、state、API client；LIFF/SPA build 基礎設施。輸出：frontend/、web/
  4. 第三方整合 — webhook endpoint、OAuth flow、SDK 設定、retry/idempotency。輸出：integration/、webhooks/
  5. 背景任務 / workers / queues — cron、queue consumer、async pipeline。輸出：jobs/、workers/
  6. 測試 — 單元、整合、e2e、fixture。輸出：tests/、*_test.go、*.spec.ts
  7. 文件 — API 參考、運維 runbook。輸出：docs/
  8. 基礎設施 / 部署 — Dockerfile、compose、CI/CD、env 設定。輸出：infra/、.github/、Dockerfile

# 任務形狀

每個任務必須有：
  - title：祈使語氣、範圍窄（「實作 LINE webhook 簽章驗證 middleware」，不要「處理 LINE 整合」）
  - description：一段話說明交付內容與驗收方式
  - prompt：足夠具體，AI 程式助手不必再問就能執行。包含檔案路徑、關鍵 API、驗收標準
  - type："code"（實作）、"research"（調查）、"design"（UI/UX 或架構設計）、"admin"（文件/模板）、"hr"（招募）
  - priority：1 = 最高，按拓撲順序：資料層 → API → 前端 → 測試 → 基礎設施 → 文件

# 數量

依 PRD 實際需求生成（典型全棧 SaaS：25-50 個任務）。**不要硬限 15 個**。也不要灌水：一檔案一個 task；10 個 endpoint 就 10 個 task。

# 輸出格式

只用有效 JSON 回應 — 不要 markdown 圍欄、不要註解：

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
func matchWorker(t *project.Task, idle []idleWorkerSnapshot, assigned map[string]bool, graph ...*knowledge.CodeGraph) string {
	// Community preference: if graph is available, prefer worker in same community
	if len(graph) > 0 && graph[0] != nil {
		communityMatch := findBestWorkerForCommunity(t, idle, graph[0], assigned)
		if communityMatch != "" {
			return communityMatch
		}
	}

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

// findBestWorkerForCommunity returns the idle worker whose last community matches
// the task's target community. Returns "" if no match found or graph is nil.
func findBestWorkerForCommunity(task *project.Task, idle []idleWorkerSnapshot, graph *knowledge.CodeGraph, assigned map[string]bool) string {
	if graph == nil || len(task.Files) == 0 {
		return ""
	}

	// Determine the task's community from its first file
	taskCommunity := knowledge.GetCommunityForFile(graph, task.Files[0])
	if taskCommunity == nil {
		return ""
	}

	for _, w := range idle {
		if assigned[w.ID] {
			continue
		}
		if w.LastCommunityID == taskCommunity.ID {
			return w.ID
		}
	}
	return ""
}
