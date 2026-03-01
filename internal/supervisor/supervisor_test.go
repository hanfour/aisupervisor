package supervisor

import (
	"context"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// fakeTmuxClient simulates tmux for testing
type fakeTmuxClient struct {
	content     string
	sentKeys    []string
	literalKeys []string
}

func (f *fakeTmuxClient) ListSessions() ([]tmux.SessionInfo, error) {
	return []tmux.SessionInfo{{Name: "test", Windows: 1}}, nil
}

func (f *fakeTmuxClient) ListPanes(session string) ([]tmux.PaneInfo, error) {
	return []tmux.PaneInfo{{SessionName: session, WindowIndex: 0, PaneIndex: 0, Active: true}}, nil
}

func (f *fakeTmuxClient) CapturePane(session string, window, pane, lines int) (string, error) {
	return f.content, nil
}

func (f *fakeTmuxClient) SendKeys(session string, window, pane int, keys string) error {
	f.sentKeys = append(f.sentKeys, keys)
	return nil
}

func (f *fakeTmuxClient) SendLiteralKeys(session string, window, pane int, text string) error {
	f.literalKeys = append(f.literalKeys, text)
	return nil
}

func (f *fakeTmuxClient) CreateSession(name string) error        { return nil }
func (f *fakeTmuxClient) KillSession(name string) error          { return nil }
func (f *fakeTmuxClient) HasSession(name string) (bool, error)   { return false, nil }

// fakeBackend returns a fixed decision
type fakeBackend struct {
	decision *ai.Decision
	err      error
}

func (f *fakeBackend) Name() string { return "fake" }
func (f *fakeBackend) Analyze(ctx context.Context, req ai.AnalysisRequest) (*ai.Decision, error) {
	return f.decision, f.err
}
func (f *fakeBackend) Healthy(ctx context.Context) error { return nil }

func makeRoleManager(backend ai.Backend, cfg *config.Config) *role.Manager {
	gk := role.NewGatekeeperRole(backend, cfg.AutoApprove, cfg.Decision.ConfidenceThreshold)
	return role.NewManager(gk)
}

func TestSupervisor_AutoApprove(t *testing.T) {
	client := &fakeTmuxClient{
		content: `
Reading file: src/main.go
Do you want to proceed?
  ❯ 1. Yes
    2. No
`,
	}

	cfg := &config.Config{
		Polling: config.PollingConfig{IntervalMs: 100, ContextLines: 50},
		Decision: config.DecisionConfig{
			ConfidenceThreshold: 0.7,
			TimeoutSeconds:      5,
		},
		AutoApprove: []config.AutoApproveRule{
			{Label: "Approve reads", PatternContains: "Reading file", Response: "1"},
		},
	}

	auditor, _ := audit.NewLogger("", false)
	registry := detector.DefaultRegistry()
	backend := &fakeBackend{}
	rm := makeRoleManager(backend, cfg)

	sup := New(cfg, client, registry, backend, auditor, false, nil, rm, nil)

	sess := &session.MonitoredSession{
		ID:          "test",
		Name:        "test",
		TmuxSession: "test",
		Window:      0,
		Pane:        0,
		Status:      session.StatusActive,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go sup.Monitor(ctx, sess)

	// Wait for events: auto-approve + sent
	timeout := time.After(2 * time.Second)
	gotAutoApprove := false
	for {
		select {
		case e := <-sup.Events():
			if e.Type == EventAutoApproved {
				if e.Decision.ChosenOption.Key != "1" {
					t.Errorf("expected key '1', got %q", e.Decision.ChosenOption.Key)
				}
				gotAutoApprove = true
			}
			if e.Type == EventSent && gotAutoApprove {
				// Verify keys were sent
				if len(client.sentKeys) == 0 {
					t.Error("expected keys to be sent")
				}
				return
			}
		case <-timeout:
			if gotAutoApprove {
				// Got auto-approve but not sent — check keys anyway (may be timing)
				if len(client.sentKeys) == 0 {
					t.Error("expected keys to be sent after auto-approve")
				}
				return
			}
			t.Fatal("timeout waiting for auto-approve event")
		}
	}
}

func TestSupervisor_DryRun(t *testing.T) {
	client := &fakeTmuxClient{
		content: `
Do you want to proceed?
  ❯ 1. Yes
    2. No
`,
	}

	cfg := &config.Config{
		Polling: config.PollingConfig{IntervalMs: 100, ContextLines: 50},
		Decision: config.DecisionConfig{
			ConfidenceThreshold: 0.7,
			TimeoutSeconds:      5,
		},
		AutoApprove: []config.AutoApproveRule{
			{Label: "Approve all", PatternContains: "proceed", Response: "1"},
		},
	}

	auditor, _ := audit.NewLogger("", false)
	registry := detector.DefaultRegistry()
	backend := &fakeBackend{}
	rm := makeRoleManager(backend, cfg)

	// dry-run = true
	sup := New(cfg, client, registry, backend, auditor, true, nil, rm, nil)

	sess := &session.MonitoredSession{
		ID:          "test",
		Name:        "test",
		TmuxSession: "test",
		Status:      session.StatusActive,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go sup.Monitor(ctx, sess)

	timeout := time.After(2 * time.Second)
	for {
		select {
		case e := <-sup.Events():
			if e.Type == EventAutoApproved {
				// In dry-run mode, no keys should be sent
				if len(client.sentKeys) > 0 {
					t.Error("dry-run should not send keys")
				}
				return
			}
		case <-timeout:
			t.Fatal("timeout")
		}
	}
}

func TestSupervisor_LowConfidencePause(t *testing.T) {
	client := &fakeTmuxClient{
		content: `
Do you want to proceed?
  ❯ 1. Yes
    2. No
`,
	}

	cfg := &config.Config{
		Polling:  config.PollingConfig{IntervalMs: 100, ContextLines: 50},
		Decision: config.DecisionConfig{ConfidenceThreshold: 0.7, TimeoutSeconds: 5},
	}

	auditor, _ := audit.NewLogger("", false)
	registry := detector.DefaultRegistry()
	backend := &fakeBackend{
		decision: &ai.Decision{
			ChosenOption: detector.ResponseOption{Key: "1", Label: "Yes"},
			Reasoning:    "unsure",
			Confidence:   0.3,
		},
	}
	rm := makeRoleManager(backend, cfg)

	sup := New(cfg, client, registry, backend, auditor, false, nil, rm, nil)

	sess := &session.MonitoredSession{
		ID:          "test",
		Name:        "test",
		TmuxSession: "test",
		Status:      session.StatusActive,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go sup.Monitor(ctx, sess)

	timeout := time.After(2 * time.Second)
	for {
		select {
		case e := <-sup.Events():
			if e.Type == EventPaused {
				if len(client.sentKeys) > 0 {
					t.Error("paused event should not send keys")
				}
				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for pause event")
		}
	}
}
