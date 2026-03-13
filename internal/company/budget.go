package company

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// MonthlyBudget tracks token usage for a given month.
type MonthlyBudget struct {
	Month       string `yaml:"month" json:"month"`             // "2026-03"
	TokenBudget int64  `yaml:"tokenBudget" json:"tokenBudget"` // monthly token limit (0 = unlimited)
	TokensUsed  int64  `yaml:"tokensUsed" json:"tokensUsed"`
	TaskCount   int    `yaml:"taskCount" json:"taskCount"`
}

// BudgetSummary provides a snapshot of budget usage for the frontend.
type BudgetSummary struct {
	CurrentMonth string  `json:"currentMonth"`
	TokenBudget  int64   `json:"tokenBudget"`
	TokensUsed   int64   `json:"tokensUsed"`
	TaskCount    int     `json:"taskCount"`
	UsagePercent float64 `json:"usagePercent"`
}

type budgetsFile struct {
	Budgets []MonthlyBudget `yaml:"budgets"`
}

// RecordTokenUsage records token consumption for a task and updates monthly stats.
// Must be called with m.mu held.
func (m *Manager) RecordTokenUsage(taskID string, tokens int64) {
	if tokens <= 0 {
		return
	}

	// Update task (best-effort; log but don't fail)
	if err := m.projectStore.UpdateTaskTokens(taskID, tokens); err != nil {
		fmt.Printf("warning: failed to update task tokens: %v\n", err)
	}

	// Update monthly budget
	month := time.Now().Format("2006-01")
	found := false
	for i := range m.budgets {
		if m.budgets[i].Month == month {
			m.budgets[i].TokensUsed += tokens
			m.budgets[i].TaskCount++
			found = true
			break
		}
	}
	if !found {
		m.budgets = append(m.budgets, MonthlyBudget{
			Month:      month,
			TokensUsed: tokens,
			TaskCount:  1,
		})
	}
	if err := m.saveBudgets(); err != nil {
		fmt.Printf("warning: failed to save budgets: %v\n", err)
	}
}

// GetBudgetSummary returns the current month's budget status.
func (m *Manager) GetBudgetSummary() BudgetSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	month := time.Now().Format("2006-01")
	for _, b := range m.budgets {
		if b.Month == month {
			pct := 0.0
			if b.TokenBudget > 0 {
				pct = float64(b.TokensUsed) / float64(b.TokenBudget) * 100
				if pct > 100 {
					pct = 100
				}
			}
			return BudgetSummary{
				CurrentMonth: month,
				TokenBudget:  b.TokenBudget,
				TokensUsed:   b.TokensUsed,
				TaskCount:    b.TaskCount,
				UsagePercent: pct,
			}
		}
	}
	return BudgetSummary{CurrentMonth: month}
}

// SetMonthlyBudget sets the token budget for the current month.
func (m *Manager) SetMonthlyBudget(tokenBudget int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	month := time.Now().Format("2006-01")
	found := false
	for i := range m.budgets {
		if m.budgets[i].Month == month {
			m.budgets[i].TokenBudget = tokenBudget
			found = true
			break
		}
	}
	if !found {
		m.budgets = append(m.budgets, MonthlyBudget{
			Month:       month,
			TokenBudget: tokenBudget,
		})
	}
	return m.saveBudgets()
}

func (m *Manager) loadBudgets() error {
	data, err := os.ReadFile(m.budgetsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f budgetsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse budgets: %w", err)
	}
	m.budgets = f.Budgets
	return nil
}

func (m *Manager) saveBudgets() error {
	f := budgetsFile{Budgets: m.budgets}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(m.budgetsPath, data, 0o644)
}

func budgetsFilePath(dataDir string) string {
	return filepath.Join(dataDir, "budgets.yaml")
}
