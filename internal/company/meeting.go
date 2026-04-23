package company

import (
	"sync"
	"time"

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
