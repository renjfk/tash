package data

import (
	"testing"
)

func TestAddMemory(t *testing.T) {
	c := NewConversation()
	c.SetSession("test-session")
	c.AddMemory("User prefers Go")

	if len(c.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(c.Entries))
	}
	e := c.Entries[0]
	if e.Type != "memory" {
		t.Errorf("expected type memory, got %q", e.Type)
	}
	if e.Content != "User prefers Go" {
		t.Errorf("expected content 'User prefers Go', got %q", e.Content)
	}
	if e.Session != "test-session" {
		t.Errorf("expected session test-session, got %q", e.Session)
	}
	if e.Time == 0 {
		t.Error("expected non-zero time")
	}
}

func TestAddMemory_TrimsCap(t *testing.T) {
	c := NewConversation()
	c.maxMemories = 3

	for i := 0; i < 5; i++ {
		c.AddMemory("fact")
	}

	memCount := 0
	for _, e := range c.Entries {
		if e.Type == "memory" {
			memCount++
		}
	}
	if memCount != 3 {
		t.Errorf("expected 3 memories after cap, got %d", memCount)
	}
}

func TestAddShellCommand(t *testing.T) {
	c := NewConversation()
	c.SetSession("shell-session")
	c.AddShellCommand("ls -la", 0)

	if len(c.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(c.Entries))
	}
	e := c.Entries[0]
	if e.Type != "shell" {
		t.Errorf("expected type shell, got %q", e.Type)
	}
	if e.Content != "ls -la" {
		t.Errorf("expected content 'ls -la', got %q", e.Content)
	}
	if e.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", e.ExitCode)
	}
	if e.Session != "shell-session" {
		t.Errorf("expected session shell-session, got %q", e.Session)
	}
}

func TestAddShellCommand_FailedCommand(t *testing.T) {
	c := NewConversation()
	c.AddShellCommand("make build", 2)

	e := c.Entries[0]
	if e.ExitCode != 2 {
		t.Errorf("expected exit code 2, got %d", e.ExitCode)
	}
}

func TestAddChatResponse(t *testing.T) {
	c := NewConversation()
	c.SetSession("chat-session")
	c.AddChatResponse("Here's the answer", "req123", 100, 50)

	if len(c.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(c.Entries))
	}
	e := c.Entries[0]
	if e.Type != "chat" {
		t.Errorf("expected type chat, got %q", e.Type)
	}
	if e.Content != "Here's the answer" {
		t.Errorf("expected content 'Here's the answer', got %q", e.Content)
	}
	if e.RequestID != "req123" {
		t.Errorf("expected request ID req123, got %q", e.RequestID)
	}
	if e.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", e.PromptTokens)
	}
	if e.CompletionTokens != 50 {
		t.Errorf("expected 50 completion tokens, got %d", e.CompletionTokens)
	}
	if e.Session != "chat-session" {
		t.Errorf("expected session chat-session, got %q", e.Session)
	}
}

func TestAddChatResponse_Trims(t *testing.T) {
	c := NewConversation()
	c.maxMemories = 50

	// Add more than defaultMaxEntries
	for i := 0; i < defaultMaxEntries+10; i++ {
		c.AddChatResponse("msg", "req", 0, 0)
	}

	if len(c.Entries) > defaultMaxEntries {
		t.Errorf("expected at most %d entries after trim, got %d", defaultMaxEntries, len(c.Entries))
	}
}

func TestAddQuery_RecordsCorrectly(t *testing.T) {
	c := NewConversation()
	c.SetSession("q-session")
	c.AddQuery("how do I list files?")

	if len(c.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(c.Entries))
	}
	e := c.Entries[0]
	if e.Type != "query" {
		t.Errorf("expected type query, got %q", e.Type)
	}
	if e.Content != "how do I list files?" {
		t.Errorf("unexpected content: %q", e.Content)
	}
}

func TestAddCommandResponse_RecordsAction(t *testing.T) {
	c := NewConversation()
	c.AddCommandResponse("git status", "skip", "req456", 200, 75)

	e := c.Entries[0]
	if e.Type != "command" {
		t.Errorf("expected type command, got %q", e.Type)
	}
	if e.Action != "skip" {
		t.Errorf("expected action skip, got %q", e.Action)
	}
	if e.RequestID != "req456" {
		t.Errorf("expected request ID req456, got %q", e.RequestID)
	}
	if e.PromptTokens != 200 || e.CompletionTokens != 75 {
		t.Errorf("unexpected token counts: %d/%d", e.PromptTokens, e.CompletionTokens)
	}
}
