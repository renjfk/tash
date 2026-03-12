package data

import (
	"bytes"
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

func TestRotateLog_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	// Should not crash on nonexistent file
	rotateLog(filepath.Join(dir, "nope.log"), filepath.Join(dir, "nope.log.old"))
}

func TestInitLogger_Rotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, logFile)

	// Create a file larger than maxLogSize
	bigData := make([]byte, maxLogSize+1)
	_ = os.WriteFile(path, bigData, 0644)

	cleanup := InitLogger(dir, "info")
	defer cleanup()

	// Old file should exist after rotation
	if _, err := os.Stat(filepath.Join(dir, logFileOld)); os.IsNotExist(err) {
		t.Error("expected rotated log file")
	}
}

func TestWarn(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	Warn("test warning")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	got := buf.String()
	if got == "" {
		t.Error("expected warning output on stderr")
	}
	if !bytes.Contains(buf.Bytes(), []byte("tash: warning:")) {
		t.Errorf("expected 'tash: warning:' prefix, got %q", got)
	}
}

func TestError(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	Error("test error")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	got := buf.String()
	if got == "" {
		t.Error("expected error output on stderr")
	}
	if !bytes.Contains(buf.Bytes(), []byte("tash:")) {
		t.Errorf("expected 'tash:' prefix, got %q", got)
	}
}

func TestInfo(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	Info("test info")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	got := buf.String()
	if got == "" {
		t.Error("expected info output on stderr")
	}
	if !bytes.Contains(buf.Bytes(), []byte("tash:")) {
		t.Errorf("expected 'tash:' prefix, got %q", got)
	}
}

func TestInitLogger_Debug(t *testing.T) {
	dir := t.TempDir()
	cleanup := InitLogger(dir, "debug")
	defer cleanup()

	if _, err := os.Stat(filepath.Join(dir, logFile)); os.IsNotExist(err) {
		t.Error("expected log file to be created for debug level")
	}
}
