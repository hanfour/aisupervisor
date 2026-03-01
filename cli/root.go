package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "aisupervisor",
	Short: "AI CLI auto-monitoring and decision tool",
	Long: `aisupervisor monitors multiple AI CLI tool sessions (Claude Code, Gemini CLI)
via tmux, detects permission prompts, and uses AI to automatically respond.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/aisupervisor/config.yaml)")
}
