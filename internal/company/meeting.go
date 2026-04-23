package company

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// MeetingType represents the kind of meeting being held.
type MeetingType string

const (
	MeetingReview   MeetingType = "review"
	MeetingPlanning MeetingType = "planning"
	MeetingDebug    MeetingType = "debug"
)

// MeetingStatus represents the current state of a meeting.
type MeetingStatus string

const (
	MeetingScheduled  MeetingStatus = "scheduled"
	MeetingInProgress MeetingStatus = "in_progress"
	MeetingCompleted  MeetingStatus = "completed"
	MeetingCancelled  MeetingStatus = "cancelled"
)

// Meeting represents a multi-participant discussion session.
type Meeting struct {
	ID           string         `yaml:"id" json:"id"`
	Type         MeetingType    `yaml:"type" json:"type"`
	Title        string         `yaml:"title" json:"title"`
	Status       MeetingStatus  `yaml:"status" json:"status"`
	ProjectID    string         `yaml:"project_id" json:"projectId"`
	TaskID       string         `yaml:"task_id,omitempty" json:"taskId,omitempty"`
	ChairID      string         `yaml:"chair_id" json:"chairId"`
	Participants []string       `yaml:"participants" json:"participants"`
	Rounds       []MeetingRound `yaml:"rounds" json:"rounds"`
	Verdict      string         `yaml:"verdict,omitempty" json:"verdict,omitempty"`
	Summary      string         `yaml:"summary,omitempty" json:"summary,omitempty"`
	Agenda       []string       `yaml:"agenda,omitempty" json:"agenda,omitempty"`
	MaxRounds    int            `yaml:"max_rounds" json:"maxRounds"`
	CreatedAt    time.Time      `yaml:"created_at" json:"createdAt"`
	CompletedAt  *time.Time     `yaml:"completed_at,omitempty" json:"completedAt,omitempty"`
}

// MeetingRound represents one round of discussion in a meeting.
type MeetingRound struct {
	Number    int      `yaml:"number" json:"number"`
	Speeches  []Speech `yaml:"speeches" json:"speeches"`
	Consensus string   `yaml:"consensus,omitempty" json:"consensus,omitempty"`
}

// Speech represents a single participant's contribution in a meeting round.
type Speech struct {
	WorkerID  string    `yaml:"worker_id" json:"workerId"`
	Role      string    `yaml:"role" json:"role"`
	Content   string    `yaml:"content" json:"content"`
	Vote      string    `yaml:"vote,omitempty" json:"vote,omitempty"`
	Findings  []Finding `yaml:"findings,omitempty" json:"findings,omitempty"`
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

// MeetingRequest contains parameters for scheduling a new meeting.
type MeetingRequest struct {
	Type         MeetingType
	Title        string
	ProjectID    string
	TaskID       string
	ChairID      string
	Participants []string
	Agenda       []string
	MaxRounds    int
	Config       interface{}
}

// ReviewMeetingConfig holds configuration specific to code review meetings.
type ReviewMeetingConfig struct {
	Diff  string
	Brief *ContextBrief
}

// PlanningMeetingConfig holds configuration specific to planning meetings.
type PlanningMeetingConfig struct {
	Goals       []string
	Constraints []string
}

// DebugMeetingConfig holds configuration specific to debugging meetings.
type DebugMeetingConfig struct {
	ErrorLog   string
	Rejections []string
}

// TaskProposal represents a task suggestion generated during a planning meeting.
type TaskProposal struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	AssignTo     string   `json:"assignTo,omitempty"`
	Priority     int      `json:"priority,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// MeetingStore provides thread-safe persistence for meetings.
type MeetingStore struct {
	mu       sync.RWMutex
	meetings map[string]*Meeting
	filePath string
}

// meetingFile is the on-disk YAML representation of stored meetings.
type meetingFile struct {
	Meetings []*Meeting `yaml:"meetings"`
}

// workerChecker allows MeetingEngine to check worker status without importing worker package directly.
type workerChecker interface {
	GetWorkerStatus(id string) (string, bool) // returns status string and exists
}

// MeetingEngine orchestrates multi-participant meetings using AI chat providers.
type MeetingEngine struct {
	chatProvider  ai.ChatProvider
	mailbox       *Mailbox
	tmuxClient    tmux.TmuxClient
	language      string
	store         *MeetingStore
	workerChecker workerChecker
}

// ---------------------------------------------------------------------------
// MeetingStore CRUD
// ---------------------------------------------------------------------------

// NewMeetingStore creates or loads a MeetingStore from the given data directory.
// It creates the "meetings/" subdirectory and loads meetings.yaml if it exists.
func NewMeetingStore(dataDir string) (*MeetingStore, error) {
	dir := filepath.Join(dataDir, "meetings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create meetings dir: %w", err)
	}

	fp := filepath.Join(dir, "meetings.yaml")
	ms := &MeetingStore{
		meetings: make(map[string]*Meeting),
		filePath: fp,
	}

	data, err := os.ReadFile(fp)
	if err == nil && len(data) > 0 {
		var mf meetingFile
		if err := yaml.Unmarshal(data, &mf); err != nil {
			return nil, fmt.Errorf("parse meetings.yaml: %w", err)
		}
		for _, m := range mf.Meetings {
			ms.meetings[m.ID] = m
		}
	}

	return ms, nil
}

// Create stores a new Meeting from the given request. It assigns a unique ID,
// sets the status to MeetingScheduled, and persists the meeting to disk.
func (ms *MeetingStore) Create(req MeetingRequest) (*Meeting, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	maxRounds := req.MaxRounds
	if maxRounds == 0 {
		maxRounds = 3
	}

	id := fmt.Sprintf("mtg-%d-%04d", time.Now().UnixNano()/1e6, rand.Intn(10000))

	m := &Meeting{
		ID:           id,
		Type:         req.Type,
		Title:        req.Title,
		Status:       MeetingScheduled,
		ProjectID:    req.ProjectID,
		TaskID:       req.TaskID,
		ChairID:      req.ChairID,
		Participants: req.Participants,
		Agenda:       req.Agenda,
		MaxRounds:    maxRounds,
		CreatedAt:    time.Now(),
	}

	ms.meetings[m.ID] = m
	if err := ms.saveLocked(); err != nil {
		return nil, fmt.Errorf("save after create: %w", err)
	}
	return m, nil
}

// Get returns a copy of the meeting with the given ID. Returns an error if not found.
func (ms *MeetingStore) Get(id string) (*Meeting, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	m, ok := ms.meetings[id]
	if !ok {
		return nil, fmt.Errorf("meeting %q not found", id)
	}

	// Return a copy to prevent external mutation.
	cp := *m
	return &cp, nil
}

// List returns all meetings sorted by CreatedAt descending (newest first).
func (ms *MeetingStore) List() []*Meeting {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make([]*Meeting, 0, len(ms.meetings))
	for _, m := range ms.meetings {
		cp := *m
		result = append(result, &cp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByProject returns all meetings for the given project, sorted by CreatedAt desc.
func (ms *MeetingStore) ListByProject(projectID string) []*Meeting {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var result []*Meeting
	for _, m := range ms.meetings {
		if m.ProjectID == projectID {
			cp := *m
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByStatus returns all meetings with the given status, sorted by CreatedAt desc.
func (ms *MeetingStore) ListByStatus(status MeetingStatus) []*Meeting {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var result []*Meeting
	for _, m := range ms.meetings {
		if m.Status == status {
			cp := *m
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Update replaces an existing meeting in the store by its ID and persists to disk.
func (ms *MeetingStore) Update(m *Meeting) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, ok := ms.meetings[m.ID]; !ok {
		return fmt.Errorf("meeting %q not found", m.ID)
	}

	cp := *m
	ms.meetings[m.ID] = &cp
	return ms.saveLocked()
}

// Save marshals all meetings to meetings.yaml.
func (ms *MeetingStore) Save() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.saveLocked()
}

// saveLocked writes to disk; caller must hold the mutex.
func (ms *MeetingStore) saveLocked() error {
	meetings := make([]*Meeting, 0, len(ms.meetings))
	for _, m := range ms.meetings {
		meetings = append(meetings, m)
	}

	// Sort by CreatedAt for stable output.
	sort.Slice(meetings, func(i, j int) bool {
		return meetings[i].CreatedAt.Before(meetings[j].CreatedAt)
	})

	mf := meetingFile{Meetings: meetings}
	data, err := yaml.Marshal(&mf)
	if err != nil {
		return fmt.Errorf("marshal meetings: %w", err)
	}
	if err := os.WriteFile(ms.filePath, data, 0644); err != nil {
		return fmt.Errorf("write meetings.yaml: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// MeetingEngine
// ---------------------------------------------------------------------------

// NewMeetingEngine creates a new MeetingEngine with the provided dependencies.
func NewMeetingEngine(cp ai.ChatProvider, mailbox *Mailbox, tc tmux.TmuxClient, lang string, store *MeetingStore, wc workerChecker) *MeetingEngine {
	return &MeetingEngine{
		chatProvider:  cp,
		mailbox:       mailbox,
		tmuxClient:    tc,
		language:      lang,
		store:         store,
		workerChecker: wc,
	}
}

// Schedule creates a new meeting via the store and sends notification envelopes
// to all participants via the Mailbox.
func (me *MeetingEngine) Schedule(req MeetingRequest) (*Meeting, error) {
	m, err := me.store.Create(req)
	if err != nil {
		return nil, fmt.Errorf("schedule meeting: %w", err)
	}

	// Notify all participants.
	if me.mailbox != nil {
		for _, pid := range m.Participants {
			env := Envelope{
				StructuredMessage: StructuredMessage{
					From:    m.ChairID,
					To:      pid,
					Type:    MsgStatusUpdate,
					Content: fmt.Sprintf("Meeting scheduled: %s (ID: %s)", m.Title, m.ID),
				},
			}
			_ = me.mailbox.Send(env) // best-effort notification
		}
	}

	return m, nil
}

// Start transitions a meeting from scheduled to in-progress. All participants
// must be available (status "idle") as reported by the workerChecker.
func (me *MeetingEngine) Start(ctx context.Context, meetingID string) error {
	m, err := me.store.Get(meetingID)
	if err != nil {
		return fmt.Errorf("start meeting: %w", err)
	}

	// Check participant availability.
	var unavailable []string
	for _, pid := range m.Participants {
		status, exists := me.workerChecker.GetWorkerStatus(pid)
		if !exists || status != "idle" {
			unavailable = append(unavailable, pid)
		}
	}
	if len(unavailable) > 0 {
		return fmt.Errorf("cannot start meeting %s: participants unavailable: %s",
			meetingID, strings.Join(unavailable, ", "))
	}

	m.Status = MeetingInProgress
	if err := me.store.Update(m); err != nil {
		return fmt.Errorf("start meeting update: %w", err)
	}

	// Notify participants that the meeting has started.
	if me.mailbox != nil {
		for _, pid := range m.Participants {
			env := Envelope{
				StructuredMessage: StructuredMessage{
					From:    m.ChairID,
					To:      pid,
					Type:    MsgStatusUpdate,
					Content: fmt.Sprintf("Meeting started: %s (ID: %s)", m.Title, m.ID),
				},
			}
			_ = me.mailbox.Send(env)
		}
	}

	return nil
}

// Cancel sets a meeting's status to cancelled and notifies all participants.
func (me *MeetingEngine) Cancel(meetingID string) error {
	m, err := me.store.Get(meetingID)
	if err != nil {
		return fmt.Errorf("cancel meeting: %w", err)
	}

	m.Status = MeetingCancelled
	if err := me.store.Update(m); err != nil {
		return fmt.Errorf("cancel meeting update: %w", err)
	}

	// Notify participants.
	if me.mailbox != nil {
		for _, pid := range m.Participants {
			env := Envelope{
				StructuredMessage: StructuredMessage{
					From:    m.ChairID,
					To:      pid,
					Type:    MsgStatusUpdate,
					Content: fmt.Sprintf("Meeting cancelled: %s (ID: %s)", m.Title, m.ID),
				},
			}
			_ = me.mailbox.Send(env)
		}
	}

	return nil
}

// checkConsensus evaluates the votes in a set of speeches against a threshold.
// Abstain votes are excluded from the denominator. Returns whether consensus was
// reached and the winning verdict value (e.g. "approve" or "reject").
func checkConsensus(speeches []Speech, threshold float64) (reached bool, verdict string) {
	if len(speeches) == 0 {
		return false, ""
	}

	votes := make(map[string]int) // vote value → count
	total := 0

	for _, s := range speeches {
		v := strings.ToLower(strings.TrimSpace(s.Vote))
		if v == "" || v == "abstain" {
			continue
		}
		votes[v]++
		total++
	}

	if total == 0 {
		return false, ""
	}

	for v, count := range votes {
		if float64(count)/float64(total) >= threshold {
			return true, v
		}
	}

	return false, ""
}

// ---------------------------------------------------------------------------
// Speech Collection (Task 4)
// ---------------------------------------------------------------------------

// collectSpeeches collects speeches from all participants in parallel.
// mode determines execution: ExecAPI (ChatProvider) or ExecCLI (tmux).
// Single participant failure is logged, not fatal.
func (me *MeetingEngine) collectSpeeches(ctx context.Context, m *Meeting, roundNum int, prompt string, mode ExecMode) ([]Speech, error) {
	if len(m.Participants) == 0 {
		return nil, fmt.Errorf("no participants in meeting %s", m.ID)
	}

	type speechResult struct {
		speech Speech
		err    error
	}

	results := make([]speechResult, len(m.Participants))
	var wg sync.WaitGroup

	for i, pid := range m.Participants {
		wg.Add(1)
		go func(idx int, workerID string) {
			defer wg.Done()

			if mode == ExecCLI {
				log.Printf("meeting: CLI mode not yet implemented for speech collection, falling back to API for worker %s", workerID)
			}

			// API mode (and CLI fallback).
			if me.chatProvider == nil {
				results[idx] = speechResult{err: fmt.Errorf("no chat provider available")}
				return
			}

			role := "participant"
			if workerID == m.ChairID {
				role = "chair"
			}

			systemPrompt := fmt.Sprintf(
				"You are worker %s participating in a %s meeting titled %q. Your role is %s. "+
					"After your analysis, you MUST end your response with a vote line: VOTE:approve, VOTE:reject, or VOTE:abstain. "+
					"If you have specific findings, include them as a JSON array like: ```json\n[{\"file\":\"...\",\"severity\":\"...\",\"body\":\"...\"}]\n```",
				workerID, m.Type, m.Title, role,
			)

			messages := []ai.ChatMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: prompt},
			}

			text, err := me.chatProvider.Chat(ctx, messages)
			if err != nil {
				results[idx] = speechResult{err: fmt.Errorf("worker %s chat failed: %w", workerID, err)}
				return
			}

			vote := parseVote(text)
			findings := parseSpeechFindings(text)

			results[idx] = speechResult{
				speech: Speech{
					WorkerID:  workerID,
					Role:      role,
					Content:   text,
					Vote:      vote,
					Findings:  findings,
					Timestamp: time.Now(),
				},
			}
		}(i, pid)
	}

	wg.Wait()

	// Collect successful speeches, log failures.
	var speeches []Speech
	for _, r := range results {
		if r.err != nil {
			log.Printf("meeting: speech collection failure: %v", r.err)
			continue
		}
		speeches = append(speeches, r.speech)
	}

	return speeches, nil
}

// parseVote scans content for a "VOTE:" prefix and returns the vote value.
// Returns "approve", "reject", "abstain", or "" if no vote found.
func parseVote(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "VOTE:") {
			val := strings.TrimSpace(trimmed[5:])
			val = strings.ToLower(val)
			switch val {
			case "approve", "reject", "abstain":
				return val
			}
		}
	}
	return ""
}

// parseSpeechFindings extracts Finding structs from a speech's content,
// reusing the same JSON extraction strategies as council's parseExpertFindings.
func parseSpeechFindings(content string) []Finding {
	content = strings.TrimSpace(content)

	// Strategy 1: Extract from ```json ... ``` block.
	if start := strings.Index(content, "```json"); start != -1 {
		inner := content[start+7:]
		if end := strings.Index(inner, "```"); end != -1 {
			extracted := strings.TrimSpace(inner[:end])
			if findings, ok := tryParseFindingsArray(extracted); ok {
				return findings
			}
		}
	}

	// Strategy 2: Find [{ ... }] substring.
	if start := strings.Index(content, "[{"); start != -1 {
		sub := content[start:]
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
			if findings, ok := tryParseFindingsArray(extracted); ok {
				return findings
			}
		}
	}

	return nil
}

// tryParseFindingsArray attempts to unmarshal a string as []Finding.
func tryParseFindingsArray(s string) ([]Finding, bool) {
	var findings []Finding
	if err := json.Unmarshal([]byte(s), &findings); err != nil {
		return nil, false
	}
	return findings, true
}

// ---------------------------------------------------------------------------
// RunRound & Synthesize (Task 5)
// ---------------------------------------------------------------------------

// RunRound executes one round of the meeting: collect speeches, check consensus.
// Returns the round, whether consensus was reached, and any error.
func (me *MeetingEngine) RunRound(ctx context.Context, m *Meeting, roundNum int, mode ExecMode, threshold float64) (*MeetingRound, bool, error) {
	prompt := buildRoundPrompt(m, roundNum, nil)

	speeches, err := me.collectSpeeches(ctx, m, roundNum, prompt, mode)
	if err != nil {
		return nil, false, fmt.Errorf("round %d collect speeches: %w", roundNum, err)
	}

	round := &MeetingRound{
		Number:   roundNum,
		Speeches: speeches,
	}

	reached, verdict := checkConsensus(speeches, threshold)
	if reached {
		round.Consensus = verdict
	}

	m.Rounds = append(m.Rounds, *round)

	if me.store != nil {
		if err := me.store.Update(m); err != nil {
			log.Printf("meeting: failed to update store after round %d: %v", roundNum, err)
		}
	}

	return round, reached, nil
}

// buildRoundPrompt builds the prompt for a round, including context from previous rounds.
func buildRoundPrompt(m *Meeting, roundNum int, meetingConfig interface{}) string {
	var sb strings.Builder

	typeStr := string(m.Type)
	if len(typeStr) > 0 {
		typeStr = strings.ToUpper(typeStr[:1]) + typeStr[1:]
	}
	sb.WriteString(fmt.Sprintf("## %s Meeting: %s\n\n", typeStr, m.Title))

	if len(m.Agenda) > 0 {
		sb.WriteString("### Agenda\n")
		for i, item := range m.Agenda {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
		}
		sb.WriteString("\n")
	}

	// Add meeting-type-specific config context.
	if meetingConfig != nil {
		switch cfg := meetingConfig.(type) {
		case *ReviewMeetingConfig:
			if cfg.Brief != nil {
				sb.WriteString("### Context\n")
				sb.WriteString(cfg.Brief.Render())
				sb.WriteString("\n\n")
			}
			if cfg.Diff != "" {
				sb.WriteString("### Diff\n```\n")
				diff := cfg.Diff
				if len(diff) > 6000 {
					diff = diff[:3000] + "\n... (truncated) ...\n" + diff[len(diff)-3000:]
				}
				sb.WriteString(diff)
				sb.WriteString("\n```\n\n")
			}
		case *PlanningMeetingConfig:
			if len(cfg.Goals) > 0 {
				sb.WriteString("### Goals\n")
				for _, g := range cfg.Goals {
					sb.WriteString(fmt.Sprintf("- %s\n", g))
				}
				sb.WriteString("\n")
			}
			if len(cfg.Constraints) > 0 {
				sb.WriteString("### Constraints\n")
				for _, c := range cfg.Constraints {
					sb.WriteString(fmt.Sprintf("- %s\n", c))
				}
				sb.WriteString("\n")
			}
		case *DebugMeetingConfig:
			if cfg.ErrorLog != "" {
				sb.WriteString("### Error Log\n```\n")
				sb.WriteString(cfg.ErrorLog)
				sb.WriteString("\n```\n\n")
			}
		}
	}

	// Include previous round summaries for rounds > 1.
	if roundNum > 1 && len(m.Rounds) > 0 {
		sb.WriteString("### Previous Round Discussions\n\n")
		for _, r := range m.Rounds {
			sb.WriteString(fmt.Sprintf("**Round %d:**\n", r.Number))
			for _, s := range r.Speeches {
				// Summarize each speech concisely.
				summary := s.Content
				if len(summary) > 200 {
					summary = summary[:200] + "..."
				}
				sb.WriteString(fmt.Sprintf("- **%s** (%s): %s [Vote: %s]\n", s.WorkerID, s.Role, summary, s.Vote))
			}
			if r.Consensus != "" {
				sb.WriteString(fmt.Sprintf("  → Consensus: %s\n", r.Consensus))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(fmt.Sprintf("### Round %d Instructions\n", roundNum))
	sb.WriteString("Please share your analysis and perspective on the agenda items. ")
	if roundNum > 1 {
		sb.WriteString("Consider the points raised in previous rounds. Try to converge toward a decision. ")
	}
	sb.WriteString("End your response with your vote: VOTE:approve, VOTE:reject, or VOTE:abstain\n")

	return sb.String()
}

// Synthesize produces the final verdict and summary after all rounds.
func (me *MeetingEngine) Synthesize(ctx context.Context, m *Meeting) error {
	if len(m.Rounds) == 0 {
		return fmt.Errorf("cannot synthesize meeting %s: no rounds completed", m.ID)
	}

	lastRound := m.Rounds[len(m.Rounds)-1]

	// If last round has consensus, use that.
	if lastRound.Consensus != "" {
		m.Verdict = lastRound.Consensus
		m.Summary = fmt.Sprintf("Consensus reached in round %d: %s", lastRound.Number, lastRound.Consensus)
	} else if me.chatProvider != nil {
		// Use AI to synthesize from all rounds.
		verdict, summary, err := me.aiSynthesize(ctx, m)
		if err != nil {
			log.Printf("meeting: AI synthesis failed, falling back to majority vote: %v", err)
			m.Verdict, m.Summary = majorityVoteSummary(lastRound)
		} else {
			m.Verdict = verdict
			m.Summary = summary
		}
	} else {
		// No chat provider — use majority vote from last round.
		m.Verdict, m.Summary = majorityVoteSummary(lastRound)
	}

	m.Status = MeetingCompleted
	now := time.Now()
	m.CompletedAt = &now

	// Notify participants.
	if me.mailbox != nil {
		for _, pid := range m.Participants {
			env := Envelope{
				StructuredMessage: StructuredMessage{
					From:    m.ChairID,
					To:      pid,
					Type:    MsgStatusUpdate,
					Content: fmt.Sprintf("Meeting completed: %s — Verdict: %s", m.Title, m.Verdict),
				},
			}
			_ = me.mailbox.Send(env)
		}
	}

	// Update store.
	if me.store != nil {
		if err := me.store.Update(m); err != nil {
			return fmt.Errorf("synthesize update store: %w", err)
		}
	}

	return nil
}

// aiSynthesize uses the chat provider to produce a final verdict and summary.
func (me *MeetingEngine) aiSynthesize(ctx context.Context, m *Meeting) (verdict string, summary string, err error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are synthesizing the results of a %s meeting titled %q.\n\n", m.Type, m.Title))

	for _, r := range m.Rounds {
		sb.WriteString(fmt.Sprintf("## Round %d\n", r.Number))
		for _, s := range r.Speeches {
			sb.WriteString(fmt.Sprintf("**%s** (%s): %s\nVote: %s\n\n", s.WorkerID, s.Role, s.Content, s.Vote))
		}
	}

	sb.WriteString("\nBased on all rounds of discussion, provide:\n")
	sb.WriteString("1. A final verdict: approve, reject, or abstain\n")
	sb.WriteString("2. A concise summary of the key points and reasoning\n\n")
	sb.WriteString("Format your response as:\nVERDICT: <approve|reject|abstain>\nSUMMARY: <your summary>\n")

	messages := []ai.ChatMessage{
		{Role: "user", Content: sb.String()},
	}

	text, err := me.chatProvider.Chat(ctx, messages)
	if err != nil {
		return "", "", fmt.Errorf("AI synthesis chat: %w", err)
	}

	// Parse verdict and summary from response.
	verdict = ""
	summary = text // default to full response as summary

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "VERDICT:") {
			val := strings.TrimSpace(trimmed[8:])
			val = strings.ToLower(val)
			switch val {
			case "approve", "reject", "abstain":
				verdict = val
			}
		}
		if strings.HasPrefix(upper, "SUMMARY:") {
			summary = strings.TrimSpace(trimmed[8:])
		}
	}

	if verdict == "" {
		// Try to extract from VOTE: pattern as fallback.
		verdict = parseVote(text)
	}
	if verdict == "" {
		verdict = "abstain" // ultimate fallback
	}

	return verdict, summary, nil
}

// majorityVoteSummary computes verdict and summary from majority vote of the last round.
func majorityVoteSummary(round MeetingRound) (string, string) {
	votes := make(map[string]int)
	total := 0

	for _, s := range round.Speeches {
		v := strings.ToLower(strings.TrimSpace(s.Vote))
		if v == "" || v == "abstain" {
			continue
		}
		votes[v]++
		total++
	}

	if total == 0 {
		return "abstain", fmt.Sprintf("No actionable votes in round %d; defaulting to abstain", round.Number)
	}

	// Find majority.
	bestVote := ""
	bestCount := 0
	for v, c := range votes {
		if c > bestCount {
			bestCount = c
			bestVote = v
		}
	}

	return bestVote, fmt.Sprintf("Majority vote in round %d: %s (%d/%d)", round.Number, bestVote, bestCount, total)
}
