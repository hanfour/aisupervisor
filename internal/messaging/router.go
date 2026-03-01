package messaging

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/company"
)

// Router handles text commands and returns formatted replies.
type Router struct {
	companyMgr *company.Manager
}

func NewRouter(mgr *company.Manager) *Router {
	return &Router{companyMgr: mgr}
}

// Handle processes a text command and returns a reply string.
func (r *Router) Handle(text string) string {
	text = strings.TrimSpace(text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return r.help()
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "help":
		return r.help()
	case "status":
		return r.status()
	case "project":
		return r.projectCmd(args)
	case "worker":
		return r.workerCmd(args)
	case "task":
		return r.taskCmd(args)
	case "assign":
		return r.assignCmd(args)
	default:
		return fmt.Sprintf("Unknown command: %s\nType 'help' for available commands.", cmd)
	}
}

func (r *Router) help() string {
	return `Available commands:
  status          - Show company overview
  project list    - List all projects
  project create <name> - Create a new project
  worker list     - List all workers
  worker hire <name> - Hire a new worker
  task list <projectID> - List tasks for a project
  assign <workerID> <taskID> - Assign a task to a worker
  help            - Show this help message`
}

func (r *Router) status() string {
	projects := r.companyMgr.ListProjects()
	workers := r.companyMgr.ListWorkers()

	idle := 0
	working := 0
	for _, w := range workers {
		switch w.Status {
		case "idle":
			idle++
		default:
			working++
		}
	}

	return fmt.Sprintf("Company Status:\n  Projects: %d\n  Workers: %d (idle: %d, busy: %d)",
		len(projects), len(workers), idle, working)
}

func (r *Router) projectCmd(args []string) string {
	if len(args) == 0 {
		return "Usage: project list | project create <name>"
	}

	switch strings.ToLower(args[0]) {
	case "list":
		projects := r.companyMgr.ListProjects()
		if len(projects) == 0 {
			return "No projects."
		}
		var sb strings.Builder
		sb.WriteString("Projects:\n")
		for _, p := range projects {
			prog := r.companyMgr.ProjectProgress(p.ID)
			sb.WriteString(fmt.Sprintf("  [%s] %s — %d/%d tasks done (%.0f%%)\n",
				p.ID, p.Name, prog.Done, prog.Total, prog.Percent))
		}
		return sb.String()

	case "create":
		if len(args) < 2 {
			return "Usage: project create <name>"
		}
		name := strings.Join(args[1:], " ")
		p, err := r.companyMgr.CreateProject(name, "", "", "main", nil)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("Project created: %s (ID: %s)", p.Name, p.ID)

	default:
		return "Usage: project list | project create <name>"
	}
}

func (r *Router) workerCmd(args []string) string {
	if len(args) == 0 {
		return "Usage: worker list | worker hire <name>"
	}

	switch strings.ToLower(args[0]) {
	case "list":
		workers := r.companyMgr.ListWorkers()
		if len(workers) == 0 {
			return "No workers."
		}
		var sb strings.Builder
		sb.WriteString("Workers:\n")
		for _, w := range workers {
			task := "-"
			if w.CurrentTaskID != "" {
				task = w.CurrentTaskID
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s — %s (task: %s)\n",
				w.ID, w.Name, w.Status, task))
		}
		return sb.String()

	case "hire":
		if len(args) < 2 {
			return "Usage: worker hire <name>"
		}
		name := strings.Join(args[1:], " ")
		w, err := r.companyMgr.CreateWorker(name, "robot")
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("Worker hired: %s (ID: %s)", w.Name, w.ID)

	default:
		return "Usage: worker list | worker hire <name>"
	}
}

func (r *Router) taskCmd(args []string) string {
	if len(args) < 2 {
		return "Usage: task list <projectID>"
	}

	switch strings.ToLower(args[0]) {
	case "list":
		projectID := args[1]
		tasks := r.companyMgr.ListTasks(projectID)
		if len(tasks) == 0 {
			return "No tasks for this project."
		}
		var sb strings.Builder
		sb.WriteString("Tasks:\n")
		for _, t := range tasks {
			sb.WriteString(fmt.Sprintf("  [%s] %s — %s (priority: %d)\n",
				t.ID, t.Title, t.Status, t.Priority))
		}
		return sb.String()

	default:
		return "Usage: task list <projectID>"
	}
}

func (r *Router) assignCmd(args []string) string {
	if len(args) < 2 {
		return "Usage: assign <workerID> <taskID>"
	}
	workerID := args[0]
	taskID := args[1]
	ctx := context.Background()
	if err := r.companyMgr.AssignTask(ctx, workerID, taskID); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Task %s assigned to worker %s", taskID, workerID)
}
