package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const usageFile = "usage.json"

// UsageRecord tracks token consumption for a single API call.
type UsageRecord struct {
	Action           string `json:"action"` // "query", "rebuild"
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Time             int64  `json:"time"`
}

// UsageStats holds accumulated usage statistics.
type UsageStats struct {
	Records     []UsageRecord `json:"records"`
	TotalPrompt int           `json:"total_prompt"`
	TotalComp   int           `json:"total_completion"`
	TotalCalls  int           `json:"total_calls"`
	FirstCall   int64         `json:"first_call"`
	LastCall    int64         `json:"last_call"`
}

// RecordUsage appends a usage record to the state file.
func RecordUsage(dataDir string, action string, model string, promptTokens int, completionTokens int) error {
	stats, _ := LoadUsage(dataDir)

	now := time.Now().Unix()
	record := UsageRecord{
		Action:           action,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Time:             now,
	}

	stats.Records = append(stats.Records, record)
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
func LoadUsage(dataDir string) (*UsageStats, error) {
	path := filepath.Join(dataDir, usageFile)

	raw, err := os.ReadFile(path)
	if err != nil {
		return &UsageStats{}, nil //nolint:nilerr
	}

	var stats UsageStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return &UsageStats{}, nil //nolint:nilerr
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
