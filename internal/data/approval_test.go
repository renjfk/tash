package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApprovalState_ActiveAndRemaining(t *testing.T) {
	now := time.Now()
	state := &ApprovalState{AllowedUntil: now.Add(30 * time.Minute).Unix(), Timeframe: "30m"}

	if !state.Active(now) {
		t.Error("expected state to be active")
	}
	if got := state.Remaining(now); got < 29*time.Minute || got > 30*time.Minute {
		t.Errorf("unexpected remaining %s", got)
	}

	expired := &ApprovalState{AllowedUntil: now.Add(-time.Minute).Unix()}
	if expired.Active(now) {
		t.Error("expected expired state to be inactive")
	}
	if got := expired.Remaining(now); got != 0 {
		t.Errorf("expected zero remaining, got %s", got)
	}
}

func TestLoadApprovalState_Missing(t *testing.T) {
	state, err := LoadApprovalState(t.TempDir())
	if err != nil {
		t.Fatalf("LoadApprovalState: %v", err)
	}
	if state.Active(time.Now()) {
		t.Error("missing file should yield inactive state")
	}
}

func TestLoadApprovalState_Corrupt(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, approvalFile), []byte("not json"), 0644)

	state, err := LoadApprovalState(dir)
	if err != nil {
		t.Fatalf("LoadApprovalState: %v", err)
	}
	if state.Active(time.Now()) {
		t.Error("corrupt file should yield inactive state")
	}
}

func TestGrantApproval_Persists(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	granted, err := GrantApproval(dir, "15m", 15*time.Minute, now)
	if err != nil {
		t.Fatalf("GrantApproval: %v", err)
	}
	if !granted.Active(now) {
		t.Error("granted state should be active")
	}

	loaded, err := LoadApprovalState(dir)
	if err != nil {
		t.Fatalf("LoadApprovalState: %v", err)
	}
	if loaded.Timeframe != "15m" || loaded.AllowedUntil != granted.AllowedUntil {
		t.Errorf("persisted state mismatch: %+v vs %+v", loaded, granted)
	}
}

func TestClearApproval(t *testing.T) {
	dir := t.TempDir()
	if _, err := GrantApproval(dir, "15m", 15*time.Minute, time.Now()); err != nil {
		t.Fatalf("GrantApproval: %v", err)
	}
	if err := ClearApproval(dir); err != nil {
		t.Fatalf("ClearApproval: %v", err)
	}

	state, err := LoadApprovalState(dir)
	if err != nil {
		t.Fatalf("LoadApprovalState: %v", err)
	}
	if state.Active(time.Now()) {
		t.Error("expected cleared state to be inactive")
	}

	// Clearing again is a no-op.
	if err := ClearApproval(dir); err != nil {
		t.Errorf("ClearApproval on missing file: %v", err)
	}
}
