package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Release tags from GitHub (v-prefixed)
		{"release tag v0.5", "v0.5", "0.5"},
		{"release tag v0.1", "v0.1", "0.1"},
		{"three part tag", "v1.2.3", "1.2.3"},

		// Makefile builds (git describe output)
		{"dirty build", "v0.4-dirty", "0.4"},
		{"describe with commits", "v0.4-3-gabcdef", "0.4"},

		// Goreleaser snapshot
		{"snapshot", "0.4-SNAPSHOT-24fc03a", "0.4"},

		// Goreleaser release (no v prefix)
		{"goreleaser release", "0.5", "0.5"},
		{"goreleaser three part", "1.2.3", "1.2.3"},

		// Pre-release
		{"pre-release", "1.0.0-rc1", "1.0.0"},
		{"pre-release two part", "0.6-beta", "0.6"},

		// Whitespace
		{"leading space", " v1.0 ", "1.0"},
		{"tabs", "\tv0.5\t", "0.5"},

		// No prefix
		{"plain version", "0.1", "0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanVersion(tt.input)
			if got != tt.want {
				t.Errorf("cleanVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		// Two-part versions (project convention: v0.1, v0.2, ...)
		{"minor bump two part", "0.6", "0.5", true},
		{"same two part", "0.5", "0.5", false},
		{"older two part", "0.4", "0.5", false},
		{"major bump two part", "1.0", "0.9", true},

		// Mixed: remote two-part vs local three-part (or vice versa)
		{"two vs three same base", "0.5", "0.5.0", false},
		{"two newer than three", "0.6", "0.5.1", true},
		{"three newer than two", "0.5.1", "0.5", true},

		// Three-part versions
		{"major bump", "2.0.0", "1.0.0", true},
		{"minor bump", "1.1.0", "1.0.0", true},
		{"patch bump", "1.0.1", "1.0.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"older version", "1.0.0", "1.0.1", false},
		{"major older", "1.0.0", "2.0.0", false},
		{"large version", "10.20.30", "10.20.29", true},

		// Single-part versions
		{"single part newer", "2", "1", true},
		{"single part same", "1", "1", false},
		{"single part older", "1", "2", false},
		{"single vs two part", "1", "0.9", true},

		// Invalid
		{"invalid a", "abc", "1.0", false},
		{"invalid b", "1.0", "abc", false},
		{"empty a", "", "1.0", false},
		{"empty b", "1.0", "", false},
		{"both empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewerVersion(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestIsNewerVersion_RealScenarios tests the full cleanVersion + isNewerVersion
// pipeline with actual version strings produced by the build system.
func TestIsNewerVersion_RealScenarios(t *testing.T) {
	tests := []struct {
		name   string
		remote string // GitHub release tag_name
		local  string // compiled version variable
		want   bool
	}{
		// New release available
		{"release vs makefile build", "v0.6", "0.5", true},
		{"release vs goreleaser build", "v0.6", "0.5", true},
		{"release vs dirty build", "v0.6", "0.5-dirty", true},
		{"release vs snapshot", "v0.6", "0.5-SNAPSHOT-24fc03a", true},

		// Already up to date
		{"same release vs makefile", "v0.5", "0.5", false},
		{"same release vs goreleaser", "v0.5", "0.5", false},
		{"same release vs dirty", "v0.5", "0.5-dirty", false},
		{"same release vs snapshot", "v0.5", "0.5-SNAPSHOT-24fc03a", false},

		// Running newer than latest release (dev ahead)
		{"older release vs dirty", "v0.4", "0.5-dirty", false},
		{"older release vs snapshot", "v0.4", "0.5-SNAPSHOT-24fc03a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote := cleanVersion(tt.remote)
			local := cleanVersion(tt.local)
			got := isNewerVersion(remote, local)
			if got != tt.want {
				t.Errorf("isNewerVersion(cleanVersion(%q)=%q, cleanVersion(%q)=%q) = %v, want %v",
					tt.remote, remote, tt.local, local, got, tt.want)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		// Project convention: two-part
		{"two part", "0.5", []int{0, 5, 0}},
		{"two part major", "1.0", []int{1, 0, 0}},

		// Standard three-part
		{"three part", "1.2.3", []int{1, 2, 3}},
		{"three part zeros", "0.0.0", []int{0, 0, 0}},
		{"three part large", "10.20.30", []int{10, 20, 30}},

		// Single-part
		{"single part", "5", []int{5, 0, 0}},

		// Invalid
		{"empty", "", nil},
		{"non-numeric", "abc", nil},
		{"partial non-numeric", "1.2.abc", nil},
		{"too many parts", "1.2.3.4", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitVersion(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitVersion(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Errorf("splitVersion(%q) = nil, want %v", tt.input, tt.want)
				return
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitVersion(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestReadUpdateAvailable(t *testing.T) {
	dir := t.TempDir()

	// No file
	if info := ReadUpdateAvailable(dir, "0.5"); info != nil {
		t.Error("expected nil when no file exists")
	}

	// Valid file — flagged version newer than current
	info := UpdateInfo{Version: "0.6"}
	d, _ := json.Marshal(info)
	_ = os.WriteFile(filepath.Join(dir, updateAvailableFile), d, 0644)

	got := ReadUpdateAvailable(dir, "0.5")
	if got == nil {
		t.Fatal("expected non-nil UpdateInfo")
	}
	if got.Version != "0.6" {
		t.Errorf("version = %q, want %q", got.Version, "0.6")
	}

	// Already updated — current matches flagged version
	if info := ReadUpdateAvailable(dir, "0.6"); info != nil {
		t.Error("expected nil when current version matches flagged version")
	}

	// Already updated — current exceeds flagged (dirty build)
	if info := ReadUpdateAvailable(dir, "0.6-dirty"); info != nil {
		t.Error("expected nil when current version matches flagged (dirty)")
	}

	// Already updated — current is ahead of flagged
	if info := ReadUpdateAvailable(dir, "0.7"); info != nil {
		t.Error("expected nil when current version is ahead of flagged")
	}

	// Invalid JSON
	_ = os.WriteFile(filepath.Join(dir, updateAvailableFile), []byte("not json"), 0644)
	if info := ReadUpdateAvailable(dir, "0.5"); info != nil {
		t.Error("expected nil for invalid JSON")
	}

	// Empty version
	_ = os.WriteFile(filepath.Join(dir, updateAvailableFile), []byte(`{"version":""}`), 0644)
	if info := ReadUpdateAvailable(dir, "0.5"); info != nil {
		t.Error("expected nil for empty version")
	}
}

func TestShouldCheckVersion(t *testing.T) {
	dir := t.TempDir()

	// No file — should check
	if !ShouldCheckVersion(dir) {
		t.Error("expected true when no file exists")
	}

	// Recent check — should not check
	path := filepath.Join(dir, lastVersionCheckFile)
	_ = os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0644)
	if ShouldCheckVersion(dir) {
		t.Error("expected false when recently checked")
	}

	// Old check — should check
	old := time.Now().Unix() - versionCheckInterval - 1
	_ = os.WriteFile(path, []byte(strconv.FormatInt(old, 10)), 0644)
	if !ShouldCheckVersion(dir) {
		t.Error("expected true when check is stale")
	}

	// Invalid content
	_ = os.WriteFile(path, []byte("garbage"), 0644)
	if !ShouldCheckVersion(dir) {
		t.Error("expected true for invalid file content")
	}
}

func TestWriteVersionCheckTimestamp(t *testing.T) {
	dir := t.TempDir()
	WriteVersionCheckTimestamp(dir)

	d, err := os.ReadFile(filepath.Join(dir, lastVersionCheckFile))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	ts, err := strconv.ParseInt(string(d), 10, 64)
	if err != nil {
		t.Fatalf("expected valid timestamp: %v", err)
	}

	now := time.Now().Unix()
	if now-ts > 5 {
		t.Errorf("timestamp too old: %d (now: %d)", ts, now)
	}
}

func TestFormatUpdateNotification(t *testing.T) {
	info := &UpdateInfo{Version: "0.6"}
	got := FormatUpdateNotification(info)
	if got == "" {
		t.Error("expected non-empty notification")
	}
	if !strings.Contains(got, "0.6") {
		t.Errorf("notification should contain version, got %q", got)
	}
	if !strings.Contains(got, "tash:") {
		t.Errorf("notification should contain 'tash:' prefix, got %q", got)
	}
}

func TestCheckForUpdate_DevVersion(t *testing.T) {
	dir := t.TempDir()
	// Should not create a flag file for dev builds
	CheckForUpdate(dir, "dev")
	if _, err := os.Stat(filepath.Join(dir, updateAvailableFile)); !os.IsNotExist(err) {
		t.Error("expected no flag file for dev version")
	}
}

func TestUpgradeCommand(t *testing.T) {
	// Just verify it returns something and doesn't panic
	cmd := UpgradeCommand("0.6")
	if cmd == "" {
		t.Error("expected non-empty upgrade command")
	}
}
