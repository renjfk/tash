package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const approvalFile = "approval.json"

// ApprovalState records the currently granted tash usage window.
type ApprovalState struct {
	AllowedUntil int64  `json:"allowed_until"` // unix seconds the window expires
	Timeframe    string `json:"timeframe"`     // timeframe label that was granted
	GrantedAt    int64  `json:"granted_at"`    // unix seconds the window started
}

// Active reports whether an approval window covers the given time.
func (s *ApprovalState) Active(now time.Time) bool {
	return s != nil && s.AllowedUntil > now.Unix()
}

// Remaining returns how much of the approval window is left, clamped at zero.
func (s *ApprovalState) Remaining(now time.Time) time.Duration {
	if s == nil || s.AllowedUntil <= now.Unix() {
		return 0
	}
	return time.Duration(s.AllowedUntil-now.Unix()) * time.Second
}

// LoadApprovalState reads the approval state file. Returns an empty state when
// the file is missing or corrupt so the gate simply requests a new approval.
func LoadApprovalState(dataDir string) (*ApprovalState, error) {
	path := filepath.Join(dataDir, approvalFile)

	raw, err := os.ReadFile(path)
	if err != nil {
		return &ApprovalState{}, nil //nolint:nilerr
	}

	var state ApprovalState
	if err := json.Unmarshal(raw, &state); err != nil {
		return &ApprovalState{}, nil //nolint:nilerr
	}

	return &state, nil
}

// SaveApprovalState writes the approval state file.
func SaveApprovalState(dataDir string, state *ApprovalState) error {
	path := filepath.Join(dataDir, approvalFile)

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval: %w", err)
	}

	return os.WriteFile(path, raw, 0644)
}

// GrantApproval sets the usage window to expire d after now and persists it.
func GrantApproval(dataDir string, timeframe string, d time.Duration, now time.Time) (*ApprovalState, error) {
	state := &ApprovalState{
		AllowedUntil: now.Add(d).Unix(),
		Timeframe:    timeframe,
		GrantedAt:    now.Unix(),
	}
	if err := SaveApprovalState(dataDir, state); err != nil {
		return nil, err
	}
	return state, nil
}

// ClearApproval removes the approval state file.
func ClearApproval(dataDir string) error {
	path := filepath.Join(dataDir, approvalFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear approval: %w", err)
	}
	return nil
}
