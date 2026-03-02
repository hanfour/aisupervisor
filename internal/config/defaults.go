package config

// DefaultSkillProfiles returns the 6 built-in skill profiles.
// User-defined profiles in config.yaml override these by matching ID.
//
// Tool names follow Claude Code CLI syntax:
//   - Basic: "Bash", "Read", "Edit", "Write", "Glob", "Grep", "WebFetch", "WebSearch", "Task"
//   - Patterns: "Bash(git *)", "Bash(npm run *)", "Read(src/**)"
//   - MCP: "mcp__servername__toolname"
//
// Permission modes: "default", "acceptEdits", "plan", "dontAsk", "bypassPermissions"
// Model aliases: "sonnet", "opus", "haiku", "sonnet[1m]", "opusplan"
func DefaultSkillProfiles() []SkillProfile {
	return []SkillProfile{
		{
			ID:          "coder",
			Name:        "Coder",
			Description: "Full-stack developer focused on writing code, tests, and debugging",
			Icon:        "\U0001F4BB",
			SystemPrompt: "You are a senior software engineer. Focus on writing clean, tested, production-ready code. " +
				"Follow existing project conventions. Write tests for new functionality. " +
				"Debug issues systematically. Prefer simple solutions over clever ones.",
			PermissionMode: "acceptEdits",
			Model:          "sonnet",
		},
		{
			ID:          "hacker",
			Name:        "Hacker",
			Description: "Security researcher for penetration testing, vulnerability analysis, and exploit research",
			Icon:        "\U0001F575",
			SystemPrompt: "You are a security researcher and penetration tester. Focus on finding vulnerabilities, " +
				"analyzing attack surfaces, and writing proof-of-concept exploits. " +
				"Always document findings with severity ratings (CVSS) and remediation advice. " +
				"Use OWASP methodology for web app testing.",
			AllowedTools:   []string{"Bash", "Edit", "Read", "Grep", "Glob", "WebFetch"},
			PermissionMode: "acceptEdits",
			Model:          "sonnet",
		},
		{
			ID:          "designer",
			Name:        "Designer",
			Description: "UI/UX designer focused on frontend aesthetics, CSS, and user experience",
			Icon:        "\U0001F3A8",
			SystemPrompt: "You are a UI/UX designer and frontend specialist. Focus on visual design, " +
				"CSS styling, responsive layouts, accessibility (WCAG), and user experience improvements. " +
				"Ensure designs are pixel-perfect and follow design system conventions. " +
				"Use semantic HTML and modern CSS features.",
			AllowedTools:    []string{"Edit", "Write", "Read", "Glob", "Grep"},
			DisallowedTools: []string{"Bash(rm *)", "Bash(git push *)"},
			PermissionMode:  "acceptEdits",
			Model:           "sonnet",
		},
		{
			ID:          "analyst",
			Name:        "Analyst",
			Description: "Code analyst for performance evaluation, architecture review, and quality assessment",
			Icon:        "\U0001F50D",
			SystemPrompt: "You are a code analyst. Focus on reading and understanding code, " +
				"identifying performance bottlenecks, evaluating architecture decisions, " +
				"and providing detailed analysis reports with actionable recommendations. " +
				"Do not modify code unless explicitly asked. Use metrics and evidence.",
			AllowedTools:   []string{"Read", "Grep", "Glob", "Bash(git log *)", "Bash(git diff *)", "Bash(wc *)", "Bash(cloc *)"},
			PermissionMode: "plan",
			Model:          "sonnet",
		},
		{
			ID:          "architect",
			Name:        "Architect",
			Description: "System architect for high-level design, architecture planning, and technical decisions",
			Icon:        "\U0001F3DB",
			SystemPrompt: "You are a software architect. Focus on system design, architecture planning, " +
				"API design, and technical decision-making. Consider scalability, maintainability, " +
				"and trade-offs. Produce architecture documents and design proposals. " +
				"Use diagrams (Mermaid) when helpful. Review code for architectural alignment.",
			AllowedTools:   []string{"Read", "Grep", "Glob", "Edit", "Write", "Task", "WebSearch"},
			PermissionMode: "acceptEdits",
			Model:          "opus",
		},
		{
			ID:          "devops",
			Name:        "DevOps",
			Description: "DevOps engineer for CI/CD, deployment, infrastructure, Docker, and Kubernetes",
			Icon:        "\U0001F680",
			SystemPrompt: "You are a DevOps engineer. Focus on CI/CD pipelines, deployment automation, " +
				"infrastructure as code, Docker containers, Kubernetes configurations, " +
				"and monitoring setup. Follow infrastructure best practices and security hardening. " +
				"Use Dockerfile multi-stage builds and least-privilege principles.",
			AllowedTools:   []string{"Bash", "Read", "Edit", "Write", "Glob", "Grep"},
			PermissionMode: "acceptEdits",
			Model:          "sonnet",
		},
	}
}

// MergeSkillProfiles merges user-defined profiles with defaults.
// User profiles override defaults by matching ID. Non-matching user profiles are appended.
func MergeSkillProfiles(userProfiles []SkillProfile) []SkillProfile {
	defaults := DefaultSkillProfiles()
	defaultMap := make(map[string]int, len(defaults))
	for i, sp := range defaults {
		defaultMap[sp.ID] = i
	}

	for _, up := range userProfiles {
		if idx, ok := defaultMap[up.ID]; ok {
			defaults[idx] = up
		} else {
			defaults = append(defaults, up)
		}
	}
	return defaults
}
