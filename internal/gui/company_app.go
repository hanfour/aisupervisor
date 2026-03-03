package gui

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/config"
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

// --- Task operations ---

func (c *CompanyApp) CreateTask(projectID, title, description, prompt string, dependsOn []string, priority int, milestone string) (*TaskDTO, error) {
	t, err := c.company.AddTask(projectID, title, description, prompt, dependsOn, priority, milestone)
	if err != nil {
		return nil, err
	}
	dto := TaskToDTO(t)
	return &dto, nil
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
func (c *CompanyApp) CreateWorkerWithTier(name, avatar, tier, parentID, backendID, cliTool, skillProfile string) (*WorkerDTO, error) {
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
	return &CharacterProfileDTO{
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
	}
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
	narrative, err := narrator.GeneratePersonality(context.Background(), name, p.Traits)
	if err != nil {
		return err
	}
	p.Narrative = *narrative
	store.Save()
	return nil
}
