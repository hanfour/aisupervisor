package gui

import (
	"context"

	"github.com/hanfourmini/aisupervisor/internal/company"
)

// CompanyApp is the Wails binding for the company management system.
// It is separate from the existing App to avoid bloating it.
type CompanyApp struct {
	ctx     context.Context
	company *company.Manager
}

func NewCompanyApp(company *company.Manager) *CompanyApp {
	return &CompanyApp{company: company}
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

// --- Worker operations ---

func (c *CompanyApp) CreateWorker(name, avatar string) (*WorkerDTO, error) {
	w, err := c.company.CreateWorker(name, avatar)
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

// --- Assignment ---

func (c *CompanyApp) AssignTask(workerID, taskID string) error {
	return c.company.AssignTask(c.ctx, workerID, taskID)
}

// --- Progress ---

func (c *CompanyApp) GetProjectProgress(projectID string) CompanyProgressDTO {
	return ProgressToDTO(c.company.ProjectProgress(projectID))
}
