package gui

import (
	"time"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

type ProjectDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RepoPath    string   `json:"repoPath"`
	BaseBranch  string   `json:"baseBranch"`
	Goals       []string `json:"goals"`
	Phase       string   `json:"phase,omitempty"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

func ProjectToDTO(p *project.Project) ProjectDTO {
	return ProjectDTO{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		RepoPath:    p.RepoPath,
		BaseBranch:  p.BaseBranch,
		Goals:       p.Goals,
		Phase:       string(p.Phase),
		Status:      string(p.Status),
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}

type TaskDTO struct {
	ID               string           `json:"id"`
	ProjectID        string           `json:"projectId"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	Prompt           string           `json:"prompt"`
	Type             string           `json:"type"`
	Status           string           `json:"status"`
	Priority         int              `json:"priority"`
	BranchName       string           `json:"branchName"`
	AssigneeID       string           `json:"assigneeId"`
	DependsOn        []string         `json:"dependsOn"`
	Milestone        string           `json:"milestone"`
	ReviewerID       string           `json:"reviewerId,omitempty"`
	ParentTaskID     string           `json:"parentTaskId,omitempty"`
	ReviewCount      int              `json:"reviewCount,omitempty"`
	RejectionCount   int              `json:"rejectionCount,omitempty"`
	RejectionHistory []RejectionDTO   `json:"rejectionHistory,omitempty"`
	BounceHistory    []BounceRecordDTO `json:"bounceHistory,omitempty"`
	CreatedAt        string           `json:"createdAt"`
	StartedAt        string           `json:"startedAt,omitempty"`
	CompletedAt      string           `json:"completedAt,omitempty"`
}

type RejectionDTO struct {
	Stage      string `json:"stage"`
	RejectorID string `json:"rejectorId"`
	Reason     string `json:"reason"`
	Timestamp  string `json:"timestamp"`
}

type BounceRecordDTO struct {
	FromID    string `json:"fromId"`
	ToID      string `json:"toId"`
	Stage     string `json:"stage"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

type HumanGateRequestDTO struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	TaskID    string `json:"taskId"`
	WorkerID  string `json:"workerId"`
	Message   string `json:"message"`
	Blocking  bool   `json:"blocking"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type DashboardAlertsDTO struct {
	StuckWorkers     int `json:"stuckWorkers"`
	EscalatedTasks   int `json:"escalatedTasks"`
	PendingApprovals int `json:"pendingApprovals"`
}

func TaskToDTO(t *project.Task) TaskDTO {
	taskType := string(t.Type)
	if taskType == "" {
		taskType = "code"
	}
	rejections := make([]RejectionDTO, len(t.RejectionHistory))
	for i, r := range t.RejectionHistory {
		rejections[i] = RejectionDTO{
			Stage:      string(r.Stage),
			RejectorID: r.RejectorID,
			Reason:     r.Reason,
			Timestamp:  r.Timestamp.Format(time.RFC3339),
		}
	}
	bounces := make([]BounceRecordDTO, len(t.BounceHistory))
	for i, b := range t.BounceHistory {
		bounces[i] = BounceRecordDTO{
			FromID:    b.FromID,
			ToID:      b.ToID,
			Stage:     string(b.Stage),
			Reason:    b.Reason,
			Timestamp: b.Timestamp.Format(time.RFC3339),
		}
	}
	dto := TaskDTO{
		ID:               t.ID,
		ProjectID:        t.ProjectID,
		Title:            t.Title,
		Description:      t.Description,
		Prompt:           t.Prompt,
		Type:             taskType,
		Status:           string(t.Status),
		Priority:         t.Priority,
		BranchName:       t.BranchName,
		AssigneeID:       t.AssigneeID,
		DependsOn:        t.DependsOn,
		Milestone:        t.Milestone,
		ReviewerID:       t.ReviewerID,
		ParentTaskID:     t.ParentTaskID,
		ReviewCount:      t.ReviewCount,
		RejectionCount:   t.RejectionCount,
		RejectionHistory: rejections,
		BounceHistory:    bounces,
		CreatedAt:        t.CreatedAt.Format(time.RFC3339),
	}
	if t.StartedAt != nil {
		dto.StartedAt = t.StartedAt.Format(time.RFC3339)
	}
	if t.CompletedAt != nil {
		dto.CompletedAt = t.CompletedAt.Format(time.RFC3339)
	}
	return dto
}

type WorkerDTO struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Avatar        string                  `json:"avatar"`
	Status        string                  `json:"status"`
	Tier          string                  `json:"tier"`
	Role          string                  `json:"role,omitempty"`
	BackendID     string                  `json:"backendId,omitempty"`
	ParentID      string                  `json:"parentId,omitempty"`
	ModelVersion  string                  `json:"modelVersion,omitempty"`
	CLITool       string                  `json:"cliTool,omitempty"`
	SkillProfile  string                  `json:"skillProfile,omitempty"`
	Gender        string                  `json:"gender,omitempty"`
	Appearance    *worker.WorkerAppearance `json:"appearance,omitempty"`
	CurrentTaskID string                  `json:"currentTaskId"`
	TmuxSession   string                  `json:"tmuxSession"`
	SessionID     string                  `json:"sessionId"`
	CreatedAt     string                  `json:"createdAt"`
}

type SkillProfileDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

func WorkerToDTO(w *worker.Worker) WorkerDTO {
	return WorkerDTO{
		ID:            w.ID,
		Name:          w.Name,
		Avatar:        w.Avatar,
		Status:        string(w.Status),
		Tier:          string(w.EffectiveTier()),
		Role:          string(w.EffectiveRole()),
		BackendID:     w.BackendID,
		ParentID:      w.ParentID,
		ModelVersion:  w.ModelVersion,
		CLITool:       w.CLITool,
		SkillProfile:  w.SkillProfile,
		Gender:        string(w.Gender),
		Appearance:    w.Appearance,
		CurrentTaskID: w.CurrentTaskID,
		TmuxSession:   w.TmuxSession,
		SessionID:     w.SessionID,
		CreatedAt:     w.CreatedAt.Format(time.RFC3339),
	}
}

type CompanyProgressDTO struct {
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	InProgress int     `json:"inProgress"`
	Failed     int     `json:"failed"`
	Percent    float64 `json:"percent"`
}

func ProgressToDTO(p company.ProgressDTO) CompanyProgressDTO {
	return CompanyProgressDTO{
		Total:      p.Total,
		Done:       p.Done,
		InProgress: p.InProgress,
		Failed:     p.Failed,
		Percent:    p.Percent,
	}
}

type CompanyEventDTO struct {
	Type      string `json:"type"`
	ProjectID string `json:"projectId,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	WorkerID  string `json:"workerId,omitempty"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type ReviewRequestDTO struct {
	TaskID     string `json:"taskId"`
	ProjectID  string `json:"projectId"`
	EngineerID string `json:"engineerId"`
	ManagerID  string `json:"managerId"`
	CreatedAt  string `json:"createdAt"`
}

func ReviewRequestToDTO(r company.ReviewRequest) ReviewRequestDTO {
	return ReviewRequestDTO{
		TaskID:     r.TaskID,
		ProjectID:  r.ProjectID,
		EngineerID: r.EngineerID,
		ManagerID:  r.ManagerID,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
	}
}

type TrainingStatsDTO struct {
	TotalPairs   int     `json:"totalPairs"`
	Accepted     int     `json:"accepted"`
	Rejected     int     `json:"rejected"`
	ApprovalRate float64 `json:"approvalRate"`
}

func CompanyEventToDTO(e company.Event) CompanyEventDTO {
	return CompanyEventDTO{
		Type:      string(e.Type),
		ProjectID: e.ProjectID,
		TaskID:    e.TaskID,
		WorkerID:  e.WorkerID,
		Message:   e.Message,
		Timestamp: e.Timestamp.Format(time.RFC3339),
	}
}

// ChatMessageDTO represents a chat message for AI project creation.
type ChatMessageDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatProjectResponseDTO is the response from AI project creation.
type ChatProjectResponseDTO struct {
	Status      string   `json:"status"`
	Questions   []string `json:"questions,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	RepoPath    string   `json:"repoPath,omitempty"`
	BaseBranch  string   `json:"baseBranch,omitempty"`
	Goals       []string `json:"goals,omitempty"`
}

type PersonalityTraitsDTO struct {
	Sociability int `json:"sociability"`
	Focus       int `json:"focus"`
	Creativity  int `json:"creativity"`
	Empathy     int `json:"empathy"`
	Ambition    int `json:"ambition"`
	Humor       int `json:"humor"`
}

type MoodDTO struct {
	Current string `json:"current"`
	Energy  int    `json:"energy"`
	Morale  int    `json:"morale"`
}

type HabitsDTO struct {
	CoffeeTime       string   `json:"coffeeTime"`
	FavoriteSpot     string   `json:"favoriteSpot"`
	WorkStyle        string   `json:"workStyle"`
	SocialPreference string   `json:"socialPreference"`
	Quirks           []string `json:"quirks"`
}

type NarrativeDTO struct {
	Description  string   `json:"description"`
	Catchphrases []string `json:"catchphrases"`
	Backstory    string   `json:"backstory"`
}

type SkillScoresDTO struct {
	Carefulness          int `json:"carefulness"`
	BoundaryChecking     int `json:"boundaryChecking"`
	TestCoverageAware    int `json:"testCoverageAware"`
	CommunicationClarity int `json:"communicationClarity"`
	CodeQuality          int `json:"codeQuality"`
	SecurityAwareness    int `json:"securityAwareness"`
}

type GrowthEntryDTO struct {
	Event   string         `json:"event"`
	Date    string         `json:"date"`
	Changes map[string]int `json:"changes"`
}

type CharacterProfileDTO struct {
	WorkerID       string               `json:"workerId"`
	Traits         PersonalityTraitsDTO `json:"traits"`
	Mood           MoodDTO              `json:"mood"`
	Habits         HabitsDTO            `json:"habits"`
	Narrative      NarrativeDTO         `json:"narrative"`
	SkillScores    SkillScoresDTO       `json:"skillScores"`
	TasksCompleted int                  `json:"tasksCompleted"`
	GrowthLog      []GrowthEntryDTO     `json:"growthLog"`
	Birthday       string               `json:"birthday,omitempty"`
}

// ResearchReportDTO represents a research report for the frontend.
type ResearchReportDTO struct {
	ID              string   `json:"id"`
	TaskID          string   `json:"taskId"`
	ProjectID       string   `json:"projectId"`
	WorkerID        string   `json:"workerId"`
	Summary         string   `json:"summary"`
	KeyFindings     []string `json:"keyFindings"`
	Recommendations []string `json:"recommendations"`
	References      []string `json:"references"`
	RawContent      string   `json:"rawContent"`
	CreatedAt       string   `json:"createdAt"`
}

func ReportToDTO(r *project.ResearchReport) ResearchReportDTO {
	return ResearchReportDTO{
		ID:              r.ID,
		TaskID:          r.TaskID,
		ProjectID:       r.ProjectID,
		WorkerID:        r.WorkerID,
		Summary:         r.Summary,
		KeyFindings:     r.KeyFindings,
		Recommendations: r.Recommendations,
		References:      r.References,
		RawContent:      r.RawContent,
		CreatedAt:       r.CreatedAt.Format(time.RFC3339),
	}
}

// WorkerChatMessageDTO represents a message in a worker NPC chat.
type WorkerChatMessageDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// WorkerChatResponseDTO is the response from a worker NPC chat.
type WorkerChatResponseDTO struct {
	Content string `json:"content"`
}

type RelationshipDTO struct {
	WorkerA          string   `json:"workerA"`
	WorkerB          string   `json:"workerB"`
	Affinity         int      `json:"affinity"`
	Trust            int      `json:"trust"`
	InteractionCount int      `json:"interactionCount"`
	Tags             []string `json:"tags"`
}

// --- Retro DTOs ---

type RetroReportDTO struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	ProjectName string         `json:"projectName"`
	Result      RetroResultDTO `json:"result"`
	AppliedAt   string         `json:"appliedAt"`
}

type RetroResultDTO struct {
	Summary          string                       `json:"summary"`
	WorkerFeedback   []WorkerFeedbackDTO          `json:"workerFeedback"`
	SkillAdjustments []SkillProfileAdjustmentDTO  `json:"skillAdjustments"`
}

type WorkerFeedbackDTO struct {
	WorkerID    string   `json:"workerId"`
	Strengths   []string `json:"strengths"`
	Weaknesses  []string `json:"weaknesses"`
	Suggestions []string `json:"suggestions"`
}

type SkillProfileAdjustmentDTO struct {
	WorkerID        string   `json:"workerId"`
	ProfileID       string   `json:"profileId"`
	PromptAdditions []string `json:"promptAdditions,omitempty"`
	PromptRemovals  []string `json:"promptRemovals,omitempty"`
	AddTools        []string `json:"addTools,omitempty"`
	RemoveTools     []string `json:"removeTools,omitempty"`
	ModelChange     string   `json:"modelChange,omitempty"`
}

type SkillProfileOverrideDTO struct {
	ExtraPrompt   string   `json:"extraPrompt,omitempty"`
	AddTools      []string `json:"addTools,omitempty"`
	RemoveTools   []string `json:"removeTools,omitempty"`
	ModelOverride string   `json:"modelOverride,omitempty"`
}

func RetroReportToDTO(r company.RetroReport) RetroReportDTO {
	fb := make([]WorkerFeedbackDTO, len(r.Result.WorkerFeedback))
	for i, f := range r.Result.WorkerFeedback {
		fb[i] = WorkerFeedbackDTO{
			WorkerID:    f.WorkerID,
			Strengths:   f.Strengths,
			Weaknesses:  f.Weaknesses,
			Suggestions: f.Suggestions,
		}
	}
	adj := make([]SkillProfileAdjustmentDTO, len(r.Result.SkillAdjustments))
	for i, a := range r.Result.SkillAdjustments {
		adj[i] = SkillProfileAdjustmentDTO{
			WorkerID:        a.WorkerID,
			ProfileID:       a.ProfileID,
			PromptAdditions: a.PromptAdditions,
			PromptRemovals:  a.PromptRemovals,
			AddTools:        a.AddTools,
			RemoveTools:     a.RemoveTools,
			ModelChange:     a.ModelChange,
		}
	}
	return RetroReportDTO{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		ProjectName: r.ProjectName,
		Result: RetroResultDTO{
			Summary:          r.Result.Summary,
			WorkerFeedback:   fb,
			SkillAdjustments: adj,
		},
		AppliedAt: r.AppliedAt.Format(time.RFC3339),
	}
}
