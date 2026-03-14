package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleHistory = `- cmd: git status
  when: 1700000001
- cmd: make build
  when: 1700000002
- cmd: docker ps -a
  when: 1700000003
- cmd: echo "hello: world"
  when: 1700000004
- cmd: kubectl get pods -n default
  when: 1700000005
`

func writeSampleHistory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fish_history")
	if err := os.WriteFile(path, []byte(sampleHistory), 0644); err != nil {
		t.Fatalf("write sample history: %v", err)
	}
	return path
}

func TestReadHistory_All(t *testing.T) {
	path := writeSampleHistory(t)

	entries, err := ReadHistory(path, 0)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	if entries[0].Command != "git status" {
		t.Errorf("expected git status, got %q", entries[0].Command)
	}
	if entries[0].Timestamp != 1700000001 {
		t.Errorf("expected timestamp 1700000001, got %d", entries[0].Timestamp)
	}
}

func TestReadHistory_SinceTimestamp(t *testing.T) {
	path := writeSampleHistory(t)

	entries, err := ReadHistory(path, 1700000003)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries since ts 1700000003, got %d", len(entries))
	}
	if entries[0].Command != "docker ps -a" {
		t.Errorf("expected docker ps -a, got %q", entries[0].Command)
	}
}

func TestReadHistory_YAMLSpecialChars(t *testing.T) {
	path := writeSampleHistory(t)

	entries, err := ReadHistory(path, 0)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	// "echo "hello: world"" contains a colon which would break yaml.Unmarshal
	found := false
	for _, e := range entries {
		if e.Command == `echo "hello: world"` {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected entry with colon in command to be parsed correctly")
	}
}

func TestReadHistory_MissingFile(t *testing.T) {
	_, err := ReadHistory("/nonexistent/path/fish_history", 0)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestAnalyzeHistory(t *testing.T) {
	path := writeSampleHistory(t)

	stats, err := AnalyzeHistory(path)
	if err != nil {
		t.Fatalf("AnalyzeHistory: %v", err)
	}

	if stats.TotalEntries != 5 {
		t.Errorf("expected 5 total entries, got %d", stats.TotalEntries)
	}

	if stats.CommandFreq["git"] != 1 {
		t.Errorf("expected git count 1, got %d", stats.CommandFreq["git"])
	}
	if stats.CommandFreq["make"] != 1 {
		t.Errorf("expected make count 1, got %d", stats.CommandFreq["make"])
	}
}

func TestSearchHistory_NoFilter(t *testing.T) {
	path := writeSampleHistory(t)

	results := SearchHistory(path, "", 10, 200)
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestSearchHistory_SubstringFilter(t *testing.T) {
	path := writeSampleHistory(t)

	results := SearchHistory(path, "docker", 10, 200)
	if len(results) != 1 {
		t.Fatalf("expected 1 docker result, got %d", len(results))
	}
	if results[0].Command != "docker ps -a" {
		t.Errorf("expected docker ps -a, got %q", results[0].Command)
	}
	if results[0].Timestamp != 1700000003 {
		t.Errorf("expected timestamp 1700000003, got %d", results[0].Timestamp)
	}
}

func TestSearchHistory_CountLimit(t *testing.T) {
	path := writeSampleHistory(t)

	results := SearchHistory(path, "", 2, 200)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	// Should be the last 2 entries
	if results[1].Command != "kubectl get pods -n default" {
		t.Errorf("expected last entry, got %q", results[1].Command)
	}
}

func TestSearchHistory_MaxResultsCap(t *testing.T) {
	path := writeSampleHistory(t)

	// Request count 10 but maxResults is 3 — should be capped to 3
	results := SearchHistory(path, "", 10, 3)
	if len(results) != 3 {
		t.Errorf("expected 3 results (capped by maxResults), got %d", len(results))
	}
}

func TestSearchHistory_MissingFile(t *testing.T) {
	results := SearchHistory("/nonexistent/path", "git", 10, 200)
	if results != nil {
		t.Errorf("expected nil for missing file, got %v", results)
	}
}

func TestFormatHistory(t *testing.T) {
	entries := []HistoryEntry{
		{Command: "git status", Timestamp: 1700000001},
		{Command: "ls", Timestamp: 0},
		{Command: "make build", Timestamp: 1700000005},
	}

	formatted := FormatHistory(entries)

	if len(formatted) != 3 {
		t.Fatalf("expected 3 formatted entries, got %d", len(formatted))
	}

	// Entry with timestamp should have [timestamp] prefix
	if !strings.Contains(formatted[0], "[") || !strings.Contains(formatted[0], "git status") {
		t.Errorf("expected timestamped entry, got %q", formatted[0])
	}

	// Entry with zero timestamp should be bare command
	if formatted[1] != "ls" {
		t.Errorf("expected bare command for zero timestamp, got %q", formatted[1])
	}

	// Entry with timestamp should have [timestamp] prefix
	if !strings.Contains(formatted[2], "[") || !strings.Contains(formatted[2], "make build") {
		t.Errorf("expected timestamped entry, got %q", formatted[2])
	}
}

func TestFormatHistory_Empty(t *testing.T) {
	formatted := FormatHistory(nil)
	if len(formatted) != 0 {
		t.Errorf("expected empty result, got %d entries", len(formatted))
	}
}

func TestFormatStats(t *testing.T) {
	stats := &HistoryStats{
		CommandFreq: map[string]int{
			"git":    50,
			"docker": 30,
			"ls":     10,
		},
		TotalEntries: 90,
	}

	got := stats.FormatStats()

	if !strings.Contains(got, "Command frequency (top 30):") {
		t.Error("expected header")
	}
	if !strings.Contains(got, "git(50)") {
		t.Error("expected git(50)")
	}
	if !strings.Contains(got, "docker(30)") {
		t.Error("expected docker(30)")
	}

	// git should appear before docker (higher count)
	gitIdx := strings.Index(got, "git(50)")
	dockerIdx := strings.Index(got, "docker(30)")
	if gitIdx > dockerIdx {
		t.Error("expected git before docker (sorted by frequency)")
	}
}

func TestFormatStats_Empty(t *testing.T) {
	stats := &HistoryStats{
		CommandFreq: map[string]int{},
	}

	got := stats.FormatStats()
	if !strings.Contains(got, "Command frequency") {
		t.Error("expected header even with empty stats")
	}
}
