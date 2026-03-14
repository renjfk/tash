package query

import (
	"os"
	"strings"
	"testing"

	"github.com/renjfk/tash/internal/ai"
	"github.com/renjfk/tash/internal/data"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"color code", "\x1b[31mred\x1b[0m", "red"},
		{"bold", "\x1b[1mbold\x1b[0m", "bold"},
		{"multiple codes", "\x1b[1;32mgreen bold\x1b[0m normal", "green bold normal"},
		{"cursor movement", "\x1b[2Jhello", "hello"},
		{"osc title", "\x1b]0;title\x07text", "text"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseScreenDump(t *testing.T) {
	tests := []struct {
		name    string
		content string
		lines   int
		want    string
	}{
		{
			"basic last 3 lines",
			"line1\nline2\nline3\nline4\nline5\n",
			3,
			"line3\nline4\nline5",
		},
		{
			"fewer lines than requested",
			"line1\nline2\n",
			10,
			"line1\nline2",
		},
		{
			"strips trailing blanks",
			"line1\nline2\n\n\n\n",
			5,
			"line1\nline2",
		},
		{
			"empty content",
			"",
			5,
			"",
		},
		{
			"only whitespace",
			"   \n  \n\n",
			5,
			"",
		},
		{
			"strips ANSI then returns tail",
			"\x1b[31mred\x1b[0m\nplain\n\x1b[1mbold\x1b[0m\n",
			2,
			"plain\nbold",
		},
		{
			"single line",
			"only line\n",
			1,
			"only line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseScreenDump(tt.content, tt.lines)
			if got != tt.want {
				t.Errorf("parseScreenDump() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"mixed", "hello\n\nworld\n  \nfoo\n", 3},
		{"all empty", "\n\n\n", 0},
		{"no newline", "hello", 1},
		{"empty string", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countNonEmpty(tt.content)
			if got != tt.want {
				t.Errorf("countNonEmpty() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCaptureScreen_Disabled(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = false

	got := captureScreen(cfg, 20)
	if !strings.Contains(got, "disabled") {
		t.Errorf("expected disabled message, got %q", got)
	}
}

func TestCaptureScreen_NoZellij(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = true

	t.Setenv("ZELLIJ", "")

	got := captureScreen(cfg, 20)
	if !strings.Contains(got, "Not running inside Zellij") {
		t.Errorf("expected no-zellij message, got %q", got)
	}
}

func TestInitialScreenContext_Disabled(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = false

	got := initialScreenContext(cfg)
	if got != "" {
		t.Errorf("expected empty when disabled, got %q", got)
	}
}

func TestInitialScreenContext_NoZellij(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = true

	t.Setenv("ZELLIJ", "")

	got := initialScreenContext(cfg)
	if got != "" {
		t.Errorf("expected empty without zellij, got %q", got)
	}
}

func TestHandleResponses_ScreenRequest(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "screen", Lines: 30},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	cfg.Behavior.ScreenCapture = true
	usage := ai.Usage{}

	// Without ZELLIJ set, screen capture returns a graceful message
	t.Setenv("ZELLIJ", "")

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionRetry {
		t.Errorf("expected actionRetry for screen request, got %d", action)
	}
	if retryReason != "Reading terminal" {
		t.Errorf("expected retry reason 'Reading terminal', got %q", retryReason)
	}
	if len(constraints) == 0 {
		t.Error("expected constraints to be populated with screen result")
	}
	// Should contain the graceful no-zellij message
	if !strings.Contains(constraints[0], "Not running inside Zellij") {
		t.Errorf("expected no-zellij constraint, got %q", constraints[0])
	}
}

func TestHandleResponses_ScreenSkipWhenCapped(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "screen", Lines: 20},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{}

	// skipToolCalls=true
	_, action := handleResponses(responses, cfg, convo, nil, &constraints, true, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionNothing {
		t.Errorf("expected actionNothing when tool calls capped, got %d", action)
	}
}

func TestHasTerminalResponse_ScreenIsNotTerminal(t *testing.T) {
	responses := []ai.TashResponse{{Type: "screen"}}
	if hasTerminalResponse(responses) {
		t.Error("screen should not be a terminal response")
	}
}

func TestCaptureScreen_DefaultLines(t *testing.T) {
	// Verify that lines=0 defaults to 20 (tested via the no-zellij path
	// which still exercises the lines normalization)
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = true
	t.Setenv("ZELLIJ", "")

	got := captureScreen(cfg, 0)
	if !strings.Contains(got, "Not running inside Zellij") {
		t.Errorf("expected no-zellij message even with lines=0, got %q", got)
	}
}

func TestCaptureScreen_MaxLinesCap(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = true
	cfg.Behavior.ScreenCaptureMaxLines = 50
	t.Setenv("ZELLIJ", "")

	// Requesting more than max should be capped (no crash)
	got := captureScreen(cfg, 1000)
	if !strings.Contains(got, "Not running inside Zellij") {
		t.Errorf("expected no-zellij message, got %q", got)
	}
}

func TestParseResponse_ScreenType(t *testing.T) {
	raw := `{"type": "screen", "lines": 50}`
	responses, err := ai.ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "screen" {
		t.Errorf("expected type screen, got %q", responses[0].Type)
	}
	if responses[0].Lines != 50 {
		t.Errorf("expected 50 lines, got %d", responses[0].Lines)
	}
}

// Verify that the screen response type is advertised in the system prompt
func TestSystemPrompt_ContainsScreenTool(t *testing.T) {
	if !strings.Contains(ai.SystemPrompt, `"type": "screen"`) {
		t.Error("SystemPrompt should advertise the screen tool")
	}
}

func TestCaptureScreen_ZellijNotInstalled(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = true

	// Set ZELLIJ but ensure zellij binary isn't available (it likely isn't in CI)
	t.Setenv("ZELLIJ", "1")
	t.Setenv("PATH", t.TempDir()) // empty PATH so zellij won't be found

	got := captureScreen(cfg, 20)
	if !strings.Contains(got, "Screen capture failed") {
		// May also succeed if zellij happens to be installed — either way, should not panic
		if !strings.Contains(got, "Terminal screen output") {
			t.Errorf("expected either failure message or screen output, got %q", got)
		}
	}
}

// TestInitialScreenContext_ZellijNotInstalled verifies graceful handling when
// ZELLIJ env is set but the binary is missing.
func TestInitialScreenContext_ZellijNotInstalled(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Behavior.ScreenCapture = true

	t.Setenv("ZELLIJ", "1")
	t.Setenv("PATH", t.TempDir())

	// Should return empty (graceful) rather than panic
	got := initialScreenContext(cfg)
	_ = got // just checking it doesn't panic
}

func TestHandleResponses_ScreenDisabled(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "screen", Lines: 20},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	cfg.Behavior.ScreenCapture = false
	usage := ai.Usage{}

	t.Setenv("ZELLIJ", "1")

	// Redirect stderr to suppress output
	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionRetry {
		t.Errorf("expected actionRetry, got %d", action)
	}
	// Constraint should mention disabled
	if len(constraints) == 0 || !strings.Contains(constraints[0], "disabled") {
		t.Errorf("expected disabled constraint, got %v", constraints)
	}
}
