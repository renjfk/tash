package data

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"DEBUG", slog.LevelDebug},
		{"Info", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestInitLogger_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	cleanup := InitLogger(dir, "info")
	defer cleanup()

	path := filepath.Join(dir, logFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected log file to be created")
	}
}

func TestInitLogger_Off(t *testing.T) {
	dir := t.TempDir()

	cleanup := InitLogger(dir, "off")
	defer cleanup()

	path := filepath.Join(dir, logFile)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no log file when level is off")
	}
}

func TestInitLogger_Empty(t *testing.T) {
	dir := t.TempDir()

	cleanup := InitLogger(dir, "")
	defer cleanup()

	path := filepath.Join(dir, logFile)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no log file when level is empty")
	}
}

func TestRotateLog_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	oldPath := filepath.Join(dir, "test.log.old")

	// Small file: should not rotate
	_ = os.WriteFile(path, []byte("small"), 0644)
	rotateLog(path, oldPath)

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("expected no rotation for small file")
	}
}

func TestRotateLog_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	oldPath := filepath.Join(dir, "test.log.old")

	// Large file: should rotate
	data := make([]byte, maxLogSize+1)
	_ = os.WriteFile(path, data, 0644)
	rotateLog(path, oldPath)

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Error("expected .old file after rotation")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected original file to be renamed")
	}
}
