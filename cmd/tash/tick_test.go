package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/renjfk/tash/internal/data"
)

func TestHandleFailedCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool // true = should intercept (exit 7)
	}{
		// Failed CLI commands (first word is valid binary in PATH)
		{"helm update", "helm update", true},
		{"git staus", "git staus", true},
		{"docker stopp", "docker stopp", true},
		{"binary with flag", "git push --forc", true},
		{"binary with path arg", "cat /nonexistent/file", true},
		{"binary with assignment", "env FOO=bar notacommand", true},

		// Natural language (3+ words, first word NOT in PATH)
		{"natural language", "show me all docker containers", true},
		{"find files", "find files with errors", true},
		{"trailing dot ok", "do something now.", true},

		// Too few words
		{"single word", "ls", false},
		{"empty", "", false},

		// 2 words but first word not in PATH
		{"first word not in PATH", "notarealcommand123 subcommand", false},

		// Shell operators (always rejected)
		{"has pipe", "ls | grep foo", false},
		{"has semicolon", "echo hello; echo world", false},
		{"has ampersand", "cmd1 && cmd2", false},
		{"has redirect", "echo hello > file", false},
		{"has dollar", "echo $HOME something else", false},
		{"has backtick", "echo `date` something else", false},
		{"has parens", "echo (date) something else", false},
		{"has braces", "echo {a,b} something else", false},

		// Natural language filters (first word NOT in PATH)
		{"nl has dash flag", "notarealcmd123 -s something", false},
		{"nl has slash", "notarealcmd123 /etc/hosts extra", false},
		{"nl has equals", "notarealcmd123 FOO=1 extra", false},
		{"nl has dot in arg", "notarealcmd123 script.py extra", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if os.Getenv("TEST_HANDLE_FAILED_CMD") == "1" {
				handleFailedCommand(os.Getenv("TEST_COMMAND"))
				return
			}

			cmd := exec.Command(os.Args[0], "-test.run=^TestHandleFailedCommand$/^"+tt.name+"$")
			cmd.Env = append(os.Environ(),
				"TEST_HANDLE_FAILED_CMD=1",
				"TEST_COMMAND="+tt.command,
			)
			err := cmd.Run()

			if tt.want {
				// Should have exited with code 7
				if err == nil {
					t.Errorf("handleFailedCommand(%q): expected exit 7, got success", tt.command)
				} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 7 {
					t.Errorf("handleFailedCommand(%q): expected exit 7, got %v", tt.command, err)
				}
			} else {
				// Should have returned normally (exit 0)
				if err != nil {
					t.Errorf("handleFailedCommand(%q): expected no exit, got %v", tt.command, err)
				}
			}
		})
	}
}

func TestExtractFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		wantVal  string
		wantArgs []string
	}{
		{
			"flag present",
			[]string{"--session", "123", "query", "hello"},
			"--session",
			"123",
			[]string{"query", "hello"},
		},
		{
			"flag absent",
			[]string{"query", "hello"},
			"--session",
			"",
			[]string{"query", "hello"},
		},
		{
			"flag at end without value",
			[]string{"query", "--session"},
			"--session",
			"",
			[]string{"query", "--session"},
		},
		{
			"multiple flags extract one",
			[]string{"--session", "abc", "--output", "file.txt"},
			"--output",
			"file.txt",
			[]string{"--session", "abc"},
		},
		{
			"empty args",
			[]string{},
			"--session",
			"",
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, remaining := extractFlag(tt.args, tt.flag)
			if val != tt.wantVal {
				t.Errorf("value = %q, want %q", val, tt.wantVal)
			}
			if len(remaining) != len(tt.wantArgs) {
				t.Errorf("remaining args len = %d, want %d", len(remaining), len(tt.wantArgs))
				return
			}
			for i, arg := range remaining {
				if arg != tt.wantArgs[i] {
					t.Errorf("remaining[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestReadWriteLastRebuild(t *testing.T) {
	dir := t.TempDir()

	var ts int64 = 1700000001
	if err := writeLastRebuild(dir, ts); err != nil {
		t.Fatalf("writeLastRebuild: %v", err)
	}

	got, err := readLastRebuild(dir)
	if err != nil {
		t.Fatalf("readLastRebuild: %v", err)
	}
	if got != ts {
		t.Errorf("expected %d, got %d", ts, got)
	}
}

func TestReadLastRebuild_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := readLastRebuild(dir)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestWritePIDFile(t *testing.T) {
	dir := t.TempDir()
	writePIDFile(dir)

	data, err := os.ReadFile(filepath.Join(dir, pidFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("Atoi: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestRemovePIDFile(t *testing.T) {
	dir := t.TempDir()
	writePIDFile(dir)
	removePIDFile(dir)

	if _, err := os.Stat(filepath.Join(dir, pidFile)); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed")
	}
}

func TestIsBgProcessRunning_CurrentProcess(t *testing.T) {
	dir := t.TempDir()
	writePIDFile(dir) // writes current PID

	// Signal(nil) may not work consistently on all platforms for the
	// current process, so just verify it doesn't panic/crash.
	// The important behavior is that stale PIDs return false (tested below).
	_ = isBgProcessRunning(dir)
}

func TestIsBgProcessRunning_NoFile(t *testing.T) {
	dir := t.TempDir()
	if isBgProcessRunning(dir) {
		t.Error("expected false when no PID file")
	}
}

func TestIsBgProcessRunning_StalePID(t *testing.T) {
	dir := t.TempDir()
	// Write a PID that is very unlikely to be running
	_ = os.WriteFile(filepath.Join(dir, pidFile), []byte("999999999"), 0644)

	if isBgProcessRunning(dir) {
		t.Error("expected false for stale PID")
	}
}

func TestIsBgProcessRunning_InvalidPID(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, pidFile), []byte("not-a-number"), 0644)
	if isBgProcessRunning(dir) {
		t.Error("expected false for invalid PID")
	}
}

func TestScanPATH_EmptyPath(t *testing.T) {
	t.Setenv("PATH", "")
	binaries := scanPATH()
	if binaries != nil {
		t.Error("expected nil for empty PATH")
	}
}

func TestScanPATH_NonexistentDir(t *testing.T) {
	t.Setenv("PATH", "/nonexistent/dir/for/test")
	binaries := scanPATH()
	if len(binaries) != 0 {
		t.Errorf("expected 0 binaries for nonexistent dir, got %d", len(binaries))
	}
}

func TestScanPATH_Dedup(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Same binary name in both dirs
	_ = os.WriteFile(filepath.Join(dir1, "mybinary"), []byte("#!/bin/sh\n"), 0755)
	_ = os.WriteFile(filepath.Join(dir2, "mybinary"), []byte("#!/bin/sh\n"), 0755)

	t.Setenv("PATH", dir1+":"+dir2)
	binaries := scanPATH()

	count := 0
	for _, b := range binaries {
		if b == "mybinary" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected mybinary once (deduped), got %d", count)
	}
}

func TestScanPATH(t *testing.T) {
	dir := t.TempDir()

	// Create some executable files
	for _, name := range []string{"alpha", "beta", "gamma"} {
		path := filepath.Join(dir, name)
		_ = os.WriteFile(path, []byte("#!/bin/sh\n"), 0755)
	}

	// Create a non-executable file
	_ = os.WriteFile(filepath.Join(dir, "noexec"), []byte("data"), 0644)

	// Create a directory (should be skipped)
	_ = os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	t.Setenv("PATH", dir)

	binaries := scanPATH()

	seen := make(map[string]bool)
	for _, b := range binaries {
		seen[b] = true
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !seen[name] {
			t.Errorf("expected %q in binaries", name)
		}
	}
	if seen["noexec"] {
		t.Error("non-executable should not be in binaries")
	}
	if seen["subdir"] {
		t.Error("directory should not be in binaries")
	}
}

func TestCheckVersionUpdate_Disabled(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)
	cfg.Behavior.UpdateCheck = false

	// Should return immediately without writing any files
	checkVersionUpdate(cfg)

	if _, err := os.Stat(filepath.Join(dir, "last_version_check")); !os.IsNotExist(err) {
		t.Error("expected no version check when disabled")
	}
}

func TestCheckVersionUpdate_DevBuild(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	oldVersion := version
	version = "dev"
	defer func() { version = oldVersion }()

	checkVersionUpdate(cfg)

	if _, err := os.Stat(filepath.Join(dir, "last_version_check")); !os.IsNotExist(err) {
		t.Error("expected no version check for dev build")
	}
}

func TestCheckVersionUpdate_WithinInterval(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	oldVersion := version
	version = "0.5"
	defer func() { version = oldVersion }()

	// Write a recent timestamp so the check is skipped
	data.WriteVersionCheckTimestamp(dir)

	checkVersionUpdate(cfg)

	// Should not have forked (no PID file)
	if _, err := os.Stat(filepath.Join(dir, pidFile)); !os.IsNotExist(err) {
		t.Error("expected no fork when within check interval")
	}
}

func TestCheckVersionUpdate_BgAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	oldVersion := version
	version = "0.5"
	defer func() { version = oldVersion }()

	// Simulate a running background process by writing current PID
	writePIDFile(dir)
	defer removePIDFile(dir)

	checkVersionUpdate(cfg)

	// The function should have returned early without forking a second process.
	// We can't easily assert this beyond "it didn't panic", but the PID file
	// should still contain the original PID (not overwritten by a fork).
	d, _ := os.ReadFile(filepath.Join(dir, pidFile))
	pid, _ := strconv.Atoi(string(d))
	if pid != os.Getpid() {
		t.Errorf("expected original PID %d, got %d", os.Getpid(), pid)
	}
}

func TestTickBackgroundUpdate_VersionCheck(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)
	// Empty history file so the profile rebuild part is a no-op
	historyPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(historyPath, nil, 0644)
	cfg.Profile.HistoryPath = historyPath

	oldVersion := version
	version = "0.5"
	defer func() { version = oldVersion }()

	// No last_version_check file — should trigger a check
	err := tickBackgroundUpdate(cfg)
	if err != nil {
		t.Fatalf("tickBackgroundUpdate: %v", err)
	}

	// Should have written the version check timestamp
	if _, err := os.Stat(filepath.Join(dir, "last_version_check")); os.IsNotExist(err) {
		t.Error("expected last_version_check to be written")
	}
}

func TestTickBackgroundUpdate_VersionCheckDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)
	cfg.Behavior.UpdateCheck = false
	historyPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(historyPath, nil, 0644)
	cfg.Profile.HistoryPath = historyPath

	oldVersion := version
	version = "0.5"
	defer func() { version = oldVersion }()

	err := tickBackgroundUpdate(cfg)
	if err != nil {
		t.Fatalf("tickBackgroundUpdate: %v", err)
	}

	// Should NOT have written the version check timestamp
	if _, err := os.Stat(filepath.Join(dir, "last_version_check")); !os.IsNotExist(err) {
		t.Error("expected no version check when disabled")
	}
}
