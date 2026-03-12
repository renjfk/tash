package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renjfk/tash/internal/data"
)

func TestTashBinPath(t *testing.T) {
	path := tashBinPath()
	if path == "" {
		t.Error("tashBinPath returned empty string")
	}
	// Should be an absolute path (or "tash" fallback)
	if path != "tash" && !filepath.IsAbs(path) {
		t.Errorf("expected absolute path or 'tash' fallback, got %q", path)
	}
}

func TestInstallFishIntegration(t *testing.T) {
	// Use a temp dir as HOME to avoid modifying real fish config
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := installFishIntegration(); err != nil {
		t.Fatalf("installFishIntegration: %v", err)
	}

	// Verify the file was created
	path := filepath.Join(home, ".config", "fish", "conf.d", "tash.fish")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected tash.fish to be created: %v", err)
	}

	content := string(data)

	// Verify key sections exist
	checks := []string{
		"fish_command_not_found",
		"tash_tick",
		"fish_greeting",
		"function t ",
		"function q ",
		"__tash_handled",
		"commandline -r",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("tash.fish missing expected content: %q", check)
		}
	}
}

func TestInstallFishIntegration_CreatesConfDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// conf.d shouldn't exist yet
	confDir := filepath.Join(home, ".config", "fish", "conf.d")
	if _, err := os.Stat(confDir); !os.IsNotExist(err) {
		t.Fatal("conf.d should not exist yet")
	}

	if err := installFishIntegration(); err != nil {
		t.Fatalf("installFishIntegration: %v", err)
	}

	if _, err := os.Stat(confDir); os.IsNotExist(err) {
		t.Error("conf.d should have been created")
	}
}

func TestRunGreet_NoError(_ *testing.T) {
	// runGreet writes to stderr. Just verify it doesn't panic.
	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull) //nolint:errcheck
	defer func() { os.Stderr = old }()

	runGreet()
}

func TestGreetMessages_NotEmpty(t *testing.T) {
	if len(greetMessages) == 0 {
		t.Error("greetMessages should not be empty")
	}
}

func TestRunUsage_NoData(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	oldArgs := os.Args
	os.Args = []string{"tash", "usage"}
	defer func() { os.Args = oldArgs }()

	// Should not panic with no usage data
	runUsage(cfg)
}

func TestRunUsage_WithData(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	// Record some usage
	_ = data.RecordUsage(dir, "query", "test-model", 100, 50)
	_ = data.RecordUsage(dir, "query", "test-model", 200, 75)
	_ = data.RecordUsage(dir, "rebuild", "test-model", 500, 100)

	oldArgs := os.Args
	os.Args = []string{"tash", "usage"}
	defer func() { os.Args = oldArgs }()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	runUsage(cfg)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "query") {
		t.Error("expected 'query' in usage output")
	}
	if !strings.Contains(got, "rebuild") {
		t.Error("expected 'rebuild' in usage output")
	}
	if !strings.Contains(got, "Token usage") {
		t.Error("expected 'Token usage' header")
	}
}

func TestRunUsage_Reset(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	// Record usage first
	_ = data.RecordUsage(dir, "query", "test-model", 100, 50)

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	oldArgs := os.Args
	os.Args = []string{"tash", "usage", "--reset"}
	defer func() { os.Args = oldArgs }()

	runUsage(cfg)

	// After reset, loading should show empty stats
	stats, _ := data.LoadUsage(dir)
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls after reset, got %d", stats.TotalCalls)
	}
}

func TestUsageText_Contains(t *testing.T) {
	checks := []string{
		"tash query",
		"tash init",
		"tash usage",
		"tash reset",
		"tash clear",
		"tash version",
	}
	for _, check := range checks {
		if !strings.Contains(usageText, check) {
			t.Errorf("usageText missing %q", check)
		}
	}
}
