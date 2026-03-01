package group

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/role"
)

// DiscussionPhase represents a phase in the two-phase discussion process.
type DiscussionPhase string

const (
	PhaseOpinion    DiscussionPhase = "opinion"
	PhaseRoundtable DiscussionPhase = "roundtable"
	PhaseDecision   DiscussionPhase = "decision"
)

// DiscussionEvent is emitted during a group discussion for real-time UI updates.
type DiscussionEvent struct {
	DiscussionID string          `json:"discussionId"`
	SessionID    string          `json:"sessionId"`
	GroupID      string          `json:"groupId"`
	Phase        DiscussionPhase `json:"phase"`
	RoleID       string          `json:"roleId"`
	RoleName     string          `json:"roleName"`
	Message      string          `json:"message"`
	Action       string          `json:"action"`
	Confidence   float64         `json:"confidence"`
	Timestamp    time.Time       `json:"timestamp"`
}

// RoundtableMessage is a single message in the roundtable discussion.
type RoundtableMessage struct {
	RoleID    string
	RoleName  string
	Message   string
	Action    string
	Confidence float64
}

// Discussion tracks a complete group discussion.
type Discussion struct {
	ID             string
	GroupID        string
	SessionID      string
	Phase          DiscussionPhase
	Opinions       []Opinion
	RoundtableMsgs []RoundtableMessage
	FinalDecision  *role.Intervention
	CreatedAt      time.Time
}

// RunDiscussion executes the two-phase discussion protocol for a group.
func (m *Manager) RunDiscussion(ctx context.Context, grp *Group, obs role.Observation, sessionID string) (*role.Intervention, error) {
	discussionID := fmt.Sprintf("%s-%s-%d", grp.ID, sessionID, time.Now().UnixMilli())

	disc := &Discussion{
		ID:        discussionID,
		GroupID:   grp.ID,
		SessionID: sessionID,
		Phase:     PhaseOpinion,
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	m.activeDiscussions[discussionID] = disc
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.activeDiscussions, discussionID)
		m.mu.Unlock()
	}()

	roles := m.resolveGroupRoles(grp)
	if len(roles) == 0 {
		return nil, nil
	}

	// Phase 1: Collect opinions in parallel
	opinions, err := m.collectOpinions(ctx, disc, roles, obs)
	if err != nil {
		return nil, fmt.Errorf("opinion phase: %w", err)
	}
	disc.Opinions = opinions

	if len(opinions) == 0 {
		return nil, nil
	}

	// If only one opinion, use it directly
	if len(opinions) == 1 {
		return m.makeDecision(disc, grp, roles, opinions[0]), nil
	}

	// Divergence detection
	divResult := DetectDivergence(opinions, grp.DivergenceThreshold)

	if !divResult.Divergent {
		// No divergence — use highest-priority opinion
		return m.makeDecision(disc, grp, roles, opinions[0]), nil
	}

	// Phase 2: Roundtable discussion (sequential, by priority)
	disc.Phase = PhaseRoundtable
	roundtableMsgs, err := m.runRoundtable(ctx, disc, grp, roles, obs, opinions)
	if err != nil {
		return nil, fmt.Errorf("roundtable phase: %w", err)
	}
	disc.RoundtableMsgs = roundtableMsgs

	// Phase 3: Leader decision
	disc.Phase = PhaseDecision
	leaderDecision, err := m.leaderDecide(ctx, disc, grp, roles, obs, opinions, roundtableMsgs)
	if err != nil {
		return nil, fmt.Errorf("leader decision: %w", err)
	}

	disc.FinalDecision = leaderDecision
	return leaderDecision, nil
}

// opinionTimeout is the maximum time to wait for a single role's opinion.
const opinionTimeout = 30 * time.Second

// collectOpinions asks each role for its opinion in parallel with timeout protection.
func (m *Manager) collectOpinions(ctx context.Context, disc *Discussion, roles []role.Role, obs role.Observation) ([]Opinion, error) {
	type result struct {
		opinion Opinion
		err     error
		idx     int
	}

	ch := make(chan result, len(roles))

	for i, r := range roles {
		go func(idx int, r role.Role) {
			// Per-role timeout
			evalCtx, cancel := context.WithTimeout(ctx, opinionTimeout)
			defer cancel()

			iv, err := r.Evaluate(evalCtx, obs)
			if err != nil {
				ch <- result{err: fmt.Errorf("role %s: %w", r.ID(), err), idx: idx}
				return
			}

			op := Opinion{
				RoleID:   r.ID(),
				RoleName: r.Name(),
			}
			if iv != nil {
				op.Action = iv.OptionKey
				if iv.Type == role.InterventionFreeText {
					op.Action = iv.Text
				}
				op.Reasoning = iv.Reasoning
				op.Confidence = iv.Confidence
			}

			// Emit opinion event
			m.emitEvent(DiscussionEvent{
				DiscussionID: disc.ID,
				SessionID:    disc.SessionID,
				GroupID:      disc.GroupID,
				Phase:        PhaseOpinion,
				RoleID:       r.ID(),
				RoleName:     r.Name(),
				Message:      op.Reasoning,
				Action:       op.Action,
				Confidence:   op.Confidence,
				Timestamp:    time.Now(),
			})

			ch <- result{opinion: op, idx: idx}
		}(i, r)
	}

	// Collect results with overall context cancellation awareness
	opinions := make([]Opinion, 0, len(roles))
	var firstErr error
	for range roles {
		select {
		case res := <-ch:
			if res.err != nil {
				if firstErr == nil {
					firstErr = res.err
				}
				continue
			}
			if res.opinion.Action != "" {
				opinions = append(opinions, res.opinion)
			}
		case <-ctx.Done():
			if len(opinions) > 0 {
				return opinions, nil
			}
			return nil, ctx.Err()
		}
	}

	if len(opinions) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return opinions, nil
}

// runRoundtable conducts sequential discussion where each role sees previous opinions.
func (m *Manager) runRoundtable(ctx context.Context, disc *Discussion, grp *Group, roles []role.Role, obs role.Observation, opinions []Opinion) ([]RoundtableMessage, error) {
	var msgs []RoundtableMessage

	for _, r := range roles {
		// Build discussion context from opinions + previous roundtable messages
		discussionCtx := buildDiscussionContext(opinions, msgs)

		// Create observation with discussion context
		roundtableObs := obs
		roundtableObs.DiscussionContext = discussionCtx

		iv, err := r.Evaluate(ctx, roundtableObs)
		if err != nil {
			return nil, fmt.Errorf("roundtable role %s: %w", r.ID(), err)
		}

		msg := RoundtableMessage{
			RoleID:   r.ID(),
			RoleName: r.Name(),
		}
		if iv != nil {
			msg.Action = iv.OptionKey
			if iv.Type == role.InterventionFreeText {
				msg.Action = iv.Text
			}
			msg.Message = iv.Reasoning
			msg.Confidence = iv.Confidence
		}
		msgs = append(msgs, msg)

		// Emit roundtable event
		m.emitEvent(DiscussionEvent{
			DiscussionID: disc.ID,
			SessionID:    disc.SessionID,
			GroupID:      disc.GroupID,
			Phase:        PhaseRoundtable,
			RoleID:       r.ID(),
			RoleName:     r.Name(),
			Message:      msg.Message,
			Action:       msg.Action,
			Confidence:   msg.Confidence,
			Timestamp:    time.Now(),
		})
	}

	return msgs, nil
}

// leaderDecide asks the leader role to make the final decision.
func (m *Manager) leaderDecide(ctx context.Context, disc *Discussion, grp *Group, roles []role.Role, obs role.Observation, opinions []Opinion, roundtableMsgs []RoundtableMessage) (*role.Intervention, error) {
	// Find leader role
	var leader role.Role
	for _, r := range roles {
		if r.ID() == grp.LeaderID {
			leader = r
			break
		}
	}
	if leader == nil {
		// Fallback to highest-priority role
		leader = roles[0]
	}

	// Build full context for leader
	discussionCtx := buildLeaderContext(opinions, roundtableMsgs)

	leaderObs := obs
	leaderObs.DiscussionContext = discussionCtx

	iv, err := leader.Evaluate(ctx, leaderObs)
	if err != nil {
		return nil, fmt.Errorf("leader %s: %w", leader.ID(), err)
	}

	if iv != nil {
		iv.RoleID = leader.ID()
	}

	// Emit decision event
	action := ""
	reasoning := ""
	confidence := 0.0
	if iv != nil {
		action = iv.OptionKey
		reasoning = iv.Reasoning
		confidence = iv.Confidence
	}
	m.emitEvent(DiscussionEvent{
		DiscussionID: disc.ID,
		SessionID:    disc.SessionID,
		GroupID:      disc.GroupID,
		Phase:        PhaseDecision,
		RoleID:       leader.ID(),
		RoleName:     leader.Name(),
		Message:      reasoning,
		Action:       action,
		Confidence:   confidence,
		Timestamp:    time.Now(),
	})

	return iv, nil
}

// makeDecision creates a final intervention from a single opinion (no roundtable needed).
func (m *Manager) makeDecision(disc *Discussion, grp *Group, roles []role.Role, opinion Opinion) *role.Intervention {
	disc.Phase = PhaseDecision

	iv := &role.Intervention{
		Type:       role.InterventionSelectOption,
		OptionKey:  opinion.Action,
		Reasoning:  opinion.Reasoning,
		Confidence: opinion.Confidence,
		RoleID:     opinion.RoleID,
	}

	// Find priority
	for _, r := range roles {
		if r.ID() == opinion.RoleID {
			iv.Priority = r.Priority()
			break
		}
	}

	m.emitEvent(DiscussionEvent{
		DiscussionID: disc.ID,
		SessionID:    disc.SessionID,
		GroupID:      disc.GroupID,
		Phase:        PhaseDecision,
		RoleID:       opinion.RoleID,
		RoleName:     opinion.RoleName,
		Message:      opinion.Reasoning,
		Action:       opinion.Action,
		Confidence:   opinion.Confidence,
		Timestamp:    time.Now(),
	})

	disc.FinalDecision = iv
	return iv
}

// buildDiscussionContext formats opinions and prior roundtable messages for injection into prompts.
func buildDiscussionContext(opinions []Opinion, priorMsgs []RoundtableMessage) string {
	var b strings.Builder

	b.WriteString("=== Group Discussion ===\n\nInitial Opinions:\n")
	for _, op := range opinions {
		fmt.Fprintf(&b, "- %s (%s): action=%q confidence=%.2f reason=%s\n",
			op.RoleName, op.RoleID, op.Action, op.Confidence, op.Reasoning)
	}

	if len(priorMsgs) > 0 {
		b.WriteString("\nRoundtable Discussion:\n")
		for _, msg := range priorMsgs {
			fmt.Fprintf(&b, "- %s (%s): action=%q confidence=%.2f — %s\n",
				msg.RoleName, msg.RoleID, msg.Action, msg.Confidence, msg.Message)
		}
	}

	b.WriteString("\nConsidering all perspectives, what is your recommendation?")
	return b.String()
}

// buildLeaderContext formats the full discussion for the leader's final decision.
func buildLeaderContext(opinions []Opinion, roundtableMsgs []RoundtableMessage) string {
	var b strings.Builder

	b.WriteString("=== Group Discussion Summary (Leader Decision) ===\n\nInitial Opinions:\n")
	for _, op := range opinions {
		fmt.Fprintf(&b, "- %s (%s): action=%q confidence=%.2f reason=%s\n",
			op.RoleName, op.RoleID, op.Action, op.Confidence, op.Reasoning)
	}

	if len(roundtableMsgs) > 0 {
		b.WriteString("\nRoundtable Discussion:\n")
		for _, msg := range roundtableMsgs {
			fmt.Fprintf(&b, "- %s (%s): action=%q confidence=%.2f — %s\n",
				msg.RoleName, msg.RoleID, msg.Action, msg.Confidence, msg.Message)
		}
	}

	b.WriteString("\nAs the group leader, please make the FINAL decision considering all opinions and discussion above.")
	return b.String()
}
