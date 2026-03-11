package query

import (
	"testing"
)

func TestExtractFirstCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"simple command", "git status", "git"},
		{"command with flags", "ls -la /tmp", "ls"},
		{"piped command", "cat file.txt | grep error", "cat"},
		{"double ampersand", "mkdir foo && cd foo", "mkdir"},
		{"double pipe", "cmd1 || cmd2", "cmd1"},
		{"semicolon", "echo hello; echo world", "echo"},
		{"env var prefix", "GOFLAGS=-mod=vendor go build", "go"},
		{"multiple env vars", "FOO=1 BAR=2 cmd arg", "cmd"},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"single command no args", "pwd", "pwd"},
		{"single pipe char", "ls | sort", "ls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFirstCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("ExtractFirstCommand(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestCommandExists(t *testing.T) {
	// "ls" should exist on any unix system
	if !CommandExists("ls") {
		t.Error("expected ls to exist")
	}

	if CommandExists("nonexistent_binary_xyz_12345") {
		t.Error("expected nonexistent binary to not exist")
	}
}
