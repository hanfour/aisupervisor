package company

import "context"

// RunReviewMeeting executes a review consensus meeting with up to 3 rounds.
// Round 1: Independent review (parallel, ExecCLI or ExecAPI)
// Round 2: Debate on divergent votes (serial discussion via ExecAPI)
// Round 3: Final vote (ExecAPI)
// Consensus threshold: 2/3 (0.67)
func (me *MeetingEngine) RunReviewMeeting(ctx context.Context, m *Meeting, cfg ReviewMeetingConfig) error {
	threshold := 0.67

	// Round 1: independent review
	_, consensus, err := me.RunRound(ctx, m, 1, ExecAPI, threshold)
	if err != nil {
		return err
	}
	if consensus {
		return me.Synthesize(ctx, m)
	}

	// Round 2: debate on divergent votes
	_, consensus, err = me.RunRound(ctx, m, 2, ExecAPI, threshold)
	if err != nil {
		return err
	}
	if consensus {
		return me.Synthesize(ctx, m)
	}

	// Round 3: final vote
	_, _, err = me.RunRound(ctx, m, 3, ExecAPI, threshold)
	if err != nil {
		return err
	}
	return me.Synthesize(ctx, m)
}
