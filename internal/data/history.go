package data

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// HistoryEntry represents a single fish history entry.
type HistoryEntry struct {
	Command   string
	Timestamp int64
}

// HistoryStats holds pre-processed history data for profile rebuilding.
type HistoryStats struct {
	CommandFreq  map[string]int
	TotalEntries int
}

// ReadHistory reads fish history entries since the given timestamp.
// Parses the fish history file line-by-line to handle commands containing
// YAML-special characters (colons, brackets, etc.) that break yaml.Unmarshal.
func ReadHistory(historyPath string, sinceTimestamp int64) ([]HistoryEntry, error) {
	f, err := os.Open(historyPath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	var all []HistoryEntry
	var current HistoryEntry
	hasCmd := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "- cmd: ") {
			if hasCmd {
				all = append(all, current)
			}
			current = HistoryEntry{Command: line[7:]}
			hasCmd = true
			continue
		}

		if hasCmd && strings.HasPrefix(line, "  when: ") {
			current.Timestamp, _ = strconv.ParseInt(strings.TrimSpace(line[8:]), 10, 64)
			continue
		}
	}

	if hasCmd {
		all = append(all, current)
	}

	if sinceTimestamp == 0 {
		return all, nil
	}

	var filtered []HistoryEntry
	for _, e := range all {
		if e.Timestamp >= sinceTimestamp {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// AnalyzeHistory reads all history and produces statistics.
func AnalyzeHistory(historyPath string) (*HistoryStats, error) {
	entries, err := ReadHistory(historyPath, 0)
	if err != nil {
		return nil, err
	}

	stats := &HistoryStats{
		CommandFreq:  make(map[string]int),
		TotalEntries: len(entries),
	}

	for _, e := range entries {
		parts := strings.Fields(e.Command)
		if len(parts) == 0 {
			continue
		}
		baseCmd := parts[0]

		if baseCmd == "sudo" && len(parts) > 1 {
			baseCmd = parts[1]
		}

		stats.CommandFreq[baseCmd]++
	}

	return stats, nil
}

// SearchHistory finds history entries matching a filter (regex or substring) and
// returns the last count commands. If filter is empty, returns the last count entries.
// Streams the file line-by-line to avoid allocating the full history into memory.
func SearchHistory(historyPath string, filter string, count int) []string {
	if count <= 0 {
		count = 50
	}
	if count > 200 {
		count = 200
	}

	f, err := os.Open(historyPath)
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck

	// Ring buffer: keep only the last `count` matching commands
	ring := make([]string, count)
	total := 0

	var re *regexp.Regexp
	if filter != "" {
		re, _ = regexp.Compile(filter)
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "- cmd: ") {
			continue
		}
		cmd := line[7:]

		if filter != "" {
			if re != nil {
				if !re.MatchString(cmd) {
					continue
				}
			} else if !strings.Contains(cmd, filter) {
				continue
			}
		}

		ring[total%count] = cmd
		total++
	}

	if total == 0 {
		return nil
	}

	// Unwind ring buffer into chronological order
	n := total
	if n > count {
		n = count
	}
	result := make([]string, n)
	start := total - n
	for i := 0; i < n; i++ {
		result[i] = ring[(start+i)%count]
	}
	return result
}

// FormatStats formats history statistics for AI consumption.
func (s *HistoryStats) FormatStats() string {
	var b strings.Builder

	b.WriteString("Command frequency (top 30):\n")

	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range s.CommandFreq {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Value > sorted[j].Value })

	count := 30
	if len(sorted) < count {
		count = len(sorted)
	}
	for _, kv := range sorted[:count] {
		b.WriteString("  " + kv.Key + "(" + strconv.Itoa(kv.Value) + ")\n")
	}

	return b.String()
}
