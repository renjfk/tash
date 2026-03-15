package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const usageFile = "usage.json"

// ActionStats holds per-action token counters.
type ActionStats struct {
	Calls  int `json:"calls"`
	Prompt int `json:"prompt"`
	Comp   int `json:"completion"`
}

// UsageStats holds accumulated usage statistics with per-action counters.
type UsageStats struct {
	Query       ActionStats `json:"query"`
	Rebuild     ActionStats `json:"rebuild"`
	TotalPrompt int         `json:"total_prompt"`
	TotalComp   int         `json:"total_completion"`
	TotalCalls  int         `json:"total_calls"`
	FirstCall   int64       `json:"first_call"`
	LastCall    int64       `json:"last_call"`
}

// RecordUsage increments counters for the given action.
func RecordUsage(dataDir string, action string, _ string, promptTokens int, completionTokens int) error {
	stats, _ := LoadUsage(dataDir)

	now := time.Now().Unix()

	switch action {
	case "query":
		stats.Query.Calls++
		stats.Query.Prompt += promptTokens
		stats.Query.Comp += completionTokens
	case "rebuild":
		stats.Rebuild.Calls++
		stats.Rebuild.Prompt += promptTokens
		stats.Rebuild.Comp += completionTokens
	}

	stats.TotalPrompt += promptTokens
	stats.TotalComp += completionTokens
	stats.TotalCalls++
	if stats.FirstCall == 0 {
		stats.FirstCall = now
	}
	stats.LastCall = now

	return saveUsage(dataDir, stats)
}

// LoadUsage reads the usage stats file. Returns empty stats if file doesn't exist.
// Old format files (with records array) are silently discarded and rewritten.
func LoadUsage(dataDir string) (*UsageStats, error) {
	path := filepath.Join(dataDir, usageFile)

	raw, err := os.ReadFile(path)
	if err != nil {
		return &UsageStats{}, nil //nolint:nilerr
	}

	var stats UsageStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		// Corrupt or old format — start fresh and overwrite.
		empty := &UsageStats{}
		_ = saveUsage(dataDir, empty)
		return empty, nil //nolint:nilerr
	}

	return &stats, nil
}

// ResetUsage clears the usage stats file.
func ResetUsage(dataDir string) error {
	path := filepath.Join(dataDir, usageFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reset usage: %w", err)
	}
	return nil
}

func saveUsage(dataDir string, stats *UsageStats) error {
	path := filepath.Join(dataDir, usageFile)

	raw, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal usage: %w", err)
	}

	return os.WriteFile(path, raw, 0644)
}
