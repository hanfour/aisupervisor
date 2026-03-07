package gui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	anthropicBackend "github.com/hanfourmini/aisupervisor/internal/ai/anthropic"
	ollamaBackend "github.com/hanfourmini/aisupervisor/internal/ai/ollama"
	openaiBackend "github.com/hanfourmini/aisupervisor/internal/ai/openai"
	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/training"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// CompanyApp is the Wails binding for the company management system.
// It is separate from the existing App to avoid bloating it.
type CompanyApp struct {
	ctx           context.Context
	company       *company.Manager
	tmuxClient    tmux.TmuxClient
	trainingDir   string
	skillProfiles []config.SkillProfile
	spawner       *worker.Spawner
}

func NewCompanyApp(company *company.Manager, tmuxClient tmux.TmuxClient) *CompanyApp {
	return &CompanyApp{company: company, tmuxClient: tmuxClient}
}

// SetTrainingDir sets the training data directory for stats queries.
func (c *CompanyApp) SetTrainingDir(dir string) {
	c.trainingDir = dir
}

// SetSkillProfiles sets the available skill profiles for listing.
func (c *CompanyApp) SetSkillProfiles(profiles []config.SkillProfile) {
	c.skillProfiles = profiles
}

// SetSpawner sets the spawner reference for language propagation.
func (c *CompanyApp) SetSpawner(s *worker.Spawner) {
	c.spawner = s
}

// SetLanguage updates the prompt language and persists it to the config file.
func (c *CompanyApp) SetLanguage(lang string) error {
	c.company.SetLanguage(lang)
	if c.spawner != nil {
		c.spawner.SetLanguage(lang)
	}
	// Persist to config file
	cfg, err := config.Load("")
	if err != nil {
		return err
	}
	cfg.Language = lang
	return cfg.Save("")
}

// GetLanguage returns the current prompt language.
func (c *CompanyApp) GetLanguage() string {
	return c.company.GetLanguage()
}

// ListSkillProfiles returns all available skill profiles.
func (c *CompanyApp) ListSkillProfiles() []SkillProfileDTO {
	dtos := make([]SkillProfileDTO, len(c.skillProfiles))
	for i, sp := range c.skillProfiles {
		dtos[i] = SkillProfileDTO{
			ID:          sp.ID,
			Name:        sp.Name,
			Description: sp.Description,
			Icon:        sp.Icon,
		}
	}
	return dtos
}

// Startup is called by Wails when the application starts.
func (c *CompanyApp) Startup(ctx context.Context) {
	c.ctx = ctx
	go startCompanyEventForwarding(ctx, c.company)
}

// Shutdown is called by Wails when the application shuts down.
func (c *CompanyApp) Shutdown(ctx context.Context) {
	c.company.Shutdown()
}

// --- Project operations ---

func (c *CompanyApp) CreateProject(name, description, repoPath, baseBranch string, goals []string) (*ProjectDTO, error) {
	p, err := c.company.CreateProject(name, description, repoPath, baseBranch, goals)
	if err != nil {
		return nil, err
	}
	dto := ProjectToDTO(p)
	return &dto, nil
}

func (c *CompanyApp) ListProjects() []ProjectDTO {
	projects := c.company.ListProjects()
	dtos := make([]ProjectDTO, len(projects))
	for i, p := range projects {
		dtos[i] = ProjectToDTO(p)
	}
	return dtos
}

func (c *CompanyApp) GetProject(id string) (*ProjectDTO, error) {
	p, ok := c.company.GetProject(id)
	if !ok {
		return nil, nil
	}
	dto := ProjectToDTO(p)
	return &dto, nil
}

// DeleteProject removes a project and all its tasks.
func (c *CompanyApp) DeleteProject(projectID string) error {
	return c.company.DeleteProject(projectID)
}

// ActiveWorkerCount returns the number of workers currently working on tasks.
func (c *CompanyApp) ActiveWorkerCount() int {
	return c.company.ActiveWorkerCount()
}

// ClearAllProjects deletes all projects and tasks. If force is true, stops active workers first.
func (c *CompanyApp) ClearAllProjects(force bool) error {
	return c.company.ClearAllProjects(force)
}

// ChatCreateProject uses AI to extract project information from a chat conversation.
func (c *CompanyApp) ChatCreateProject(messages []ChatMessageDTO) (*ChatProjectResponseDTO, error) {
	// Convert DTOs to domain types
	chatMessages := make([]company.ChatMessage, len(messages))
	for i, m := range messages {
		chatMessages[i] = company.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	resp, err := c.company.ChatCreateProject(c.ctx, chatMessages)
	if err != nil {
		return nil, err
	}

	return &ChatProjectResponseDTO{
		Status:      resp.Status,
		Questions:   resp.Questions,
		Name:        resp.Name,
		Description: resp.Description,
		RepoPath:    resp.RepoPath,
		BaseBranch:  resp.BaseBranch,
		Goals:       resp.Goals,
	}, nil
}

// --- Task operations ---

func (c *CompanyApp) CreateTask(projectID, title, description, prompt string, dependsOn []string, priority int, milestone string, taskType string) (*TaskDTO, error) {
	t, err := c.company.AddTask(projectID, title, description, prompt, dependsOn, priority, milestone, taskType)
	if err != nil {
		return nil, err
	}
	dto := TaskToDTO(t)
	return &dto, nil
}

// DecomposeGoals uses AI to break project goals into tasks.
func (c *CompanyApp) DecomposeGoals(projectID string) error {
	return c.company.DecomposeGoals(c.ctx, projectID)
}

func (c *CompanyApp) ListTasks(projectID string) []TaskDTO {
	tasks := c.company.ListTasks(projectID)
	dtos := make([]TaskDTO, len(tasks))
	for i, t := range tasks {
		dtos[i] = TaskToDTO(t)
	}
	return dtos
}

func (c *CompanyApp) CompleteTask(taskID string) error {
	return c.company.CompleteTask(taskID)
}

// UpdateTaskStatus changes a task's status directly (used by board drag-and-drop).
func (c *CompanyApp) UpdateTaskStatus(taskID, status string) error {
	return c.company.UpdateTaskStatusDirect(taskID, status)
}

// --- Worker operations ---

func (c *CompanyApp) CreateWorker(name, avatar string) (*WorkerDTO, error) {
	w, err := c.company.CreateWorker(name, avatar)
	if err != nil {
		return nil, err
	}
	dto := WorkerToDTO(w)
	return &dto, nil
}

// CreateWorkerWithTier creates a worker with tier and hierarchy options.
func (c *CompanyApp) CreateWorkerWithTier(name, avatar, tier, parentID, backendID, cliTool, skillProfile, gender string) (*WorkerDTO, error) {
	var opts []company.WorkerOption
	if tier != "" {
		opts = append(opts, company.WithTier(worker.WorkerTier(tier)))
	}
	if parentID != "" {
		opts = append(opts, company.WithParent(parentID))
	}
	if backendID != "" {
		opts = append(opts, company.WithBackend(backendID))
	}
	if cliTool != "" {
		opts = append(opts, company.WithCLITool(cliTool))
	}
	if skillProfile != "" {
		opts = append(opts, company.WithSkillProfile(skillProfile))
	}
	if gender != "" {
		g := worker.WorkerGender(gender)
		if g != worker.GenderMale && g != worker.GenderFemale {
			return nil, fmt.Errorf("invalid gender %q: must be 'male' or 'female'", gender)
		}
		opts = append(opts, company.WithGender(g))
	}
	w, err := c.company.CreateWorker(name, avatar, opts...)
	if err != nil {
		return nil, err
	}
	dto := WorkerToDTO(w)
	return &dto, nil
}

func (c *CompanyApp) ListWorkers() []WorkerDTO {
	workers := c.company.ListWorkers()
	dtos := make([]WorkerDTO, len(workers))
	for i, w := range workers {
		dtos[i] = WorkerToDTO(w)
	}
	return dtos
}

// GetSubordinates returns workers managed by the given worker.
func (c *CompanyApp) GetSubordinates(workerID string) []WorkerDTO {
	workers := c.company.GetSubordinates(workerID)
	dtos := make([]WorkerDTO, len(workers))
	for i, w := range workers {
		dtos[i] = WorkerToDTO(w)
	}
	return dtos
}

// PromoteWorker upgrades a worker's tier.
func (c *CompanyApp) PromoteWorker(workerID, newTier string) error {
	return c.company.PromoteWorker(workerID, worker.WorkerTier(newTier))
}

// GetWorker returns a single worker by ID.
func (c *CompanyApp) GetWorker(workerID string) (*WorkerDTO, error) {
	w, ok := c.company.GetWorker(workerID)
	if !ok {
		return nil, nil
	}
	dto := WorkerToDTO(w)
	return &dto, nil
}

// GetManager returns the parent (manager) of a worker.
func (c *CompanyApp) GetManager(workerID string) (*WorkerDTO, error) {
	w, ok := c.company.GetManager(workerID)
	if !ok {
		return nil, nil
	}
	dto := WorkerToDTO(w)
	return &dto, nil
}

// DeleteWorker removes a worker by ID.
func (c *CompanyApp) DeleteWorker(workerID string) error {
	return c.company.DeleteWorker(workerID)
}

// UpdateWorkerFields updates optional fields on a worker.
func (c *CompanyApp) UpdateWorkerFields(workerID, parentID, modelVersion, backendID, skillProfile string) error {
	return c.company.UpdateWorkerFields(workerID, parentID, modelVersion, backendID, skillProfile)
}

// GetHierarchy returns workers organized by tier.
func (c *CompanyApp) GetHierarchy() map[string][]WorkerDTO {
	workers := c.company.ListWorkers()
	result := map[string][]WorkerDTO{
		"consultant": {},
		"manager":    {},
		"engineer":   {},
	}
	for _, w := range workers {
		tier := string(w.EffectiveTier())
		result[tier] = append(result[tier], WorkerToDTO(w))
	}
	return result
}

// --- Assignment ---

func (c *CompanyApp) AssignTask(workerID, taskID string) error {
	return c.company.AssignTask(c.ctx, workerID, taskID)
}

// --- Progress ---

func (c *CompanyApp) GetProjectProgress(projectID string) CompanyProgressDTO {
	return ProgressToDTO(c.company.ProjectProgress(projectID))
}

// --- Worker pane content ---

func (c *CompanyApp) GetPaneContent(workerID string) (string, error) {
	return c.GetPaneContentLines(workerID, 100)
}

func (c *CompanyApp) GetPaneContentLines(workerID string, lines int) (string, error) {
	w, ok := c.company.GetWorker(workerID)
	if !ok {
		return "", fmt.Errorf("worker %q not found", workerID)
	}
	if w.TmuxSession == "" {
		return "", fmt.Errorf("worker %q has no active tmux session", workerID)
	}
	if c.tmuxClient == nil {
		return "", fmt.Errorf("tmux client not available")
	}
	if lines <= 0 {
		lines = 100
	}
	return c.tmuxClient.CapturePane(w.TmuxSession, 0, 0, lines)
}

// --- Review Queue ---

func (c *CompanyApp) GetReviewQueue() []ReviewRequestDTO {
	reviews := c.company.PendingReviews()
	dtos := make([]ReviewRequestDTO, len(reviews))
	for i, r := range reviews {
		dtos[i] = ReviewRequestToDTO(r)
	}
	return dtos
}

// --- Training Stats ---

func (c *CompanyApp) GetTrainingStats() (*TrainingStatsDTO, error) {
	if c.trainingDir == "" {
		return &TrainingStatsDTO{}, nil
	}
	stats, err := training.ComputeReviewStats(c.trainingDir)
	if err != nil {
		return nil, err
	}
	return &TrainingStatsDTO{
		TotalPairs:   stats.TotalPairs,
		Accepted:     stats.Accepted,
		Rejected:     stats.Rejected,
		ApprovalRate: stats.ApprovalRate,
	}, nil
}

// --- Research Report operations ---

// GetReport returns the research report for a given task.
func (c *CompanyApp) GetReport(taskID string) *ResearchReportDTO {
	r, ok := c.company.ProjectStore().GetReport(taskID)
	if !ok {
		return nil
	}
	dto := ReportToDTO(r)
	return &dto
}

// ListReports returns all research reports for a project.
func (c *CompanyApp) ListReports(projectID string) []ResearchReportDTO {
	reports := c.company.ProjectStore().ListReports(projectID)
	dtos := make([]ResearchReportDTO, len(reports))
	for i, r := range reports {
		dtos[i] = ReportToDTO(r)
	}
	return dtos
}

// --- Worker Chat (NPC Dialogue) ---

// ChatWithWorker sends a conversation to a worker's NPC persona and returns its response.
func (c *CompanyApp) ChatWithWorker(workerID string, messages []WorkerChatMessageDTO) (*WorkerChatResponseDTO, error) {
	chatMessages := make([]company.WorkerChatMessage, len(messages))
	for i, m := range messages {
		chatMessages[i] = company.WorkerChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	resp, err := c.company.ChatWithWorker(c.ctx, workerID, chatMessages)
	if err != nil {
		return nil, err
	}

	return &WorkerChatResponseDTO{
		Content: resp.Content,
	}, nil
}

// --- Chat Backend Settings ---

// GetChatBackend returns the current chat backend name.
func (c *CompanyApp) GetChatBackend() string {
	cfg, err := config.Load("")
	if err != nil {
		return ""
	}
	return cfg.ChatBackend
}

// SetChatBackend changes the chat backend and persists to config.
func (c *CompanyApp) SetChatBackend(name string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	// Build new ChatProvider from the selected backend
	var provider ai.ChatProvider
	for _, bc := range cfg.Backends {
		if bc.Name != name {
			continue
		}
		var err2 error
		switch bc.Type {
		case "openai":
			apiKey := os.Getenv(bc.APIKeyEnv)
			if apiKey == "" && bc.APIKeyEnv != "" {
				return fmt.Errorf("environment variable %s not set for backend %q", bc.APIKeyEnv, name)
			}
			provider = openaiBackend.NewBackend(bc.Name, apiKey, bc.Model)
		case "ollama":
			provider = ollamaBackend.NewBackend(bc.Name, bc.BaseURL, bc.Model)
		case "anthropic_api":
			apiKey := os.Getenv(bc.APIKeyEnv)
			if apiKey == "" && bc.APIKeyEnv != "" {
				return fmt.Errorf("environment variable %s not set for backend %q", bc.APIKeyEnv, name)
			}
			provider = anthropicBackend.NewAPIBackend(bc.Name, apiKey, bc.Model)
		case "anthropic_oauth":
			provider, err2 = anthropicBackend.NewOAuthBackend(bc.Name, bc.Model)
			if err2 != nil {
				return fmt.Errorf("OAuth backend init failed: %w", err2)
			}
		default:
			return fmt.Errorf("backend type %q does not support chat", bc.Type)
		}
		break
	}
	if provider == nil {
		return fmt.Errorf("chat backend %q not found", name)
	}

	c.company.SetChatProvider(provider)
	cfg.ChatBackend = name
	return cfg.Save("")
}

// GetAvailableChatBackends returns backend names that support chat.
func (c *CompanyApp) GetAvailableChatBackends() []string {
	cfg, err := config.Load("")
	if err != nil {
		return nil
	}
	chatTypes := map[string]bool{"openai": true, "ollama": true, "anthropic_api": true, "anthropic_oauth": true}
	var result []string
	for _, bc := range cfg.Backends {
		if chatTypes[bc.Type] {
			result = append(result, bc.Name)
		}
	}
	return result
}

// --- Personality operations ---

func (c *CompanyApp) GetCharacterProfile(workerID string) *CharacterProfileDTO {
	store := c.company.GetPersonalityStore()
	if store == nil {
		return nil
	}
	p := store.GetProfile(workerID)
	if p == nil {
		return nil
	}
	growthLog := make([]GrowthEntryDTO, len(p.GrowthLog))
	for i, g := range p.GrowthLog {
		growthLog[i] = GrowthEntryDTO{
			Event:   g.Event,
			Date:    g.Date.Format(time.RFC3339),
			Changes: g.Changes,
		}
	}
	dto := &CharacterProfileDTO{
		WorkerID: p.WorkerID,
		Traits: PersonalityTraitsDTO{
			Sociability: p.Traits.Sociability,
			Focus:       p.Traits.Focus,
			Creativity:  p.Traits.Creativity,
			Empathy:     p.Traits.Empathy,
			Ambition:    p.Traits.Ambition,
			Humor:       p.Traits.Humor,
		},
		Mood: MoodDTO{
			Current: p.Mood.Current,
			Energy:  p.Mood.Energy,
			Morale:  p.Mood.Morale,
		},
		Habits: HabitsDTO{
			CoffeeTime:       p.Habits.CoffeeTime,
			FavoriteSpot:     p.Habits.FavoriteSpot,
			WorkStyle:        p.Habits.WorkStyle,
			SocialPreference: p.Habits.SocialPreference,
			Quirks:           p.Habits.Quirks,
		},
		Narrative: NarrativeDTO{
			Description:  p.Narrative.Description,
			Catchphrases: p.Narrative.Catchphrases,
			Backstory:    p.Narrative.Backstory,
		},
		SkillScores: SkillScoresDTO{
			Carefulness:          p.SkillScores.Carefulness,
			BoundaryChecking:     p.SkillScores.BoundaryChecking,
			TestCoverageAware:    p.SkillScores.TestCoverageAware,
			CommunicationClarity: p.SkillScores.CommunicationClarity,
			CodeQuality:          p.SkillScores.CodeQuality,
			SecurityAwareness:    p.SkillScores.SecurityAwareness,
		},
		TasksCompleted: p.TasksCompleted,
		GrowthLog:      growthLog,
	}
	if p.Birthday != nil {
		dto.Birthday = p.Birthday.Format("2006-01-02")
	}
	return dto
}

func (c *CompanyApp) GetWorkerRelationships(workerID string) []RelationshipDTO {
	store := c.company.GetPersonalityStore()
	if store == nil {
		return nil
	}
	rels := store.GetWorkerRelationships(workerID)
	dtos := make([]RelationshipDTO, len(rels))
	for i, r := range rels {
		dtos[i] = RelationshipDTO{
			WorkerA:          r.WorkerA,
			WorkerB:          r.WorkerB,
			Affinity:         r.Affinity,
			Trust:            r.Trust,
			InteractionCount: r.InteractionCount,
			Tags:             r.Tags,
		}
	}
	return dtos
}

func (c *CompanyApp) GenerateNarrative(workerID string) error {
	store := c.company.GetPersonalityStore()
	if store == nil {
		return fmt.Errorf("personality store not initialized")
	}
	p := store.GetProfile(workerID)
	if p == nil {
		return fmt.Errorf("profile not found: %s", workerID)
	}
	narrator := c.company.GetNarrator()
	if narrator == nil {
		return fmt.Errorf("narrator not available (Ollama not configured)")
	}
	w, ok := c.company.GetWorker(workerID)
	name := workerID
	if ok && w != nil {
		name = w.Name
	}
	// Copy traits before the long AI call to avoid holding references to store internals
	traits := p.Traits
	narrative, err := narrator.GeneratePersonality(context.Background(), name, traits)
	if err != nil {
		return err
	}
	store.UpdateProfile(workerID, func(p *personality.CharacterProfile) {
		p.Narrative = *narrative
	})
	return nil
}

// --- Retro operations ---

// GetRetroReports returns all retro reports.
func (c *CompanyApp) GetRetroReports() []RetroReportDTO {
	reports := c.company.LoadRetroReports()
	dtos := make([]RetroReportDTO, len(reports))
	for i, r := range reports {
		dtos[i] = RetroReportToDTO(r)
	}
	return dtos
}

// GetRetroReport returns a single retro report by ID.
func (c *CompanyApp) GetRetroReport(id string) *RetroReportDTO {
	r := c.company.GetRetroReport(id)
	if r == nil {
		return nil
	}
	dto := RetroReportToDTO(*r)
	return &dto
}

// TriggerRetro manually triggers a retrospective for a project.
func (c *CompanyApp) TriggerRetro(projectID string) error {
	return c.company.RunRetro(c.ctx, projectID)
}

// GetWorkerSkillOverrides returns the skill profile override for a worker.
func (c *CompanyApp) GetWorkerSkillOverrides(workerID string) *SkillProfileOverrideDTO {
	override := c.company.GetWorkerSkillOverride(workerID)
	if override == nil {
		return nil
	}
	return &SkillProfileOverrideDTO{
		ExtraPrompt:   override.ExtraPrompt,
		AddTools:      override.AddTools,
		RemoveTools:   override.RemoveTools,
		ModelOverride: override.ModelOverride,
	}
}

// UpdateWorkerBirthday updates a worker's birthday (ISO date string, e.g. "1998-03-15").
func (c *CompanyApp) UpdateWorkerBirthday(workerID, birthday string) error {
	store := c.company.GetPersonalityStore()
	if store == nil {
		return fmt.Errorf("personality store not initialized")
	}
	bd, err := time.Parse("2006-01-02", birthday)
	if err != nil {
		return fmt.Errorf("invalid birthday format (expected YYYY-MM-DD): %w", err)
	}
	ok := store.UpdateProfile(workerID, func(p *personality.CharacterProfile) {
		p.Birthday = &bd
	})
	if !ok {
		return fmt.Errorf("profile not found: %s", workerID)
	}
	return nil
}
