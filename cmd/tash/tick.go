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
	lastRebuildFile = "last_profile_rebuild"
	pidFile         = "tash_bg.pid"
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

	// Check for version updates independently of profile rebuild.
	// This forks a background process only if a check is due.
	checkVersionUpdate(cfg)

	lastRebuild, err := readLastRebuild(dataDir)
	if err != nil {
		slog.Debug("no last_profile_rebuild file", "error", err)
		lastRebuild = 0
	}

	now := time.Now().Unix()
	if now-lastRebuild < int64(cfg.Profile.RebuildInterval) {
		slog.Debug("profile rebuild skipped, within interval",
			"last_update", lastRebuild,
			"now", now,
			"interval", cfg.Profile.RebuildInterval,
		)
		return nil
	}

	historyPath := cfg.ResolvedHistoryPath()
	entries, _ := data.ReadHistory(historyPath, lastRebuild)
	if len(entries) == 0 {
		slog.Debug("no new history entries since last update")
		return nil
	}

	if isBgProcessRunning(dataDir) {
		slog.Debug("background update already running, skipping fork")
		return nil
	}

	slog.Info("forking background profile update", "new_entries", len(entries))
	return forkUpdate()
}

// checkVersionUpdate forks a background update if a version check is due,
// even when the profile rebuild is not needed.
func checkVersionUpdate(cfg *data.Config) {
	if !cfg.Behavior.UpdateCheck {
		slog.Debug("version check disabled in config")
		return
	}
	if version == "dev" {
		slog.Debug("version check skipped, dev build")
		return
	}

	dataDir := cfg.DataDir()
	if !data.ShouldCheckVersion(dataDir) {
		slog.Debug("version check skipped, within interval")
		return
	}

	if isBgProcessRunning(dataDir) {
		slog.Debug("version check skipped, background update already running")
		return
	}

	slog.Info("forking background update for version check")
	_ = forkUpdate()
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

	// Only send tools the user has actually used — avoids sending 2000+ PATH
	// binaries when the AI only needs the ~50 the user actively uses.
	usedTools := filterUsedTools(binaries, stats.CommandFreq)
	slog.Debug("filtered tools", "used", len(usedTools), "total", len(binaries))

	input := &RebuildInput{
		HistoryStats:   stats.FormatStats(),
		InstalledTools: usedTools,
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
	if err := writeLastRebuild(dataDir, time.Now().Unix()); err != nil {
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

	// Version check runs in the background process regardless of profile rebuild.
	if cfg.Behavior.UpdateCheck && data.ShouldCheckVersion(dataDir) {
		slog.Info("checking for new tash release")
		data.CheckForUpdate(dataDir, version)
		data.WriteVersionCheckTimestamp(dataDir)
	}

	lastRebuild, _ := readLastRebuild(dataDir)

	historyPath := cfg.ResolvedHistoryPath()
	entries, err := data.ReadHistory(historyPath, lastRebuild)
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

	// Skip PATH scan for incremental rebuilds — the current profile already
	// has the tools list and history stats show what changed. This saves the
	// bulk of the prompt tokens (2000+ binaries → 0).
	input := &RebuildInput{
		CurrentProfile: currentContent,
		HistoryStats:   stats.FormatStats(),
		IsIncremental:  true,
	}

	content, err := rebuildProfile(cfg, input)
	if err != nil {
		return err
	}

	if strings.TrimSpace(content) == "NO_CHANGE" {
		slog.Info("background update: profile unchanged")
		return writeLastRebuild(dataDir, time.Now().Unix())
	}

	if err := data.WriteProfile(dataDir, content); err != nil {
		return err
	}

	slog.Info("background update: profile updated", "content_len", len(content))
	return writeLastRebuild(dataDir, time.Now().Unix())
}

// handleFailedCommand checks whether a failed command should be auto-intercepted.
// It catches failed CLI commands where the binary exists in PATH (e.g. "helm update",
// "git push --forc"), and natural language typed into the shell (3+ words with no
// shell syntax). Exits with code 7 to signal the fish hook to invoke tash query.
func handleFailedCommand(command string) {
	words := strings.Fields(command)
	if len(words) < 2 {
		slog.Debug("auto-intercept: too few words, skipping", "command", command)
		return
	}

	// Reject shell pipelines and compound commands — these are intentional
	// constructs, not typos or natural language
	if strings.ContainsAny(command, "|;&><$`(){}") {
		slog.Debug("auto-intercept: has shell operators, skipping", "command", command)
		return
	}

	// If the first word is a real binary in PATH, intercept — the user ran a
	// valid tool with wrong args/subcommand (e.g. "helm update", "git push --forc")
	if _, err := exec.LookPath(words[0]); err == nil {
		slog.Debug("auto-intercept: failed command with valid binary", "command", command)
		os.Exit(7)
	}

	// 3+ words without shell syntax: likely natural language
	if len(words) < 3 {
		slog.Debug("auto-intercept: 2 words, first not in PATH, skipping", "command", command)
		return
	}

	// Reject flags, paths, assignments, and file-like arguments
	for _, w := range words {
		if strings.HasPrefix(w, "-") ||
			strings.Contains(w, "/") ||
			strings.Contains(w, "=") {
			slog.Debug("auto-intercept: has flags/paths/assignments, skipping", "command", command)
			return
		}
	}
	for _, w := range words[1:] {
		if strings.Contains(w, ".") && !strings.HasSuffix(w, ".") {
			slog.Debug("auto-intercept: has file-like argument, skipping", "command", command)
			return
		}
	}

	slog.Debug("auto-intercept: natural language detected", "command", command)
	os.Exit(7)
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

func readLastRebuild(dataDir string) (int64, error) {
	d, err := os.ReadFile(filepath.Join(dataDir, lastRebuildFile))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(d)), 10, 64)
}

func writeLastRebuild(dataDir string, ts int64) error {
	path := filepath.Join(dataDir, lastRebuildFile)
	return os.WriteFile(path, []byte(strconv.FormatInt(ts, 10)), 0644)
}

func isBgProcessRunning(dataDir string) bool {
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

// filterUsedTools returns only binaries that appear in the command frequency map.
func filterUsedTools(binaries []string, commandFreq map[string]int) []string {
	var used []string
	for _, b := range binaries {
		if commandFreq[b] > 0 {
			used = append(used, b)
		}
	}
	return used
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
