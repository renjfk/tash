package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/renjfk/tash/internal/data"
)

const (
	lastUpdateFile = "last_update"
	pidFile        = "tash_update.pid"
)

func tickRun(cfg *data.Config, exitCode int, command string, session string) error {
	dataDir := cfg.DataDir()

	slog.Debug("tick run",
		"command", command,
		"exit_code", exitCode,
		"session", session,
	)

	if command != "" {
		e := data.Entry{
			Type:     "shell",
			Content:  command,
			Session:  session,
			ExitCode: exitCode,
			Time:     time.Now().Unix(),
		}
		if err := data.AppendEntry(dataDir, e); err != nil {
			slog.Warn("append entry failed", "error", err)
		}
	}

	// Skip auto-intercept for SIGINT (130) — user cancelled deliberately
	if exitCode != 0 && exitCode != 130 && cfg.Behavior.AutoIntercept {
		slog.Debug("checking auto-intercept", "command", command, "exit_code", exitCode)
		handleFailedCommand(command)
	}

	lastUpdate, err := readLastUpdate(dataDir)
	if err != nil {
		slog.Debug("no last_update file", "error", err)
		lastUpdate = 0
	}

	now := time.Now().Unix()
	if now-lastUpdate < int64(cfg.Profile.RebuildInterval) {
		slog.Debug("profile rebuild skipped, within interval",
			"last_update", lastUpdate,
			"now", now,
			"interval", cfg.Profile.RebuildInterval,
		)
		return nil
	}

	historyPath := cfg.ResolvedHistoryPath()
	entries, _ := data.ReadHistory(historyPath, lastUpdate)
	if len(entries) == 0 {
		slog.Debug("no new history entries since last update")
		return nil
	}

	if isUpdateRunning(dataDir) {
		slog.Debug("background update already running, skipping fork")
		return nil
	}

	slog.Info("forking background profile update", "new_entries", len(entries))
	return forkUpdate()
}

// InitStats holds stats from the init process for display.
type InitStats struct {
	HistoryEntries int
	UniqueCommands int
	Binaries       int
}

func tickInit(cfg *data.Config) (*InitStats, error) {
	dataDir := cfg.DataDir()

	slog.Info("tick init start")

	binaries := scanPATH()
	slog.Debug("PATH scan complete", "binaries", len(binaries))

	historyPath := cfg.ResolvedHistoryPath()
	stats, err := data.AnalyzeHistory(historyPath)
	if err != nil {
		return nil, fmt.Errorf("history analysis: %w", err)
	}
	slog.Debug("history analysis complete", "total_entries", stats.TotalEntries)

	input := &RebuildInput{
		HistoryStats:   stats.FormatStats(),
		InstalledTools: binaries,
		IsIncremental:  false,
	}

	content, err := rebuildProfile(cfg, input)
	if err != nil {
		return nil, fmt.Errorf("profile generation: %w", err)
	}
	slog.Debug("profile generated", "content_len", len(content))

	if err := data.WriteProfile(dataDir, content); err != nil {
		return nil, fmt.Errorf("write profile: %w", err)
	}

	slog.Info("tick init complete")
	if err := writeLastUpdate(dataDir, time.Now().Unix()); err != nil {
		return nil, err
	}

	return &InitStats{
		HistoryEntries: stats.TotalEntries,
		UniqueCommands: len(stats.CommandFreq),
		Binaries:       len(binaries),
	}, nil
}

func tickBackgroundUpdate(cfg *data.Config) error {
	dataDir := cfg.DataDir()

	slog.Info("background update start")

	writePIDFile(dataDir)
	defer removePIDFile(dataDir)

	lastUpdate, _ := readLastUpdate(dataDir)

	historyPath := cfg.ResolvedHistoryPath()
	entries, err := data.ReadHistory(historyPath, lastUpdate)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		slog.Debug("background update: no new history entries")
		return nil
	}

	slog.Debug("background update: new history entries", "count", len(entries))

	stats := &data.HistoryStats{
		CommandFreq:  make(map[string]int),
		TotalEntries: len(entries),
	}
	for _, e := range entries {
		parts := strings.Fields(e.Command)
		if len(parts) > 0 {
			stats.CommandFreq[parts[0]]++
		}
	}

	currentProfile, _ := data.ReadProfile(dataDir)
	currentContent := ""
	if currentProfile != nil {
		currentContent = currentProfile.Content
	}

	binaries := scanPATH()

	input := &RebuildInput{
		CurrentProfile: currentContent,
		HistoryStats:   stats.FormatStats(),
		InstalledTools: binaries,
		IsIncremental:  true,
	}

	content, err := rebuildProfile(cfg, input)
	if err != nil {
		return err
	}

	if strings.TrimSpace(content) == "NO_CHANGE" {
		slog.Info("background update: profile unchanged")
		return writeLastUpdate(dataDir, time.Now().Unix())
	}

	if err := data.WriteProfile(dataDir, content); err != nil {
		return err
	}

	slog.Info("background update: profile updated", "content_len", len(content))
	return writeLastUpdate(dataDir, time.Now().Unix())
}

func handleFailedCommand(command string) {
	if looksLikeNaturalLanguage(command) {
		slog.Debug("auto-intercept: natural language detected", "command", command)
		os.Exit(7)
	}
	slog.Debug("auto-intercept: not natural language, skipping", "command", command)
}

func forkUpdate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "tick", "--bg-update")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return err
	}

	slog.Debug("forked background update", "pid", cmd.Process.Pid)
	return nil
}

func looksLikeNaturalLanguage(command string) bool {
	words := strings.Fields(command)
	if len(words) < 3 {
		return false
	}

	for _, w := range words {
		if strings.HasPrefix(w, "-") ||
			strings.Contains(w, "/") ||
			strings.Contains(w, "=") {
			return false
		}
	}

	if strings.ContainsAny(command, "|;&><$`(){}") {
		return false
	}

	for _, w := range words[1:] {
		if strings.Contains(w, ".") && !strings.HasSuffix(w, ".") {
			return false
		}
	}

	return true
}

func readLastUpdate(dataDir string) (int64, error) {
	d, err := os.ReadFile(filepath.Join(dataDir, lastUpdateFile))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(d)), 10, 64)
}

func writeLastUpdate(dataDir string, ts int64) error {
	path := filepath.Join(dataDir, lastUpdateFile)
	return os.WriteFile(path, []byte(strconv.FormatInt(ts, 10)), 0644)
}

func isUpdateRunning(dataDir string) bool {
	d, err := os.ReadFile(filepath.Join(dataDir, pidFile))
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(d)))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(os.Signal(nil))
	if err == nil {
		slog.Debug("existing update process running", "pid", pid)
	}
	return err == nil
}

func writePIDFile(dataDir string) {
	path := filepath.Join(dataDir, pidFile)
	pid := strconv.Itoa(os.Getpid())
	_ = os.WriteFile(path, []byte(pid), 0644)
}

func removePIDFile(dataDir string) {
	_ = os.Remove(filepath.Join(dataDir, pidFile))
}

// scanPATH scans all directories in PATH and returns a deduplicated list of available binaries.
func scanPATH() []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}

	seen := make(map[string]bool)
	var binaries []string

	dirs := strings.Split(pathEnv, ":")
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if seen[name] {
				continue
			}

			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Mode()&0111 != 0 {
				seen[name] = true
				binaries = append(binaries, name)
			}
		}
	}

	sort.Strings(binaries)
	return binaries
}
