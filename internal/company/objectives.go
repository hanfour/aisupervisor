package company

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"gopkg.in/yaml.v3"
)

// ObjectiveStatus tracks the lifecycle of a company objective.
type ObjectiveStatus string

const (
	ObjectiveActive    ObjectiveStatus = "active"
	ObjectiveCompleted ObjectiveStatus = "completed"
	ObjectiveArchived  ObjectiveStatus = "archived"
)

// Objective represents a top-level company goal that drives projects.
type Objective struct {
	ID          string          `yaml:"id" json:"id"`
	Title       string          `yaml:"title" json:"title"`
	Description string          `yaml:"description" json:"description"`
	KeyResults  []KeyResult     `yaml:"keyResults" json:"keyResults"`
	ProjectIDs  []string        `yaml:"projectIds" json:"projectIds"`
	Status      ObjectiveStatus `yaml:"status" json:"status"`
	BudgetLimit int64           `yaml:"budgetLimit" json:"budgetLimit"`
	CreatedAt   time.Time       `yaml:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time       `yaml:"updatedAt" json:"updatedAt"`
}

// KeyResult tracks measurable progress toward an objective.
type KeyResult struct {
	ID      string  `yaml:"id" json:"id"`
	Title   string  `yaml:"title" json:"title"`
	Target  float64 `yaml:"target" json:"target"`
	Current float64 `yaml:"current" json:"current"`
	Unit    string  `yaml:"unit" json:"unit"`
}

// Progress returns the completion ratio (0.0–1.0) for this key result.
func (kr *KeyResult) Progress() float64 {
	if kr.Target <= 0 {
		return 0
	}
	p := kr.Current / kr.Target
	if p > 1 {
		p = 1
	}
	return p
}

type objectivesFile struct {
	Objectives []Objective `yaml:"objectives"`
}

// CreateObjective creates a new company objective.
func (m *Manager) CreateObjective(title, description string, budgetLimit int64) (*Objective, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj := Objective{
		ID:          fmt.Sprintf("obj-%d", time.Now().UnixMilli()),
		Title:       title,
		Description: description,
		Status:      ObjectiveActive,
		BudgetLimit: budgetLimit,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.objectives = append(m.objectives, obj)
	if err := m.saveObjectives(); err != nil {
		// Rollback: remove the appended objective
		m.objectives = m.objectives[:len(m.objectives)-1]
		return nil, fmt.Errorf("save objectives: %w", err)
	}

	m.emit(Event{
		Type:    EventObjectiveCreated,
		Message: m.msgf("Objective %q created", "目標 %q 已建立", title),
	})
	return &obj, nil
}

// ListObjectives returns all objectives.
func (m *Manager) ListObjectives() []Objective {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Objective, len(m.objectives))
	copy(out, m.objectives)
	return out
}

// GetObjective returns a single objective by ID.
func (m *Manager) GetObjective(id string) (*Objective, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.objectives {
		if m.objectives[i].ID == id {
			obj := m.objectives[i]
			return &obj, true
		}
	}
	return nil, false
}

// UpdateObjective updates an objective's fields.
func (m *Manager) UpdateObjective(id, title, description string, status ObjectiveStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.objectives {
		if m.objectives[i].ID == id {
			if title != "" {
				m.objectives[i].Title = title
			}
			if description != "" {
				m.objectives[i].Description = description
			}
			if status != "" {
				m.objectives[i].Status = status
			}
			m.objectives[i].UpdatedAt = time.Now()
			return m.saveObjectives()
		}
	}
	return fmt.Errorf("objective %q not found", id)
}

// UpdateKeyResult updates a specific key result's current value.
func (m *Manager) UpdateKeyResult(objectiveID, krID string, current float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.objectives {
		if m.objectives[i].ID == objectiveID {
			for j := range m.objectives[i].KeyResults {
				if m.objectives[i].KeyResults[j].ID == krID {
					m.objectives[i].KeyResults[j].Current = current
					m.objectives[i].UpdatedAt = time.Now()
					return m.saveObjectives()
				}
			}
			return fmt.Errorf("key result %q not found", krID)
		}
	}
	return fmt.Errorf("objective %q not found", objectiveID)
}

// DeleteObjective removes an objective.
func (m *Manager) DeleteObjective(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.objectives {
		if m.objectives[i].ID == id {
			m.objectives = append(m.objectives[:i], m.objectives[i+1:]...)
			return m.saveObjectives()
		}
	}
	return fmt.Errorf("objective %q not found", id)
}

// LinkProjectToObjective associates a project with an objective.
func (m *Manager) LinkProjectToObjective(objectiveID, projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.objectives {
		if m.objectives[i].ID == objectiveID {
			for _, pid := range m.objectives[i].ProjectIDs {
				if pid == projectID {
					return nil // already linked
				}
			}
			m.objectives[i].ProjectIDs = append(m.objectives[i].ProjectIDs, projectID)
			m.objectives[i].UpdatedAt = time.Now()
			return m.saveObjectives()
		}
	}
	return fmt.Errorf("objective %q not found", objectiveID)
}

func (m *Manager) loadObjectives() error {
	data, err := os.ReadFile(m.objectivesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f objectivesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse objectives: %w", err)
	}
	m.objectives = f.Objectives
	return nil
}

// updateObjectiveProgress updates KeyResult progress for objectives linked to a completed project.
// Safe to call without locks — acquires m.mu internally.
func (m *Manager) updateObjectiveProgress(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.objectives {
		obj := &m.objectives[i]
		if obj.Status != ObjectiveActive {
			continue
		}
		linked := false
		for _, pid := range obj.ProjectIDs {
			if pid == projectID {
				linked = true
				break
			}
		}
		if !linked {
			continue
		}

		// Count completed vs total linked projects
		completed := 0
		total := len(obj.ProjectIDs)
		for _, pid := range obj.ProjectIDs {
			if p, ok := m.projectStore.GetProject(pid); ok {
				if p.Status == project.ProjectCompleted {
					completed++
				}
			}
		}

		// Auto-update any "projects_completed" key results
		for j := range obj.KeyResults {
			kr := &obj.KeyResults[j]
			if kr.Unit == "projects" {
				kr.Current = float64(completed)
			}
		}

		// If all linked projects are completed, mark objective as completed
		if total > 0 && completed == total {
			obj.Status = ObjectiveCompleted
			m.emit(Event{
				Type:    EventObjectiveCompleted,
				Message: m.msgf("Objective %q completed (all %d projects done)", "目標 %q 已完成（全部 %d 個專案完成）", obj.Title, total),
			})
		}

		obj.UpdatedAt = time.Now()
	}
	if err := m.saveObjectives(); err != nil {
		fmt.Printf("warning: failed to save objectives after progress update: %v\n", err)
	}
}

func (m *Manager) saveObjectives() error {
	f := objectivesFile{Objectives: m.objectives}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(m.objectivesPath, data, 0o644)
}

// objectivesPath returns the file path for objectives.
func objectivesFilePath(dataDir string) string {
	return filepath.Join(dataDir, "objectives.yaml")
}
