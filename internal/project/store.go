package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

var idCounter atomic.Int64

type Store struct {
	mu           sync.RWMutex
	projects     map[string]*Project
	tasks        map[string]*Task
	reports      map[string]*ResearchReport // keyed by TaskID
	projectsPath string
	tasksPath    string
	reportsPath  string
}

type projectsFile struct {
	Projects []*Project `yaml:"projects"`
}

type tasksFile struct {
	Tasks []*Task `yaml:"tasks"`
}

type reportsFile struct {
	Reports []*ResearchReport `yaml:"reports"`
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	s := &Store{
		projects:     make(map[string]*Project),
		tasks:        make(map[string]*Task),
		reports:      make(map[string]*ResearchReport),
		projectsPath: filepath.Join(dataDir, "projects.yaml"),
		tasksPath:    filepath.Join(dataDir, "tasks.yaml"),
		reportsPath:  filepath.Join(dataDir, "reports.yaml"),
	}

	if err := s.loadProjects(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.loadTasks(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.loadReports(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

// SaveProject creates or updates a project.
func (s *Store) SaveProject(p *Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		p.ID = fmt.Sprintf("p%d-%d", time.Now().UnixMilli(), idCounter.Add(1))
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.Status == "" {
		p.Status = ProjectActive
	}
	p.UpdatedAt = time.Now()

	s.projects[p.ID] = p
	return s.saveProjects()
}

// GetProject returns a project by ID.
func (s *Store) GetProject(id string) (*Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.projects[id]
	return p, ok
}

// ListProjects returns all projects.
func (s *Store) ListProjects() []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Project, 0, len(s.projects))
	for _, p := range s.projects {
		result = append(result, p)
	}
	return result
}

// DeleteProject removes a project and all its associated tasks.
func (s *Store) DeleteProject(projectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[projectID]; !ok {
		return fmt.Errorf("project %q not found", projectID)
	}

	delete(s.projects, projectID)

	// Cascade delete all tasks and reports belonging to this project
	for id, t := range s.tasks {
		if t.ProjectID == projectID {
			delete(s.tasks, id)
		}
	}
	for taskID, r := range s.reports {
		if r.ProjectID == projectID {
			delete(s.reports, taskID)
		}
	}

	if err := s.saveProjects(); err != nil {
		return err
	}
	if err := s.saveTasks(); err != nil {
		return err
	}
	return s.saveReports()
}

// SaveTask creates or updates a task.
func (s *Store) SaveTask(t *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.ID == "" {
		t.ID = fmt.Sprintf("t%d-%d", time.Now().UnixMilli(), idCounter.Add(1))
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	if t.Status == "" {
		t.Status = TaskBacklog
	}

	s.tasks[t.ID] = t
	return s.saveTasks()
}

// GetTask returns a task by ID.
func (s *Store) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

// TasksForProject returns all tasks belonging to a project.
func (s *Store) TasksForProject(projectID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.ProjectID == projectID {
			result = append(result, t)
		}
	}
	return result
}

// ReadyTasks returns tasks whose dependencies are all done.
func (s *Store) ReadyTasks(projectID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.ProjectID != projectID {
			continue
		}
		if t.Status != TaskBacklog && t.Status != TaskReady {
			continue
		}
		if s.depsResolved(t) {
			result = append(result, t)
		}
	}
	return result
}

// ReadyTasksByPriority returns all ready tasks across all projects, sorted by priority (1=highest).
func (s *Store) ReadyTasksByPriority() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.Status == TaskReady {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority == result[j].Priority {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		}
		return result[i].Priority < result[j].Priority
	})
	return result
}

// UpdateTaskStatus updates a task's status with appropriate timestamp tracking.
func (s *Store) UpdateTaskStatus(taskID string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}

	t.Status = status
	now := time.Now()
	switch status {
	case TaskInProgress:
		t.StartedAt = &now
	case TaskDone, TaskFailed:
		t.CompletedAt = &now
	}

	return s.saveTasks()
}

// depsResolved checks if all dependencies of a task are done. Must be called with lock held.
func (s *Store) depsResolved(t *Task) bool {
	for _, depID := range t.DependsOn {
		dep, ok := s.tasks[depID]
		if !ok || dep.Status != TaskDone {
			return false
		}
	}
	return true
}

// PromoteReady scans backlog tasks and promotes those with resolved deps to ready.
func (s *Store) PromoteReady(projectID string) ([]*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var promoted []*Task
	for _, t := range s.tasks {
		if t.ProjectID != projectID || t.Status != TaskBacklog {
			continue
		}
		if s.depsResolved(t) {
			t.Status = TaskReady
			promoted = append(promoted, t)
		}
	}
	if len(promoted) > 0 {
		if err := s.saveTasks(); err != nil {
			return nil, err
		}
	}
	return promoted, nil
}

func (s *Store) loadProjects() error {
	data, err := os.ReadFile(s.projectsPath)
	if err != nil {
		return err
	}
	var f projectsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, p := range f.Projects {
		s.projects[p.ID] = p
	}
	return nil
}

func (s *Store) loadTasks() error {
	data, err := os.ReadFile(s.tasksPath)
	if err != nil {
		return err
	}
	var f tasksFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, t := range f.Tasks {
		s.tasks[t.ID] = t
	}
	return nil
}

func (s *Store) saveProjects() error {
	f := projectsFile{
		Projects: make([]*Project, 0, len(s.projects)),
	}
	for _, p := range s.projects {
		f.Projects = append(f.Projects, p)
	}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(s.projectsPath, data, 0o644)
}

func (s *Store) saveTasks() error {
	f := tasksFile{
		Tasks: make([]*Task, 0, len(s.tasks)),
	}
	for _, t := range s.tasks {
		f.Tasks = append(f.Tasks, t)
	}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(s.tasksPath, data, 0o644)
}

// --- Research Report operations ---

// SaveReport creates or updates a research report (keyed by TaskID).
func (s *Store) SaveReport(r *ResearchReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.ID == "" {
		r.ID = fmt.Sprintf("r%d-%d", time.Now().UnixMilli(), idCounter.Add(1))
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}

	s.reports[r.TaskID] = r
	return s.saveReports()
}

// GetReport returns a report by task ID.
func (s *Store) GetReport(taskID string) (*ResearchReport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.reports[taskID]
	return r, ok
}

// ListReports returns all reports for a project.
func (s *Store) ListReports(projectID string) []*ResearchReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ResearchReport
	for _, r := range s.reports {
		if r.ProjectID == projectID {
			result = append(result, r)
		}
	}
	return result
}

func (s *Store) loadReports() error {
	data, err := os.ReadFile(s.reportsPath)
	if err != nil {
		return err
	}
	var f reportsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, r := range f.Reports {
		s.reports[r.TaskID] = r
	}
	return nil
}

func (s *Store) saveReports() error {
	f := reportsFile{
		Reports: make([]*ResearchReport, 0, len(s.reports)),
	}
	for _, r := range s.reports {
		f.Reports = append(f.Reports, r)
	}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(s.reportsPath, data, 0o644)
}
