package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	anthropicBackend "github.com/hanfourmini/aisupervisor/internal/ai/anthropic"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/spf13/cobra"
)

var backendsCmd = &cobra.Command{
	Use:   "backends",
	Short: "Manage AI backends",
}

var backendsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured backends",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		fmt.Printf("%-15s %-20s %-20s %-10s\n", "NAME", "TYPE", "MODEL", "DEFAULT")
		for _, b := range cfg.Backends {
			def := ""
			if b.Name == cfg.DefaultBackend {
				def = "*"
			}
			fmt.Printf("%-15s %-20s %-20s %-10s\n", b.Name, b.Type, b.Model, def)
		}
		return nil
	},
}

var backendsTestCmd = &cobra.Command{
	Use:   "test [name]",
	Short: "Test a backend connection",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		name := cfg.DefaultBackend
		if len(args) > 0 {
			name = args[0]
		}

		for _, bc := range cfg.Backends {
			if bc.Name == name {
				switch bc.Type {
				case "anthropic_api":
					apiKey := os.Getenv(bc.APIKeyEnv)
					b := anthropicBackend.NewAPIBackend(bc.Name, apiKey, bc.Model)
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					if err := b.Healthy(ctx); err != nil {
						fmt.Printf("Backend %q: FAILED (%v)\n", name, err)
						return nil
					}
					fmt.Printf("Backend %q: OK\n", name)
					return nil
				default:
					return fmt.Errorf("unsupported backend type: %s", bc.Type)
				}
			}
		}
		return fmt.Errorf("backend %q not found", name)
	},
}

func init() {
	backendsCmd.AddCommand(backendsListCmd)
	backendsCmd.AddCommand(backendsTestCmd)
	rootCmd.AddCommand(backendsCmd)
}
