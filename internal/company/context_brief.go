package company

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const charsPerToken = 4

// ContextBrief is the unified context document passed to review agents.
// It replaces scattered prompt assembly with a structured, token-budgeted brief.
type ContextBrief struct {
	TaskID      string
	ProjectID   string
	GeneratedAt time.Time
	Sections    []BriefSection
	FilePath    string // path where brief was written (for CLI mode)
}

// BriefSection is a single named section within a context brief.
type BriefSection struct {
	Name        string // "project_summary", "diff_stats", "architecture",
	            //        "phase0_results", "conventions", "graph_context",
	            //        "rejection_history", "karpathy_overlay"
	Content     string
	TokenBudget int // max tokens for this section
	Priority    int // 1=never truncate, 4=truncate first
}

// ContextBriefBuilder assembles a ContextBrief from individual fields.
type ContextBriefBuilder struct {
	totalBudget      int // default 6000 tokens (~24000 chars)
	taskID           string
	projectID        string
	projectName      string
	techStack        string
	baseBranch       string
	diffStats        string
	changedFiles     []string
	architectureCtx  string
	phase0Summary    string
	conventions      string
	graphContext      string
	rejectionHistory string
	karpathyOverlay  string
}

// truncate reduces Content to fit within TokenBudget * charsPerToken.
// If the content exceeds the budget, it keeps the first half and last half
// with a truncation marker in between.
func (s *BriefSection) truncate() {
	maxChars := s.TokenBudget * charsPerToken
	if len(s.Content) <= maxChars {
		return
	}

	marker := "... (truncated) ..."
	available := maxChars - len(marker)
	if available < 0 {
		s.Content = marker[:maxChars]
		return
	}

	half := available / 2
	s.Content = s.Content[:half] + marker + s.Content[len(s.Content)-half:]
}

// Render produces a markdown representation of the context brief.
// Empty sections are skipped.
func (b *ContextBrief) Render() string {
	var sb strings.Builder

	sb.WriteString("# Context Brief\n\n")
	sb.WriteString(fmt.Sprintf("Task: %s | Project: %s | Generated: %s\n",
		b.TaskID, b.ProjectID, b.GeneratedAt.Format(time.RFC3339)))

	for _, s := range b.Sections {
		if strings.TrimSpace(s.Content) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n## %s\n\n%s\n", sectionTitle(s.Name), s.Content))
	}

	return sb.String()
}

// WriteToDir writes the rendered brief to a `.ais-review/context-brief.md` file
// inside the given directory. It sets b.FilePath and returns the written path.
func (b *ContextBrief) WriteToDir(dir string) (string, error) {
	reviewDir := filepath.Join(dir, ".ais-review")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		return "", fmt.Errorf("create .ais-review dir: %w", err)
	}

	filePath := filepath.Join(reviewDir, "context-brief.md")
	content := b.Render()

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write context-brief.md: %w", err)
	}

	b.FilePath = filePath
	return filePath, nil
}

// Build constructs a ContextBrief from the builder's fields.
// It creates sections with predefined budgets and priorities, truncates each
// to its own budget, then enforces the global token budget.
func (b *ContextBriefBuilder) Build() (*ContextBrief, error) {
	if b.totalBudget == 0 {
		b.totalBudget = 6000
	}

	sections := []BriefSection{
		{Name: "project_summary", Content: b.buildProjectSummary(), TokenBudget: 500, Priority: 1},
		{Name: "diff_stats", Content: b.diffStats, TokenBudget: 200, Priority: 1},
		{Name: "architecture", Content: b.architectureCtx, TokenBudget: 1500, Priority: 2},
		{Name: "phase0_results", Content: b.phase0Summary, TokenBudget: 800, Priority: 1},
		{Name: "conventions", Content: b.conventions, TokenBudget: 1000, Priority: 3},
		{Name: "rejection_history", Content: b.rejectionHistory, TokenBudget: 500, Priority: 4},
		{Name: "karpathy_overlay", Content: b.karpathyOverlay, TokenBudget: 500, Priority: 4},
	}

	// Optionally include graph_context if present
	if b.graphContext != "" {
		sections = append(sections, BriefSection{
			Name: "graph_context", Content: b.graphContext, TokenBudget: 500, Priority: 3,
		})
	}

	// Truncate each section to its own budget
	for i := range sections {
		sections[i].truncate()
	}

	// Enforce the global budget
	b.enforceGlobalBudget(sections)

	return &ContextBrief{
		TaskID:      b.taskID,
		ProjectID:   b.projectID,
		GeneratedAt: time.Now(),
		Sections:    sections,
	}, nil
}

// buildProjectSummary formats the project summary from builder fields.
func (b *ContextBriefBuilder) buildProjectSummary() string {
	var parts []string

	if b.projectName != "" {
		parts = append(parts, fmt.Sprintf("**Project**: %s", b.projectName))
	}
	if b.techStack != "" {
		parts = append(parts, fmt.Sprintf("**Tech Stack**: %s", b.techStack))
	}
	if b.baseBranch != "" {
		parts = append(parts, fmt.Sprintf("**Base Branch**: %s", b.baseBranch))
	}
	if len(b.changedFiles) > 0 {
		parts = append(parts, fmt.Sprintf("**Changed Files**: %s", strings.Join(b.changedFiles, ", ")))
	}

	return strings.Join(parts, "\n")
}

// enforceGlobalBudget compresses sections if the total character count exceeds
// totalBudget * charsPerToken. It compresses from highest priority number
// (lowest importance) first.
func (b *ContextBriefBuilder) enforceGlobalBudget(sections []BriefSection) {
	globalMax := b.totalBudget * charsPerToken

	totalChars := 0
	for _, s := range sections {
		totalChars += len(s.Content)
	}

	if totalChars <= globalMax {
		return
	}

	// Compress from highest priority number (least important) first
	for pri := 4; pri >= 1 && totalChars > globalMax; pri-- {
		for i := range sections {
			if sections[i].Priority != pri {
				continue
			}
			if totalChars <= globalMax {
				break
			}

			excess := totalChars - globalMax
			currentLen := len(sections[i].Content)
			if currentLen == 0 {
				continue
			}

			targetLen := currentLen - excess
			if targetLen < 100 {
				targetLen = 100
			}
			if targetLen >= currentLen {
				continue
			}

			marker := "... (budget truncated) ..."
			available := targetLen - len(marker)
			if available < 0 {
				// Section too small for marker; just keep marker or minimum
				totalChars -= currentLen - len(marker)
				sections[i].Content = marker
				continue
			}

			half := available / 2
			oldLen := len(sections[i].Content)
			sections[i].Content = sections[i].Content[:half] + marker + sections[i].Content[oldLen-half:]
			totalChars -= oldLen - len(sections[i].Content)
		}
	}
}

// sectionTitle maps internal section names to human-readable titles.
func sectionTitle(name string) string {
	titles := map[string]string{
		"project_summary":  "Project Summary",
		"diff_stats":       "Diff Stats",
		"architecture":     "Architecture",
		"phase0_results":   "Phase 0 Results",
		"conventions":      "Conventions",
		"graph_context":    "Graph Context",
		"rejection_history": "Rejection History",
		"karpathy_overlay": "Karpathy Overlay",
	}

	if title, ok := titles[name]; ok {
		return title
	}

	// Fallback: capitalize words and replace underscores
	words := strings.Split(name, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
