package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	anthropicBackend "github.com/hanfourmini/aisupervisor/internal/ai/anthropic"
	geminiBackend "github.com/hanfourmini/aisupervisor/internal/ai/gemini"
	ollamaBackend "github.com/hanfourmini/aisupervisor/internal/ai/ollama"
	openaiBackend "github.com/hanfourmini/aisupervisor/internal/ai/openai"
	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/config"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/gui"
	"github.com/hanfourmini/aisupervisor/internal/messaging"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/training"
	"github.com/hanfourmini/aisupervisor/internal/worker"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var (
	flagConfig  string
	flagDryRun  bool
	flagSession string
	flagBackend string
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	flag.StringVar(&flagConfig, "config", "", "path to config file")
	flag.BoolVar(&flagDryRun, "dry-run", false, "detect and decide but don't send keys")
	flag.StringVar(&flagSession, "session", "", "monitor a specific tmux session (format: session:window.pane)")
	flag.StringVar(&flagBackend, "backend", "", "AI backend to use (overrides config)")
	flag.Parse()

	cfg, err := config.Load(flagConfig)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if flagBackend != "" {
		cfg.DefaultBackend = flagBackend
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		log.Fatalf("connecting to tmux: %v", err)
	}

	backend, err := setupBackend(cfg)
	if err != nil {
		log.Fatalf("setting up backend: %v", err)
	}

	auditor, err := audit.NewLogger(cfg.Audit.Path, cfg.Audit.Enabled)
	if err != nil {
		log.Fatalf("setting up audit: %v", err)
	}

	registry := detector.DefaultRegistry()

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

	rm := buildRoleManager(cfg, backend)

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

	var resolver *role.SessionRoleResolver
	if len(cfg.SessionRoles) > 0 {
		resolver = role.NewResolver(rm, cfg.SessionRoles)
	}

	sup := supervisor.New(cfg, tmuxClient, registry, backend, auditor, flagDryRun, ctxStore, rm, gm, resolver)

	home, _ := os.UserHomeDir()
	mgr, _ := session.NewManager(home + "/.local/share/aisupervisor")
	sessions := discoverSessions(cfg, tmuxClient, mgr)

	app := gui.NewApp(sup, mgr, tmuxClient, cfg, gm, resolver, sessions)

	// Company management system setup
	companyDataDir := filepath.Join(home, ".local", "share", "aisupervisor", "company")
	projectStore, err := project.NewStore(companyDataDir)
	if err != nil {
		log.Fatalf("setting up project store: %v", err)
	}
	git := gitops.New()
	spawner := worker.NewSpawner(tmuxClient, git, sup, mgr)
	if len(cfg.WorkerTiers) > 0 {
		spawner.LoadTierConfigs(cfg.WorkerTiers)
	}
	spawner.LoadSkillProfiles(config.MergeSkillProfiles(cfg.SkillProfiles))
	completionMon := worker.NewCompletionMonitor(tmuxClient)
	chatProvider := setupChatProvider(cfg)
	companyMgr, err := company.New(projectStore, spawner, git, completionMon, tmuxClient, companyDataDir, chatProvider)
	if err != nil {
		log.Fatalf("setting up company manager: %v", err)
	}

	// Wire training collector if enabled
	if cfg.Training.Enabled {
		trainingDir := cfg.Training.DataDir
		if trainingDir == "" {
			trainingDir = filepath.Join(home, ".local", "share", "aisupervisor", "training")
		} else if strings.HasPrefix(trainingDir, "~/") {
			trainingDir = filepath.Join(home, trainingDir[2:])
		}
		if tLogger, logErr := training.NewLogger(trainingDir); logErr == nil {
			collector := training.NewCollector(tLogger, git, tmuxClient, cfg.Training.CaptureDiffs)
			companyMgr.SetCollector(collector)
		} else {
			log.Printf("WARNING: training collector init failed: %v", logErr)
		}
	}

	// Wire language settings
	companyMgr.SetLanguage(cfg.Language)
	spawner.SetLanguage(cfg.Language)

	companyApp := gui.NewCompanyApp(companyMgr, tmuxClient)
	companyApp.SetSpawner(spawner)
	if cfg.Training.Enabled {
		trainingDir := cfg.Training.DataDir
		if trainingDir == "" {
			trainingDir = filepath.Join(home, ".local", "share", "aisupervisor", "training")
		} else if strings.HasPrefix(trainingDir, "~/") {
			trainingDir = filepath.Join(home, trainingDir[2:])
		}
		companyApp.SetTrainingDir(trainingDir)
	}
	companyApp.SetSkillProfiles(config.MergeSkillProfiles(cfg.SkillProfiles))

	// Start messaging integrations if configured
	startMessaging(cfg, companyMgr)

	if err := wails.Run(&options.App{
		Title:  "aisupervisor",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			companyApp.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			companyApp.Shutdown(ctx)
			app.Shutdown(ctx)
		},
		Bind: []interface{}{
			app,
			companyApp,
		},
	}); err != nil {
		log.Fatalf("wails error: %v", err)
	}

	_ = auditor.Close()
}

func setupBackend(cfg *config.Config) (ai.Backend, error) {
	for _, bc := range cfg.Backends {
		if bc.Name == cfg.DefaultBackend {
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
	return nil, fmt.Errorf("backend %q not found in config", cfg.DefaultBackend)
}

func buildRoleManager(cfg *config.Config, backend ai.Backend) *role.Manager {
	var roles []role.Role
	if len(cfg.Roles) == 0 {
		gk := role.NewGatekeeperRole(backend, cfg.AutoApprove, cfg.Decision.ConfidenceThreshold)
		roles = append(roles, gk)
	} else {
		loaded := role.LoadFromConfig(cfg.Roles, backend)
		roles = append(roles, loaded...)
	}
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
	return role.NewManager(roles...)
}

func startMessaging(cfg *config.Config, companyMgr *company.Manager) {
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

func setupChatProvider(cfg *config.Config) ai.ChatProvider {
	return buildChatProvider(cfg)
}

// buildChatProvider resolves a ChatProvider from config. If ChatBackend is set
// and matches a backend, use that. Otherwise fall back to the first openai or
// ollama backend found.
func buildChatProvider(cfg *config.Config) ai.ChatProvider {
	// Try to find explicit chat backend first
	if cfg.ChatBackend != "" {
		for _, bc := range cfg.Backends {
			if bc.Name == cfg.ChatBackend {
				if p := newChatProviderFromBackendConfig(bc); p != nil {
					return p
				}
			}
		}
		log.Printf("WARNING: chat backend %q not found or unsupported, trying fallback", cfg.ChatBackend)
	}

	// Fallback: first compatible backend (prefer openai, then anthropic_api, then ollama)
	// NOTE: anthropic_oauth is excluded — Anthropic restricts OAuth tokens to Claude Code only.
	preferOrder := []string{"openai", "anthropic_api", "ollama"}
	for _, pref := range preferOrder {
		for _, bc := range cfg.Backends {
			if bc.Type == pref {
				if p := newChatProviderFromBackendConfig(bc); p != nil {
					log.Printf("Using %q as chat backend (fallback)", bc.Name)
					return p
				}
			}
		}
	}
	log.Printf("WARNING: no compatible chat backend found")
	return nil
}

func newChatProviderFromBackendConfig(bc config.BackendConfig) ai.ChatProvider {
	switch bc.Type {
	case "openai":
		apiKey := os.Getenv(bc.APIKeyEnv)
		if apiKey == "" && bc.APIKeyEnv != "" {
			log.Printf("WARNING: %s not set for chat backend %q", bc.APIKeyEnv, bc.Name)
			return nil
		}
		return openaiBackend.NewBackend(bc.Name, apiKey, bc.Model)
	case "ollama":
		return ollamaBackend.NewBackend(bc.Name, bc.BaseURL, bc.Model)
	case "anthropic_api":
		apiKey := os.Getenv(bc.APIKeyEnv)
		if apiKey == "" && bc.APIKeyEnv != "" {
			log.Printf("WARNING: %s not set for chat backend %q", bc.APIKeyEnv, bc.Name)
			return nil
		}
		return anthropicBackend.NewAPIBackend(bc.Name, apiKey, bc.Model)
	case "anthropic_oauth":
		b, err := anthropicBackend.NewOAuthBackend(bc.Name, bc.Model)
		if err != nil {
			log.Printf("WARNING: OAuth backend %q init failed: %v", bc.Name, err)
			return nil
		}
		return b
	default:
		return nil
	}
}

func discoverSessions(cfg *config.Config, client tmux.TmuxClient, mgr *session.Manager) []*session.MonitoredSession {
	// If --session flag is set, use that
	if flagSession != "" {
		sess := &session.MonitoredSession{
			Status:   session.StatusActive,
			ToolType: "auto",
		}
		sessionName := flagSession
		var window, pane int
		if idx := strings.LastIndex(flagSession, ":"); idx >= 0 {
			sessionName = flagSession[:idx]
			rest := flagSession[idx+1:]
			fmt.Sscanf(rest, "%d.%d", &window, &pane)
		}
		sess.ID = sessionName
		sess.Name = sessionName
		sess.TmuxSession = sessionName
		sess.Window = window
		sess.Pane = pane
		return []*session.MonitoredSession{sess}
	}

	if mgr != nil {
		active := mgr.Active()
		if len(active) > 0 {
			return active
		}
	}
	tmuxSessions, err := client.ListSessions()
	if err != nil {
		return nil
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
	return result
}
