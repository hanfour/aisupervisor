package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/ai"
	anthropicBackend "github.com/hanfourmini/aisupervisor/internal/ai/anthropic"
	geminiBackend "github.com/hanfourmini/aisupervisor/internal/ai/gemini"
	ollamaBackend "github.com/hanfourmini/aisupervisor/internal/ai/ollama"
	openaiBackend "github.com/hanfourmini/aisupervisor/internal/ai/openai"
	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/messaging"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/training"
	"github.com/hanfourmini/aisupervisor/internal/tui"
	"github.com/hanfourmini/aisupervisor/internal/worker"
	"github.com/spf13/cobra"
)

var (
	monitorDryRun  bool
	monitorSession string
	monitorBackend string
	monitorTUI     bool
	monitorRoles   string
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start monitoring AI CLI sessions",
	Long:  `Start the supervisor loop that monitors tmux sessions for AI CLI permission prompts.`,
	RunE:  runMonitor,
}

func init() {
	monitorCmd.Flags().BoolVar(&monitorDryRun, "dry-run", false, "detect and decide but don't send keys")
	monitorCmd.Flags().StringVar(&monitorSession, "session", "", "monitor a specific tmux session (format: session:window.pane)")
	monitorCmd.Flags().StringVar(&monitorBackend, "backend", "", "AI backend to use (overrides config)")
	monitorCmd.Flags().BoolVar(&monitorTUI, "tui", false, "launch interactive TUI dashboard")
	monitorCmd.Flags().StringVar(&monitorRoles, "roles", "", "comma-separated role IDs to enable")
	rootCmd.AddCommand(monitorCmd)
}

func runMonitor(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize tmux client
	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return fmt.Errorf("connecting to tmux: %w", err)
	}

	// Initialize AI backend
	backend, err := setupBackend(cfg)
	if err != nil {
		return fmt.Errorf("setting up backend: %w", err)
	}

	// Initialize audit logger
	auditor, err := audit.NewLogger(cfg.Audit.Path, cfg.Audit.Enabled)
	if err != nil {
		return fmt.Errorf("setting up audit: %w", err)
	}
	defer auditor.Close()

	// Initialize detector registry
	registry := detector.DefaultRegistry()

	// Initialize context store
	var ctxStore sessionctx.Store
	if cfg.Context.Enabled {
		home, _ := os.UserHomeDir()
		store, err := sessionctx.NewFileStore(filepath.Join(home, ".local", "share", "aisupervisor", "context"))
		if err != nil {
			log.Printf("warning: context store init failed: %v", err)
		} else {
			ctxStore = store
		}
	}

	// Build role manager
	rm := buildRoleManager(cfg, backend)

	// Build group manager (if groups configured)
	var gm *group.Manager
	if len(cfg.Groups) > 0 {
		var groups []*group.Group
		for _, gc := range cfg.Groups {
			groups = append(groups, &group.Group{
				ID:                  gc.ID,
				Name:                gc.Name,
				LeaderID:            gc.LeaderID,
				RoleIDs:             gc.RoleIDs,
				DivergenceThreshold: gc.DivergenceThreshold,
			})
		}
		// Build session role resolver first (needed as group filter)
		var filter group.SessionRoleFilter
		if len(cfg.SessionRoles) > 0 {
			r := role.NewResolver(rm, cfg.SessionRoles)
			filter = r.RolesForSession
		}
		var opts []group.ManagerOption
		if filter != nil {
			opts = append(opts, group.WithSessionFilter(filter))
		}
		opts = append(opts, group.WithAuditor(auditor))
		gm = group.NewManager(rm, groups, opts...)
	}

	// Build session role resolver (if session_roles configured)
	var resolver *role.SessionRoleResolver
	if len(cfg.SessionRoles) > 0 {
		resolver = role.NewResolver(rm, cfg.SessionRoles)
	}

	// Initialize supervisor
	sup := supervisor.New(cfg, tmuxClient, registry, backend, auditor, monitorDryRun, ctxStore, rm, gm, resolver)

	// Set up sessions to monitor
	sessions, err := getSessions(cfg, tmuxClient)
	if err != nil {
		return fmt.Errorf("getting sessions: %w", err)
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no sessions to monitor. Use --session flag or add sessions with 'aisupervisor sessions add'")
	}

	// TUI mode
	if monitorTUI {
		home, _ := os.UserHomeDir()
		mgr, _ := session.NewManager(home + "/.local/share/aisupervisor")

		// Build company manager for TUI company view
		companyDataDir := filepath.Join(home, ".local", "share", "aisupervisor", "company")
		projectStore, _ := project.NewStore(companyDataDir)
		git := gitops.New()
		spawner := worker.NewSpawner(tmuxClient, git, sup, mgr)
		if len(cfg.WorkerTiers) > 0 {
			spawner.LoadTierConfigs(cfg.WorkerTiers)
		}
		completionMon := worker.NewCompletionMonitor(tmuxClient)
		companyMgr, _ := company.New(projectStore, spawner, git, completionMon, tmuxClient, companyDataDir)

		// Load per-tier MaxWorkers
		if len(cfg.WorkerTiers) > 0 {
			companyMgr.LoadMaxWorkers(cfg.WorkerTiers)
		}

		// Wire training collector if enabled
		if cfg.Training.Enabled {
			trainingDir := cfg.Training.DataDir
			if trainingDir == "" {
				trainingDir = filepath.Join(home, ".local", "share", "aisupervisor", "training")
			} else if strings.HasPrefix(trainingDir, "~/") {
				trainingDir = filepath.Join(home, trainingDir[2:])
			}
			if logger, err := training.NewLogger(trainingDir); err == nil {
				collector := training.NewCollector(logger, git, tmuxClient, cfg.Training.CaptureDiffs)
				companyMgr.SetCollector(collector)

				// Wire finetune auto-trigger
				if cfg.Training.Finetune.AutoTrigger > 0 {
					registry, regErr := training.NewModelRegistry(trainingDir)
					if regErr == nil {
						exporter := training.NewExporter(trainingDir)
						runner := training.NewFinetuneRunner(trainingDir, registry, exporter)
						ftCfg := training.FinetuneConfig{
							Method:      cfg.Training.Finetune.Method,
							BaseModel:   cfg.Training.Finetune.BaseModel,
							OutputModel: cfg.Training.Finetune.OutputModel,
							ScriptPath:  cfg.Training.Finetune.ScriptPath,
							AutoTrigger: cfg.Training.Finetune.AutoTrigger,
							ValRatio:    cfg.Training.Finetune.ValRatio,
						}
						companyMgr.SetFinetuneRunner(runner, ftCfg)
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "WARNING: training collector init failed: %v\n", err)
			}
		}

		// Start messaging if configured
		startMessagingCLI(cfg, companyMgr)

		app := tui.NewApp(sup, tmuxClient, mgr, sessions, tui.WithCompanyManager(companyMgr))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for _, sess := range sessions {
			go sup.Monitor(ctx, sess)
		}

		p := tea.NewProgram(app, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		cancel()
		return nil
	}

	// Headless mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build company manager for headless mode too (training + review pipeline)
	{
		home, _ := os.UserHomeDir()
		companyDataDir := filepath.Join(home, ".local", "share", "aisupervisor", "company")
		projectStore, _ := project.NewStore(companyDataDir)
		git := gitops.New()
		hMgr, _ := session.NewManager(home + "/.local/share/aisupervisor")
		spawner := worker.NewSpawner(tmuxClient, git, sup, hMgr)
		if len(cfg.WorkerTiers) > 0 {
			spawner.LoadTierConfigs(cfg.WorkerTiers)
		}
		completionMon := worker.NewCompletionMonitor(tmuxClient)
		companyMgr, _ := company.New(projectStore, spawner, git, completionMon, tmuxClient, companyDataDir)
		if companyMgr != nil {
			if len(cfg.WorkerTiers) > 0 {
				companyMgr.LoadMaxWorkers(cfg.WorkerTiers)
			}
			if cfg.Training.Enabled {
				trainingDir := cfg.Training.DataDir
				if trainingDir == "" {
					trainingDir = filepath.Join(home, ".local", "share", "aisupervisor", "training")
				} else if strings.HasPrefix(trainingDir, "~/") {
					trainingDir = filepath.Join(home, trainingDir[2:])
				}
				if logger, err := training.NewLogger(trainingDir); err == nil {
					collector := training.NewCollector(logger, git, tmuxClient, cfg.Training.CaptureDiffs)
					companyMgr.SetCollector(collector)

					if cfg.Training.Finetune.AutoTrigger > 0 {
						registry, regErr := training.NewModelRegistry(trainingDir)
						if regErr == nil {
							exporter := training.NewExporter(trainingDir)
							runner := training.NewFinetuneRunner(trainingDir, registry, exporter)
							ftCfg := training.FinetuneConfig{
								Method:      cfg.Training.Finetune.Method,
								BaseModel:   cfg.Training.Finetune.BaseModel,
								OutputModel: cfg.Training.Finetune.OutputModel,
								ScriptPath:  cfg.Training.Finetune.ScriptPath,
								AutoTrigger: cfg.Training.Finetune.AutoTrigger,
								ValRatio:    cfg.Training.Finetune.ValRatio,
							}
							companyMgr.SetFinetuneRunner(runner, ftCfg)
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "WARNING: training collector init failed: %v\n", err)
				}
			}
			startMessagingCLI(cfg, companyMgr)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for _, sess := range sessions {
		go sup.Monitor(ctx, sess)
		if monitorDryRun {
			fmt.Printf("[dry-run] Monitoring session: %s (%s:%d.%d)\n", sess.Name, sess.TmuxSession, sess.Window, sess.Pane)
		} else {
			fmt.Printf("Monitoring session: %s (%s:%d.%d)\n", sess.Name, sess.TmuxSession, sess.Window, sess.Pane)
		}
	}

	fmt.Println("Supervisor running. Press Ctrl+C to stop.")

	go func() {
		for event := range sup.Events() {
			printEvent(event)
		}
	}()

	<-sigCh
	fmt.Println("\nShutting down...")
	cancel()
	return nil
}

func startMessagingCLI(cfg *config.Config, companyMgr *company.Manager) {
	var messengers []messaging.Messenger
	var perMessengerFilters [][]string

	if cfg.Messaging.Slack.Enabled {
		botToken := os.Getenv(cfg.Messaging.Slack.BotTokenEnv)
		appToken := os.Getenv(cfg.Messaging.Slack.AppTokenEnv)
		if botToken != "" && appToken != "" {
			m := messaging.NewSlackMessenger(botToken, appToken, cfg.Messaging.Slack.ChannelID)
			messengers = append(messengers, m)
			perMessengerFilters = append(perMessengerFilters, cfg.Messaging.Slack.NotifyEvents)
			log.Println("Slack messenger enabled")
		}
	}

	if cfg.Messaging.Line.Enabled {
		secret := os.Getenv(cfg.Messaging.Line.ChannelSecretEnv)
		token := os.Getenv(cfg.Messaging.Line.ChannelTokenEnv)
		if secret != "" && token != "" {
			m, err := messaging.NewLineMessenger(secret, token, cfg.Messaging.Line.NotifyUserID, cfg.Messaging.Line.Port)
			if err == nil {
				messengers = append(messengers, m)
				perMessengerFilters = append(perMessengerFilters, cfg.Messaging.Line.NotifyEvents)
				log.Println("LINE messenger enabled")
			}
		}
	}

	if len(messengers) > 0 {
		notifier := messaging.NewNotifier(companyMgr, messengers,
			messaging.WithGlobalFilter(cfg.Messaging.NotifyEvents))
		for i, f := range perMessengerFilters {
			if len(f) > 0 {
				notifier.SetMessengerFilter(i, f)
			}
		}
		ctx := context.Background()
		notifier.Start(ctx)
	}
}

func buildRoleManager(cfg *config.Config, backend ai.Backend) *role.Manager {
	var roles []role.Role

	if len(cfg.Roles) == 0 {
		// No custom roles configured — use the default gatekeeper
		gk := role.NewGatekeeperRole(backend, cfg.AutoApprove, cfg.Decision.ConfidenceThreshold)
		roles = append(roles, gk)
	} else {
		// Load roles from config
		loaded := role.LoadFromConfig(cfg.Roles, backend)
		roles = append(roles, loaded...)
	}

	// Load roles from directory
	if cfg.RolesDir != "" {
		dirRoles := role.LoadFromDir(cfg.RolesDir, backend)
		roles = append(roles, dirRoles...)
	} else {
		home, _ := os.UserHomeDir()
		defaultDir := filepath.Join(home, ".config", "aisupervisor", "roles")
		if info, err := os.Stat(defaultDir); err == nil && info.IsDir() {
			dirRoles := role.LoadFromDir(defaultDir, backend)
			roles = append(roles, dirRoles...)
		}
	}

	// Filter by --roles flag if provided
	if monitorRoles != "" {
		roles = filterRoles(roles, monitorRoles)
	}

	return role.NewManager(roles...)
}

func filterRoles(roles []role.Role, filter string) []role.Role {
	ids := make(map[string]bool)
	for _, part := range splitComma(filter) {
		ids[part] = true
	}
	var result []role.Role
	for _, r := range roles {
		if ids[r.ID()] {
			result = append(result, r)
		}
	}
	return result
}

func splitComma(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else if ch != ' ' {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func setupBackend(cfg *config.Config) (ai.Backend, error) {
	backendName := cfg.DefaultBackend
	if monitorBackend != "" {
		backendName = monitorBackend
	}

	for _, bc := range cfg.Backends {
		if bc.Name == backendName {
			switch bc.Type {
			case "anthropic_api":
				apiKey := os.Getenv(bc.APIKeyEnv)
				if apiKey == "" && bc.APIKeyEnv != "" {
					return nil, fmt.Errorf("environment variable %s not set", bc.APIKeyEnv)
				}
				return anthropicBackend.NewAPIBackend(bc.Name, apiKey, bc.Model), nil
			case "anthropic_oauth":
				return anthropicBackend.NewOAuthBackend(bc.Name, bc.Model)
			case "openai":
				apiKey := os.Getenv(bc.APIKeyEnv)
				if apiKey == "" && bc.APIKeyEnv != "" {
					return nil, fmt.Errorf("environment variable %s not set", bc.APIKeyEnv)
				}
				return openaiBackend.NewBackend(bc.Name, apiKey, bc.Model), nil
			case "gemini":
				apiKey := os.Getenv(bc.APIKeyEnv)
				if apiKey == "" && bc.APIKeyEnv != "" {
					return nil, fmt.Errorf("environment variable %s not set", bc.APIKeyEnv)
				}
				return geminiBackend.NewBackend(bc.Name, apiKey, bc.Model)
			case "ollama":
				return ollamaBackend.NewBackend(bc.Name, bc.BaseURL, bc.Model), nil
			default:
				return nil, fmt.Errorf("unsupported backend type: %s", bc.Type)
			}
		}
	}
	return nil, fmt.Errorf("backend %q not found in config", backendName)
}

func getSessions(cfg *config.Config, client tmux.TmuxClient) ([]*session.MonitoredSession, error) {
	if monitorSession != "" {
		sess, err := parseSessionFlag(monitorSession)
		if err != nil {
			return nil, err
		}
		return []*session.MonitoredSession{sess}, nil
	}

	// Load from persistent sessions
	home, _ := os.UserHomeDir()
	mgr, err := session.NewManager(home + "/.local/share/aisupervisor")
	if err != nil {
		return nil, err
	}

	active := mgr.Active()
	if len(active) > 0 {
		return active, nil
	}

	// Auto-discover: list all tmux sessions
	tmuxSessions, err := client.ListSessions()
	if err != nil {
		return nil, err
	}

	var result []*session.MonitoredSession
	for _, ts := range tmuxSessions {
		result = append(result, &session.MonitoredSession{
			ID:          ts.Name,
			Name:        ts.Name,
			TmuxSession: ts.Name,
			Window:      0,
			Pane:        0,
			ToolType:    "auto",
			Status:      session.StatusActive,
		})
	}
	return result, nil
}

func parseSessionFlag(flag string) (*session.MonitoredSession, error) {
	sess := &session.MonitoredSession{
		Status:   session.StatusActive,
		ToolType: "auto",
	}

	sessionName := flag
	var window, pane int

	// Parse "session:window.pane" format manually (Go's Sscanf doesn't support %[^:])
	if idx := strings.LastIndex(flag, ":"); idx >= 0 {
		sessionName = flag[:idx]
		rest := flag[idx+1:]
		fmt.Sscanf(rest, "%d.%d", &window, &pane)
	}

	sess.ID = sessionName
	sess.Name = sessionName
	sess.TmuxSession = sessionName
	sess.Window = window
	sess.Pane = pane
	return sess, nil
}

func printEvent(e supervisor.Event) {
	switch e.Type {
	case supervisor.EventDetected:
		fmt.Printf("[%s] DETECTED in %s: %s\n", e.Timestamp.Format("15:04:05"), e.SessionName, e.Match.Summary)
	case supervisor.EventDecision:
		roleInfo := ""
		if e.RoleID != "" {
			roleInfo = fmt.Sprintf(" role=%s", e.RoleID)
		}
		fmt.Printf("[%s] DECISION for %s: send %q (%s) confidence=%.2f reason=%s%s\n",
			e.Timestamp.Format("15:04:05"), e.SessionName,
			e.Decision.ChosenOption.Key, e.Decision.ChosenOption.Label,
			e.Decision.Confidence, e.Decision.Reasoning, roleInfo)
	case supervisor.EventAutoApproved:
		fmt.Printf("[%s] AUTO-APPROVED in %s: send %q (%s)\n",
			e.Timestamp.Format("15:04:05"), e.SessionName,
			e.Decision.ChosenOption.Key, e.Decision.Reasoning)
	case supervisor.EventSent:
		fmt.Printf("[%s] SENT to %s: %q\n", e.Timestamp.Format("15:04:05"), e.SessionName, e.Decision.ChosenOption.Key)
	case supervisor.EventPaused:
		fmt.Printf("[%s] PAUSED %s: low confidence (%.2f) — waiting for human\n",
			e.Timestamp.Format("15:04:05"), e.SessionName, e.Decision.Confidence)
	case supervisor.EventRoleIntervention:
		fmt.Printf("[%s] ROLE %s intervened in %s: %s\n",
			e.Timestamp.Format("15:04:05"), e.RoleID, e.SessionName, e.Intervention.Reasoning)
	case supervisor.EventError:
		log.Printf("[%s] ERROR in %s: %v\n", e.Timestamp.Format("15:04:05"), e.SessionName, e.Error)
	}
}
