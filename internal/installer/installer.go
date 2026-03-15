package installer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/binpath"
)

// DepStatus describes the status of a single dependency.
type DepStatus struct {
	Name           string `json:"name"`
	Label          string `json:"label"`
	Installed      bool   `json:"installed"`
	Version        string `json:"version"`
	Source         string `json:"source"`         // "bundled", "system", "missing"
	CanAutoInstall bool   `json:"canAutoInstall"`
	HelpText       string `json:"helpText"`
}

// InstallProgress reports installation progress for a single dependency.
type InstallProgress struct {
	Dep     string  `json:"dep"`
	Phase   string  `json:"phase"`   // "downloading", "installing", "verifying", "done", "error"
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
}

// ProgressFunc is a callback for reporting installation progress.
type ProgressFunc func(InstallProgress)

// CheckAll returns the status of all required dependencies.
// It first ensures common binary directories are in PATH,
// since macOS .app bundles launched from Finder have a minimal PATH.
func CheckAll() []DepStatus {
	ensureFullPath()
	return []DepStatus{
		checkGit(),
		checkBrew(),
		checkTmux(),
		checkNode(),
		checkClaude(),
	}
}

// ensureFullPath adds well-known binary directories to PATH so that
// tools installed by the user can be found even when the app is launched
// from Finder (which provides only /usr/bin:/bin:/usr/sbin:/sbin).
func ensureFullPath() {
	home, _ := os.UserHomeDir()
	extraPaths := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		filepath.Join(home, ".local/bin"),
		filepath.Join(home, ".claude/local/bin"),
		filepath.Join(home, ".nvm/versions/node"),
		filepath.Join(home, ".volta/bin"),
		filepath.Join(home, ".fnm/aliases/default/bin"),
		"/usr/local/lib/node_modules/.bin",
	}

	// Scan for nvm-managed node versions (pick latest)
	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				extraPaths = append(extraPaths, filepath.Join(nvmDir, e.Name(), "bin"))
			}
		}
	}

	currentPath := os.Getenv("PATH")
	changed := false
	for _, p := range extraPaths {
		if _, err := os.Stat(p); err == nil && !strings.Contains(currentPath, p) {
			currentPath = p + ":" + currentPath
			changed = true
		}
	}

	// Also try to discover npm global prefix
	if npmPath, err := exec.LookPath("npm"); err == nil {
		if out, err := exec.Command(npmPath, "config", "get", "prefix").Output(); err == nil {
			prefix := strings.TrimSpace(string(out))
			binDir := filepath.Join(prefix, "bin")
			if prefix != "" && !strings.Contains(currentPath, binDir) {
				if _, err := os.Stat(binDir); err == nil {
					currentPath = binDir + ":" + currentPath
					changed = true
				}
			}
		}
	}

	if changed {
		os.Setenv("PATH", currentPath)
		log.Printf("PATH expanded for dep detection: %s", currentPath)
	}
}

func checkGit() DepStatus {
	ds := DepStatus{
		Name:  "git",
		Label: "Git",
	}
	path := findDepPath("git")
	if path != "" {
		ds.Installed = true
		ds.Source = classifySource(path)
		ds.Version = getVersion("git", "--version")
	} else {
		ds.Source = "missing"
		ds.CanAutoInstall = runtime.GOOS == "darwin"
		ds.HelpText = "xcode-select --install"
	}
	return ds
}

func checkBrew() DepStatus {
	ds := DepStatus{
		Name:  "brew",
		Label: "Homebrew",
	}
	path, err := exec.LookPath("brew")
	if err == nil && path != "" {
		ds.Installed = true
		ds.Source = "system"
		ds.Version = getBrewVersion()
	} else {
		ds.Source = "missing"
		ds.CanAutoInstall = runtime.GOOS == "darwin"
		ds.HelpText = "https://brew.sh"
	}
	return ds
}

func getBrewVersion() string {
	out, err := exec.Command("brew", "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if idx := strings.Index(v, "\n"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

func checkTmux() DepStatus {
	ds := DepStatus{
		Name:  "tmux",
		Label: "tmux",
	}
	path := findDepPath("tmux")
	if path != "" {
		ds.Installed = true
		ds.Source = classifySource(path)
		ds.Version = getVersion("tmux", "-V")
	} else {
		ds.Source = "missing"
		// Can auto-install on macOS — will install Homebrew first if needed
		ds.CanAutoInstall = runtime.GOOS == "darwin"
		ds.HelpText = "brew install tmux"
	}
	return ds
}

func checkNode() DepStatus {
	ds := DepStatus{
		Name:  "node",
		Label: "Node.js",
	}
	path := findDepPath("node")
	if path != "" {
		ds.Installed = true
		ds.Source = classifySource(path)
		ds.Version = getVersion("node", "--version")
	} else {
		ds.Source = "missing"
		ds.CanAutoInstall = runtime.GOOS == "darwin"
		ds.HelpText = "Download from https://nodejs.org"
	}
	return ds
}

func checkClaude() DepStatus {
	ds := DepStatus{
		Name:  "claude",
		Label: "Claude CLI",
	}
	path := findDepPath("claude")
	if path != "" {
		ds.Installed = true
		ds.Source = classifySource(path)
		ds.Version = getVersion("claude", "--version")
	} else {
		ds.Source = "missing"
		// Can auto-install only if npm is available
		ds.CanAutoInstall = findDepPath("npm") != ""
		ds.HelpText = "npm install -g @anthropic-ai/claude-code"
	}
	return ds
}

// findDepPath checks bundled bin directory first, then system PATH.
func findDepPath(name string) string {
	if bundled := binpath.BundledBinDir(); bundled != "" {
		candidate := filepath.Join(bundled, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	path, err := exec.LookPath(name)
	if err == nil {
		return path
	}
	return ""
}

func classifySource(path string) string {
	bundled := binpath.BundledBinDir()
	if bundled != "" && strings.HasPrefix(path, bundled) {
		return "bundled"
	}
	return "system"
}

func getVersion(name string, flag string) string {
	path := findDepPath(name)
	if path == "" {
		return ""
	}
	out, err := exec.Command(path, flag).CombinedOutput()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	// Take first line only
	if idx := strings.Index(v, "\n"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

func hasHomebrew() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

// InstallGit triggers Xcode Command Line Tools installation on macOS.
func InstallGit(ctx context.Context, onProgress ProgressFunc) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("automatic git installation only supported on macOS")
	}
	onProgress(InstallProgress{Dep: "git", Phase: "installing", Percent: 10, Message: "Launching Xcode Command Line Tools installer..."})

	cmd := exec.CommandContext(ctx, "xcode-select", "--install")
	if err := cmd.Start(); err != nil {
		onProgress(InstallProgress{Dep: "git", Phase: "error", Percent: 0, Message: err.Error()})
		return fmt.Errorf("xcode-select --install: %w", err)
	}

	// xcode-select --install launches a GUI dialog — don't wait for it
	onProgress(InstallProgress{Dep: "git", Phase: "installing", Percent: 50, Message: "System installer dialog opened. Please follow the prompts."})

	// We can't truly wait for completion — the dialog is async.
	// Just verify after a short pause.
	onProgress(InstallProgress{Dep: "git", Phase: "verifying", Percent: 90, Message: "Waiting for installation to complete..."})

	// Check if git is available now
	if findDepPath("git") != "" {
		onProgress(InstallProgress{Dep: "git", Phase: "done", Percent: 100, Message: "Git installed successfully"})
		return nil
	}

	onProgress(InstallProgress{Dep: "git", Phase: "done", Percent: 100, Message: "Installer launched. Click 'Recheck' after installation completes."})
	return nil
}

// InstallHomebrew installs Homebrew (the macOS package manager).
func InstallHomebrew(ctx context.Context, onProgress ProgressFunc) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("Homebrew is only supported on macOS")
	}
	if hasHomebrew() {
		onProgress(InstallProgress{Dep: "brew", Phase: "done", Percent: 100, Message: "Homebrew already installed"})
		return nil
	}

	onProgress(InstallProgress{Dep: "brew", Phase: "installing", Percent: 10, Message: "Installing Homebrew..."})

	// Use NONINTERACTIVE=1 to skip confirmation prompts.
	// The official install script from https://brew.sh
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c",
		`NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
	cmd.Env = append(os.Environ(), "NONINTERACTIVE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		// Take last 200 chars to keep error readable
		if len(msg) > 200 {
			msg = "..." + msg[len(msg)-200:]
		}
		if msg == "" {
			msg = err.Error()
		}
		onProgress(InstallProgress{Dep: "brew", Phase: "error", Percent: 0, Message: msg})
		return fmt.Errorf("installing Homebrew: %w", err)
	}

	// Refresh PATH to find brew
	refreshPath()

	if !hasHomebrew() {
		// On Apple Silicon, brew is at /opt/homebrew/bin which may not be in PATH yet
		brewPaths := []string{"/opt/homebrew/bin", "/usr/local/bin"}
		for _, bp := range brewPaths {
			candidate := filepath.Join(bp, "brew")
			if _, err := os.Stat(candidate); err == nil {
				currentPath := os.Getenv("PATH")
				if !strings.Contains(currentPath, bp) {
					os.Setenv("PATH", bp+":"+currentPath)
				}
				break
			}
		}
	}

	if !hasHomebrew() {
		onProgress(InstallProgress{Dep: "brew", Phase: "error", Percent: 0, Message: "Homebrew not found after installation"})
		return fmt.Errorf("brew not found after installation")
	}

	onProgress(InstallProgress{Dep: "brew", Phase: "done", Percent: 100, Message: "Homebrew installed successfully"})
	return nil
}

// InstallTmux installs tmux via Homebrew (installing Homebrew first if needed).
func InstallTmux(ctx context.Context, onProgress ProgressFunc) error {
	// Install Homebrew first if missing
	if !hasHomebrew() {
		onProgress(InstallProgress{Dep: "tmux", Phase: "installing", Percent: 5, Message: "Installing Homebrew first..."})
		if err := InstallHomebrew(ctx, onProgress); err != nil {
			onProgress(InstallProgress{Dep: "tmux", Phase: "error", Percent: 0, Message: "Cannot install tmux: Homebrew installation failed"})
			return fmt.Errorf("homebrew prerequisite failed: %w", err)
		}
	}

	onProgress(InstallProgress{Dep: "tmux", Phase: "installing", Percent: 40, Message: "Running brew install tmux..."})

	cmd := exec.CommandContext(ctx, "brew", "install", "tmux")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		onProgress(InstallProgress{Dep: "tmux", Phase: "error", Percent: 0, Message: msg})
		return fmt.Errorf("brew install tmux: %w", err)
	}

	onProgress(InstallProgress{Dep: "tmux", Phase: "verifying", Percent: 90, Message: "Verifying tmux installation..."})

	if findDepPath("tmux") == "" {
		onProgress(InstallProgress{Dep: "tmux", Phase: "error", Percent: 0, Message: "tmux not found after installation"})
		return fmt.Errorf("tmux not found after brew install")
	}

	onProgress(InstallProgress{Dep: "tmux", Phase: "done", Percent: 100, Message: "tmux installed successfully"})
	return nil
}

// nodeVersion is the Node.js version to download if not installed.
const nodeVersion = "22.14.0"

// InstallNode downloads and installs Node.js via the official .pkg installer on macOS.
func InstallNode(ctx context.Context, onProgress ProgressFunc) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("automatic Node.js installation only supported on macOS")
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	url := fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s.pkg", nodeVersion, nodeVersion)
	if arch == "arm64" {
		url = fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-darwin-arm64.pkg", nodeVersion, nodeVersion)
	}

	tmpDir, err := os.MkdirTemp("", "aisupervisor-node-")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pkgPath := filepath.Join(tmpDir, "node.pkg")

	onProgress(InstallProgress{Dep: "node", Phase: "downloading", Percent: 5, Message: "Downloading Node.js installer..."})

	err = downloadFile(ctx, url, pkgPath, func(pct float64) {
		onProgress(InstallProgress{Dep: "node", Phase: "downloading", Percent: 5 + pct*0.7, Message: fmt.Sprintf("Downloading Node.js... %.0f%%", pct*100)})
	})
	if err != nil {
		onProgress(InstallProgress{Dep: "node", Phase: "error", Percent: 0, Message: err.Error()})
		return fmt.Errorf("downloading Node.js: %w", err)
	}

	onProgress(InstallProgress{Dep: "node", Phase: "installing", Percent: 75, Message: "Opening Node.js installer..."})

	// Use `open -W` to launch the standard macOS pkg installer and wait for it
	cmd := exec.CommandContext(ctx, "open", "-W", pkgPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		onProgress(InstallProgress{Dep: "node", Phase: "error", Percent: 0, Message: msg})
		return fmt.Errorf("opening installer: %w", err)
	}

	onProgress(InstallProgress{Dep: "node", Phase: "verifying", Percent: 95, Message: "Verifying Node.js installation..."})

	// Refresh PATH to find newly installed node
	refreshPath()

	if findDepPath("node") == "" {
		onProgress(InstallProgress{Dep: "node", Phase: "error", Percent: 0, Message: "Node.js not found after installation"})
		return fmt.Errorf("node not found after installation")
	}

	onProgress(InstallProgress{Dep: "node", Phase: "done", Percent: 100, Message: "Node.js installed successfully"})
	return nil
}

// InstallClaude installs Claude Code CLI via npm.
func InstallClaude(ctx context.Context, onProgress ProgressFunc) error {
	npmPath := findDepPath("npm")
	if npmPath == "" {
		onProgress(InstallProgress{Dep: "claude", Phase: "error", Percent: 0, Message: "npm not found. Install Node.js first."})
		return fmt.Errorf("npm not found — install Node.js first")
	}

	onProgress(InstallProgress{Dep: "claude", Phase: "installing", Percent: 20, Message: "Running npm install -g @anthropic-ai/claude-code..."})

	cmd := exec.CommandContext(ctx, npmPath, "install", "-g", "@anthropic-ai/claude-code")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		onProgress(InstallProgress{Dep: "claude", Phase: "error", Percent: 0, Message: msg})
		return fmt.Errorf("npm install claude-code: %w", err)
	}

	onProgress(InstallProgress{Dep: "claude", Phase: "verifying", Percent: 90, Message: "Verifying Claude CLI installation..."})

	// Refresh PATH
	refreshPath()

	if findDepPath("claude") == "" {
		onProgress(InstallProgress{Dep: "claude", Phase: "error", Percent: 0, Message: "Claude CLI not found after installation"})
		return fmt.Errorf("claude not found after npm install")
	}

	onProgress(InstallProgress{Dep: "claude", Phase: "done", Percent: 100, Message: "Claude CLI installed successfully"})
	return nil
}

// InstallAll installs all missing dependencies in order: git → brew → tmux → node → claude.
// It skips already-installed dependencies.
func InstallAll(ctx context.Context, onProgress ProgressFunc) error {
	statuses := CheckAll()

	var errs []string
	for _, ds := range statuses {
		if ds.Installed {
			onProgress(InstallProgress{Dep: ds.Name, Phase: "done", Percent: 100, Message: ds.Label + " already installed"})
			continue
		}
		if !ds.CanAutoInstall {
			onProgress(InstallProgress{Dep: ds.Name, Phase: "error", Percent: 0, Message: "Cannot auto-install: " + ds.HelpText})
			errs = append(errs, fmt.Sprintf("%s: cannot auto-install", ds.Name))
			continue
		}

		var err error
		switch ds.Name {
		case "git":
			err = InstallGit(ctx, onProgress)
		case "brew":
			err = InstallHomebrew(ctx, onProgress)
		case "tmux":
			err = InstallTmux(ctx, onProgress)
		case "node":
			err = InstallNode(ctx, onProgress)
		case "claude":
			err = InstallClaude(ctx, onProgress)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ds.Name, err))
			// If brew failed, skip tmux
			if ds.Name == "brew" {
				onProgress(InstallProgress{Dep: "tmux", Phase: "error", Percent: 0, Message: "Skipped: Homebrew installation failed"})
				errs = append(errs, "tmux: skipped (brew failed)")
				continue
			}
			// If node failed, skip claude
			if ds.Name == "node" {
				onProgress(InstallProgress{Dep: "claude", Phase: "error", Percent: 0, Message: "Skipped: Node.js installation failed"})
				errs = append(errs, "claude: skipped (node failed)")
				break
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("some dependencies failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// refreshPath adds common binary directories to PATH so newly installed
// tools can be found without restarting the app.
func refreshPath() {
	home, _ := os.UserHomeDir()
	extraPaths := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
		filepath.Join(home, ".nvm/versions/node"),
		"/usr/local/lib/node_modules/.bin",
	}

	// Also add npm global bin path if npm is available
	if npmPath := findDepPath("npm"); npmPath != "" {
		if out, err := exec.Command(npmPath, "config", "get", "prefix").Output(); err == nil {
			prefix := strings.TrimSpace(string(out))
			if prefix != "" {
				extraPaths = append(extraPaths, filepath.Join(prefix, "bin"))
			}
		}
	}

	currentPath := os.Getenv("PATH")
	for _, p := range extraPaths {
		if _, err := os.Stat(p); err == nil && !strings.Contains(currentPath, p) {
			currentPath = p + ":" + currentPath
		}
	}
	os.Setenv("PATH", currentPath)
	log.Printf("PATH refreshed: %s", currentPath)
}
