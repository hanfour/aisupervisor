package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/spf13/cobra"
)

var rolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "Manage supervisor roles",
	Long:  `List, enable, disable, and inspect supervisor roles.`,
}

var rolesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured roles",
	RunE:  runRolesList,
}

var rolesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of a specific role",
	Args:  cobra.ExactArgs(1),
	RunE:  runRolesShow,
}

var rolesEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a role in config",
	Args:  cobra.ExactArgs(1),
	RunE:  runRolesEnable,
}

var rolesDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a role in config",
	Args:  cobra.ExactArgs(1),
	RunE:  runRolesDisable,
}

func init() {
	rolesCmd.AddCommand(rolesListCmd)
	rolesCmd.AddCommand(rolesShowCmd)
	rolesCmd.AddCommand(rolesEnableCmd)
	rolesCmd.AddCommand(rolesDisableCmd)
	rootCmd.AddCommand(rolesCmd)
}

func loadAllRoles(cfg *config.Config) []role.Role {
	// Use a nil backend stub for listing — we only need metadata
	var stub ai.Backend = &noopBackend{}

	var roles []role.Role

	// Always include default gatekeeper
	gk := role.NewGatekeeperRole(stub, cfg.AutoApprove, cfg.Decision.ConfidenceThreshold)
	roles = append(roles, gk)

	// Load from config
	if len(cfg.Roles) > 0 {
		roles = append(roles, role.LoadFromConfig(cfg.Roles, stub)...)
	}

	// Load from roles dir
	rolesDir := cfg.RolesDir
	if rolesDir == "" {
		home, _ := os.UserHomeDir()
		rolesDir = filepath.Join(home, ".config", "aisupervisor", "roles")
	}
	if info, err := os.Stat(rolesDir); err == nil && info.IsDir() {
		roles = append(roles, role.LoadFromDir(rolesDir, stub)...)
	}

	return roles
}

func runRolesList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	roles := loadAllRoles(cfg)

	if len(roles) == 0 {
		fmt.Println("No roles configured.")
		return nil
	}

	fmt.Printf("%-25s %-12s %-8s %-10s\n", "ID", "MODE", "PRI", "NAME")
	fmt.Printf("%-25s %-12s %-8s %-10s\n", "---", "----", "---", "----")
	for _, r := range roles {
		fmt.Printf("%-25s %-12s %-8d %-10s\n", r.ID(), string(r.Mode()), r.Priority(), r.Name())
	}

	return nil
}

func runRolesShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	roles := loadAllRoles(cfg)
	id := args[0]

	for _, r := range roles {
		if r.ID() == id {
			fmt.Printf("ID:       %s\n", r.ID())
			fmt.Printf("Name:     %s\n", r.Name())
			fmt.Printf("Mode:     %s\n", r.Mode())
			fmt.Printf("Priority: %d\n", r.Priority())
			return nil
		}
	}

	return fmt.Errorf("role %q not found", id)
}

func runRolesEnable(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	id := args[0]
	for i, rc := range cfg.Roles {
		if rc.ID == id {
			cfg.Roles[i].Enabled = true
			if err := cfg.Save(cfgFile); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("Role %q enabled.\n", id)
			return nil
		}
	}

	return fmt.Errorf("role %q not found in config (is it in a roles directory?)", id)
}

func runRolesDisable(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	id := args[0]
	for i, rc := range cfg.Roles {
		if rc.ID == id {
			cfg.Roles[i].Enabled = false
			if err := cfg.Save(cfgFile); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Printf("Role %q disabled.\n", id)
			return nil
		}
	}

	return fmt.Errorf("role %q not found in config (is it in a roles directory?)", id)
}

// noopBackend is used for listing roles without making AI calls.
type noopBackend struct{}

func (b *noopBackend) Name() string { return "noop" }
func (b *noopBackend) Analyze(_ context.Context, _ ai.AnalysisRequest) (*ai.Decision, error) {
	return nil, fmt.Errorf("noop backend")
}
func (b *noopBackend) Healthy(_ context.Context) error { return nil }
