package ai

import (
	"testing"
)

func TestParseResponse_SingleCommand(t *testing.T) {
	raw := `{"type": "command", "commands": ["git status"]}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "command" {
		t.Errorf("expected type command, got %q", responses[0].Type)
	}
	if len(responses[0].Commands) != 1 || responses[0].Commands[0] != "git status" {
		t.Errorf("expected [git status], got %v", responses[0].Commands)
	}
}

func TestParseResponse_CommandWithMessage(t *testing.T) {
	raw := `{"type": "command", "commands": ["curl -s https://api.example.com | jq '.[]'"], "message": "Fetches the API."}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Message != "Fetches the API." {
		t.Errorf("expected message, got %q", responses[0].Message)
	}
}

func TestParseResponse_MultiCommand(t *testing.T) {
	raw := `{"type": "command", "commands": ["mkdir -p src tests", "uv init && uv add pytest"]}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if len(responses[0].Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(responses[0].Commands))
	}
}

func TestParseResponse_Chat(t *testing.T) {
	raw := `{"type": "chat", "message": "Hello there!"}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "chat" {
		t.Errorf("expected type chat, got %q", responses[0].Type)
	}
	if responses[0].Message != "Hello there!" {
		t.Errorf("expected Hello there!, got %q", responses[0].Message)
	}
}

func TestParseResponse_HistoryRequest(t *testing.T) {
	raw := `{"type": "history", "filter": "git", "count": 30}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "history" {
		t.Errorf("expected type history, got %q", responses[0].Type)
	}
	if responses[0].Filter != "git" {
		t.Errorf("expected filter git, got %q", responses[0].Filter)
	}
	if responses[0].Count != 30 {
		t.Errorf("expected count 30, got %d", responses[0].Count)
	}
}

func TestParseResponse_DefaultCount(t *testing.T) {
	raw := `{"type": "history", "filter": "docker"}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if responses[0].Count != 50 {
		t.Errorf("expected default count 50, got %d", responses[0].Count)
	}
}

func TestParseResponse_DefaultCountScreen(t *testing.T) {
	raw := `{"type": "screen"}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if responses[0].Count != 50 {
		t.Errorf("expected default count 50, got %d", responses[0].Count)
	}
}

func TestParseResponse_ExplicitCountOverridesDefault(t *testing.T) {
	raw := `{"type": "context", "count": 100}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if responses[0].Count != 100 {
		t.Errorf("expected count 100, got %d", responses[0].Count)
	}
}

func TestParseResponse_ZeroCountDefaultsTo50(t *testing.T) {
	raw := `{"type": "history", "count": 0}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if responses[0].Count != 50 {
		t.Errorf("expected 0 to be normalized to default 50, got %d", responses[0].Count)
	}
}

func TestParseResponse_ExplicitCountScreen(t *testing.T) {
	raw := `{"type": "screen", "count": 30}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if responses[0].Count != 30 {
		t.Errorf("expected count 30, got %d", responses[0].Count)
	}
}

func TestParseResponse_JSONL(t *testing.T) {
	raw := `{"type": "memory", "message": "User likes Go"}
{"type": "chat", "message": "Got it!"}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	if responses[0].Type != "memory" {
		t.Errorf("expected memory, got %q", responses[0].Type)
	}
	if responses[1].Type != "chat" {
		t.Errorf("expected chat, got %q", responses[1].Type)
	}
}

func TestParseResponse_PreambleText(t *testing.T) {
	raw := `Here's what I found:
{"type": "command", "commands": ["ls -la"]}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Message != "Here's what I found:" {
		t.Errorf("expected preamble as message, got %q", responses[0].Message)
	}
}

func TestParseResponse_CodeFences(t *testing.T) {
	raw := "```json\n{\"type\": \"command\", \"commands\": [\"echo hello\"]}\n```"
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "command" {
		t.Errorf("expected command, got %q", responses[0].Type)
	}
}

func TestParseResponse_PlanType(t *testing.T) {
	raw := `{"type": "plan", "commands": ["kubectl get pods -A"], "message": "Finding pods first.", "steps_remaining": 2}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "plan" {
		t.Errorf("expected plan, got %q", responses[0].Type)
	}
	if responses[0].StepsRemaining != 2 {
		t.Errorf("expected steps_remaining 2, got %d", responses[0].StepsRemaining)
	}
}

func TestParseResponse_NonJSON(t *testing.T) {
	raw := "This is just plain text from the AI."
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "chat" {
		t.Errorf("expected fallback to chat, got %q", responses[0].Type)
	}
	if responses[0].Message != raw {
		t.Errorf("expected raw text as message, got %q", responses[0].Message)
	}
}

func TestParseResponse_EmptyTypeSkipped(t *testing.T) {
	raw := `{"commands": ["ls"]}
{"type": "command", "commands": ["pwd"]}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response (empty type skipped), got %d", len(responses))
	}
	if responses[0].Commands[0] != "pwd" {
		t.Errorf("expected pwd, got %q", responses[0].Commands[0])
	}
}

func TestSanitizeJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid JSON passthrough",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "valid escapes preserved",
			input: `{"key": "line\nbreak\ttab"}`,
			want:  `{"key": "line\nbreak\ttab"}`,
		},
		{
			name:  "invalid pipe escape",
			input: `{"filter": "grep\|awk"}`,
			want:  `{"filter": "grep\\|awk"}`,
		},
		{
			name:  "invalid dot escape",
			input: `{"filter": "foo\.bar"}`,
			want:  `{"filter": "foo\\.bar"}`,
		},
		{
			name:  "multiple invalid escapes",
			input: `{"re": "\d+\.\d+"}`,
			want:  `{"re": "\\d+\\.\\d+"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeJSON(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fences",
			input: `{"type": "chat"}`,
			want:  `{"type": "chat"}`,
		},
		{
			name:  "json fences",
			input: "```json\n{\"type\": \"chat\"}\n```",
			want:  `{"type": "chat"}`,
		},
		{
			name:  "plain fences",
			input: "```\n{\"type\": \"chat\"}\n```",
			want:  `{"type": "chat"}`,
		},
		{
			name:  "only opening fence",
			input: "```json\n{\"type\": \"chat\"}",
			want:  `{"type": "chat"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.input)
			if got != tt.want {
				t.Errorf("stripCodeFences() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrimPrefixAny(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefixes []string
		want     string
	}{
		{"matching first", "```json\nrest", []string{"```json\n", "```\n"}, "rest"},
		{"matching second", "```\nrest", []string{"```json\n", "```\n"}, "rest"},
		{"no match", "hello", []string{"```json\n"}, "hello"},
		{"empty input", "", []string{"```\n"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimPrefixAny(tt.input, tt.prefixes...)
			if got != tt.want {
				t.Errorf("trimPrefixAny() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLastIndex(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   int
	}{
		{"found at end", "hello\n```", "\n```", 5},
		{"found middle", "a\n```b\n```", "\n```", 6},
		{"not found", "hello", "\n```", -1},
		{"empty string", "", "\n```", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastIndex(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("lastIndex(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestParseResponse_InvalidJSONWithSanitization(t *testing.T) {
	// AI produces invalid escape like \| in regex
	raw := `{"type": "command", "commands": ["grep 'foo\|bar' file.txt"]}`
	responses, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Type != "command" {
		t.Errorf("expected command after sanitization, got %q", responses[0].Type)
	}
}
