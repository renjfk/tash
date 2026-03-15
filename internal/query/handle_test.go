package query

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renjfk/tash/internal/ai"
	"github.com/renjfk/tash/internal/data"
)

func TestHandleResponses_ChatOnly(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "chat", Message: "Hello there!"},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{PromptTokens: 10, CompletionTokens: 5}

	// Redirect stderr to suppress showChat output
	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	result, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionDone {
		t.Errorf("expected actionDone, got %d", action)
	}
	if result != "" {
		t.Errorf("expected empty result for chat, got %q", result)
	}

	// Chat should be recorded in conversation
	found := false
	for _, e := range convo.Entries {
		if e.Type == "chat" && e.Content == "Hello there!" {
			found = true
		}
	}
	if !found {
		t.Error("chat response should be recorded in conversation")
	}
}

func TestHandleResponses_Memory(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "memory", Message: "User prefers Go"},
		{Type: "chat", Message: "Noted!"},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{}

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionDone {
		t.Errorf("expected actionDone (chat present), got %d", action)
	}

	// Memory should be recorded
	mem := convo.Memories()
	if !strings.Contains(mem, "User prefers Go") {
		t.Error("memory should be recorded in conversation")
	}
}

func TestHandleResponses_HistoryRequest(t *testing.T) {
	dir := t.TempDir()
	// Create a minimal history file
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: git status\n  when: 1000\n- cmd: docker ps\n  when: 1001\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "history", Filter: "git", Count: 10},
	}
	usage := ai.Usage{}

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionRetry {
		t.Errorf("expected actionRetry for history request, got %d", action)
	}
	if retryReason != "Searching history" {
		t.Errorf("expected retry reason 'Searching history', got %q", retryReason)
	}
	if len(constraints) == 0 {
		t.Error("expected constraints to be populated with search results")
	}
}

func TestHandleResponses_HistorySkipWhenCapped(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "history", Filter: "git", Count: 10},
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

func TestHandleResponses_CommandWithInvalidTool(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "command", Commands: []string{"nonexistent_tool_xyz --flag"}},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{}

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionRetry {
		t.Errorf("expected actionRetry for invalid tool, got %d", action)
	}
	if retryReason != "Checking alternatives" {
		t.Errorf("expected retry reason 'Checking alternatives', got %q", retryReason)
	}

	foundConstraint := false
	for _, c := range constraints {
		if strings.Contains(c, "nonexistent_tool_xyz") && strings.Contains(c, "not installed") {
			foundConstraint = true
		}
	}
	if !foundConstraint {
		t.Error("expected constraint about nonexistent tool")
	}
}

func TestHandleResponses_Plan(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "plan", Commands: []string{"echo hello"}, StepsRemaining: 2},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{}

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	result, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionPlan {
		t.Errorf("expected actionPlan, got %d", action)
	}
	if result != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", result)
	}
	if stepsRemaining != 2 {
		t.Errorf("expected stepsRemaining 2, got %d", stepsRemaining)
	}
}

func TestHandleResponses_EmptyResponses(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{}

	_, action := handleResponses(nil, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionNothing {
		t.Errorf("expected actionNothing for empty responses, got %d", action)
	}
}

func TestHandleResponses_MemoryThenHistoryThenChat(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	// Memory + chat (no history) should store memory and return chat
	responses := []ai.TashResponse{
		{Type: "memory", Message: "Fact about user"},
		{Type: "chat", Message: "Understood!"},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{PromptTokens: 50, CompletionTokens: 25}

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionDone {
		t.Errorf("expected actionDone, got %d", action)
	}

	mem := convo.Memories()
	if !strings.Contains(mem, "Fact about user") {
		t.Error("memory should be stored")
	}
}

func TestHandleResponses_CommandWithMessage(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "command", Commands: []string{"echo hello"}, Message: "Here's the command"},
	}

	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	usage := ai.Usage{PromptTokens: 10, CompletionTokens: 5}

	old := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = old }()

	// "echo" exists as a command, so validation passes
	result, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionDone {
		t.Errorf("expected actionDone, got %d", action)
	}
	if result != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", result)
	}
}

func TestSearchContext_NoMatches(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: ls\n  when: 1000\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()

	result := searchContext(cfg, convo, nil, "nonexistent_pattern_xyz", 10)
	if !strings.Contains(result, "No matching") {
		t.Errorf("expected 'No matching' message, got %q", result)
	}
}

func TestSearchContext_WithResults(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: git status\n  when: 1000\n- cmd: git log\n  when: 1001\n- cmd: docker ps\n  when: 1002\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()

	result := searchContext(cfg, convo, nil, "git", 10)
	if strings.Contains(result, "No matching") {
		t.Error("expected results, got no matching message")
	}
	if !strings.Contains(result, "git") {
		t.Error("expected git-related results")
	}
	if !strings.Contains(result, "filtered: git") {
		t.Error("expected filter label in result")
	}
}

func TestSearchContext_DeduplicatesShellHistory(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: git status\n  when: 1000\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()

	// shellHistory already has "git status" — should be deduped
	shellHistory := []data.ShellCommand{{Command: "git status", ExitCode: 0}}

	result := searchContext(cfg, convo, shellHistory, "git", 10)
	if !strings.Contains(result, "No matching") {
		t.Errorf("expected deduplication to remove all matches, got %q", result)
	}
}

func TestSearchContext_CountCap(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")

	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("- cmd: test command\n  when: 1000\n")
	}
	_ = os.WriteFile(histPath, []byte(b.String()), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()

	result := searchContext(cfg, convo, nil, "test", 3)
	// Result should be capped (not more than 3 lines of content)
	lines := strings.Split(result, "\n")
	// First line is the label, rest are entries
	entries := 0
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) != "" {
			entries++
		}
	}
	if entries > 3 {
		t.Errorf("expected at most 3 entries, got %d", entries)
	}
}

func TestSearchContext_DefaultCount(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: ls\n  when: 1000\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()

	// count=0 should default to 50
	result := searchContext(cfg, convo, nil, "", 0)
	// Should not crash and should return something
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestShowChat(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	showChat("Hello from AI")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "Hello from AI") {
		t.Errorf("showChat output should contain message, got %q", got)
	}
}

func TestTraceAction_Accept(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	traceAction("accept", "ls -la")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "ls -la") {
		t.Errorf("traceAction output should contain command, got %q", got)
	}
}

func TestTraceAction_Skip(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	traceAction("skip", "rm -rf /")

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "rm -rf /") {
		t.Errorf("traceAction output should contain command, got %q", got)
	}
}

func TestPresentResults_SingleCommand(t *testing.T) {
	convo := data.NewConversation()
	usage := ai.Usage{PromptTokens: 100, CompletionTokens: 50}

	result := presentResults([]string{"ls -la"}, convo, "req1", usage)

	if result != "ls -la" {
		t.Errorf("expected 'ls -la', got %q", result)
	}

	// Should record in conversation
	found := false
	for _, e := range convo.Entries {
		if e.Type == "command" && e.Content == "ls -la" && e.Action == "accept" {
			found = true
			if e.PromptTokens != 100 || e.CompletionTokens != 50 {
				t.Errorf("unexpected token counts: %d/%d", e.PromptTokens, e.CompletionTokens)
			}
			if e.RequestID != "req1" {
				t.Errorf("expected request ID req1, got %q", e.RequestID)
			}
		}
	}
	if !found {
		t.Error("command response should be recorded in conversation")
	}
}

func TestSearchContext_MergesConvoAndHistory(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: git push\n  when: 1000\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()
	convo.AddQuery("how to use git rebase")

	result := searchContext(cfg, convo, nil, "git", 10)
	if strings.Contains(result, "No matching") {
		t.Error("expected results from both convo and history")
	}
	// Should contain both conversation and history results
	if !strings.Contains(result, "git") {
		t.Error("expected git in results")
	}
}

func TestSearchContext_DeduplicatesConvoEntries(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: docker ps\n  when: 1000\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()
	// Add a convo entry that matches what's already in prompt context
	convo.AddQuery("docker ps")

	// "docker ps" is both a convo entry and a history result, but also
	// already in the conversation's Entries which seeds 'seen'
	result := searchContext(cfg, convo, nil, "docker", 10)
	// Should not crash regardless of dedup outcome
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestSearchContext_NoFilter(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "fish_history")
	_ = os.WriteFile(histPath, []byte("- cmd: ls\n  when: 1000\n"), 0644)

	cfg := data.DefaultConfig()
	cfg.Profile.HistoryPath = histPath

	convo := data.NewConversation()

	result := searchContext(cfg, convo, nil, "", 10)
	if strings.Contains(result, "filtered:") {
		t.Error("should not contain filter label when filter is empty")
	}
}

func TestHandleResponses_ContextRequest(t *testing.T) {
	dir := t.TempDir()

	// Write some conversation entries so LoadMoreContext has something to load
	convoPath := filepath.Join(dir, "conversation.jsonl")
	var b strings.Builder
	for i := 0; i < 10; i++ {
		entry := `{"type":"query","content":"q` + string(rune('A'+i)) + `","time":` + string(rune('1'+i)) + `}`
		b.WriteString(entry + "\n")
	}
	_ = os.WriteFile(convoPath, []byte(b.String()), 0644)

	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	convo, _ := data.LoadConversation(dir, 0, 50)
	if convo == nil {
		convo = data.NewConversation()
	}

	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "context", Count: 50},
	}
	usage := ai.Usage{}

	_, action := handleResponses(responses, cfg, convo, nil, &constraints, false, &retryReason, &stepsRemaining, "req1", usage)

	if action != actionRetry {
		t.Errorf("expected actionRetry for context request, got %d", action)
	}
	if retryReason != "Loading context" {
		t.Errorf("expected retry reason 'Loading context', got %q", retryReason)
	}
	if len(constraints) == 0 {
		t.Error("expected constraints to be populated")
	}
}

func TestHandleResponses_ContextSkipWhenCapped(t *testing.T) {
	convo := data.NewConversation()
	constraints := []string{}
	retryReason := ""
	stepsRemaining := 0

	responses := []ai.TashResponse{
		{Type: "context", Count: 50},
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
