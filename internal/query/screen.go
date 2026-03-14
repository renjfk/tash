package query

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/renjfk/tash/internal/data"
)

// ansiPattern matches ANSI escape sequences (CSI, OSC, and single-char escapes).
var ansiPattern = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07]*\x07|[()][0-9A-B]|[=>])`)

// captureScreen runs zellij dump-screen and returns the parsed terminal output
// as a constraint string. The lines parameter controls how many lines to return
// from the bottom of the dump. If lines exceeds the visible screen, it escalates
// to a full scrollback dump.
func captureScreen(cfg *data.Config, lines int) string {
	if !cfg.Behavior.ScreenCapture {
		return "Screen capture is disabled"
	}

	if os.Getenv("ZELLIJ") == "" {
		return "Not running inside Zellij — screen capture unavailable"
	}

	if lines <= 0 {
		lines = 20
	}

	maxLines := cfg.Behavior.ScreenCaptureMaxLines
	if maxLines <= 0 {
		maxLines = 200
	}
	if lines > maxLines {
		lines = maxLines
	}

	// First try visible screen dump
	content, err := zellijDumpScreen(false)
	if err != nil {
		slog.Warn("zellij dump-screen failed", "error", err)
		return fmt.Sprintf("Screen capture failed: %s", err)
	}

	parsed := parseScreenDump(content, lines)

	// If requested lines exceed visible content, escalate to full scrollback
	visibleLines := countNonEmpty(content)
	if lines > visibleLines {
		fullContent, err := zellijDumpScreen(true)
		if err != nil {
			slog.Warn("zellij dump-screen --full failed", "error", err)
			// Fall back to the visible content we already have
		} else {
			parsed = parseScreenDump(fullContent, lines)
		}
	}

	if parsed == "" {
		return "Screen capture returned empty content"
	}

	return "Terminal screen output:\n" + parsed
}

// zellijDumpScreen runs zellij action dump-screen to a temp file and returns its contents.
// If full is true, captures the full scrollback buffer.
func zellijDumpScreen(full bool) (string, error) {
	tmpFile, err := os.CreateTemp("", "tash-screen-*.txt")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	args := []string{"action", "dump-screen", tmpPath}
	if full {
		args = []string{"action", "dump-screen", "--full", tmpPath}
	}

	cmd := exec.Command("zellij", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("zellij dump-screen: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read screen dump: %w", err)
	}

	return string(content), nil
}

// parseScreenDump strips ANSI codes and trailing blank lines, then returns
// the last N lines from the dump.
func parseScreenDump(content string, lines int) string {
	content = stripANSI(content)

	// Split and trim trailing empty lines
	allLines := strings.Split(content, "\n")
	for len(allLines) > 0 && strings.TrimSpace(allLines[len(allLines)-1]) == "" {
		allLines = allLines[:len(allLines)-1]
	}

	if len(allLines) == 0 {
		return ""
	}

	// Take last N lines
	if lines < len(allLines) {
		allLines = allLines[len(allLines)-lines:]
	}

	return strings.Join(allLines, "\n")
}

// stripANSI removes ANSI escape sequences from text.
func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// countNonEmpty returns the number of non-whitespace-only lines in a string.
func countNonEmpty(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// initialScreenContext captures a small initial screen context (last 20 lines)
// to inject at query start, giving the AI visibility into what's on screen.
// Returns empty string if screen capture is unavailable or disabled.
func initialScreenContext(cfg *data.Config) string {
	if !cfg.Behavior.ScreenCapture {
		return ""
	}

	if os.Getenv("ZELLIJ") == "" {
		return ""
	}

	content, err := zellijDumpScreen(false)
	if err != nil {
		slog.Debug("initial screen capture skipped", "error", err)
		return ""
	}

	parsed := parseScreenDump(content, 20)
	if parsed == "" {
		return ""
	}

	return "Terminal screen (last 20 lines):\n" + parsed
}
