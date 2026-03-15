package data

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all tash configuration from config.yaml.
type Config struct {
	Model    ModelConfig    `yaml:"model"`
	Behavior BehaviorConfig `yaml:"behavior"`
	Profile  ProfileConfig  `yaml:"profile"`
	Theme    ThemeConfig    `yaml:"theme"`
	Terminal TerminalConfig `yaml:"terminal"`
	LogLevel string         `yaml:"log_level"`
	dataDir  string
}

// ThemeConfig holds color theme settings.
type ThemeConfig struct {
	Name  string `yaml:"name"`  // preset name: blue, green, purple, orange, pink, red, cyan
	Color string `yaml:"color"` // custom hex color (e.g. "#FF4085"), overrides name
}

// TerminalConfig holds terminal compatibility settings.
type TerminalConfig struct {
	ASCII bool   `yaml:"ascii"` // use ASCII-only characters when true
	Color string `yaml:"color"` // color profile override: auto, 256, 16, none
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
	MaxRetries             int  `yaml:"max_retries"`
	MaxToolCalls           int  `yaml:"max_tool_calls"`
	MaxMemories            int  `yaml:"max_memories"`
	MaxConversationEntries int  `yaml:"max_conversation_entries"`
	MaxContext             int  `yaml:"max_context"`
	MaxHistoryResults      int  `yaml:"max_history_results"`
	AutoIntercept          bool `yaml:"auto_intercept"`
	ScreenCapture          bool `yaml:"screen_capture"`
	ScreenCaptureMaxLines  int  `yaml:"screen_capture_max_lines"`
	UpdateCheck            bool `yaml:"update_check"`
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
			MaxRetries:             3,
			MaxToolCalls:           3,
			MaxMemories:            50,
			MaxConversationEntries: 250,
			MaxContext:             500,
			MaxHistoryResults:      200,
			AutoIntercept:          true,
			ScreenCapture:          true,
			ScreenCaptureMaxLines:  200,
			UpdateCheck:            true,
		},
		Theme: ThemeConfig{
			Name: "solarized",
		},
		Terminal: TerminalConfig{
			Color: "auto",
		},
		LogLevel: "info",
		Profile: ProfileConfig{
			RebuildInterval: 86400,
			HistoryPath:     "~/.local/share/fish/fish_history",
		},
	}
}

// LoadConfig reads config.yaml from the tash data directory.
// Returns default config if file doesn't exist. Invalid values are
// silently reset to their defaults.
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

	cfg.validate()
	return cfg, nil
}

// validate resets any invalid config values to their defaults.
func (c *Config) validate() {
	def := DefaultConfig()

	if !isValid(c.LogLevel, logLevels) {
		c.LogLevel = def.LogLevel
	}
	if !isValid(c.Terminal.Color, terminalColors) {
		c.Terminal.Color = def.Terminal.Color
	}
	if c.Model.MaxTokens <= 0 {
		c.Model.MaxTokens = def.Model.MaxTokens
	}
	if c.Behavior.MaxRetries <= 0 {
		c.Behavior.MaxRetries = def.Behavior.MaxRetries
	}
	if c.Behavior.MaxToolCalls <= 0 {
		c.Behavior.MaxToolCalls = def.Behavior.MaxToolCalls
	}
	if c.Behavior.MaxMemories <= 0 {
		c.Behavior.MaxMemories = def.Behavior.MaxMemories
	}
	if c.Behavior.MaxConversationEntries <= 0 {
		c.Behavior.MaxConversationEntries = def.Behavior.MaxConversationEntries
	}
	if c.Behavior.MaxContext <= 0 {
		c.Behavior.MaxContext = def.Behavior.MaxContext
	}
	if c.Behavior.MaxHistoryResults <= 0 {
		c.Behavior.MaxHistoryResults = def.Behavior.MaxHistoryResults
	}
	if c.Behavior.ScreenCaptureMaxLines <= 0 {
		c.Behavior.ScreenCaptureMaxLines = def.Behavior.ScreenCaptureMaxLines
	}
	if c.Profile.RebuildInterval <= 0 {
		c.Profile.RebuildInterval = def.Profile.RebuildInterval
	}
}

var logLevels = []string{"debug", "info", "warn", "error"}
var terminalColors = []string{"auto", "256", "16", "none"}

// themeNames is set by RegisterThemeNames at startup. Falls back to empty.
var themeNames []string

// RegisterThemeNames sets the available theme names for config comments.
// Called once at startup from cmd/tash with tui.ThemeNames().
func RegisterThemeNames(names []string) {
	themeNames = names
}

func isValid(value string, options []string) bool {
	for _, opt := range options {
		if value == opt {
			return true
		}
	}
	return false
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

	data, err := renderConfig(c)
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

	data, err := renderConfig(c)
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(path, data, 0644)
}

// renderConfig produces YAML with descriptive comments above each field.
func renderConfig(c *Config) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(c); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}

	doc := &node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}

	annotateNode(doc, buildConfigComments())

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return buf.Bytes(), nil
}

// fieldComment holds the comment and optional nested comments for a YAML key.
type fieldComment struct {
	comment  string
	children map[string]fieldComment
}

// buildConfigComments returns the comment hierarchy for config rendering.
// Selection lists (themes, log levels, terminal colors) are derived from
// their canonical slices so comments stay in sync with validation.
func buildConfigComments() map[string]fieldComment {
	themes := "preset theme name"
	if len(themeNames) > 0 {
		themes = "preset: " + strings.Join(themeNames, ", ")
	}

	return map[string]fieldComment{
		"model": {
			comment: "AI model and endpoint configuration",
			children: map[string]fieldComment{
				"name":        {comment: "model identifier sent to the API"},
				"endpoint":    {comment: "base URL for the OpenAI-compatible API"},
				"api_key_env": {comment: "environment variable containing the API key"},
				"max_tokens":  {comment: "maximum tokens in the AI response"},
			},
		},
		"behavior": {
			comment: "runtime behavior settings",
			children: map[string]fieldComment{
				"max_retries":              {comment: "retry attempts on API or parse failures"},
				"max_tool_calls":           {comment: "maximum tool calls (history, context, screen) per query"},
				"max_memories":             {comment: "maximum durable memories kept in conversation history"},
				"max_conversation_entries": {comment: "maximum conversation entries kept in memory and on disk"},
				"max_context":              {comment: "maximum conversation entries AI can load via context requests (scroll buffer)"},
				"max_history_results":      {comment: "maximum results returned from shell history searches"},
				"auto_intercept":           {comment: "re-invoke tash automatically after a failed command (true/false)"},
				"screen_capture":           {comment: "allow AI to read terminal screen via Zellij (true/false)"},
				"screen_capture_max_lines": {comment: "maximum lines the AI can request from terminal scrollback"},
				"update_check":             {comment: "check for new releases and show update notices (true/false)"},
			},
		},
		"profile": {
			comment: "user profile rebuild settings",
			children: map[string]fieldComment{
				"rebuild_interval": {comment: "seconds between automatic profile rebuilds"},
				"history_path":     {comment: "path to fish shell history file (~ is expanded)"},
			},
		},
		"theme": {
			comment: "spinner and accent color theme",
			children: map[string]fieldComment{
				"name":  {comment: themes},
				"color": {comment: "custom hex color (e.g. \"#FF4085\"), overrides name when set"},
			},
		},
		"terminal": {
			comment: "terminal compatibility settings",
			children: map[string]fieldComment{
				"ascii": {comment: "use ASCII-only characters, no Unicode (true/false)"},
				"color": {comment: "color profile: " + strings.Join(terminalColors, ", ")},
			},
		},
		"log_level": {
			comment: "log verbosity written to tash.log: " + strings.Join(logLevels, ", "),
		},
	}
}

// annotateNode walks a mapping node and sets HeadComment from the comments map.
// yaml.v3 renders HeadComment as "# <text>" automatically.
func annotateNode(node *yaml.Node, comments map[string]fieldComment) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]

		fc, ok := comments[key.Value]
		if !ok {
			continue
		}

		key.HeadComment = fc.comment

		if val.Kind == yaml.MappingNode && fc.children != nil {
			annotateNode(val, fc.children)
		}
	}
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
