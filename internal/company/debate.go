package company

import (
	"fmt"
	"sort"
)

type ReviewStrategy string

const (
	ReviewLight    ReviewStrategy = "light"
	ReviewStandard ReviewStrategy = "standard"
	ReviewDebate   ReviewStrategy = "debate"
)

type Finding struct {
	ID       string `json:"id"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"`
	Body     string `json:"body"`
	Source   string `json:"source"`
}

type DebateResult struct {
	Status   string    `json:"status"`
	Summary  string    `json:"summary"`
	Comments []Finding `json:"comments"`
}

func selectStrategy(diffLines, fileCount int, debateThreshold, lightMaxLines, lightMaxFiles int) ReviewStrategy {
	if diffLines >= debateThreshold {
		return ReviewDebate
	}
	if diffLines < lightMaxLines && fileCount <= lightMaxFiles {
		return ReviewLight
	}
	return ReviewStandard
}

func severityRank(s string) int {
	switch s {
	case "CRITICAL":
		return 3
	case "HIGH":
		return 2
	case "MEDIUM":
		return 1
	default:
		return 0
	}
}

// mergeFindings deduplicates findings by file:line, keeping higher severity.
func mergeFindings(all []Finding) []Finding {
	type key struct {
		file string
		line int
	}
	best := make(map[key]Finding)
	for _, f := range all {
		k := key{f.File, f.Line}
		if existing, ok := best[k]; !ok || severityRank(f.Severity) > severityRank(existing.Severity) {
			best[k] = f
		}
	}

	result := make([]Finding, 0, len(best))
	for _, f := range best {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		return severityRank(result[i].Severity) > severityRank(result[j].Severity)
	})

	for i := range result {
		result[i].ID = fmt.Sprintf("#%d", i+1)
	}
	return result
}

// tallyVotes returns findings that survive voting.
// A finding survives if net score (KEEP=+1, DROP=-1) >= 1.
func tallyVotes(findings []Finding, votes ...map[string]string) []Finding {
	scores := make(map[string]int)
	for _, v := range votes {
		for id, decision := range v {
			switch decision {
			case "KEEP":
				scores[id]++
			case "DROP":
				scores[id]--
			}
		}
	}

	var survived []Finding
	for _, f := range findings {
		if scores[f.ID] >= 1 {
			survived = append(survived, f)
		}
	}
	return survived
}
