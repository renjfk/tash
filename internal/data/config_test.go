package data

import (
	"os"
	"path/filepath"
	"testing"
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
	if cfg.Behavior.MaxMemories != 50 {
		t.Errorf("expected 50 memories, got %d", cfg.Behavior.MaxMemories)
	}
	if !cfg.Behavior.AutoIntercept {
		t.Error("expected auto_intercept true")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected info, got %q", cfg.LogLevel)
	}
	if cfg.Profile.RebuildInterval != 3600 {
		t.Errorf("expected 3600, got %d", cfg.Profile.RebuildInterval)
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
