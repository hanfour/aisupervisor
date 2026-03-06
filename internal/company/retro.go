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
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"gopkg.in/yaml.v3"
)

// RetroData holds collected data for a project retrospective.
type RetroData struct {
	ProjectID   string            `json:"projectId"`
	ProjectName string            `json:"projectName"`
	Workers     []WorkerRetroData `json:"workers"`
	TotalTasks  int               `json:"totalTasks"`
	CompletedAt time.Time         `json:"completedAt"`
}

// WorkerRetroData holds per-worker data for the retro.
type WorkerRetroData struct {
	WorkerID         string                  `json:"workerId"`
	WorkerName       string                  `json:"workerName"`
	SkillProfileID   string                  `json:"skillProfileId"`
	TasksCompleted   int                     `json:"tasksCompleted"`
	TasksFailed      int                     `json:"tasksFailed"`
	ReviewCount      int                     `json:"reviewCount"`
	RejectionCount   int                     `json:"rejectionCount"`
	ApprovalRate     float64                 `json:"approvalRate"`
	RejectionReasons []string                `json:"rejectionReasons"`
	SkillScores      personality.SkillScores `json:"skillScores"`
}

// RetroResult is the LLM-generated retro analysis.
type RetroResult struct {
	Summary          string                   `json:"summary" yaml:"summary"`
	WorkerFeedback   []WorkerFeedback         `json:"workerFeedback" yaml:"worker_feedback"`
	SkillAdjustments []SkillProfileAdjustment `json:"skillAdjustments" yaml:"skill_adjustments"`
}

// WorkerFeedback provides per-worker analysis.
type WorkerFeedback struct {
	WorkerID    string   `json:"workerId" yaml:"worker_id"`
	Strengths   []string `json:"strengths" yaml:"strengths"`
	Weaknesses  []string `json:"weaknesses" yaml:"weaknesses"`
	Suggestions []string `json:"suggestions" yaml:"suggestions"`
}

// SkillProfileAdjustment recommends changes to a worker's skill profile.
type SkillProfileAdjustment struct {
	WorkerID        string   `json:"workerId" yaml:"worker_id"`
	ProfileID       string   `json:"profileId" yaml:"profile_id"`
	PromptAdditions []string `json:"promptAdditions,omitempty" yaml:"prompt_additions,omitempty"`
	PromptRemovals  []string `json:"promptRemovals,omitempty" yaml:"prompt_removals,omitempty"`
	AddTools        []string `json:"addTools,omitempty" yaml:"add_tools,omitempty"`
	RemoveTools     []string `json:"removeTools,omitempty" yaml:"remove_tools,omitempty"`
	ModelChange     string   `json:"modelChange,omitempty" yaml:"model_change,omitempty"`
}

// RetroReport is the persisted retro result.
type RetroReport struct {
	ID          string      `yaml:"id" json:"id"`
	ProjectID   string      `yaml:"project_id" json:"projectId"`
	ProjectName string      `yaml:"project_name" json:"projectName"`
	Result      RetroResult `yaml:"result" json:"result"`
	AppliedAt   time.Time   `yaml:"applied_at" json:"appliedAt"`
}

type retrosFile struct {
	Retros []RetroReport `yaml:"retros"`
}

// collectRetroData gathers worker performance data for a completed project.
func (m *Manager) collectRetroData(projectID string) (*RetroData, error) {
	p, ok := m.projectStore.GetProject(projectID)
	if !ok {
		return nil, fmt.Errorf("project %q not found", projectID)
	}

	tasks := m.projectStore.TasksForProject(projectID)

	// Find workers who participated (assigned to at least one task)
	workerTaskMap := make(map[string][]*project.Task)
	for _, t := range tasks {
		if t.AssigneeID != "" && t.ParentTaskID == "" {
			workerTaskMap[t.AssigneeID] = append(workerTaskMap[t.AssigneeID], t)
		}
	}

	var workerData []WorkerRetroData
	m.mu.RLock()
	for wID, wTasks := range workerTaskMap {
		w, ok := m.workers[wID]
		if !ok {
			continue
		}

		wd := WorkerRetroData{
			WorkerID:       wID,
			WorkerName:     w.Name,
			SkillProfileID: w.SkillProfile,
		}

		for _, t := range wTasks {
			if t.Status == project.TaskDone {
				wd.TasksCompleted++
			} else if t.Status == project.TaskFailed {
				wd.TasksFailed++
			}
			wd.ReviewCount += t.ReviewCount
			wd.RejectionCount += t.RejectionCount
			for _, r := range t.RejectionHistory {
				if len(r.Reason) > 200 {
					wd.RejectionReasons = append(wd.RejectionReasons, r.Reason[:200])
				} else {
					wd.RejectionReasons = append(wd.RejectionReasons, r.Reason)
				}
			}
		}

		total := wd.TasksCompleted + wd.TasksFailed
		if total > 0 && wd.ReviewCount > 0 {
			wd.ApprovalRate = float64(wd.ReviewCount-wd.RejectionCount) / float64(wd.ReviewCount) * 100
		}

		// Get skill scores from personality
		if m.personalityStore != nil {
			if profile := m.personalityStore.GetProfile(wID); profile != nil {
				wd.SkillScores = profile.SkillScores
			}
		}

		workerData = append(workerData, wd)
	}
	m.mu.RUnlock()

	return &RetroData{
		ProjectID:   projectID,
		ProjectName: p.Name,
		Workers:     workerData,
		TotalTasks:  len(tasks),
		CompletedAt: time.Now(),
	}, nil
}

// RunRetro executes a retrospective for the given project.
func (m *Manager) RunRetro(ctx context.Context, projectID string) error {
	if m.chatProvider == nil {
		return fmt.Errorf("chat provider not configured")
	}

	m.emit(Event{
		Type:      EventRetroStarted,
		ProjectID: projectID,
		Message:   m.msgf("Retro started for project %s", "專案 %s 的回顧已開始", projectID),
	})

	data, err := m.collectRetroData(projectID)
	if err != nil {
		return fmt.Errorf("collecting retro data: %w", err)
	}

	if len(data.Workers) == 0 {
		log.Printf("retro: no workers participated in project %s, skipping", projectID)
		return nil
	}

	result, err := m.runRetroAnalysis(ctx, data)
	if err != nil {
		return fmt.Errorf("running retro analysis: %w", err)
	}

	if err := m.applyRetroResult(result); err != nil {
		log.Printf("WARNING: applying retro result failed: %v", err)
		// Don't return error — still save the report
	}

	if err := m.saveRetroReport(projectID, data.ProjectName, result); err != nil {
		log.Printf("WARNING: saving retro report failed: %v", err)
	}

	m.emit(Event{
		Type:      EventRetroCompleted,
		ProjectID: projectID,
		Message:   m.msgf("Retro completed for project %s", "專案 %s 的回顧已完成", projectID),
	})

	return nil
}

// runRetroAnalysis calls the LLM to analyze retro data.
func (m *Manager) runRetroAnalysis(ctx context.Context, data *RetroData) (*RetroResult, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling retro data: %w", err)
	}

	systemPrompt := `You are a software engineering retrospective facilitator. Analyze the project performance data and provide actionable feedback.

Output ONLY valid JSON with this exact structure:
{
  "summary": "Brief overall project summary (2-3 sentences)",
  "workerFeedback": [
    {
      "workerId": "worker-id",
      "strengths": ["strength1", "strength2"],
      "weaknesses": ["weakness1"],
      "suggestions": ["actionable suggestion1"]
    }
  ],
  "skillAdjustments": [
    {
      "workerId": "worker-id",
      "profileId": "current-profile-id",
      "promptAdditions": ["additional instruction to add to system prompt"],
      "promptRemovals": ["instruction to remove if present"],
      "addTools": ["ToolName"],
      "removeTools": ["ToolName"],
      "modelChange": "opus or sonnet (only if needed)"
    }
  ]
}

Guidelines:
- Focus on patterns, not individual incidents
- Prompt additions should be concise, actionable instructions (max 1-2 sentences each)
- Only suggest model changes for workers with consistently low quality (approval rate < 50%)
- Tool changes should be specific and justified
- If a worker performed well (high approval rate, few rejections), minimal or no adjustments needed
- Keep skillAdjustments empty for workers who don't need changes`

	userPrompt := fmt.Sprintf("Analyze this project retrospective data and provide structured feedback:\n\n%s", string(dataJSON))

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	response, err := m.chatProvider.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM chat: %w", err)
	}

	// Extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in LLM response")
	}

	var result RetroResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parsing retro result JSON: %w", err)
	}

	return &result, nil
}

// applyRetroResult converts retro adjustments into per-worker skill overrides and saves to config.
func (m *Manager) applyRetroResult(result *RetroResult) error {
	if len(result.SkillAdjustments) == 0 {
		return nil
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.WorkerSkillOverrides == nil {
		cfg.WorkerSkillOverrides = make(map[string]config.SkillProfileOverride)
	}

	for _, adj := range result.SkillAdjustments {
		existing := cfg.WorkerSkillOverrides[adj.WorkerID]

		// Merge prompt additions
		if len(adj.PromptAdditions) > 0 {
			for _, addition := range adj.PromptAdditions {
				if existing.ExtraPrompt != "" {
					existing.ExtraPrompt += "\n" + addition
				} else {
					existing.ExtraPrompt = addition
				}
			}
		}

		// Merge tool additions
		if len(adj.AddTools) > 0 {
			existing.AddTools = mergeStringSlice(existing.AddTools, adj.AddTools)
		}

		// Merge tool removals
		if len(adj.RemoveTools) > 0 {
			existing.RemoveTools = mergeStringSlice(existing.RemoveTools, adj.RemoveTools)
		}

		// Model change (last one wins)
		if adj.ModelChange != "" {
			existing.ModelOverride = adj.ModelChange
		}

		cfg.WorkerSkillOverrides[adj.WorkerID] = existing
	}

	if err := cfg.Save(""); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// Reload overrides into spawner
	if m.spawner != nil {
		m.spawner.LoadSkillOverrides(cfg.WorkerSkillOverrides)
	}

	return nil
}

// saveRetroReport persists the retro report to retros.yaml.
func (m *Manager) saveRetroReport(projectID, projectName string, result *RetroResult) error {
	dataDir := filepath.Dir(m.workersPath)
	retrosPath := filepath.Join(dataDir, "retros.yaml")

	var rf retrosFile
	data, err := os.ReadFile(retrosPath)
	if err == nil {
		yaml.Unmarshal(data, &rf)
	}

	report := RetroReport{
		ID:          fmt.Sprintf("retro-%d", time.Now().UnixMilli()),
		ProjectID:   projectID,
		ProjectName: projectName,
		Result:      *result,
		AppliedAt:   time.Now(),
	}

	rf.Retros = append(rf.Retros, report)

	out, err := yaml.Marshal(&rf)
	if err != nil {
		return err
	}
	return os.WriteFile(retrosPath, out, 0o644)
}

// LoadRetroReports reads all retro reports from disk.
func (m *Manager) LoadRetroReports() []RetroReport {
	dataDir := filepath.Dir(m.workersPath)
	retrosPath := filepath.Join(dataDir, "retros.yaml")

	data, err := os.ReadFile(retrosPath)
	if err != nil {
		return nil
	}

	var rf retrosFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil
	}
	return rf.Retros
}

// GetRetroReport returns a single retro report by ID.
func (m *Manager) GetRetroReport(id string) *RetroReport {
	reports := m.LoadRetroReports()
	for _, r := range reports {
		if r.ID == id {
			return &r
		}
	}
	return nil
}

// GetWorkerSkillOverride returns the skill override for a worker from config.
func (m *Manager) GetWorkerSkillOverride(workerID string) *config.SkillProfileOverride {
	cfg, err := config.Load("")
	if err != nil {
		return nil
	}
	if override, ok := cfg.WorkerSkillOverrides[workerID]; ok {
		return &override
	}
	return nil
}

// mergeStringSlice appends items from b to a, skipping duplicates.
func mergeStringSlice(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			a = append(a, s)
			set[s] = true
		}
	}
	return a
}
