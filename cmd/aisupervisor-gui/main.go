package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	anthropicBackend "github.com/hanfourmini/aisupervisor/internal/ai/anthropic"
	geminiBackend "github.com/hanfourmini/aisupervisor/internal/ai/gemini"
	ollamaBackend "github.com/hanfourmini/aisupervisor/internal/ai/ollama"
	openaiBackend "github.com/hanfourmini/aisupervisor/internal/ai/openai"
	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/config"
	sessionctx "github.com/hanfourmini/aisupervisor/internal/context"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/gui"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
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

	if err := wails.Run(&options.App{
		Title:  "aisupervisor",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
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

func discoverSessions(cfg *config.Config, client tmux.TmuxClient, mgr *session.Manager) []*session.MonitoredSession {
	// If --session flag is set, use that
	if flagSession != "" {
		sess := &session.MonitoredSession{
			Status:   session.StatusActive,
			ToolType: "auto",
		}
		var sessionName string
		var window, pane int
		n, _ := fmt.Sscanf(flagSession, "%[^:]:%d.%d", &sessionName, &window, &pane)
		if n == 0 {
			sessionName = flagSession
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
