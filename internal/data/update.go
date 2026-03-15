package data

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	updateAvailableFile  = "update_available"
	lastVersionCheckFile = "last_version_check"
	versionCheckInterval = 86400 // 24 hours in seconds
	githubReleasesURL    = "https://api.github.com/repos/renjfk/tash/releases/latest"
)

// UpdateInfo holds information about an available update, written as the flag file.
type UpdateInfo struct {
	Version string `json:"version"`
}

// githubRelease is the minimal GitHub API response for /releases/latest.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// CheckForUpdate queries the GitHub releases API and writes a flag file if a newer
// version is available. Fails silently on network errors or rate limits.
// Skips the check if version is "dev" (development build).
func CheckForUpdate(dataDir string, currentVersion string) {
	if currentVersion == "dev" {
		slog.Debug("version check skipped, dev build")
		return
	}

	release, err := fetchLatestRelease()
	if err != nil {
		slog.Debug("version check failed", "error", err)
		return // fail silently
	}

	latestVersion := cleanVersion(release.TagName)
	localVersion := cleanVersion(currentVersion)
	slog.Debug("version check", "latest", latestVersion, "current", localVersion)

	if !isNewerVersion(latestVersion, localVersion) {
		slog.Debug("version check: up to date")
		_ = os.Remove(filepath.Join(dataDir, updateAvailableFile))
		return
	}

	slog.Info("new version available", "version", latestVersion)
	info := UpdateInfo{Version: latestVersion}
	d, err := json.Marshal(info)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dataDir, updateAvailableFile), d, 0644)
}

// ReadUpdateAvailable reads the update flag file and compares against the
// running version. Returns nil if no update is available, the file doesn't
// exist, or the running binary already matches or exceeds the flagged version.
func ReadUpdateAvailable(dataDir string, currentVersion string) *UpdateInfo {
	d, err := os.ReadFile(filepath.Join(dataDir, updateAvailableFile))
	if err != nil {
		return nil
	}

	var info UpdateInfo
	if err := json.Unmarshal(d, &info); err != nil {
		return nil
	}
	if info.Version == "" {
		return nil
	}
	if !isNewerVersion(info.Version, cleanVersion(currentVersion)) {
		return nil
	}
	return &info
}

// ShouldCheckVersion returns true if enough time has passed since the last version check.
func ShouldCheckVersion(dataDir string) bool {
	d, err := os.ReadFile(filepath.Join(dataDir, lastVersionCheckFile))
	if err != nil {
		return true // no file means never checked
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(string(d)), 10, 64)
	if err != nil {
		return true
	}

	return time.Now().Unix()-ts >= versionCheckInterval
}

// WriteVersionCheckTimestamp records that a version check was just performed.
func WriteVersionCheckTimestamp(dataDir string) {
	path := filepath.Join(dataDir, lastVersionCheckFile)
	_ = os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0644)
}

// UpgradeCommand returns the suggested upgrade command based on the binary's install path.
// The version parameter is used to build an exact release URL for the fallback case.
func UpgradeCommand(version string) string {
	releaseURL := fmt.Sprintf("https://github.com/renjfk/tash/releases/tag/v%s", version)

	exe, err := os.Executable()
	if err != nil {
		return "visit " + releaseURL
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}

	switch {
	case strings.HasPrefix(resolved, "/opt/homebrew/") || strings.HasPrefix(resolved, "/usr/local/Cellar/"):
		return "brew upgrade tash"
	case strings.Contains(resolved, "/.local/bin/"):
		return "curl -fsSL https://raw.githubusercontent.com/renjfk/tash/main/install.fish | fish"
	case strings.HasSuffix(resolved, "/bin/tash"):
		repoDir := filepath.Dir(filepath.Dir(resolved))
		_, hasGit := os.Stat(filepath.Join(repoDir, ".git"))
		_, hasMakefile := os.Stat(filepath.Join(repoDir, "Makefile"))
		if hasGit == nil && hasMakefile == nil {
			return "cd " + repoDir + " && git pull && make build"
		}
		return "visit " + releaseURL
	default:
		return "visit " + releaseURL
	}
}

// FormatUpdateNotification returns a one-liner notification string for display.
func FormatUpdateNotification(info *UpdateInfo) string {
	return fmt.Sprintf("tash: update available: v%s (%s)", info.Version, UpgradeCommand(info.Version))
}

func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	return &release, nil
}

// cleanVersion normalises a version string for comparison. It strips a leading
// "v" prefix, trims whitespace, and removes any suffix after a hyphen so that
// dirty builds ("0.4-dirty"), snapshots ("0.4-SNAPSHOT-24fc03a"), and
// pre-release tags ("1.0-rc1") all reduce to their numeric base ("0.4", "1.0").
func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		v = v[:idx]
	}
	return v
}

// isNewerVersion returns true if version a is strictly newer than version b.
// Versions may have 1–3 numeric parts (e.g. "0.5", "1.2.3"); missing parts
// are treated as zero. Non-numeric or empty versions return false.
func isNewerVersion(a, b string) bool {
	aParts := splitVersion(a)
	bParts := splitVersion(b)
	if aParts == nil || bParts == nil {
		return false
	}

	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return true
		}
		if aParts[i] < bParts[i] {
			return false
		}
	}
	return false
}

// splitVersion parses a dotted version string with 1–3 numeric parts into
// [3]int, padding missing parts with zero. Returns nil on invalid input.
func splitVersion(v string) []int {
	if v == "" {
		return nil
	}

	parts := strings.SplitN(v, ".", 3)
	if len(parts) == 0 || len(parts) > 3 {
		return nil
	}

	result := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		result[i] = n
	}
	return result
}
