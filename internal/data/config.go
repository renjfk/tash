package data

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all tash configuration from config.yaml.
type Config struct {
	Model    ModelConfig    `yaml:"model"`
	Behavior BehaviorConfig `yaml:"behavior"`
	Profile  ProfileConfig  `yaml:"profile"`
	Theme    ThemeConfig    `yaml:"theme"`
	LogLevel string         `yaml:"log_level"`
	dataDir  string
}

// ThemeConfig holds color theme settings.
type ThemeConfig struct {
	Name  string `yaml:"name"`  // preset name: blue, green, purple, orange, pink, red, cyan
	Color string `yaml:"color"` // custom hex color (e.g. "#FF4085"), overrides name
}

// ModelConfig holds AI model settings.
type ModelConfig struct {
	Name      string `yaml:"name"`
	Endpoint  string `yaml:"endpoint"`
	APIKeyEnv string `yaml:"api_key_env"`
	MaxTokens int    `yaml:"max_tokens"`
}

// BehaviorConfig holds runtime behavior settings.
type BehaviorConfig struct {
	MaxRetries    int  `yaml:"max_retries"`
	MaxMemories   int  `yaml:"max_memories"`
	AutoIntercept bool `yaml:"auto_intercept"`
}

// ProfileConfig holds profile rebuild settings.
type ProfileConfig struct {
	RebuildInterval int    `yaml:"rebuild_interval"`
	HistoryPath     string `yaml:"history_path"`
}

// DefaultConfig returns configuration with sane defaults.
func DefaultConfig() *Config {
	return &Config{
		Model: ModelConfig{
			Name:      "claude-sonnet-4-6",
			Endpoint:  "https://api.anthropic.com/v1",
			APIKeyEnv: "ANTHROPIC_API_KEY",
			MaxTokens: 2048,
		},
		Behavior: BehaviorConfig{
			MaxRetries:    3,
			MaxMemories:   50,
			AutoIntercept: true,
		},
		Theme: ThemeConfig{
			Name: "solarized",
		},
		LogLevel: "info",
		Profile: ProfileConfig{
			RebuildInterval: 3600,
			HistoryPath:     "~/.local/share/fish/fish_history",
		},
	}
}

// LoadConfig reads config.yaml from the tash data directory.
// Returns default config if file doesn't exist.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()
	dataDir, err := resolveDataDir()
	if err != nil {
		return nil, err
	}
	cfg.dataDir = dataDir

	configPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// DataDir returns the tash data directory path.
func (c *Config) DataDir() string {
	return c.dataDir
}

// SetDataDir overrides the data directory path.
// Intended for testing only.
func (c *Config) SetDataDir(dir string) {
	c.dataDir = dir
}

// APIKey returns the API key from the configured environment variable.
func (c *Config) APIKey() string {
	return os.Getenv(c.Model.APIKeyEnv)
}

// ResolvedHistoryPath expands ~ in the history path.
func (c *Config) ResolvedHistoryPath() string {
	if len(c.Profile.HistoryPath) > 0 && c.Profile.HistoryPath[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, c.Profile.HistoryPath[1:])
	}
	return c.Profile.HistoryPath
}

// Migrate re-writes config.yaml with the current in-memory config, which is the
// result of DefaultConfig() merged with the user's existing file. This ensures
// newly added fields get persisted with their defaults and stale fields are pruned.
func (c *Config) Migrate() (bool, error) {
	path := filepath.Join(c.dataDir, "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil // nothing to migrate
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return false, fmt.Errorf("marshal config: %w", err)
	}
	return true, os.WriteFile(path, data, 0644)
}

// WriteDefault writes a default config.yaml to the data directory if one doesn't exist.
// Returns true if a new file was created.
func (c *Config) WriteDefault() (bool, error) {
	path := filepath.Join(c.dataDir, "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(path, data, 0644)
}

func resolveDataDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}

	dataDir := filepath.Join(configHome, "tash")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", err
	}

	return dataDir, nil
}
