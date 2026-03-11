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
	convo := data.NewConversation()
	got := buildSystemPrompt(nil, convo)

	if !strings.Contains(got, ai.SystemPrompt) {
		t.Error("expected base system prompt")
	}
	if strings.Contains(got, "User Profile") {
		t.Error("should not contain profile section when nil")
	}
}

func TestBuildSystemPrompt_WithProfile(t *testing.T) {
	prof := &data.Profile{Content: "## Tools\n- docker\n- kubectl"}
	convo := data.NewConversation()
	got := buildSystemPrompt(prof, convo)

	if !strings.Contains(got, "User Profile") {
		t.Error("expected profile section header")
	}
	if !strings.Contains(got, "docker") {
		t.Error("expected profile content")
	}
}

func TestBuildSystemPrompt_WithMemories(t *testing.T) {
	convo := data.NewConversation()
	convo.AddMemory("User is John, backend engineer")

	got := buildSystemPrompt(nil, convo)

	if !strings.Contains(got, "Memories") {
		t.Error("expected memories section header")
	}
	if !strings.Contains(got, "John") {
		t.Error("expected memory content")
	}
}
