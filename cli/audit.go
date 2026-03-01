package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
}

var (
	auditTailN int
)

var auditTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Show recent audit entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return err
		}

		f, err := os.Open(cfg.Audit.Path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No audit log found.")
				return nil
			}
			return err
		}
		defer f.Close()

		var entries []audit.Entry
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var e audit.Entry
			if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
				continue
			}
			entries = append(entries, e)
		}

		start := 0
		if len(entries) > auditTailN {
			start = len(entries) - auditTailN
		}

		for _, e := range entries[start:] {
			auto := ""
			if e.AutoApprove {
				auto = " [auto]"
			}
			dry := ""
			if e.DryRun {
				dry = " [dry-run]"
			}
			fmt.Printf("[%s] %s: %s → %s (%s) conf=%.2f%s%s\n",
				e.Timestamp.Format("2006-01-02 15:04:05"),
				e.SessionName, e.Summary,
				e.ChosenKey, e.ChosenLabel,
				e.Confidence, auto, dry)
		}
		return nil
	},
}

func init() {
	auditTailCmd.Flags().IntVarP(&auditTailN, "lines", "n", 20, "number of entries to show")
	auditCmd.AddCommand(auditTailCmd)
	rootCmd.AddCommand(auditCmd)
}
