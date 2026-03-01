package training

import (
	"time"

	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Collector gathers training data from completed reviews.
type Collector struct {
	logger       *Logger
	gitOps       gitops.GitOps
	tmuxClient   tmux.TmuxClient
	captureDiffs bool
}

// NewCollector creates a Collector that captures review data.
func NewCollector(logger *Logger, gitOps gitops.GitOps, tmuxClient tmux.TmuxClient, captureDiffs bool) *Collector {
	return &Collector{
		logger:       logger,
		gitOps:       gitOps,
		tmuxClient:   tmuxClient,
		captureDiffs: captureDiffs,
	}
}

// CaptureReviewInput holds the data needed to capture a review pair.
type CaptureReviewInput struct {
	TaskID         string
	ProjectID      string
	RepoPath       string
	BranchName     string
	EngineerID     string
	ManagerID      string
	EngineerModel  string
	ManagerModel   string
	ModelVersion   string
	Prompt         string
	EngineerTmux   string
	EngineerWindow int
	EngineerPane   int
	ManagerTmux    string
	ManagerWindow  int
	ManagerPane    int
	Verdict        Verdict
	Feedback       string
	StartTime      time.Time
}

// CaptureReview captures a review pair and logs it.
func (c *Collector) CaptureReview(input CaptureReviewInput) error {
	pair := ReviewPair{
		Timestamp:     time.Now(),
		TaskID:        input.TaskID,
		ProjectID:     input.ProjectID,
		EngineerID:    input.EngineerID,
		ManagerID:     input.ManagerID,
		EngineerModel: input.EngineerModel,
		ManagerModel:  input.ManagerModel,
		ModelVersion:  input.ModelVersion,
		Prompt:        input.Prompt,
		Verdict:       input.Verdict,
		Feedback:      input.Feedback,
	}

	if !input.StartTime.IsZero() {
		pair.DurationMs = time.Since(input.StartTime).Milliseconds()
	}

	// Capture engineer output from tmux pane
	if input.EngineerTmux != "" {
		if content, err := c.tmuxClient.CapturePane(input.EngineerTmux, input.EngineerWindow, input.EngineerPane, 200); err == nil {
			pair.EngineerOutput = content
		}
	}

	// Capture manager output from tmux pane
	if input.ManagerTmux != "" {
		if content, err := c.tmuxClient.CapturePane(input.ManagerTmux, input.ManagerWindow, input.ManagerPane, 200); err == nil {
			pair.ManagerOutput = content
		}
	}

	// Capture git diff if enabled
	if c.captureDiffs && input.RepoPath != "" && input.BranchName != "" {
		if diff, err := c.gitOps.DiffBranch(input.RepoPath, input.BranchName); err == nil {
			pair.DiffSummary = diff
		}
	}

	return c.logger.Log(pair)
}
