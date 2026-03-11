package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// UpdateInfo describes an available update.
type UpdateInfo struct {
	Version      string `json:"version"`
	DownloadURL  string `json:"download_url"`
	ReleaseNotes string `json:"release_notes"`
}

// CheckForUpdates fetches version.json from updateURL and compares with currentVersion.
// Returns nil if already up to date or if the check fails gracefully.
func CheckForUpdates(currentVersion, updateURL string) (*UpdateInfo, error) {
	if updateURL == "" {
		return nil, nil
	}

	url := strings.TrimRight(updateURL, "/") + "/version.json"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching update info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update server returned %d", resp.StatusCode)
	}

	var info UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parsing update info: %w", err)
	}

	if info.Version == "" || info.Version == currentVersion {
		return nil, nil
	}

	// Simple comparison: if remote version differs from current, it's an update
	if compareVersions(info.Version, currentVersion) > 0 {
		return &info, nil
	}

	return nil, nil
}

// compareVersions compares two semver-like version strings.
// Returns >0 if a > b, 0 if equal, <0 if a < b.
func compareVersions(a, b string) int {
	aParts := parseVersion(a)
	bParts := parseVersion(b)

	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return 1
		}
		if aParts[i] < bParts[i] {
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	var parts [3]int
	fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}
