package cli

import (
	"fmt"
	"os"

	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage monitored sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List monitored sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		mgr, err := session.NewManager(home + "/.local/share/aisupervisor")
		if err != nil {
			return err
		}

		sessions := mgr.List()
		if len(sessions) == 0 {
			fmt.Println("No monitored sessions.")
			return nil
		}

		fmt.Printf("%-12s %-20s %-20s %-8s %-10s\n", "ID", "NAME", "TMUX", "TYPE", "STATUS")
		for _, s := range sessions {
			tmuxRef := fmt.Sprintf("%s:%d.%d", s.TmuxSession, s.Window, s.Pane)
			fmt.Printf("%-12s %-20s %-20s %-8s %-10s\n", s.ID, s.Name, tmuxRef, s.ToolType, s.Status)
		}
		return nil
	},
}

var (
	addName       string
	addTmux       string
	addToolType   string
	addGoal       string
	addProjectDir string
)

var sessionsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a session to monitor",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		mgr, err := session.NewManager(home + "/.local/share/aisupervisor")
		if err != nil {
			return err
		}

		sess, err := parseSessionFlag(addTmux)
		if err != nil {
			return err
		}
		sess.Name = addName
		sess.ToolType = addToolType
		sess.TaskGoal = addGoal
		sess.ProjectDir = addProjectDir

		if err := mgr.Add(sess); err != nil {
			return err
		}

		fmt.Printf("Added session %s (tmux: %s:%d.%d)\n", sess.Name, sess.TmuxSession, sess.Window, sess.Pane)
		return nil
	},
}

var sessionsRemoveCmd = &cobra.Command{
	Use:   "remove [id]",
	Short: "Remove a monitored session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		mgr, err := session.NewManager(home + "/.local/share/aisupervisor")
		if err != nil {
			return err
		}

		if err := mgr.Remove(args[0]); err != nil {
			return err
		}
		fmt.Printf("Removed session %s\n", args[0])
		return nil
	},
}

func init() {
	sessionsAddCmd.Flags().StringVar(&addName, "name", "", "session display name")
	sessionsAddCmd.Flags().StringVar(&addTmux, "tmux", "", "tmux target (session:window.pane)")
	sessionsAddCmd.Flags().StringVar(&addToolType, "type", "auto", "tool type (claude_code, gemini, auto)")
	sessionsAddCmd.Flags().StringVar(&addGoal, "goal", "", "task goal for AI context")
	sessionsAddCmd.Flags().StringVar(&addProjectDir, "project-dir", "", "project directory for context detection")
	sessionsAddCmd.MarkFlagRequired("name")
	sessionsAddCmd.MarkFlagRequired("tmux")

	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsAddCmd)
	sessionsCmd.AddCommand(sessionsRemoveCmd)
	rootCmd.AddCommand(sessionsCmd)
}
