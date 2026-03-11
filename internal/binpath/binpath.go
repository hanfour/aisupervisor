package binpath

import (
	"os"
	"path/filepath"
)

// PrependBundledBin checks if the app is running from a macOS .app bundle
// and prepends the bundled bin directory to PATH so that bundled tmux/git
// are found before (or instead of) system-installed versions.
func PrependBundledBin() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	// Resolve symlinks so we get the real path inside .app/Contents/MacOS/
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	// .app/Contents/MacOS/aisupervisor-gui → .app/Contents/Resources/bin
	bundledBin := filepath.Join(filepath.Dir(exe), "..", "Resources", "bin")
	if info, err := os.Stat(bundledBin); err == nil && info.IsDir() {
		os.Setenv("PATH", bundledBin+":"+os.Getenv("PATH"))
	}
}

// BundledBinDir returns the path to the bundled bin directory if it exists,
// or an empty string otherwise.
func BundledBinDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	bundledBin := filepath.Join(filepath.Dir(exe), "..", "Resources", "bin")
	if info, err := os.Stat(bundledBin); err == nil && info.IsDir() {
		return bundledBin
	}
	return ""
}
