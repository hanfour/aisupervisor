package context

import (
	"sync"
	"time"
)

// ProjectInfo holds detected project metadata.
type ProjectInfo struct {
	Directory string   `yaml:"directory,omitempty"`
	GitBranch string   `yaml:"git_branch,omitempty"`
	GitRemote string   `yaml:"git_remote,omitempty"`
	Language  string   `yaml:"language,omitempty"`
	Framework string   `yaml:"framework,omitempty"`
	BuildTool string   `yaml:"build_tool,omitempty"`
	Files     []string `yaml:"-"` // not persisted
}

// DecisionRecord stores a past AI or auto-approve decision.
type DecisionRecord struct {
	Timestamp  time.Time `yaml:"timestamp"`
	Summary    string    `yaml:"summary"`
	ChosenKey  string    `yaml:"chosen_key"`
	Reasoning  string    `yaml:"reasoning"`
	Confidence float64   `yaml:"confidence"`
	Auto       bool      `yaml:"auto,omitempty"`
}

// ActivitySummary is a periodic snapshot of pane activity.
type ActivitySummary struct {
	Timestamp time.Time `yaml:"timestamp"`
	Summary   string    `yaml:"summary"`
}

// ProjectRule is a per-project auto-approve rule.
type ProjectRule struct {
	Label           string `yaml:"label"`
	PatternContains string `yaml:"pattern_contains"`
	Response        string `yaml:"response"`
}

// SessionContext tracks cumulative context for a single monitored session.
type SessionContext struct {
	SessionID  string            `yaml:"session_id"`
	Project    ProjectInfo       `yaml:"project"`
	Decisions  []DecisionRecord  `yaml:"decisions,omitempty"`
	Activities []ActivitySummary `yaml:"activities,omitempty"`
	Rules      []ProjectRule     `yaml:"rules,omitempty"`

	maxDecisions  int
	maxActivities int
	mu            sync.RWMutex
}

// NewSessionContext creates a SessionContext with the given limits.
func NewSessionContext(sessionID string, maxDecisions, maxActivities int) *SessionContext {
	if maxDecisions <= 0 {
		maxDecisions = 20
	}
	if maxActivities <= 0 {
		maxActivities = 10
	}
	return &SessionContext{
		SessionID:     sessionID,
		maxDecisions:  maxDecisions,
		maxActivities: maxActivities,
	}
}

// SetProject updates the project info (thread-safe).
func (sc *SessionContext) SetProject(p ProjectInfo) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Project = p
}

// AddDecision appends a decision and trims to maxDecisions (thread-safe).
func (sc *SessionContext) AddDecision(d DecisionRecord) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Decisions = append(sc.Decisions, d)
	if len(sc.Decisions) > sc.maxDecisions {
		sc.Decisions = sc.Decisions[len(sc.Decisions)-sc.maxDecisions:]
	}
}

// AddActivity appends an activity summary and trims to maxActivities (thread-safe).
func (sc *SessionContext) AddActivity(a ActivitySummary) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Activities = append(sc.Activities, a)
	if len(sc.Activities) > sc.maxActivities {
		sc.Activities = sc.Activities[len(sc.Activities)-sc.maxActivities:]
	}
}

// SetRules replaces the project rules (thread-safe).
func (sc *SessionContext) SetRules(rules []ProjectRule) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Rules = rules
}

// Snapshot returns a read-only deep copy (thread-safe).
func (sc *SessionContext) Snapshot() SessionContext {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	snap := SessionContext{
		SessionID: sc.SessionID,
		Project:   sc.Project,
	}
	if len(sc.Decisions) > 0 {
		snap.Decisions = make([]DecisionRecord, len(sc.Decisions))
		copy(snap.Decisions, sc.Decisions)
	}
	if len(sc.Activities) > 0 {
		snap.Activities = make([]ActivitySummary, len(sc.Activities))
		copy(snap.Activities, sc.Activities)
	}
	if len(sc.Rules) > 0 {
		snap.Rules = make([]ProjectRule, len(sc.Rules))
		copy(snap.Rules, sc.Rules)
	}
	return snap
}
