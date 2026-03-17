package data

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	logFile    = "tash.log"
	logFileOld = "tash.log.old"
	maxLogSize = 5 * 1024 * 1024 // 5 MB
)

// InitLogger configures the global slog logger based on the configured level.
// Supported levels: "debug", "info", "warn", "error". Default is "info".
// Set to "off" or empty string to disable logging.
// Logs are written to tash.log in the tash data directory.
// When the log exceeds 5 MB, the current file is rotated to tash.log.old.
// Returns a cleanup function to close the log file.
func InitLogger(dataDir string, level string) func() {
	if level == "" || level == "off" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() {}
	}

	path := filepath.Join(dataDir, logFile)
	rotateLog(path, filepath.Join(dataDir, logFileOld))

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() {}
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: parseLevel(level),
	})))

	return func() { f.Close() } //nolint:errcheck
}

// LogStyler provides styled rendering for user-facing log messages on stderr.
// The tui package registers a real implementation after theme setup.
type LogStyler struct {
	Warn   func(string) string // yellow/warning styling
	Error  func(string) string // red/error styling
	Dimmed func(string) string // faint/dimmed styling
}

var logStyler *LogStyler

// SetLogStyler registers style functions for user-facing log output.
// Must be called after theme initialization.
func SetLogStyler(s *LogStyler) {
	logStyler = s
}

// Warn prints a warning to stderr and logs it.
func Warn(msg string) {
	text := "tash: warning: " + msg
	if logStyler != nil && logStyler.Warn != nil {
		text = logStyler.Warn(text)
	}
	fmt.Fprintln(os.Stderr, text)
	slog.Warn(msg)
}

// Error prints an error to stderr and logs it.
func Error(msg string) {
	text := "tash: " + msg
	if logStyler != nil && logStyler.Error != nil {
		text = logStyler.Error(text)
	}
	fmt.Fprintln(os.Stderr, text)
	slog.Error(msg)
}

// Info prints a dimmed info message to stderr and logs it.
func Info(msg string) {
	text := "tash: " + msg
	if logStyler != nil && logStyler.Dimmed != nil {
		text = logStyler.Dimmed(text)
	}
	fmt.Fprintln(os.Stderr, text)
	slog.Info(msg)
}

func rotateLog(path string, oldPath string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogSize {
		return
	}
	os.Remove(oldPath)       //nolint:errcheck
	os.Rename(path, oldPath) //nolint:errcheck
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
