package data

import (
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
	if len(stats.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(stats.Records))
	}
	if stats.Records[0].Action != "query" {
		t.Errorf("expected query action, got %q", stats.Records[0].Action)
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
}
