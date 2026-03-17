# AI Supervisor

> A virtual office desktop app that manages AI workers — hire, assign tasks, review code, and watch your AI team collaborate in real time.

![Pixel Office](docs/screenshots/08-office.png)

## What is AI Supervisor?

AI Supervisor is a **Wails v2 desktop application** (Go + Svelte) that turns AI coding assistants into a managed team of virtual employees. Think of it as a pixel-art office simulator where each "worker" is a real AI agent (Claude Code CLI) running in its own tmux session, writing actual code on your projects.

You are the boss. You hire workers, assign tasks, and they autonomously write code, create branches, submit for review, and iterate based on feedback — all while you watch from a retro-styled dashboard.

## Key Features

- **Guided Onboarding** — A conversational setup wizard helps you build your first AI team, complete with an HR specialist who recommends the right roles
- **Worker Management** — Hire AI workers with different skill profiles (coder, architect, QA, security, devops, designer, analyst, reviewer) and tiers (engineer, manager, consultant)
- **Task Pipeline** — Assign coding/research tasks to workers; each task gets its own git branch for isolation
- **Automated Code Review** — Completed tasks are automatically routed to a reviewer worker; approved code gets merged
- **Company Hierarchy** — Organize workers into teams with managers and consultants overseeing engineers
- **Personality System** — Each worker has unique personality traits, skill scores, moods, and relationships that evolve over time
- **AI-Generated Narratives** — Generate backstories and personality descriptions for your workers using AI
- **Training Loop** — Agentic training pipeline for autonomous code iteration and model fine-tuning
- **Multi-Backend Support** — Works with Claude Code CLI, Anthropic API, OpenAI, Ollama, and Google Gemini
- **Pixel Office** — A virtual office view where you can see your workers at their desks
- **Bilingual UI** — Full support for English and 繁體中文 (Traditional Chinese)

## Screenshots

| Pixel Office | Dashboard |
|:------------:|:---------:|
| ![Pixel Office](docs/screenshots/08-office.png) | ![Dashboard](docs/screenshots/01-dashboard.png) |

| Workers | Worker Detail |
|:-------:|:-------------:|
| ![Workers](docs/screenshots/02-workers.png) | ![Worker Detail](docs/screenshots/09-worker-detail.png) |

| Hierarchy | Kanban Board |
|:---------:|:------------:|
| ![Hierarchy](docs/screenshots/03-hierarchy.png) | ![Board](docs/screenshots/05-board.png) |

| Projects | Settings |
|:--------:|:--------:|
| ![Projects](docs/screenshots/04-projects.png) | ![Settings](docs/screenshots/10-settings.png) |

## How It Works

```
You (the Boss)
  │
  ├── Create a Project (linked to a git repo)
  ├── Break it into Tasks (code, research, review)
  └── AI Workers pick up tasks autonomously
        │
        ├── Each worker runs Claude Code CLI in a tmux pane
        ├── Creates a git branch per task
        ├── Writes code, runs tests
        ├── Submits for code review (another AI worker reviews)
        └── Approved → merged; Rejected → iterate
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23+, Wails v2 |
| Frontend | Svelte + Vite, NES.css (retro pixel theme) |
| AI Workers | Claude Code CLI in tmux sessions |
| Data Storage | YAML files (`~/.local/share/aisupervisor/company/`) |
| Configuration | `~/.config/aisupervisor/config.yaml` |

## Getting Started

### Prerequisites

- **Go 1.23+**
- **Node.js 18+**
- **[Wails v2](https://wails.io/)** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **tmux** — `brew install tmux` (macOS)
- **Claude Code CLI** — or another supported AI backend

### Build & Run

```bash
# Development (hot-reload for frontend)
cd cmd/aisupervisor-gui && wails dev

# Production build
wails build

# Build + install on macOS
make install-mac
```

### Configuration

On first launch, the onboarding wizard will guide you through setup. You can also manually configure:

```yaml
# ~/.config/aisupervisor/config.yaml
backends:
  - name: anthropic
    provider: anthropic
    apiKey: sk-ant-...
    model: claude-sonnet-4-20250514

polling:
  intervalMs: 500
  contextLines: 100
```

## Architecture

```
cmd/
  aisupervisor-gui/   # Wails v2 GUI entry point
  aisupervisor/       # TUI entry point (terminal mode)
internal/
  ai/                 # AI backend abstraction (anthropic, openai, ollama, gemini)
  company/            # Core business logic — task management, review pipeline
  config/             # App config + skill profiles
  gui/                # Wails bindings (Go ↔ Svelte bridge)
  personality/        # Worker personality traits, skill scores, narratives
  project/            # Project & Task data models
  worker/             # Worker spawner, monitor, session management
  tmux/               # tmux client for managing AI sessions
frontend/
  src/lib/
    components/       # Svelte UI components
    office/           # Pixel office simulation
    pages/            # Route pages
    stores/           # Svelte stores + i18n
```

## License

This project is proprietary software. All rights reserved.
