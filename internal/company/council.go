package company

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/knowledge"
)

// CouncilEngine orchestrates the full council review pipeline:
// Phase 0 → Expert Selection → Parallel Dispatch → Merge → Carmack Filter → Verdict.
type CouncilEngine struct {
	chatProvider ai.ChatProvider
	registry     *ExpertRegistry
	conventions  *ConventionStore
	graph        *knowledge.CodeGraph
	language     string
	reviewCfg    config.ReviewConfig
}

// CouncilRequest encapsulates all inputs needed to run a council review.
type CouncilRequest struct {
	Diff      string
	DiffLines int
	FileCount int
	Brief     *ContextBrief
	Phase0    *Phase0Report
}

// RunCouncil executes the full council review pipeline and returns the result.
func (c *CouncilEngine) RunCouncil(ctx context.Context, req CouncilRequest) (*CouncilResult, error) {
	start := time.Now()

	// (a) Early reject: if all phase0 checks failed, return CHANGES_REQUESTED immediately.
	if req.Phase0 != nil && req.Phase0.AllCriticalFailed() {
		phase0Findings := req.Phase0.ToFindings()
		expertFindings := make([]ExpertFinding, 0, len(phase0Findings))
		for _, f := range phase0Findings {
			expertFindings = append(expertFindings, ExpertFinding{
				Finding: f,
				Expert:  "phase0",
			})
		}
		return &CouncilResult{
			Status:      "CHANGES_REQUESTED",
			Summary:     "All Phase 0 mechanical checks failed",
			Findings:    expertFindings,
			ExpertCount: 0,
			Phase0:      req.Phase0,
			Duration:    time.Since(start),
		}, nil
	}

	// (b) Extract changed files from the diff.
	changedFiles := extractChangedFiles(req.Diff)

	// (c) Select experts via the registry.
	selectedExperts := c.registry.SelectExperts(changedFiles, req.Diff, c.graph, req.Phase0)

	// (d) Dispatch experts in parallel.
	expertFindings, err := c.dispatchExperts(ctx, selectedExperts, req.Brief, req.Diff)
	if err != nil {
		return nil, fmt.Errorf("dispatch experts: %w", err)
	}

	// (e) Add phase0 findings to expert findings.
	if req.Phase0 != nil {
		for _, f := range req.Phase0.ToFindings() {
			expertFindings = append(expertFindings, ExpertFinding{
				Finding: f,
				Expert:  "phase0",
			})
		}
	}

	// (f) Merge findings via mergeCouncilFindings.
	merged := mergeCouncilFindings(expertFindings)

	// (g) Apply Carmack filter.
	scale := detectProjectScale(req.FileCount)
	filtered := applyCarmackFilter(merged, CarmackFilterConfig{
		MaxFindings:     15,
		ProjectScale:    scale,
		ConventionStore: c.conventions,
	})

	// (h) Determine verdict mechanically.
	verdict := determineVerdict(filtered)

	// (i) If verdict is ambiguous (1-2 HIGH, no CRITICAL, chatProvider available) → AI summary.
	// (j) Otherwise build summary mechanically.
	summary := c.buildSummary(ctx, filtered, req.Brief, verdict)

	return &CouncilResult{
		Status:      verdict,
		Summary:     summary,
		Findings:    filtered,
		ExpertCount: len(selectedExperts),
		Phase0:      req.Phase0,
		Duration:    time.Since(start),
	}, nil
}

// buildSummary creates a human-readable summary. Uses AI synthesis when the
// verdict is ambiguous (1-2 HIGH findings and chatProvider is available),
// falls back to mechanical summary otherwise.
func (c *CouncilEngine) buildSummary(ctx context.Context, findings []ExpertFinding, brief *ContextBrief, verdict string) string {
	// Count HIGH findings to detect ambiguity.
	highCount := 0
	for _, f := range findings {
		if f.Severity == "HIGH" {
			highCount++
		}
	}

	// Ambiguous case: 1-2 HIGH, no CRITICAL, chatProvider available.
	isAmbiguous := highCount >= 1 && highCount <= 2 && verdict == "APPROVED" && c.chatProvider != nil
	if isAmbiguous {
		aiSummary, err := synthesizeWithAI(ctx, c.chatProvider, findings, brief, c.reviewCfg.SynthesisModel, c.language)
		if err != nil {
			log.Printf("council: AI synthesis failed, using mechanical summary: %v", err)
		} else {
			return aiSummary
		}
	}

	// Mechanical summary.
	return buildMechanicalSummary(findings, verdict)
}

// buildMechanicalSummary constructs a summary string from findings without AI.
func buildMechanicalSummary(findings []ExpertFinding, verdict string) string {
	if len(findings) == 0 {
		return "No issues found. Code looks good."
	}

	criticalCount := 0
	highCount := 0
	mediumCount := 0
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			criticalCount++
		case "HIGH":
			highCount++
		case "MEDIUM":
			mediumCount++
		}
	}

	var parts []string
	if criticalCount > 0 {
		parts = append(parts, fmt.Sprintf("%d CRITICAL", criticalCount))
	}
	if highCount > 0 {
		parts = append(parts, fmt.Sprintf("%d HIGH", highCount))
	}
	if mediumCount > 0 {
		parts = append(parts, fmt.Sprintf("%d MEDIUM", mediumCount))
	}

	summary := fmt.Sprintf("Found %d issue(s): %s.", len(findings), strings.Join(parts, ", "))
	if verdict == "CHANGES_REQUESTED" {
		summary += " Changes requested."
	}
	return summary
}

// dispatchExperts runs all selected experts in parallel, collecting findings.
// Single expert failures are logged but do not abort the entire dispatch.
func (c *CouncilEngine) dispatchExperts(ctx context.Context, experts []SelectedExpert, brief *ContextBrief, diff string) ([]ExpertFinding, error) {
	if len(experts) == 0 {
		return nil, nil
	}

	type expertResult struct {
		findings []ExpertFinding
		err      error
		domain   ExpertDomain
	}

	results := make([]expertResult, len(experts))
	var wg sync.WaitGroup

	for i, expert := range experts {
		wg.Add(1)
		go func(idx int, exp SelectedExpert) {
			defer wg.Done()

			switch exp.Mode {
			case ExecAPI:
				findings, err := c.runExpertAPI(ctx, exp, brief, diff)
				results[idx] = expertResult{findings: findings, err: err, domain: exp.Domain}
			case ExecCLI:
				// CLI mode is a placeholder for Task 9.
				log.Printf("council: skipping CLI expert %s (not yet implemented)", exp.Domain)
				results[idx] = expertResult{domain: exp.Domain}
			default:
				log.Printf("council: unknown exec mode %q for expert %s", exp.Mode, exp.Domain)
				results[idx] = expertResult{domain: exp.Domain}
			}
		}(i, expert)
	}

	wg.Wait()

	// Collect all findings, logging individual failures.
	var allFindings []ExpertFinding
	for _, r := range results {
		if r.err != nil {
			log.Printf("council: expert %s failed: %v", r.domain, r.err)
			continue
		}
		allFindings = append(allFindings, r.findings...)
	}

	return allFindings, nil
}

// runExpertAPI dispatches a single expert via the AI chat API and parses findings.
func (c *CouncilEngine) runExpertAPI(ctx context.Context, expert SelectedExpert, brief *ContextBrief, diff string) ([]ExpertFinding, error) {
	// Apply timeout.
	timeout := time.Duration(c.reviewCfg.APIExpertTimeoutS) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Filter diff to only include expert's assigned files.
	filteredDiff := filterDiffForFiles(diff, expert.AssignedFiles)

	// Build messages.
	var userContent strings.Builder
	if brief != nil {
		userContent.WriteString(brief.Render())
		userContent.WriteString("\n\n---\n\n")
	}
	userContent.WriteString("Diff to review:\n")
	userContent.WriteString(filteredDiff)

	messages := []ai.ChatMessage{
		{Role: "system", Content: expert.SystemPrompt},
		{Role: "user", Content: userContent.String()},
	}

	// Call AI.
	text, err := ai.ChatWithModelOrFallback(timeoutCtx, c.chatProvider, messages, expert.Model)
	if err != nil {
		return nil, fmt.Errorf("expert %s API call: %w", expert.Domain, err)
	}

	// Parse response.
	return parseExpertFindings(text, expert.Domain)
}

// parseExpertFindings extracts ExpertFinding structs from an expert's raw text response.
// It tries several strategies to find a JSON array of findings:
//  1. Direct json.Unmarshal as []Finding
//  2. Extract from ```json ... ``` markdown block
//  3. Find [{ ... }] substring
//
// If no JSON found, returns empty (expert reported no issues).
func parseExpertFindings(raw string, domain ExpertDomain) ([]ExpertFinding, error) {
	raw = strings.TrimSpace(raw)

	// Strategy 1: Direct parse.
	if findings, ok := tryParseFindingsJSON(raw, domain); ok {
		return findings, nil
	}

	// Strategy 2: Extract from markdown ```json ... ``` block.
	if start := strings.Index(raw, "```json"); start != -1 {
		content := raw[start+7:]
		if end := strings.Index(content, "```"); end != -1 {
			extracted := strings.TrimSpace(content[:end])
			if findings, ok := tryParseFindingsJSON(extracted, domain); ok {
				return findings, nil
			}
		}
	}

	// Also try ``` without "json" qualifier.
	if start := strings.Index(raw, "```\n["); start != -1 {
		content := raw[start+4:]
		if end := strings.Index(content, "```"); end != -1 {
			extracted := strings.TrimSpace(content[:end])
			if findings, ok := tryParseFindingsJSON(extracted, domain); ok {
				return findings, nil
			}
		}
	}

	// Strategy 3: Find [{ ... }] substring.
	if start := strings.Index(raw, "[{"); start != -1 {
		// Find the matching closing bracket.
		sub := raw[start:]
		depth := 0
		end := -1
		for i, ch := range sub {
			switch ch {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					end = i + 1
				}
			}
			if end > 0 {
				break
			}
		}
		if end > 0 {
			extracted := sub[:end]
			if findings, ok := tryParseFindingsJSON(extracted, domain); ok {
				return findings, nil
			}
		}
	}

	// No JSON found — expert reported no issues.
	return nil, nil
}

// tryParseFindingsJSON attempts to unmarshal a string as []Finding and convert to []ExpertFinding.
func tryParseFindingsJSON(s string, domain ExpertDomain) ([]ExpertFinding, bool) {
	var findings []Finding
	if err := json.Unmarshal([]byte(s), &findings); err != nil {
		return nil, false
	}

	result := make([]ExpertFinding, 0, len(findings))
	for _, f := range findings {
		result = append(result, ExpertFinding{
			Finding: f,
			Expert:  domain,
		})
	}
	return result, true
}

// filterDiffForFiles filters a unified diff to only include hunks for the specified files.
// If files is nil/empty or contains "*", the full diff is returned.
func filterDiffForFiles(fullDiff string, files []string) string {
	if len(files) == 0 {
		return fullDiff
	}
	for _, f := range files {
		if f == "*" {
			return fullDiff
		}
	}

	// Build a set of target files for fast lookup.
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	// Split diff into per-file sections by "diff --git" headers.
	sections := splitDiffSections(fullDiff)

	var kept []string
	for _, section := range sections {
		// Extract the file path from "+++ b/..." line.
		filePath := extractDiffFilePath(section)
		if filePath != "" && fileSet[filePath] {
			kept = append(kept, section)
		}
	}

	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n")
}

// splitDiffSections splits a unified diff string into per-file sections.
// Each section starts with "diff --git".
func splitDiffSections(diff string) []string {
	const marker = "diff --git"
	lines := strings.Split(diff, "\n")

	var sections []string
	var current []string

	for _, line := range lines {
		if strings.HasPrefix(line, marker) {
			if len(current) > 0 {
				sections = append(sections, strings.Join(current, "\n"))
			}
			current = []string{line}
		} else {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	return sections
}

// extractDiffFilePath extracts the file path from a "+++ b/..." line in a diff section.
func extractDiffFilePath(section string) string {
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			return strings.TrimPrefix(line, "+++ b/")
		}
	}
	return ""
}
