package company

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
)

// CouncilResult is the final output of the council review pipeline.
type CouncilResult struct {
	Status      string          `json:"status"`      // "APPROVED" or "CHANGES_REQUESTED"
	Summary     string          `json:"summary"`
	Findings    []ExpertFinding `json:"findings"`
	ExpertCount int             `json:"expertCount"`
	Phase0      *Phase0Report   `json:"-"`
	Duration    time.Duration   `json:"-"`
	TokensUsed  int64           `json:"tokensUsed,omitempty"`
}

// ProjectScale categorizes repository size for filtering decisions.
type ProjectScale string

const (
	ScaleSmall  ProjectScale = "small"  // < 50 files
	ScaleMedium ProjectScale = "medium" // 50-500 files
	ScaleLarge  ProjectScale = "large"  // > 500 files
)

// CarmackFilterConfig controls the Carmack Filter behavior.
type CarmackFilterConfig struct {
	MaxFindings     int
	ProjectScale    ProjectScale
	ConventionStore *ConventionStore
}

// domainPriority maps pairs of competing domains to the winner.
// When two experts flag the same file:line with similar body, the domain
// with higher priority wins. The map is checked in both orderings.
var domainPriority = map[[2]ExpertDomain]ExpertDomain{
	{DomainSecurity, DomainBackend}:     DomainSecurity,
	{DomainSecurity, DomainAPI}:         DomainSecurity,
	{DomainConcurrency, DomainBackend}:  DomainConcurrency,
	{DomainFrontend, DomainRefactoring}: DomainFrontend,
	{DomainTesting, DomainRefactoring}:  DomainTesting,
	{DomainArchitecture, DomainBackend}: DomainArchitecture,
	{DomainArchitecture, DomainFrontend}: DomainArchitecture,
	{DomainDatabase, DomainBackend}:     DomainDatabase,
	{DomainDatabase, DomainPerformance}: DomainDatabase,
}

// resolveDomainPriority returns the winning domain when two domains conflict
// on the same file:line. Returns the winner and true if a priority exists,
// or empty string and false otherwise.
func resolveDomainPriority(a, b ExpertDomain) (ExpertDomain, bool) {
	if winner, ok := domainPriority[[2]ExpertDomain{a, b}]; ok {
		return winner, true
	}
	if winner, ok := domainPriority[[2]ExpertDomain{b, a}]; ok {
		return winner, true
	}
	return "", false
}

// mergeCouncilFindings deduplicates expert findings by file:line using these rules:
//   - Same file AND same line: keep highest severity; if same severity, keep highest confidence.
//     When domains differ, use domainPriority to pick the winning expert.
//   - Same file, different line, body similarity > 80%: merge into one, keep earlier line.
//   - Different file, similar body: keep both (different locations are worth separate mention).
//
// After merge, findings are sorted by severity descending and assigned sequential IDs.
func mergeCouncilFindings(findings []ExpertFinding) []ExpertFinding {
	if len(findings) == 0 {
		return nil
	}

	// Phase 1: Deduplicate same file:line.
	type fileLineKey struct {
		file string
		line int
	}
	best := make(map[fileLineKey]ExpertFinding)
	order := []fileLineKey{} // preserve insertion order for determinism

	for _, f := range findings {
		k := fileLineKey{f.File, f.Line}
		existing, exists := best[k]
		if !exists {
			order = append(order, k)
			best[k] = f
			continue
		}

		// Same file:line — decide which to keep.
		winner := pickWinner(existing, f)
		best[k] = winner
	}

	// Collect into a slice grouped by file for phase 2.
	intermediate := make([]ExpertFinding, 0, len(best))
	for _, k := range order {
		intermediate = append(intermediate, best[k])
	}

	// Phase 2: Merge same-file, different-line findings with similar body (>80%).
	merged := mergeSimilarSameFile(intermediate)

	// Phase 3: Sort by severity descending (CRITICAL > HIGH > MEDIUM).
	sort.SliceStable(merged, func(i, j int) bool {
		return severityRank(merged[i].Severity) > severityRank(merged[j].Severity)
	})

	// Phase 4: Assign sequential IDs.
	for i := range merged {
		merged[i].ID = fmt.Sprintf("#%d", i+1)
	}

	return merged
}

// pickWinner chooses between two findings at the same file:line.
func pickWinner(a, b ExpertFinding) ExpertFinding {
	rankA := severityRank(a.Severity)
	rankB := severityRank(b.Severity)

	if rankA != rankB {
		if rankB > rankA {
			return b
		}
		return a
	}

	// Same severity — check domain priority if bodies are similar.
	if a.Expert != b.Expert && wordSimilarity(a.Body, b.Body) > 0.4 {
		if winner, ok := resolveDomainPriority(a.Expert, b.Expert); ok {
			if winner == b.Expert {
				return b
			}
			return a
		}
	}

	// Same severity, no domain priority — keep highest confidence.
	if b.Confidence > a.Confidence {
		return b
	}
	return a
}

// mergeSimilarSameFile merges findings that are in the same file but different lines
// when their body similarity exceeds 80%. The merged finding keeps the earlier line.
func mergeSimilarSameFile(findings []ExpertFinding) []ExpertFinding {
	if len(findings) <= 1 {
		return findings
	}

	// Group by file.
	fileGroups := make(map[string][]int) // file -> indices
	for i, f := range findings {
		fileGroups[f.File] = append(fileGroups[f.File], i)
	}

	merged := make(map[int]bool) // indices that have been merged into another

	for _, indices := range fileGroups {
		if len(indices) < 2 {
			continue
		}
		for i := 0; i < len(indices); i++ {
			if merged[indices[i]] {
				continue
			}
			for j := i + 1; j < len(indices); j++ {
				if merged[indices[j]] {
					continue
				}
				a := findings[indices[i]]
				b := findings[indices[j]]
				if a.Line != b.Line && wordSimilarity(a.Body, b.Body) > 0.80 {
					// Merge: keep earlier line.
					if b.Line < a.Line {
						findings[indices[i]].Line = b.Line
					}
					// Keep higher severity if they differ.
					if severityRank(b.Severity) > severityRank(a.Severity) {
						findings[indices[i]].Severity = b.Severity
					}
					merged[indices[j]] = true
				}
			}
		}
	}

	result := make([]ExpertFinding, 0, len(findings))
	for i, f := range findings {
		if !merged[i] {
			result = append(result, f)
		}
	}
	return result
}

// applyCarmackFilter applies the Carmack Filter — a set of economic relevance
// filters named after John Carmack's principle that code review effort should
// be proportional to actual project risk.
//
// Steps:
//
//	(a) Convention filtering — remove findings matching accepted conventions.
//	(b) Scale filtering — remove noise inappropriate for project size.
//	(c) Pattern compression — collapse repetitive findings into one.
//	(d) Cap at MaxFindings, sorted by severity.
func applyCarmackFilter(findings []ExpertFinding, cfg CarmackFilterConfig) []ExpertFinding {
	if len(findings) == 0 {
		return nil
	}

	maxFindings := cfg.MaxFindings
	if maxFindings == 0 {
		maxFindings = 15
	}

	result := make([]ExpertFinding, len(findings))
	copy(result, findings)

	// (a) Convention filtering.
	if cfg.ConventionStore != nil {
		var kept []ExpertFinding
		for _, f := range result {
			if cfg.ConventionStore.MatchesFinding(f) == nil {
				kept = append(kept, f)
			}
		}
		result = kept
	}

	// (b) Scale filtering.
	result = applyScaleFilter(result, cfg.ProjectScale)

	// (c) Pattern compression.
	result = compressPatterns(result)

	// (d) Sort by severity and cap.
	sort.SliceStable(result, func(i, j int) bool {
		return severityRank(result[i].Severity) > severityRank(result[j].Severity)
	})

	if len(result) > maxFindings {
		result = result[:maxFindings]
	}

	return result
}

// applyScaleFilter removes findings that are economically irrelevant for the project scale.
func applyScaleFilter(findings []ExpertFinding, scale ProjectScale) []ExpertFinding {
	switch scale {
	case ScaleSmall:
		// Remove MEDIUM performance findings — small projects don't need perf nitpicks.
		var kept []ExpertFinding
		for _, f := range findings {
			if f.Severity == "MEDIUM" && f.Expert == DomainPerformance {
				continue
			}
			kept = append(kept, f)
		}
		return kept

	case ScaleMedium:
		// Keep all CRITICAL and HIGH. Cap MEDIUM at 5.
		var critical, high, medium []ExpertFinding
		for _, f := range findings {
			switch f.Severity {
			case "CRITICAL":
				critical = append(critical, f)
			case "HIGH":
				high = append(high, f)
			default:
				medium = append(medium, f)
			}
		}
		if len(medium) > 5 {
			medium = medium[:5]
		}
		result := make([]ExpertFinding, 0, len(critical)+len(high)+len(medium))
		result = append(result, critical...)
		result = append(result, high...)
		result = append(result, medium...)
		return result

	case ScaleLarge:
		// Keep all.
		return findings

	default:
		// Unknown scale — keep all.
		return findings
	}
}

// compressPatterns detects repetitive findings: if 3+ findings share the same
// Expert domain AND have pairwise body similarity > 0.6, they are compressed
// into a single finding listing all locations.
func compressPatterns(findings []ExpertFinding) []ExpertFinding {
	if len(findings) < 3 {
		return findings
	}

	// Group by expert domain.
	domainGroups := make(map[ExpertDomain][]int) // domain -> indices
	for i, f := range findings {
		domainGroups[f.Expert] = append(domainGroups[f.Expert], i)
	}

	compressed := make(map[int]bool) // indices consumed by compression

	var newFindings []ExpertFinding

	for _, indices := range domainGroups {
		if len(indices) < 3 {
			continue
		}

		// Find clusters of similar findings within this domain.
		clusters := clusterBySimilarity(findings, indices, 0.6)
		for _, cluster := range clusters {
			if len(cluster) < 3 {
				continue
			}

			// Compress cluster into one finding.
			first := findings[cluster[0]]
			var locations []string
			highestSeverity := first.Severity
			for _, idx := range cluster {
				f := findings[idx]
				locations = append(locations, fmt.Sprintf("%s:%d", f.File, f.Line))
				if severityRank(f.Severity) > severityRank(highestSeverity) {
					highestSeverity = f.Severity
				}
				compressed[idx] = true
			}

			newFindings = append(newFindings, ExpertFinding{
				Finding: Finding{
					File:     first.File,
					Line:     first.Line,
					Severity: highestSeverity,
					Body:     fmt.Sprintf("%s (%d occurrences: %s)", first.Body, len(cluster), strings.Join(locations, ", ")),
				},
				Expert:     first.Expert,
				Confidence: first.Confidence,
			})
		}
	}

	// Collect uncompressed findings.
	result := make([]ExpertFinding, 0, len(findings))
	for i, f := range findings {
		if !compressed[i] {
			result = append(result, f)
		}
	}
	result = append(result, newFindings...)

	return result
}

// clusterBySimilarity groups indices into clusters where each member has
// body similarity > threshold with the cluster's first member.
func clusterBySimilarity(findings []ExpertFinding, indices []int, threshold float64) [][]int {
	used := make(map[int]bool)
	var clusters [][]int

	for _, i := range indices {
		if used[i] {
			continue
		}
		cluster := []int{i}
		used[i] = true

		for _, j := range indices {
			if used[j] {
				continue
			}
			if wordSimilarity(findings[i].Body, findings[j].Body) > threshold {
				cluster = append(cluster, j)
				used[j] = true
			}
		}
		clusters = append(clusters, cluster)
	}

	return clusters
}

// determineVerdict mechanically determines the review status from findings:
//   - 0 findings: "APPROVED"
//   - Any CRITICAL: "CHANGES_REQUESTED"
//   - >= 3 HIGH: "CHANGES_REQUESTED"
//   - 1-2 HIGH, no CRITICAL: "APPROVED" (with notes)
//   - All MEDIUM: "APPROVED"
func determineVerdict(findings []ExpertFinding) string {
	if len(findings) == 0 {
		return "APPROVED"
	}

	criticalCount := 0
	highCount := 0
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			criticalCount++
		case "HIGH":
			highCount++
		}
	}

	if criticalCount > 0 {
		return "CHANGES_REQUESTED"
	}
	if highCount >= 3 {
		return "CHANGES_REQUESTED"
	}
	return "APPROVED"
}

// detectProjectScale categorizes repository size by file count.
func detectProjectScale(fileCount int) ProjectScale {
	switch {
	case fileCount < 50:
		return ScaleSmall
	case fileCount <= 500:
		return ScaleMedium
	default:
		return ScaleLarge
	}
}

// synthesizeWithAI makes a single API call to produce a human-readable summary
// of the review findings. Used when the verdict is ambiguous (1-2 HIGH findings).
func synthesizeWithAI(ctx context.Context, cp ai.ChatProvider, findings []ExpertFinding, brief *ContextBrief, model, lang string) (string, error) {
	var sb strings.Builder
	sb.WriteString("Summarize these code review findings into a concise paragraph.\n")
	sb.WriteString("Focus on what matters most and whether the issues are blocking.\n\n")

	if brief != nil {
		sb.WriteString("Context:\n")
		sb.WriteString(brief.Render())
		sb.WriteString("\n\n")
	}

	sb.WriteString("Findings:\n")
	for _, f := range findings {
		sb.WriteString(fmt.Sprintf("- [%s] %s:%d (%s) — %s\n", f.Severity, f.File, f.Line, f.Expert, f.Body))
	}

	systemPrompt := "You are a senior code reviewer writing a concise summary of review findings. Be direct and actionable."
	if lang != "en" {
		systemPrompt = "你是一位資深程式碼審查員，撰寫審查結果的簡潔摘要。請直接且可操作。"
	}

	msgs := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: sb.String()},
	}

	text, err := ai.ChatWithModelOrFallback(ctx, cp, msgs, model)
	if err != nil {
		return "", fmt.Errorf("synthesize with AI: %w", err)
	}

	return strings.TrimSpace(text), nil
}
