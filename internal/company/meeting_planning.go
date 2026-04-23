package company

import (
	"context"
	"encoding/json"
	"strings"
)

// RunPlanningMeeting executes a planning meeting to decompose a project into tasks.
// Round 1: Independent proposals (parallel, ExecAPI)
// Round 2: Cross-evaluate proposals (ExecAPI)
// Round 3: Finalize (ExecAPI, chair synthesizes)
// Returns task proposals extracted from the final round.
func (me *MeetingEngine) RunPlanningMeeting(ctx context.Context, m *Meeting, cfg PlanningMeetingConfig) ([]TaskProposal, error) {
	threshold := 0.5

	// Round 1: proposals
	_, _, err := me.RunRound(ctx, m, 1, ExecAPI, threshold)
	if err != nil {
		return nil, err
	}

	// Round 2: cross-evaluate
	_, _, err = me.RunRound(ctx, m, 2, ExecAPI, threshold)
	if err != nil {
		return nil, err
	}

	// Round 3: finalize
	round3, _, err := me.RunRound(ctx, m, 3, ExecAPI, threshold)
	if err != nil {
		return nil, err
	}

	if err := me.Synthesize(ctx, m); err != nil {
		return nil, err
	}

	// Extract task proposals from final round speeches
	return extractTaskProposals(round3), nil
}

// extractTaskProposals parses TaskProposal JSON arrays from speeches.
func extractTaskProposals(round *MeetingRound) []TaskProposal {
	if round == nil {
		return nil
	}
	var proposals []TaskProposal
	for _, s := range round.Speeches {
		// Try to find JSON array of TaskProposal in content
		// Look for ```json ... ``` blocks first
		content := s.Content
		if idx := strings.Index(content, "```json"); idx >= 0 {
			end := strings.Index(content[idx+7:], "```")
			if end >= 0 {
				content = content[idx+7 : idx+7+end]
			}
		} else if idx := strings.Index(content, "[{"); idx >= 0 {
			end := strings.LastIndex(content, "}]")
			if end >= 0 {
				content = content[idx : end+2]
			}
		}
		var tp []TaskProposal
		if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &tp); err == nil {
			proposals = append(proposals, tp...)
		}
	}
	// Deduplicate by title
	seen := map[string]bool{}
	var unique []TaskProposal
	for _, p := range proposals {
		if !seen[p.Title] {
			seen[p.Title] = true
			unique = append(unique, p)
		}
	}
	return unique
}
