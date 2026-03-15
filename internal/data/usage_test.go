package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUsageCRUD(t *testing.T) {
	dir := t.TempDir()

	// Initially empty
	stats, err := LoadUsage(dir)
	if err != nil {
		t.Fatalf("LoadUsage: %v", err)
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 calls, got %d", stats.TotalCalls)
	}

	// Record usage
	if err := RecordUsage(dir, "query", "claude-sonnet-4-6", 100, 50); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	// Verify
	stats, err = LoadUsage(dir)
	if err != nil {
		t.Fatalf("LoadUsage: %v", err)
	}
	if stats.TotalCalls != 1 {
		t.Errorf("expected 1 call, got %d", stats.TotalCalls)
	}
	if stats.TotalPrompt != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", stats.TotalPrompt)
	}
	if stats.TotalComp != 50 {
		t.Errorf("expected 50 completion tokens, got %d", stats.TotalComp)
	}
	if stats.Query.Calls != 1 {
		t.Errorf("expected 1 query call, got %d", stats.Query.Calls)
	}
	if stats.Query.Prompt != 100 {
		t.Errorf("expected 100 query prompt tokens, got %d", stats.Query.Prompt)
	}

	// Record another
	_ = RecordUsage(dir, "rebuild", "claude-sonnet-4-6", 200, 100)
	stats, _ = LoadUsage(dir)
	if stats.TotalCalls != 2 {
		t.Errorf("expected 2 calls, got %d", stats.TotalCalls)
	}
	if stats.TotalPrompt != 300 {
		t.Errorf("expected 300 prompt tokens, got %d", stats.TotalPrompt)
	}
	if stats.Rebuild.Calls != 1 {
		t.Errorf("expected 1 rebuild call, got %d", stats.Rebuild.Calls)
	}

	// Reset
	if err := ResetUsage(dir); err != nil {
		t.Fatalf("ResetUsage: %v", err)
	}

	stats, _ = LoadUsage(dir)
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 calls after reset, got %d", stats.TotalCalls)
	}
}

func TestResetUsage_NoFile(t *testing.T) {
	dir := t.TempDir()
	if err := ResetUsage(dir); err != nil {
		t.Errorf("ResetUsage should not error on missing file: %v", err)
	}
}

func TestLoadUsage_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, usageFile), []byte("corrupt{"), 0644)

	stats, err := LoadUsage(dir)
	if err != nil {
		t.Fatalf("expected nil error for corrupt file, got %v", err)
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected empty stats for corrupt file, got %d calls", stats.TotalCalls)
	}

	// File should be rewritten in new format
	raw, err := os.ReadFile(filepath.Join(dir, usageFile))
	if err != nil {
		t.Fatalf("expected file to be rewritten, got %v", err)
	}
	var rewritten UsageStats
	if err := json.Unmarshal(raw, &rewritten); err != nil {
		t.Fatalf("rewritten file should be valid JSON: %v", err)
	}
}

func TestLoadUsage_OldFormat(t *testing.T) {
	dir := t.TempDir()
	// Old format had a "records" array — new format ignores unknown fields but
	// an incompatible old file would fail to unmarshal into the new struct.
	// Simulate by writing something that won't parse into UsageStats.
	old := `{"records":[{"action":"query","model":"m","prompt_tokens":100,"completion_tokens":50,"time":1}],"total_calls":1}`
	_ = os.WriteFile(filepath.Join(dir, usageFile), []byte(old), 0644)

	stats, err := LoadUsage(dir)
	if err != nil {
		t.Fatalf("expected nil error for old format, got %v", err)
	}
	// Old format JSON is still valid JSON that partially maps to new struct —
	// total_calls would survive but records would be silently ignored.
	// Either way, subsequent writes use the new format.
	if stats.TotalCalls != 1 {
		t.Errorf("expected old total_calls to carry over, got %d", stats.TotalCalls)
	}
}
