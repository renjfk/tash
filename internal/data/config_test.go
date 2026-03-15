package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model.Name != "claude-sonnet-4-6" {
		t.Errorf("expected claude-sonnet-4-6, got %q", cfg.Model.Name)
	}
	if cfg.Model.Endpoint != "https://api.anthropic.com/v1" {
		t.Errorf("expected anthropic endpoint, got %q", cfg.Model.Endpoint)
	}
	if cfg.Model.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("expected ANTHROPIC_API_KEY, got %q", cfg.Model.APIKeyEnv)
	}
	if cfg.Model.MaxTokens != 2048 {
		t.Errorf("expected 2048, got %d", cfg.Model.MaxTokens)
	}
	if cfg.Behavior.MaxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", cfg.Behavior.MaxRetries)
	}
	if cfg.Behavior.MaxToolCalls != 3 {
		t.Errorf("expected 3 max_tool_calls, got %d", cfg.Behavior.MaxToolCalls)
	}
	if cfg.Behavior.MaxMemories != 50 {
		t.Errorf("expected 50 memories, got %d", cfg.Behavior.MaxMemories)
	}
	if !cfg.Behavior.AutoIntercept {
		t.Error("expected auto_intercept true")
	}
	if !cfg.Behavior.ScreenCapture {
		t.Error("expected screen_capture true")
	}
	if cfg.Behavior.MaxContext != 500 {
		t.Errorf("expected 500 max_context, got %d", cfg.Behavior.MaxContext)
	}
	if cfg.Behavior.MaxHistoryResults != 200 {
		t.Errorf("expected 200 max_history_results, got %d", cfg.Behavior.MaxHistoryResults)
	}
	if cfg.Behavior.ScreenCaptureMaxLines != 200 {
		t.Errorf("expected 200 screen_capture_max_lines, got %d", cfg.Behavior.ScreenCaptureMaxLines)
	}
	if !cfg.Behavior.UpdateCheck {
		t.Error("expected update_check true")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected info, got %q", cfg.LogLevel)
	}
	if cfg.Profile.RebuildInterval != 86400 {
		t.Errorf("expected 86400, got %d", cfg.Profile.RebuildInterval)
	}
	if cfg.Terminal.ASCII {
		t.Error("expected terminal.ascii false by default")
	}
	if cfg.Terminal.Color != "auto" {
		t.Errorf("expected terminal.color auto, got %q", cfg.Terminal.Color)
	}
}

func TestDataDir(t *testing.T) {
	cfg := &Config{dataDir: "/tmp/test-tash"}
	if cfg.DataDir() != "/tmp/test-tash" {
		t.Errorf("expected /tmp/test-tash, got %q", cfg.DataDir())
	}
}

func TestAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	t.Setenv("ANTHROPIC_API_KEY", "test-key-123")

	key := cfg.APIKey()
	if key != "test-key-123" {
		t.Errorf("expected test-key-123, got %q", key)
	}
}

func TestAPIKey_Missing(t *testing.T) {
	cfg := DefaultConfig()
	t.Setenv("ANTHROPIC_API_KEY", "")

	key := cfg.APIKey()
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
	}
}

func TestResolvedHistoryPath_Absolute(t *testing.T) {
	cfg := &Config{Profile: ProfileConfig{HistoryPath: "/absolute/path/history"}}
	got := cfg.ResolvedHistoryPath()
	if got != "/absolute/path/history" {
		t.Errorf("expected absolute path unchanged, got %q", got)
	}
}

func TestResolvedHistoryPath_Tilde(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.ResolvedHistoryPath()

	// Should not start with ~ after resolution
	if len(got) > 0 && got[0] == '~' {
		t.Errorf("expected ~ to be expanded, got %q", got)
	}
}

func TestWriteDefault_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.dataDir = dir

	created, err := cfg.WriteDefault()
	if err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}
	if !created {
		t.Error("expected created=true for new file")
	}

	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); os.IsNotExist(err) {
		t.Error("expected config.yaml to exist")
	}
}

func TestWriteDefault_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("existing"), 0644)

	cfg := DefaultConfig()
	cfg.dataDir = dir

	created, err := cfg.WriteDefault()
	if err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}
	if created {
		t.Error("expected created=false for existing file")
	}
}

func TestMigrate_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.dataDir = dir

	migrated, err := cfg.Migrate()
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if migrated {
		t.Error("expected migrated=false when no file exists")
	}
}

func TestMigrate_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("model:\n  name: old-model\n"), 0644)

	cfg := DefaultConfig()
	cfg.dataDir = dir

	migrated, err := cfg.Migrate()
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !migrated {
		t.Error("expected migrated=true for existing file")
	}

	// Verify the file was rewritten
	data, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if len(data) == 0 {
		t.Error("expected non-empty migrated config")
	}
}

func TestLoadConfig_WithEnv(t *testing.T) {
	dir := t.TempDir()
	tashDir := filepath.Join(dir, "tash")
	_ = os.MkdirAll(tashDir, 0755)

	t.Setenv("XDG_CONFIG_HOME", dir)

	configContent := `model:
  name: test-model
  endpoint: https://test.example.com/v1
  api_key_env: TEST_KEY
  max_tokens: 1024
`
	_ = os.WriteFile(filepath.Join(tashDir, "config.yaml"), []byte(configContent), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Model.Name != "test-model" {
		t.Errorf("expected test-model, got %q", cfg.Model.Name)
	}
	if cfg.Model.MaxTokens != 1024 {
		t.Errorf("expected 1024, got %d", cfg.Model.MaxTokens)
	}
	if cfg.DataDir() != tashDir {
		t.Errorf("expected %q, got %q", tashDir, cfg.DataDir())
	}
}

func TestLoadConfig_DefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	tashDir := filepath.Join(dir, "tash")
	_ = os.MkdirAll(tashDir, 0755)

	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Should return defaults
	if cfg.Model.Name != "claude-sonnet-4-6" {
		t.Errorf("expected default model, got %q", cfg.Model.Name)
	}
}

func TestRenderConfig_Comments(t *testing.T) {
	RegisterThemeNames([]string{"solarized", "gruvbox", "nord"})
	defer RegisterThemeNames(nil)

	cfg := DefaultConfig()
	data, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}

	out := string(data)

	// Verify section comments
	for _, want := range []string{
		"# AI model and endpoint configuration",
		"# runtime behavior settings",
		"# user profile rebuild settings",
		"# spinner and accent color theme",
		"# terminal compatibility settings",
		"# log verbosity written to tash.log",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing comment %q in rendered config", want)
		}
	}

	// Verify field-level comments are dynamically generated from slices
	for _, want := range []string{
		"# model identifier sent to the API",
		"# retry attempts on API or parse failures",
		"# check for new releases and show update notices",
		"# seconds between automatic profile rebuilds",
		"# color profile: auto, 256, 16, none",
		"# preset: solarized, gruvbox, nord",
		"# log verbosity written to tash.log: debug, info, warn, error",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing comment %q in rendered config", want)
		}
	}

	// Verify values are still present
	if !strings.Contains(out, "claude-sonnet-4-6") {
		t.Error("missing default model name in rendered config")
	}
	if !strings.Contains(out, "solarized") {
		t.Error("missing default theme name in rendered config")
	}
}

func TestRenderConfig_NoThemes(t *testing.T) {
	RegisterThemeNames(nil)

	cfg := DefaultConfig()
	data, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "# preset theme name") {
		t.Error("expected fallback theme comment when no themes registered")
	}
}

func TestRenderConfig_Roundtrip(t *testing.T) {
	cfg := DefaultConfig()
	data, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}

	// Parse the commented YAML back and verify values survive
	var parsed Config
	if err := parseYAML(data, &parsed); err != nil {
		t.Fatalf("re-parse rendered config: %v", err)
	}

	if parsed.Model.Name != cfg.Model.Name {
		t.Errorf("roundtrip model.name: want %q, got %q", cfg.Model.Name, parsed.Model.Name)
	}
	if parsed.Behavior.MaxRetries != cfg.Behavior.MaxRetries {
		t.Errorf("roundtrip max_retries: want %d, got %d", cfg.Behavior.MaxRetries, parsed.Behavior.MaxRetries)
	}
	if parsed.Theme.Name != cfg.Theme.Name {
		t.Errorf("roundtrip theme.name: want %q, got %q", cfg.Theme.Name, parsed.Theme.Name)
	}
	if parsed.LogLevel != cfg.LogLevel {
		t.Errorf("roundtrip log_level: want %q, got %q", cfg.LogLevel, parsed.LogLevel)
	}
}

func TestMigrate_PrunesStaleFields(t *testing.T) {
	dir := t.TempDir()

	// Simulate an old config with fields that no longer exist in the struct
	oldConfig := `model:
  name: claude-sonnet-4-6
  endpoint: https://api.anthropic.com/v1
  api_key_env: ANTHROPIC_API_KEY
  max_tokens: 2048
  temperature: 0.7
behavior:
  max_retries: 3
  max_memories: 50
  auto_intercept: true
  screen_capture: true
  screen_capture_max_lines: 200
  deprecated_flag: true
old_section:
  removed_key: stale_value
log_level: info
`
	_ = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(oldConfig), 0644)

	cfg := DefaultConfig()
	cfg.dataDir = dir

	migrated, err := cfg.Migrate()
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !migrated {
		t.Error("expected migrated=true")
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	out := string(data)

	// Stale fields and sections must be gone
	for _, stale := range []string{"temperature", "deprecated_flag", "old_section", "removed_key"} {
		if strings.Contains(out, stale) {
			t.Errorf("stale field %q should have been pruned from migrated config", stale)
		}
	}

	// Current fields must still be present
	for _, current := range []string{"model:", "max_retries:", "log_level:"} {
		if !strings.Contains(out, current) {
			t.Errorf("expected field %q to be present in migrated config", current)
		}
	}

	// Comments should be present after migration
	if !strings.Contains(out, "# AI model and endpoint configuration") {
		t.Error("expected comments in migrated config")
	}
}

func TestWriteDefault_HasComments(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.dataDir = dir

	created, err := cfg.WriteDefault()
	if err != nil {
		t.Fatalf("WriteDefault: %v", err)
	}
	if !created {
		t.Error("expected created=true")
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "# AI model and endpoint configuration") {
		t.Error("expected comments in default config")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogLevel = "verbose"
	cfg.validate()

	if cfg.LogLevel != "info" {
		t.Errorf("expected log_level reset to info, got %q", cfg.LogLevel)
	}
}

func TestValidate_InvalidTerminalColor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Terminal.Color = "truecolor"
	cfg.validate()

	if cfg.Terminal.Color != "auto" {
		t.Errorf("expected terminal.color reset to auto, got %q", cfg.Terminal.Color)
	}
}

func TestValidate_NegativeInts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.MaxTokens = -1
	cfg.Behavior.MaxRetries = 0
	cfg.Behavior.MaxToolCalls = -1
	cfg.Behavior.MaxMemories = -5
	cfg.Behavior.MaxContext = 0
	cfg.Behavior.MaxHistoryResults = -1
	cfg.Behavior.ScreenCaptureMaxLines = 0
	cfg.Profile.RebuildInterval = -100
	cfg.validate()

	def := DefaultConfig()
	if cfg.Model.MaxTokens != def.Model.MaxTokens {
		t.Errorf("max_tokens: want %d, got %d", def.Model.MaxTokens, cfg.Model.MaxTokens)
	}
	if cfg.Behavior.MaxRetries != def.Behavior.MaxRetries {
		t.Errorf("max_retries: want %d, got %d", def.Behavior.MaxRetries, cfg.Behavior.MaxRetries)
	}
	if cfg.Behavior.MaxToolCalls != def.Behavior.MaxToolCalls {
		t.Errorf("max_tool_calls: want %d, got %d", def.Behavior.MaxToolCalls, cfg.Behavior.MaxToolCalls)
	}
	if cfg.Behavior.MaxMemories != def.Behavior.MaxMemories {
		t.Errorf("max_memories: want %d, got %d", def.Behavior.MaxMemories, cfg.Behavior.MaxMemories)
	}
	if cfg.Behavior.MaxContext != def.Behavior.MaxContext {
		t.Errorf("max_context: want %d, got %d", def.Behavior.MaxContext, cfg.Behavior.MaxContext)
	}
	if cfg.Behavior.MaxHistoryResults != def.Behavior.MaxHistoryResults {
		t.Errorf("max_history_results: want %d, got %d", def.Behavior.MaxHistoryResults, cfg.Behavior.MaxHistoryResults)
	}
	if cfg.Behavior.ScreenCaptureMaxLines != def.Behavior.ScreenCaptureMaxLines {
		t.Errorf("screen_capture_max_lines: want %d, got %d", def.Behavior.ScreenCaptureMaxLines, cfg.Behavior.ScreenCaptureMaxLines)
	}
	if cfg.Profile.RebuildInterval != def.Profile.RebuildInterval {
		t.Errorf("rebuild_interval: want %d, got %d", def.Profile.RebuildInterval, cfg.Profile.RebuildInterval)
	}
}

func TestValidate_ValidValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogLevel = "debug"
	cfg.Terminal.Color = "256"
	cfg.Model.MaxTokens = 4096
	cfg.Behavior.MaxRetries = 5
	cfg.validate()

	if cfg.LogLevel != "debug" {
		t.Errorf("valid log_level changed: got %q", cfg.LogLevel)
	}
	if cfg.Terminal.Color != "256" {
		t.Errorf("valid terminal.color changed: got %q", cfg.Terminal.Color)
	}
	if cfg.Model.MaxTokens != 4096 {
		t.Errorf("valid max_tokens changed: got %d", cfg.Model.MaxTokens)
	}
	if cfg.Behavior.MaxRetries != 5 {
		t.Errorf("valid max_retries changed: got %d", cfg.Behavior.MaxRetries)
	}
}

func TestLoadConfig_ValidatesValues(t *testing.T) {
	dir := t.TempDir()
	tashDir := filepath.Join(dir, "tash")
	_ = os.MkdirAll(tashDir, 0755)

	t.Setenv("XDG_CONFIG_HOME", dir)

	configContent := `log_level: verbose
terminal:
  color: truecolor
model:
  max_tokens: -1
behavior:
  max_retries: 0
`
	_ = os.WriteFile(filepath.Join(tashDir, "config.yaml"), []byte(configContent), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("expected log_level reset to info, got %q", cfg.LogLevel)
	}
	if cfg.Terminal.Color != "auto" {
		t.Errorf("expected terminal.color reset to auto, got %q", cfg.Terminal.Color)
	}
	if cfg.Model.MaxTokens != 2048 {
		t.Errorf("expected max_tokens reset to 2048, got %d", cfg.Model.MaxTokens)
	}
	if cfg.Behavior.MaxRetries != 3 {
		t.Errorf("expected max_retries reset to 3, got %d", cfg.Behavior.MaxRetries)
	}
}

// parseYAML is a test helper that unmarshals YAML into a target.
func parseYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
