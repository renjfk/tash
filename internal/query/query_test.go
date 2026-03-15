package query

import (
	"strings"
	"testing"

	"github.com/renjfk/tash/internal/ai"
	"github.com/renjfk/tash/internal/data"
)

func TestHasTerminalResponse(t *testing.T) {
	tests := []struct {
		name      string
		responses []ai.TashResponse
		want      bool
	}{
		{"chat", []ai.TashResponse{{Type: "chat"}}, true},
		{"command", []ai.TashResponse{{Type: "command"}}, true},
		{"plan", []ai.TashResponse{{Type: "plan"}}, true},
		{"history only", []ai.TashResponse{{Type: "history"}}, false},
		{"conversation only", []ai.TashResponse{{Type: "conversation"}}, false},
		{"memory only", []ai.TashResponse{{Type: "memory"}}, false},
		{"empty", nil, false},
		{"memory then chat", []ai.TashResponse{{Type: "memory"}, {Type: "chat"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasTerminalResponse(tt.responses)
			if got != tt.want {
				t.Errorf("hasTerminalResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildMessages_EmptyConversation(t *testing.T) {
	convo := data.NewConversation()
	convo.AddQuery("list files")

	messages := buildMessages(convo, nil, "list files", nil)

	if len(messages) == 0 {
		t.Fatal("expected at least one message")
	}

	last := messages[len(messages)-1]
	if last.Role != "user" {
		t.Errorf("expected last message role user, got %q", last.Role)
	}
	if !strings.Contains(last.Content, "list files") {
		t.Error("expected input in last message")
	}
}

func TestBuildMessages_WithShellHistory(t *testing.T) {
	convo := data.NewConversation()
	convo.AddQuery("fix the error")

	history := []data.ShellCommand{
		{Command: "make build", ExitCode: 2},
	}

	messages := buildMessages(convo, history, "fix the error", nil)

	last := messages[len(messages)-1]
	if !strings.Contains(last.Content, "make build") {
		t.Error("expected shell history in prompt")
	}
	if !strings.Contains(last.Content, "exit 2") {
		t.Error("expected exit code in prompt")
	}
}

func TestBuildMessages_WithConstraints(t *testing.T) {
	convo := data.NewConversation()
	convo.AddQuery("search files")

	constraints := []string{"rg is not installed, use an alternative"}
	messages := buildMessages(convo, nil, "search files", constraints)

	last := messages[len(messages)-1]
	if !strings.Contains(last.Content, "rg is not installed") {
		t.Error("expected constraint in prompt")
	}
}

func TestBuildMessages_ReplacesLastUserMessage(t *testing.T) {
	convo := data.NewConversation()
	convo.AddQuery("hello")

	messages := buildMessages(convo, nil, "hello", nil)

	// The AddQuery creates a user entry, and buildMessages should replace
	// the last user message with the full prompt (not duplicate it)
	userCount := 0
	for _, m := range messages {
		if m.Role == "user" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Errorf("expected exactly 1 user message, got %d", userCount)
	}
}

func TestBuildSystemPrompt_NoProfile(t *testing.T) {
	cfg := &data.Config{}
	convo := data.NewConversation()
	got := buildSystemPrompt(cfg, nil, convo)

	if !strings.Contains(got, ai.SystemPrompt) {
		t.Error("expected base system prompt")
	}
	if strings.Contains(got, "User Profile") {
		t.Error("should not contain profile section when nil")
	}
	if !strings.Contains(got, "--- Session ---") {
		t.Error("expected session section")
	}
	if !strings.Contains(got, "tash version:") {
		t.Error("expected version in session section")
	}
	if !strings.Contains(got, "Current time:") {
		t.Error("expected current time in session section")
	}
}

func TestBuildSystemPrompt_WithProfile(t *testing.T) {
	cfg := &data.Config{}
	prof := &data.Profile{Content: "## Tools\n- docker\n- kubectl"}
	convo := data.NewConversation()
	got := buildSystemPrompt(cfg, prof, convo)

	if !strings.Contains(got, "User Profile") {
		t.Error("expected profile section header")
	}
	if !strings.Contains(got, "docker") {
		t.Error("expected profile content")
	}
}

func TestBuildSystemPrompt_WithMemories(t *testing.T) {
	cfg := &data.Config{}
	convo := data.NewConversation()
	convo.AddMemory("User is John, backend engineer")

	got := buildSystemPrompt(cfg, nil, convo)

	if !strings.Contains(got, "Memories") {
		t.Error("expected memories section header")
	}
	if !strings.Contains(got, "John") {
		t.Error("expected memory content")
	}
	if !strings.Contains(got, "1/50 slots used") {
		t.Errorf("expected memory count in header, got section: %s", got[strings.Index(got, "--- Memories"):strings.Index(got, "--- Memories")+120])
	}
}

func TestBuildSystemPrompt_ASCIIMode(t *testing.T) {
	cfg := &data.Config{}
	cfg.Terminal.ASCII = true
	convo := data.NewConversation()
	got := buildSystemPrompt(cfg, nil, convo)

	if !strings.Contains(got, "Do NOT use emojis") {
		t.Error("expected emoji restriction in ASCII mode")
	}
	if !strings.Contains(got, "ASCII art is fine") {
		t.Error("expected ASCII art to be allowed")
	}
}

func TestBuildSystemPrompt_NonASCIIMode(t *testing.T) {
	cfg := &data.Config{}
	convo := data.NewConversation()
	got := buildSystemPrompt(cfg, nil, convo)

	if strings.Contains(got, "Do NOT use emojis") {
		t.Error("should not contain emoji restriction when not in ASCII mode")
	}
}
