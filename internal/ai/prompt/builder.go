package prompt

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
)

type templateData struct {
	SessionName       string
	TaskGoal          string
	ContextBlock      string
	DiscussionContext string
	PromptType        detector.PromptType
	Summary           string
	Options           []detector.ResponseOption
	PaneContent       string
}

var userTmpl = template.Must(template.New("user").Parse(UserPromptTemplate))

func BuildUserPrompt(req ai.AnalysisRequest) (string, error) {
	var contextBlock string
	if req.SessionContext != nil {
		snap := req.SessionContext.Snapshot()
		contextBlock = renderContext(snap, 2000)
	}

	data := templateData{
		SessionName:       req.SessionName,
		TaskGoal:          req.TaskGoal,
		ContextBlock:      contextBlock,
		DiscussionContext: req.DiscussionContext,
		PromptType:        req.Prompt.Type,
		Summary:           req.Prompt.Summary,
		Options:           req.Prompt.Options,
		PaneContent:       req.PaneContent,
	}

	var buf bytes.Buffer
	if err := userTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderContext builds a text block from the session context snapshot,
// prioritized: project > decisions > activities > rules.
// It truncates to stay within the budget (in characters).
func renderContext(snap sessionctx.SessionContext, budget int) string {
	if budget <= 0 {
		budget = 2000
	}

	var sections []string

	// 1. Project info (highest priority)
	if snap.Project.Directory != "" {
		var parts []string
		parts = append(parts, fmt.Sprintf("Project: %s", snap.Project.Directory))
		if snap.Project.Language != "" {
			parts = append(parts, fmt.Sprintf("Language: %s", snap.Project.Language))
		}
		if snap.Project.Framework != "" {
			parts = append(parts, fmt.Sprintf("Framework: %s", snap.Project.Framework))
		}
		if snap.Project.GitBranch != "" {
			parts = append(parts, fmt.Sprintf("Branch: %s", snap.Project.GitBranch))
		}
		if snap.Project.BuildTool != "" {
			parts = append(parts, fmt.Sprintf("Build: %s", snap.Project.BuildTool))
		}
		sections = append(sections, strings.Join(parts, " | "))
	}

	// 2. Decision history
	if len(snap.Decisions) > 0 {
		var lines []string
		lines = append(lines, "Recent decisions:")
		for _, d := range snap.Decisions {
			action := "approved"
			if d.ChosenKey == "n" || d.ChosenKey == "N" || d.ChosenKey == "2" {
				action = "denied"
			}
			line := fmt.Sprintf("  [%s] %s → %s (%.0f%%)", d.Timestamp.Format("15:04"), d.Summary, action, d.Confidence*100)
			if d.Auto {
				line += " (auto)"
			}
			lines = append(lines, line)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// 3. Activity summaries
	if len(snap.Activities) > 0 {
		var lines []string
		lines = append(lines, "Recent activity:")
		for _, a := range snap.Activities {
			lines = append(lines, fmt.Sprintf("  [%s] %s", a.Timestamp.Format("15:04"), a.Summary))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// 4. Project rules (lowest priority)
	if len(snap.Rules) > 0 {
		var lines []string
		lines = append(lines, "Project rules:")
		for _, r := range snap.Rules {
			lines = append(lines, fmt.Sprintf("  %s: if contains %q → %s", r.Label, r.PatternContains, r.Response))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// Assemble with budget truncation
	result := strings.Join(sections, "\n")
	if len(result) > budget {
		result = result[:budget]
		// Trim to last complete line
		if idx := strings.LastIndex(result, "\n"); idx > 0 {
			result = result[:idx]
		}
		result += "\n[...truncated]"
	}
	return result
}
