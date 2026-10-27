package query

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renjfk/tash/internal/ai"
	aitools "github.com/renjfk/tash/internal/ai/tools"
	"github.com/renjfk/tash/internal/data"
)

func TestParseModelResponses_PrefersToolCalls(t *testing.T) {
	resp := &ai.Response{
		Content: "ignored",
		ToolCalls: []ai.ToolCall{
			{Function: ai.FunctionCall{Name: "command", Arguments: `{"commands":["git status"]}`}},
		},
	}
	responses, err := parseModelResponses(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 || responses[0].Type != "command" {
		t.Fatalf("expected command tool call, got %#v", responses)
	}
}

func TestParseModelResponses_FallsBackToContent(t *testing.T) {
	resp := &ai.Response{Content: `{"type": "chat", "message": "hi"}`}
	responses, err := parseModelResponses(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 || responses[0].Type != "chat" || responses[0].Message != "hi" {
		t.Fatalf("expected chat response, got %#v", responses)
	}
}

func TestParseModelResponses_UnparseableToolCallErrors(t *testing.T) {
	resp := &ai.Response{
		ToolCalls: []ai.ToolCall{{Function: ai.FunctionCall{Name: "bogus", Arguments: `{}`}}},
	}
	if _, err := parseModelResponses(resp); err == nil {
		t.Fatal("expected error when tool calls cannot be parsed and content is empty")
	}
}

func TestParseModelResponses_KeepsContentBesideMemoryToolCall(t *testing.T) {
	resp := &ai.Response{
		Content: `{"type":"chat","message":"Nice to meet you, John!"}`,
		ToolCalls: []ai.ToolCall{
			{Function: ai.FunctionCall{Name: "memory", Arguments: `{"message":"User is John"}`}},
		},
	}
	responses, err := parseModelResponses(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected memory + chat, got %#v", responses)
	}
	if responses[0].Type != "memory" {
		t.Errorf("expected memory first, got %q", responses[0].Type)
	}
	if responses[1].Type != "chat" || responses[1].Message != "Nice to meet you, John!" {
		t.Errorf("expected chat with message, got %#v", responses[1])
	}
}

func TestParseModelResponses_IgnoresContentBesideHistoryToolCall(t *testing.T) {
	resp := &ai.Response{
		Content: "I'll check your shell history.",
		ToolCalls: []ai.ToolCall{
			{Function: ai.FunctionCall{Name: "history", Arguments: `{"filter":"git"}`}},
		},
	}
	responses, err := parseModelResponses(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasTerminalResponse(responses) {
		t.Fatalf("expected no terminal response so the query retries, got %#v", responses)
	}
	if len(responses) != 1 || responses[0].Type != "history" {
		t.Fatalf("expected only history, got %#v", responses)
	}
}

func TestParseModelResponses_IgnoresWhitespaceContentBesideMemory(t *testing.T) {
	resp := &ai.Response{
		Content: "   ",
		ToolCalls: []ai.ToolCall{
			{Function: ai.FunctionCall{Name: "memory", Arguments: `{"message":"User is John"}`}},
		},
	}
	responses, err := parseModelResponses(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasTerminalResponse(responses) {
		t.Fatalf("expected no terminal response, got %#v", responses)
	}
	if len(responses) != 1 || responses[0].Type != "memory" {
		t.Fatalf("expected only memory, got %#v", responses)
	}
}

func TestDropEmptyChats(t *testing.T) {
	got := dropEmptyChats([]ai.TashResponse{
		{Type: "memory", Message: "User is John"},
		{Type: "chat", Message: "   "},
		{Type: "chat", Message: "hi"},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 responses, got %#v", got)
	}
	if got[0].Type != "memory" || got[1].Type != "chat" || got[1].Message != "hi" {
		t.Fatalf("unexpected %#v", got)
	}
}

func TestParseModelResponses_IgnoresContentBesideTerminalToolCall(t *testing.T) {
	resp := &ai.Response{
		Content: "let me run that",
		ToolCalls: []ai.ToolCall{
			{Function: ai.FunctionCall{Name: "command", Arguments: `{"commands":["ls"]}`}},
		},
	}
	responses, err := parseModelResponses(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 || responses[0].Type != "command" {
		t.Fatalf("expected only command, got %#v", responses)
	}
}

func TestCompleteQuery_FallsBackWithoutToolsOnError(t *testing.T) {
	var sawTools []bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools []json.RawMessage `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sawTools = append(sawTools, len(body.Tools) > 0)
		if len(body.Tools) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"tools unsupported"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": `{"type":"chat","message":"ok"}`}}},
		})
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	client := ai.NewClient(cfg)

	resp, toolsEnabled, err := completeQuery(
		context.Background(),
		client,
		ai.Request{Model: "test", Messages: []ai.Message{{Role: "user", Content: "hi"}}},
		aitools.All(),
		true,
	)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if toolsEnabled {
		t.Error("expected tools to be disabled after fallback")
	}
	if resp.Content == "" {
		t.Error("expected content from the no-tools retry")
	}
	if len(sawTools) != 2 || !sawTools[0] || sawTools[1] {
		t.Errorf("expected first request with tools then retry without, got %v", sawTools)
	}
}

func TestCompleteQuery_DoesNotFallBackOnServerError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	client := ai.NewClient(cfg)

	_, toolsEnabled, err := completeQuery(
		context.Background(),
		client,
		ai.Request{Model: "test", Messages: []ai.Message{{Role: "user", Content: "hi"}}},
		aitools.All(),
		true,
	)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !toolsEnabled {
		t.Error("expected tools to stay enabled on a server error")
	}
	if requests != 1 {
		t.Errorf("expected no retry on 5xx, got %d requests", requests)
	}
}

func TestCompleteQuery_KeepsToolsOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{
				"tool_calls": []map[string]any{{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "command",
						"arguments": `{"commands":["ls"]}`,
					},
				}},
			}}},
		})
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	client := ai.NewClient(cfg)

	resp, toolsEnabled, err := completeQuery(
		context.Background(),
		client,
		ai.Request{Model: "test", Messages: []ai.Message{{Role: "user", Content: "list"}}},
		aitools.All(),
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !toolsEnabled {
		t.Error("expected tools to remain enabled after success")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
}

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

	messages := buildMessages(convo, nil, "list files", nil, time.Time{})

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

	messages := buildMessages(convo, history, "fix the error", nil, time.Time{})

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
	messages := buildMessages(convo, nil, "search files", constraints, time.Time{})

	last := messages[len(messages)-1]
	if !strings.Contains(last.Content, "rg is not installed") {
		t.Error("expected constraint in prompt")
	}
}

func TestBuildMessages_ReplacesLastUserMessage(t *testing.T) {
	convo := data.NewConversation()
	convo.AddQuery("hello")

	messages := buildMessages(convo, nil, "hello", nil, time.Time{})

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
	if strings.Contains(got, "Current time:") {
		t.Error("current time must stay out of the system prompt to keep the cacheable prefix stable")
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
