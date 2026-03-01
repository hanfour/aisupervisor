package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/gitops"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/worker"
	"github.com/spf13/cobra"
)

var companyCmd = &cobra.Command{
	Use:   "company",
	Short: "Manage AI company (projects, workers, tasks)",
}

// --- Project subcommands ---

var companyCreateProjectCmd = &cobra.Command{
	Use:   "create-project",
	Short: "Create a new project",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		repoPath, _ := cmd.Flags().GetString("repo")
		baseBranch, _ := cmd.Flags().GetString("base-branch")
		goalsStr, _ := cmd.Flags().GetString("goals")

		if name == "" || repoPath == "" {
			return fmt.Errorf("--name and --repo are required")
		}

		var goals []string
		if goalsStr != "" {
			goals = splitComma(goalsStr)
		}

		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		p, err := mgr.CreateProject(name, desc, repoPath, baseBranch, goals)
		if err != nil {
			return err
		}
		fmt.Printf("Project created: %s (ID: %s)\n", p.Name, p.ID)
		return nil
	},
}

var companyListProjectsCmd = &cobra.Command{
	Use:   "list-projects",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		projects := mgr.ListProjects()
		if len(projects) == 0 {
			fmt.Println("No projects.")
			return nil
		}
		fmt.Printf("%-20s %-12s %-40s %s\n", "ID", "STATUS", "NAME", "REPO")
		for _, p := range projects {
			fmt.Printf("%-20s %-12s %-40s %s\n", p.ID, p.Status, p.Name, p.RepoPath)
		}
		return nil
	},
}

// --- Worker subcommands ---

var companyCreateWorkerCmd = &cobra.Command{
	Use:   "create-worker",
	Short: "Create a new worker",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		avatar, _ := cmd.Flags().GetString("avatar")
		tier, _ := cmd.Flags().GetString("tier")
		parentID, _ := cmd.Flags().GetString("parent")
		backendID, _ := cmd.Flags().GetString("backend")
		cliTool, _ := cmd.Flags().GetString("cli-tool")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}

		var opts []company.WorkerOption
		if tier != "" {
			opts = append(opts, company.WithTier(worker.WorkerTier(tier)))
		}
		if parentID != "" {
			opts = append(opts, company.WithParent(parentID))
		}
		if backendID != "" {
			opts = append(opts, company.WithBackend(backendID))
		}
		if cliTool != "" {
			opts = append(opts, company.WithCLITool(cliTool))
		}

		w, err := mgr.CreateWorker(name, avatar, opts...)
		if err != nil {
			return err
		}
		fmt.Printf("Worker created: %s (ID: %s, Tier: %s)\n", w.Name, w.ID, w.EffectiveTier())
		return nil
	},
}

var companyListWorkersCmd = &cobra.Command{
	Use:   "list-workers",
	Short: "List all workers",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		workers := mgr.ListWorkers()
		if len(workers) == 0 {
			fmt.Println("No workers.")
			return nil
		}
		fmt.Printf("%-20s %-15s %-12s %-10s %-8s %s\n", "ID", "NAME", "TIER", "STATUS", "CLI", "PARENT")
		for _, w := range workers {
			fmt.Printf("%-20s %-15s %-12s %-10s %-8s %s\n",
				w.ID, w.Name, w.EffectiveTier(), w.Status, w.CLITool, w.ParentID)
		}
		return nil
	},
}

// --- Task subcommands ---

var companyCreateTaskCmd = &cobra.Command{
	Use:   "create-task",
	Short: "Create a new task",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project")
		title, _ := cmd.Flags().GetString("title")
		desc, _ := cmd.Flags().GetString("description")
		prompt, _ := cmd.Flags().GetString("prompt")
		priority, _ := cmd.Flags().GetInt("priority")
		depsStr, _ := cmd.Flags().GetString("depends-on")
		milestone, _ := cmd.Flags().GetString("milestone")

		if projectID == "" || title == "" {
			return fmt.Errorf("--project and --title are required")
		}
		if prompt == "" {
			prompt = title
		}

		var deps []string
		if depsStr != "" {
			deps = splitComma(depsStr)
		}

		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		t, err := mgr.AddTask(projectID, title, desc, prompt, deps, priority, milestone)
		if err != nil {
			return err
		}
		fmt.Printf("Task created: %s (ID: %s, Status: %s)\n", t.Title, t.ID, t.Status)
		return nil
	},
}

var companyListTasksCmd = &cobra.Command{
	Use:   "list-tasks",
	Short: "List tasks for a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project")
		if projectID == "" {
			return fmt.Errorf("--project is required")
		}

		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		tasks := mgr.ListTasks(projectID)
		if len(tasks) == 0 {
			fmt.Println("No tasks.")
			return nil
		}
		fmt.Printf("%-20s %-12s %-4s %-20s %s\n", "ID", "STATUS", "PRI", "ASSIGNEE", "TITLE")
		for _, t := range tasks {
			fmt.Printf("%-20s %-12s %-4d %-20s %s\n", t.ID, t.Status, t.Priority, t.AssigneeID, t.Title)
		}
		return nil
	},
}

// --- Assignment ---

var companyAssignTaskCmd = &cobra.Command{
	Use:   "assign-task",
	Short: "Assign a task to a worker",
	RunE: func(cmd *cobra.Command, args []string) error {
		workerID, _ := cmd.Flags().GetString("worker")
		taskID, _ := cmd.Flags().GetString("task")
		if workerID == "" || taskID == "" {
			return fmt.Errorf("--worker and --task are required")
		}

		mgr, err := buildCompanyManagerWithTmux()
		if err != nil {
			return err
		}
		defer mgr.Shutdown()

		ctx := context.Background()
		if err := mgr.AssignTask(ctx, workerID, taskID); err != nil {
			return err
		}
		fmt.Printf("Task %s assigned to worker %s\n", taskID, workerID)
		return nil
	},
}

// --- Promote ---

var companyPromoteWorkerCmd = &cobra.Command{
	Use:   "promote-worker",
	Short: "Promote a worker to a higher tier",
	RunE: func(cmd *cobra.Command, args []string) error {
		workerID, _ := cmd.Flags().GetString("worker")
		newTier, _ := cmd.Flags().GetString("tier")
		if workerID == "" || newTier == "" {
			return fmt.Errorf("--worker and --tier are required")
		}

		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		if err := mgr.PromoteWorker(workerID, worker.WorkerTier(newTier)); err != nil {
			return err
		}
		fmt.Printf("Worker %s promoted to %s\n", workerID, newTier)
		return nil
	},
}

// --- Progress ---

var companyProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Show project progress",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project")
		if projectID == "" {
			return fmt.Errorf("--project is required")
		}

		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		p := mgr.ProjectProgress(projectID)
		fmt.Printf("Total: %d | Done: %d | InProgress: %d | Failed: %d | Progress: %.1f%%\n",
			p.Total, p.Done, p.InProgress, p.Failed, p.Percent)
		return nil
	},
}

// --- Hierarchy view ---

var companyHierarchyCmd = &cobra.Command{
	Use:   "hierarchy",
	Short: "Show worker hierarchy tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := buildCompanyManager()
		if err != nil {
			return err
		}
		workers := mgr.ListWorkers()

		// Find root workers (no parent)
		var roots []*worker.Worker
		childrenMap := make(map[string][]*worker.Worker)
		for _, w := range workers {
			if w.ParentID == "" {
				roots = append(roots, w)
			} else {
				childrenMap[w.ParentID] = append(childrenMap[w.ParentID], w)
			}
		}

		if len(roots) == 0 && len(workers) > 0 {
			// All workers have parents but no roots found — show flat list
			roots = workers
		}

		for _, r := range roots {
			printWorkerTree(r, childrenMap, "", true)
		}
		return nil
	},
}

func printWorkerTree(w *worker.Worker, children map[string][]*worker.Worker, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	status := string(w.Status)
	fmt.Printf("%s%s[%s] %s (%s) %s\n", prefix, connector, w.EffectiveTier(), w.Name, w.ID, status)

	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	kids := children[w.ID]
	for i, kid := range kids {
		printWorkerTree(kid, children, childPrefix, i == len(kids)-1)
	}
}

// --- Helpers ---

func buildCompanyManager() (*company.Manager, error) {
	home, _ := os.UserHomeDir()
	companyDataDir := filepath.Join(home, ".local", "share", "aisupervisor", "company")
	projectStore, err := project.NewStore(companyDataDir)
	if err != nil {
		return nil, fmt.Errorf("creating project store: %w", err)
	}
	git := gitops.New()
	// No tmux/spawner/monitor for offline commands
	return company.New(projectStore, nil, git, nil, nil, companyDataDir)
}

func buildCompanyManagerWithTmux() (*company.Manager, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	home, _ := os.UserHomeDir()
	companyDataDir := filepath.Join(home, ".local", "share", "aisupervisor", "company")
	projectStore, err := project.NewStore(companyDataDir)
	if err != nil {
		return nil, fmt.Errorf("creating project store: %w", err)
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connecting to tmux: %w", err)
	}

	git := gitops.New()
	spawner := worker.NewSpawner(tmuxClient, git, nil, nil)

	// Load tier configs if available
	if len(cfg.WorkerTiers) > 0 {
		spawner.LoadTierConfigs(cfg.WorkerTiers)
	}

	completionMon := worker.NewCompletionMonitor(tmuxClient)
	return company.New(projectStore, spawner, git, completionMon, tmuxClient, companyDataDir)
}

func init() {
	// Project commands
	companyCreateProjectCmd.Flags().String("name", "", "Project name")
	companyCreateProjectCmd.Flags().String("description", "", "Project description")
	companyCreateProjectCmd.Flags().String("repo", "", "Git repository path")
	companyCreateProjectCmd.Flags().String("base-branch", "main", "Base branch")
	companyCreateProjectCmd.Flags().String("goals", "", "Comma-separated goals")

	companyListProjectsCmd.Flags().String("project", "", "Project ID (unused)")

	// Worker commands
	companyCreateWorkerCmd.Flags().String("name", "", "Worker name")
	companyCreateWorkerCmd.Flags().String("avatar", "", "Worker avatar/emoji")
	companyCreateWorkerCmd.Flags().String("tier", "", "Worker tier (consultant|manager|engineer)")
	companyCreateWorkerCmd.Flags().String("parent", "", "Parent worker ID")
	companyCreateWorkerCmd.Flags().String("backend", "", "Backend ID")
	companyCreateWorkerCmd.Flags().String("cli-tool", "", "CLI tool (claude|aider)")

	// Task commands
	companyCreateTaskCmd.Flags().String("project", "", "Project ID")
	companyCreateTaskCmd.Flags().String("title", "", "Task title")
	companyCreateTaskCmd.Flags().String("description", "", "Task description")
	companyCreateTaskCmd.Flags().String("prompt", "", "Task prompt for AI")
	companyCreateTaskCmd.Flags().Int("priority", 5, "Priority (1=highest)")
	companyCreateTaskCmd.Flags().String("depends-on", "", "Comma-separated dependency task IDs")
	companyCreateTaskCmd.Flags().String("milestone", "", "Milestone tag")

	companyListTasksCmd.Flags().String("project", "", "Project ID")

	// Assignment
	companyAssignTaskCmd.Flags().String("worker", "", "Worker ID")
	companyAssignTaskCmd.Flags().String("task", "", "Task ID")

	// Promote
	companyPromoteWorkerCmd.Flags().String("worker", "", "Worker ID")
	companyPromoteWorkerCmd.Flags().String("tier", "", "New tier (consultant|manager|engineer)")

	// Progress
	companyProgressCmd.Flags().String("project", "", "Project ID")

	// Build command tree
	companyCmd.AddCommand(companyCreateProjectCmd)
	companyCmd.AddCommand(companyListProjectsCmd)
	companyCmd.AddCommand(companyCreateWorkerCmd)
	companyCmd.AddCommand(companyListWorkersCmd)
	companyCmd.AddCommand(companyCreateTaskCmd)
	companyCmd.AddCommand(companyListTasksCmd)
	companyCmd.AddCommand(companyAssignTaskCmd)
	companyCmd.AddCommand(companyPromoteWorkerCmd)
	companyCmd.AddCommand(companyProgressCmd)
	companyCmd.AddCommand(companyHierarchyCmd)
	rootCmd.AddCommand(companyCmd)
}
