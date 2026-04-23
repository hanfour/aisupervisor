package company

import "context"

// RunDebugMeeting executes a debugging meeting to diagnose and resolve task failures.
// Round 1: Diagnosis (parallel, ExecAPI -- hypotheses on root cause)
// Round 2: Evaluate hypotheses (ExecAPI)
// Round 3: Action plan (ExecAPI)
// Returns fix task proposals.
func (me *MeetingEngine) RunDebugMeeting(ctx context.Context, m *Meeting, cfg DebugMeetingConfig) ([]TaskProposal, error) {
	threshold := 0.5

	// Round 1: diagnosis
	_, _, err := me.RunRound(ctx, m, 1, ExecAPI, threshold)
	if err != nil {
		return nil, err
	}

	// Round 2: evaluate hypotheses
	_, _, err = me.RunRound(ctx, m, 2, ExecAPI, threshold)
	if err != nil {
		return nil, err
	}

	// Round 3: action plan
	round3, _, err := me.RunRound(ctx, m, 3, ExecAPI, threshold)
	if err != nil {
		return nil, err
	}

	if err := me.Synthesize(ctx, m); err != nil {
		return nil, err
	}

	// Extract fix task proposals from final round
	return extractTaskProposals(round3), nil
}
