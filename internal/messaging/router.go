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

// lang returns the current language from the company manager.
func (r *Router) lang() string {
	return r.companyMgr.GetLanguage()
}

// m returns en or zh string based on current language.
func (r *Router) m(en, zh string) string {
	if r.lang() == "en" {
		return en
	}
	return zh
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
		if r.lang() == "en" {
			return fmt.Sprintf("Unknown command: %s\nType 'help' for available commands.", cmd)
		}
		return fmt.Sprintf("未知指令：%s\n輸入 'help' 查看可用指令。", cmd)
	}
}

func (r *Router) help() string {
	if r.lang() == "en" {
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
	return `可用指令：
  status          - 顯示公司概覽
  project list    - 列出所有專案
  project create <名稱> - 建立新專案
  worker list     - 列出所有員工
  worker hire <名稱> - 雇用新員工
  task list <專案ID> - 列出專案的任務
  assign <員工ID> <任務ID> - 分配任務給員工
  help            - 顯示此說明`
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

	if r.lang() == "en" {
		return fmt.Sprintf("Company Status:\n  Projects: %d\n  Workers: %d (idle: %d, busy: %d)",
			len(projects), len(workers), idle, working)
	}
	return fmt.Sprintf("公司狀態：\n  專案：%d\n  員工：%d（閒置：%d，忙碌：%d）",
		len(projects), len(workers), idle, working)
}

func (r *Router) projectCmd(args []string) string {
	if len(args) == 0 {
		return r.m("Usage: project list | project create <name>", "用法：project list | project create <名稱>")
	}

	switch strings.ToLower(args[0]) {
	case "list":
		projects := r.companyMgr.ListProjects()
		if len(projects) == 0 {
			return r.m("No projects.", "沒有專案。")
		}
		var sb strings.Builder
		sb.WriteString(r.m("Projects:\n", "專案列表：\n"))
		for _, p := range projects {
			prog := r.companyMgr.ProjectProgress(p.ID)
			sb.WriteString(fmt.Sprintf("  [%s] %s — %d/%d tasks done (%.0f%%)\n",
				p.ID, p.Name, prog.Done, prog.Total, prog.Percent))
		}
		return sb.String()

	case "create":
		if len(args) < 2 {
			return r.m("Usage: project create <name>", "用法：project create <名稱>")
		}
		name := strings.Join(args[1:], " ")
		p, err := r.companyMgr.CreateProject(name, "", "", "main", nil)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		if r.lang() == "en" {
			return fmt.Sprintf("Project created: %s (ID: %s)", p.Name, p.ID)
		}
		return fmt.Sprintf("專案已建立：%s（ID：%s）", p.Name, p.ID)

	default:
		return r.m("Usage: project list | project create <name>", "用法：project list | project create <名稱>")
	}
}

func (r *Router) workerCmd(args []string) string {
	if len(args) == 0 {
		return r.m("Usage: worker list | worker hire <name>", "用法：worker list | worker hire <名稱>")
	}

	switch strings.ToLower(args[0]) {
	case "list":
		workers := r.companyMgr.ListWorkers()
		if len(workers) == 0 {
			return r.m("No workers.", "沒有員工。")
		}
		var sb strings.Builder
		sb.WriteString(r.m("Workers:\n", "員工列表：\n"))
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
			return r.m("Usage: worker hire <name>", "用法：worker hire <名稱>")
		}
		name := strings.Join(args[1:], " ")
		w, err := r.companyMgr.CreateWorker(name, "robot")
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		if r.lang() == "en" {
			return fmt.Sprintf("Worker hired: %s (ID: %s)", w.Name, w.ID)
		}
		return fmt.Sprintf("已雇用員工：%s（ID：%s）", w.Name, w.ID)

	default:
		return r.m("Usage: worker list | worker hire <name>", "用法：worker list | worker hire <名稱>")
	}
}

func (r *Router) taskCmd(args []string) string {
	if len(args) < 2 {
		return r.m("Usage: task list <projectID>", "用法：task list <專案ID>")
	}

	switch strings.ToLower(args[0]) {
	case "list":
		projectID := args[1]
		tasks := r.companyMgr.ListTasks(projectID)
		if len(tasks) == 0 {
			return r.m("No tasks for this project.", "此專案沒有任務。")
		}
		var sb strings.Builder
		sb.WriteString(r.m("Tasks:\n", "任務列表：\n"))
		for _, t := range tasks {
			sb.WriteString(fmt.Sprintf("  [%s] %s — %s (priority: %d)\n",
				t.ID, t.Title, t.Status, t.Priority))
		}
		return sb.String()

	default:
		return r.m("Usage: task list <projectID>", "用法：task list <專案ID>")
	}
}

func (r *Router) assignCmd(args []string) string {
	if len(args) < 2 {
		return r.m("Usage: assign <workerID> <taskID>", "用法：assign <員工ID> <任務ID>")
	}
	workerID := args[0]
	taskID := args[1]
	ctx := context.Background()
	if err := r.companyMgr.AssignTask(ctx, workerID, taskID); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if r.lang() == "en" {
		return fmt.Sprintf("Task %s assigned to worker %s", taskID, workerID)
	}
	return fmt.Sprintf("任務 %s 已分配給員工 %s", taskID, workerID)
}
