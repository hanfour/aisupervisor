package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectProject inspects the given directory for project metadata.
func DetectProject(dir string) ProjectInfo {
	if dir == "" {
		return ProjectInfo{}
	}

	p := ProjectInfo{Directory: dir}
	p.GitBranch = gitBranch(dir)
	p.GitRemote = gitRemote(dir)
	p.Language, p.BuildTool = detectLanguage(dir)
	p.Framework = detectFramework(dir, p.Language)
	return p
}

func gitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitRemote(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectLanguage returns (language, buildTool) based on marker files.
func detectLanguage(dir string) (string, string) {
	markers := []struct {
		file      string
		language  string
		buildTool string
	}{
		{"go.mod", "Go", "go"},
		{"Cargo.toml", "Rust", "cargo"},
		{"package.json", "JavaScript/TypeScript", "npm"},
		{"pyproject.toml", "Python", "pyproject"},
		{"requirements.txt", "Python", "pip"},
		{"setup.py", "Python", "setuptools"},
		{"Gemfile", "Ruby", "bundler"},
		{"pom.xml", "Java", "maven"},
		{"build.gradle", "Java", "gradle"},
		{"build.gradle.kts", "Kotlin", "gradle"},
		{"mix.exs", "Elixir", "mix"},
		{"composer.json", "PHP", "composer"},
		{"CMakeLists.txt", "C/C++", "cmake"},
		{"Makefile", "", "make"},
	}

	for _, m := range markers {
		if fileExists(filepath.Join(dir, m.file)) {
			lang := m.language
			// Refine: package.json with tsconfig → TypeScript
			if m.file == "package.json" && fileExists(filepath.Join(dir, "tsconfig.json")) {
				lang = "TypeScript"
			}
			// Refine: detect yarn/pnpm
			bt := m.buildTool
			if m.file == "package.json" {
				if fileExists(filepath.Join(dir, "yarn.lock")) {
					bt = "yarn"
				} else if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
					bt = "pnpm"
				} else if fileExists(filepath.Join(dir, "bun.lock")) || fileExists(filepath.Join(dir, "bun.lockb")) {
					bt = "bun"
				}
			}
			return lang, bt
		}
	}
	return "", ""
}

// detectFramework attempts to identify the framework.
func detectFramework(dir, language string) string {
	switch language {
	case "JavaScript/TypeScript", "TypeScript":
		return detectJSFramework(dir)
	case "Python":
		return detectPythonFramework(dir)
	case "Ruby":
		if fileExists(filepath.Join(dir, "config", "routes.rb")) {
			return "Rails"
		}
	case "Go":
		// Could check imports but keep it simple
		return ""
	}
	return ""
}

func detectJSFramework(dir string) string {
	pkg := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkg)
	if err != nil {
		return ""
	}
	content := string(data)

	frameworks := []struct {
		dep  string
		name string
	}{
		{`"next"`, "Next.js"},
		{`"nuxt"`, "Nuxt"},
		{`"@angular/core"`, "Angular"},
		{`"svelte"`, "Svelte"},
		{`"vue"`, "Vue"},
		{`"react"`, "React"},
		{`"express"`, "Express"},
		{`"fastify"`, "Fastify"},
	}
	for _, f := range frameworks {
		if strings.Contains(content, f.dep) {
			return f.name
		}
	}
	return ""
}

func detectPythonFramework(dir string) string {
	// Check common files
	candidates := []struct {
		marker string
		name   string
	}{
		{"manage.py", "Django"},
		{"app.py", "Flask"}, // best-effort
	}
	for _, c := range candidates {
		if fileExists(filepath.Join(dir, c.marker)) {
			return c.name
		}
	}

	// Check pyproject.toml or requirements.txt content
	for _, f := range []string{"pyproject.toml", "requirements.txt"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "django") || strings.Contains(content, "Django") {
			return "Django"
		}
		if strings.Contains(content, "flask") || strings.Contains(content, "Flask") {
			return "Flask"
		}
		if strings.Contains(content, "fastapi") || strings.Contains(content, "FastAPI") {
			return "FastAPI"
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
