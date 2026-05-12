# PRD: AI Supervisor

| Field | Content |
|-------|---------|
| Document Version | 0.1 |
| Product | AI Supervisor |
| Status | Product direction baseline |
| Audience | Indie developers, small software teams, and agentic engineering operators |

## 1. Product Definition

AI Supervisor is a macOS desktop application for managing AI coding agents as a
supervised software delivery team. It turns agentic development from individual
terminal sessions into an observable workflow with projects, tasks, workers,
reviews, verification, human approvals, cost tracking, and delivery history.

The product is not a generic SaaS application builder. It is the supervisor layer
that manages agents while they build external projects.

## 2. Core Positioning

AI Supervisor helps small teams run multiple AI workers without losing control of
branch isolation, review quality, cost, and human approval.

Primary promise:

> Assign software work to AI workers, watch progress, review outputs, and keep
> risky actions behind human gates.

## 3. Target Users

### Solo Builder

- Uses Claude Code, Codex, Aider, or similar tools directly.
- Wants more parallelism without manually babysitting every terminal.
- Needs task tracking, review, and rollback boundaries.

### Small Engineering Team

- Experiments with agentic coding workflows.
- Wants AI workers to operate on real repositories with safer branch/worktree
  isolation.
- Needs review, verification, and auditability before trusting outputs.

### Agentic Engineering Operator

- Runs multiple models and tools.
- Wants to compare backends, manage worker profiles, and route tasks by skill.
- Needs observability across worker state, costs, approvals, and failures.

## 4. Product Goals

1. Create a guided path from project idea to PRD, task breakdown, implementation,
   review, and verification.
2. Make multiple AI workers observable and controllable from one desktop app.
3. Reduce the risk of agentic coding through branch/worktree isolation, review,
   human gates, and audit logs.
4. Support multiple agent runtimes through a stable plugin interface.
5. Provide a differentiated virtual office interface without making the visual
   simulation more important than the delivery workflow.

## 5. MVP Scope

P0 capabilities:

- Setup wizard for dependencies, language, AI backend, and first team.
- Project creation with repository path, goals, PRD phase, and task board.
- Worker creation with skill profiles and tiers.
- Task assignment to agent runtimes in tmux sessions.
- Git branch and optional worktree isolation.
- Completion monitoring and task state transitions.
- Automated review queue with reviewer workers.
- Human gate for PRD approval and dangerous or budget-sensitive events.
- Verification command support per project.
- Cost and token budget visibility.
- Event log and terminal inspection.

P1 capabilities:

- Multi-expert review/council as an optional escalation layer.
- Inter-worker mailbox and synchronous ASK/REPLY.
- Structured planning, review, and debug meetings.
- Worker growth/personality as retention and operator experience features.
- Case-study replay flow showing how a real external project moves through the
  system.

Out of scope for the first commercial release:

- Hosted multi-tenant control plane.
- Enterprise SSO and centralized policy management.
- Fully autonomous production deployment.
- Guarantees that generated code is correct without user review.

## 6. Commercial Readiness Bar

The product should not be positioned as generally available until it can pass
these checks on real repositories:

- A new user can complete setup and create a working team without editing YAML.
- A project can move from PRD to approved task list without manual data repair.
- At least 30 consecutive task runs complete without corrupting project state.
- Review and verification failures are visible and routed back into the workflow.
- API keys and credentials have a clear storage policy.
- The release package is signed, notarized, and update metadata points to a real
  distribution URL.

## 7. Showcase Strategy

The first worked example shipped with AI Supervisor is the LINE Wi-Fi Ad SaaS
case study (see `docs/case-studies/line-wifi-saas/`). Its purpose is purely
illustrative: demonstrate that AI Supervisor can manage an external SaaS
project from requirements to implementation tasks. Completing the case study
is **not** a GA gate, and customers are not expected to use it as a template —
they point AI Supervisor at their own repositories. Future case studies may
be added to broaden coverage (e.g. internal tools, CLI utilities, data
pipelines), but each remains separate from AI Supervisor's own product
direction.

