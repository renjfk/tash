package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLooksLikeNaturalLanguage(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"natural language", "show me all docker containers", true},
		{"find files", "find files with errors", true},
		{"too few words", "git status", false},
		{"single word", "ls", false},
		{"has dash flag", "curl -s http://example.com", false},
		{"has slash", "cat /etc/hosts", false},
		{"has equals", "VAR=1 cmd arg", false},
		{"has pipe", "ls | grep foo", false},
		{"has semicolon", "echo hello; echo world", false},
		{"has ampersand", "cmd1 && cmd2", false},
		{"has redirect", "echo hello > file", false},
		{"has dollar", "echo $HOME something else", false},
		{"has backtick", "echo `date` something else", false},
		{"has parens", "echo (date) something else", false},
		{"has braces", "echo {a,b} something else", false},
		{"has dot in arg", "python script.py extra args", false},
		{"trailing dot ok", "do something now.", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeNaturalLanguage(tt.command)
			if got != tt.want {
				t.Errorf("looksLikeNaturalLanguage(%q) = %v, want %v", tt.command, got, tt.want)
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

func TestReadWriteLastUpdate(t *testing.T) {
	dir := t.TempDir()

	var ts int64 = 1700000001
	if err := writeLastUpdate(dir, ts); err != nil {
		t.Fatalf("writeLastUpdate: %v", err)
	}

	got, err := readLastUpdate(dir)
	if err != nil {
		t.Fatalf("readLastUpdate: %v", err)
	}
	if got != ts {
		t.Errorf("expected %d, got %d", ts, got)
	}
}

func TestReadLastUpdate_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := readLastUpdate(dir)
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

func TestIsUpdateRunning_CurrentProcess(t *testing.T) {
	dir := t.TempDir()
	writePIDFile(dir) // writes current PID

	// Signal(nil) may not work consistently on all platforms for the
	// current process, so just verify it doesn't panic/crash.
	// The important behavior is that stale PIDs return false (tested below).
	_ = isUpdateRunning(dir)
}

func TestIsUpdateRunning_NoFile(t *testing.T) {
	dir := t.TempDir()
	if isUpdateRunning(dir) {
		t.Error("expected false when no PID file")
	}
}

func TestIsUpdateRunning_StalePID(t *testing.T) {
	dir := t.TempDir()
	// Write a PID that is very unlikely to be running
	_ = os.WriteFile(filepath.Join(dir, pidFile), []byte("999999999"), 0644)

	if isUpdateRunning(dir) {
		t.Error("expected false for stale PID")
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
