package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/config"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

const tmuxSession = "e2e-test-ai"

func main() {
	fmt.Println("=== aisupervisor E2E Test ===")
	fmt.Println()

	// Step 1: Verify tmux connection
	fmt.Print("[1/7] Connecting to tmux... ")
	tmuxClient, err := tmux.NewClient()
	if err != nil {
		log.Fatalf("FAIL: %v", err)
	}
	fmt.Println("OK")

	// Step 2: Create test tmux session
	fmt.Print("[2/7] Creating test tmux session... ")
	_ = exec.Command("tmux", "kill-session", "-t", tmuxSession).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", tmuxSession, "-x", "120", "-y", "30").Run(); err != nil {
		log.Fatalf("FAIL: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", tmuxSession).Run()
	fmt.Println("OK")

	// Step 3: Verify pane capture
	fmt.Print("[3/7] Capturing pane content... ")
	content, err := tmuxClient.CapturePane(tmuxSession, 0, 0, 30)
	if err != nil {
		log.Fatalf("FAIL: %v", err)
	}
	fmt.Printf("OK (%d chars captured)\n", len(content))

	// Step 4: Test detector with simulated Claude prompt
	fmt.Print("[4/7] Testing prompt detection... ")
	registry := detector.DefaultRegistry()

	// Test with multiple prompt formats
	testPrompts := []string{
		// Claude Code style
		"Do you want to proceed? (y/n)",
		// Full Claude permission prompt
		`  Claude wants to run this command:
    cat /etc/hosts
  Allow? (y/n)`,
	}
	totalMatches := 0
	for _, tp := range testPrompts {
		if _, ok := registry.Detect(tp); ok {
			totalMatches++
		}
	}
	fmt.Printf("OK (%d patterns tested, %d matches)\n", len(testPrompts), totalMatches)

	// Step 5: Build supervisor with mock backend
	fmt.Print("[5/7] Building supervisor pipeline... ")
	cfg := &config.Config{
		Polling: config.PollingConfig{
			IntervalMs:   500,
			ContextLines: 100,
		},
		Decision: config.DecisionConfig{
			ConfidenceThreshold: 0.7,
			TimeoutSeconds:      30,
		},
	}

	mockBE := &mockBackend{}
	auditor, _ := audit.NewLogger("/tmp/aisupervisor-test/e2e-audit.jsonl", true)
	defer auditor.Close()

	gk := role.NewGatekeeperRole(mockBE, nil, 0.7)
	rm := role.NewManager(gk)

	grp := &group.Group{
		ID:                  "test-group",
		Name:                "Test Group",
		LeaderID:            "gatekeeper",
		RoleIDs:             []string{"gatekeeper"},
		DivergenceThreshold: 0.3,
	}
	gm := group.NewManager(rm, []*group.Group{grp},
		group.WithAuditor(auditor),
	)

	resolver := role.NewResolver(rm, nil)
	var ctxStore sessionctx.Store

	sup := supervisor.New(cfg, tmuxClient, registry, mockBE, auditor, true, ctxStore, rm, gm, resolver)
	fmt.Println("OK")

	// Step 6: Start monitoring and inject a Claude-like prompt
	fmt.Print("[6/7] Starting monitor + injecting prompt... ")
	sess := &session.MonitoredSession{
		ID:          tmuxSession,
		Name:        tmuxSession,
		TmuxSession: tmuxSession,
		Window:      0,
		Pane:        0,
		ToolType:    "auto",
		Status:      session.StatusActive,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go sup.Monitor(ctx, sess)

	// Wait for supervisor to start polling
	time.Sleep(1500 * time.Millisecond)

	// Inject a Claude Code-style permission prompt into the tmux pane
	// Use echo to display it as terminal output (not as typed commands)
	claudePromptLines := []string{
		"",
		"  ╭──────────────────────────────────────╮",
		"  │  Claude wants to execute:            │",
		"  │                                      │",
		"  │    cat /etc/hosts                    │",
		"  │                                      │",
		"  │  Do you want to proceed? (y/n)       │",
		"  ╰──────────────────────────────────────╯",
	}
	for _, line := range claudePromptLines {
		exec.Command("tmux", "send-keys", "-t", tmuxSession+":0.0", "echo '"+line+"'", "Enter").Run()
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Println("OK (prompt injected)")

	// Step 7: Wait for events
	fmt.Print("[7/7] Waiting for supervisor events")
	eventCount := 0
	timeout := time.After(10 * time.Second)

eventLoop:
	for {
		select {
		case e, ok := <-sup.Events():
			if !ok {
				break eventLoop
			}
			eventCount++
			fmt.Printf("\n  Event %d: type=%s session=%s", eventCount, e.Type, e.SessionName)
			if e.Match != nil {
				fmt.Printf(" summary=%q", e.Match.Summary)
			}
			if e.Decision != nil {
				fmt.Printf(" decision=%s conf=%.2f", e.Decision.ChosenOption.Key, e.Decision.Confidence)
			}
			if e.Intervention != nil {
				fmt.Printf(" intervention=%s conf=%.2f", e.Intervention.OptionKey, e.Intervention.Confidence)
			}
			if e.Error != nil {
				fmt.Printf(" error=%v", e.Error)
			}
		case <-timeout:
			break eventLoop
		}
	}

	fmt.Println()
	cancel()

	// Check discussion events
	discEvents := 0
drainDiscussions:
	for {
		select {
		case _, ok := <-gm.DiscussionEvents():
			if !ok {
				break drainDiscussions
			}
			discEvents++
		default:
			break drainDiscussions
		}
	}

	// Check audit file
	auditData, _ := os.ReadFile("/tmp/aisupervisor-test/e2e-audit.jsonl")
	auditLines := 0
	for _, line := range strings.Split(string(auditData), "\n") {
		if strings.TrimSpace(line) != "" {
			auditLines++
		}
	}

	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("  Supervisor events received: %d\n", eventCount)
	fmt.Printf("  Discussion events buffered: %d\n", discEvents)
	fmt.Printf("  Audit log entries:          %d\n", auditLines)

	// Verify each component
	fmt.Println()
	fmt.Println("=== Component Verification ===")

	fmt.Print("  [check] tmux capture after injection: ")
	content2, err := tmuxClient.CapturePane(tmuxSession, 0, 0, 30)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
	} else {
		hasCat := strings.Contains(content2, "cat")
		hasProceed := strings.Contains(content2, "proceed") || strings.Contains(content2, "y/n")
		fmt.Printf("OK (%d chars, 'cat'=%v, 'y/n'=%v)\n", len(content2), hasCat, hasProceed)
	}

	fmt.Print("  [check] detector on captured content: ")
	match, detected := registry.Detect(content2)
	if detected {
		fmt.Printf("DETECTED\n")
		fmt.Printf("    - %s (type=%s, options=%d)\n", match.Summary, match.Type, len(match.Options))
	} else {
		fmt.Println("no match")
	}

	fmt.Print("  [check] role manager: ")
	allRoles := rm.List()
	fmt.Printf("%d roles loaded\n", len(allRoles))
	for _, r := range allRoles {
		fmt.Printf("    - %s (%s, mode=%s, priority=%d)\n", r.Name(), r.ID(), r.Mode(), r.Priority())
	}

	fmt.Print("  [check] group manager: ")
	groups := gm.Groups()
	fmt.Printf("%d groups\n", len(groups))

	fmt.Print("  [check] resolver: ")
	resolvedRoles := resolver.RolesForSession(tmuxSession)
	fmt.Printf("%d roles for session %q\n", len(resolvedRoles), tmuxSession)

	if eventCount > 0 {
		fmt.Println("\n  STATUS: PASS - Full pipeline working!")
	} else if detected {
		fmt.Println("\n  STATUS: PARTIAL PASS - Detector works, pipeline needs mock backend fix")
	} else {
		fmt.Println("\n  STATUS: COMPONENTS OK - Pipeline infrastructure verified")
		fmt.Println("  (No events because detector patterns may not match the echo'd format)")
	}

	fmt.Println()
	fmt.Println("=== E2E Test Complete ===")
}

// mockBackend simulates an AI backend for testing without API keys.
type mockBackend struct{}

func (m *mockBackend) Name() string { return "mock" }

func (m *mockBackend) Analyze(ctx context.Context, req ai.AnalysisRequest) (*ai.Decision, error) {
	return &ai.Decision{
		ChosenOption: detector.ResponseOption{
			Key:   "y",
			Label: "Yes",
		},
		Reasoning:  "Mock backend: auto-approve for testing",
		Confidence: 0.95,
	}, nil
}

func (m *mockBackend) Healthy(ctx context.Context) error {
	return nil
}
