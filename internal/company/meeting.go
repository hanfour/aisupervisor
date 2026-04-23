package company

import (
	"context"
	"fmt"
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
