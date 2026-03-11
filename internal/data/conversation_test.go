package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConversation(t *testing.T) {
	c := NewConversation()
	if c == nil {
		t.Fatal("expected non-nil conversation")
	}
	if len(c.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(c.Entries))
	}
}

func TestSetSession(t *testing.T) {
	c := NewConversation()
	c.SetSession("12345")
	c.AddQuery("test")
	if c.Entries[0].Session != "12345" {
		t.Errorf("expected session 12345, got %q", c.Entries[0].Session)
	}
}

func TestNewRequestID_Format(t *testing.T) {
	id := NewRequestID()
	if len(id) != 8 {
		t.Errorf("expected 8 char hex string, got %q (len %d)", id, len(id))
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("expected hex char, got %c in %q", c, id)
		}
	}
}

func TestNewRequestID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewRequestID()
		if seen[id] {
			t.Fatalf("duplicate request ID: %q", id)
		}
		seen[id] = true
	}
}

func TestRecentShellCommands(t *testing.T) {
	c := NewConversation()
	c.Entries = []Entry{
		{Type: "shell", Content: "ls", ExitCode: 0},
		{Type: "query", Content: "help me"},
		{Type: "shell", Content: "git status", ExitCode: 0},
		{Type: "shell", Content: "make build", ExitCode: 2},
	}

	cmds := c.RecentShellCommands(10)
	if len(cmds) != 3 {
		t.Fatalf("expected 3 shell commands, got %d", len(cmds))
	}

	// Should be in chronological order
	if cmds[0].Command != "ls" {
		t.Errorf("expected first cmd ls, got %q", cmds[0].Command)
	}
	if cmds[2].Command != "make build" || cmds[2].ExitCode != 2 {
		t.Errorf("expected last cmd make build with exit 2, got %q exit %d", cmds[2].Command, cmds[2].ExitCode)
	}
}

func TestRecentShellCommands_ExcludesQueries(t *testing.T) {
	c := NewConversation()
	c.Entries = []Entry{
		{Type: "query", Content: "show me docker containers"},
		{Type: "shell", Content: "show me docker containers", ExitCode: 127},
		{Type: "shell", Content: "docker ps", ExitCode: 0},
	}

	cmds := c.RecentShellCommands(10)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (query-matching shell filtered), got %d", len(cmds))
	}
	if cmds[0].Command != "docker ps" {
		t.Errorf("expected docker ps, got %q", cmds[0].Command)
	}
}

func TestRecentShellCommands_Limit(t *testing.T) {
	c := NewConversation()
	for i := 0; i < 20; i++ {
		c.Entries = append(c.Entries, Entry{Type: "shell", Content: "cmd"})
	}

	cmds := c.RecentShellCommands(5)
	if len(cmds) != 5 {
		t.Errorf("expected 5 commands, got %d", len(cmds))
	}
}

func TestMemories(t *testing.T) {
	c := NewConversation()
	c.Entries = []Entry{
		{Type: "memory", Content: "User is John"},
		{Type: "shell", Content: "ls"},
		{Type: "memory", Content: "User prefers Go"},
	}

	got := c.Memories()
	if !strings.Contains(got, "- User is John") {
		t.Error("expected first memory")
	}
	if !strings.Contains(got, "- User prefers Go") {
		t.Error("expected second memory")
	}
	if strings.Contains(got, "ls") {
		t.Error("should not contain non-memory entries")
	}
}

func TestMemories_Empty(t *testing.T) {
	c := NewConversation()
	got := c.Memories()
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatForAI(t *testing.T) {
	c := NewConversation()
	c.Entries = []Entry{
		{Type: "query", Content: "show files"},
		{Type: "command", Content: "ls -la", Action: "accept"},
		{Type: "shell", Content: "make build", ExitCode: 2},
		{Type: "chat", Content: "Here's the info"},
		{Type: "memory", Content: "User likes Go"},
		{Type: "command", Content: "skipped cmd", Action: "skip"},
	}

	msgs := c.FormatForAI()

	// query -> user, accepted command -> assistant, failed shell -> user, chat -> assistant
	// memory is excluded, skipped command is excluded
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "show files" {
		t.Errorf("msg 0: expected user/show files, got %s/%s", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "ls -la" {
		t.Errorf("msg 1: expected assistant/ls -la, got %s/%s", msgs[1].Role, msgs[1].Content)
	}
	if msgs[2].Role != "user" || !strings.Contains(msgs[2].Content, "exit 2") {
		t.Errorf("msg 2: expected user with exit code, got %s/%s", msgs[2].Role, msgs[2].Content)
	}
	if msgs[3].Role != "assistant" || msgs[3].Content != "Here's the info" {
		t.Errorf("msg 3: expected assistant/info, got %s/%s", msgs[3].Role, msgs[3].Content)
	}
}

func TestSearch_Regex(t *testing.T) {
	c := NewConversation()
	c.Entries = []Entry{
		{Type: "query", Content: "how to use docker"},
		{Type: "chat", Content: "Use docker run"},
		{Type: "query", Content: "kubernetes help"},
	}

	results := c.Search("docker", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}
}

func TestSearch_NoFilter(t *testing.T) {
	c := NewConversation()
	c.Entries = []Entry{
		{Type: "query", Content: "one"},
		{Type: "chat", Content: "two"},
		{Type: "shell", Content: "three"}, // shell entries are excluded from search
	}

	results := c.Search("", 10)
	if len(results) != 2 {
		t.Errorf("expected 2 results (shell excluded), got %d", len(results))
	}
}

func TestSearch_CountLimit(t *testing.T) {
	c := NewConversation()
	for i := 0; i < 20; i++ {
		c.Entries = append(c.Entries, Entry{Type: "query", Content: "test"})
	}

	results := c.Search("test", 5)
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestTrim(t *testing.T) {
	c := NewConversation()
	c.maxMemories = 50

	// Add memory + lots of ephemeral entries
	c.Entries = append(c.Entries, Entry{Type: "memory", Content: "fact"})
	for i := 0; i < 300; i++ {
		c.Entries = append(c.Entries, Entry{Type: "shell", Content: "cmd"})
	}

	c.trim()

	if len(c.Entries) > maxEntries {
		t.Errorf("expected at most %d entries, got %d", maxEntries, len(c.Entries))
	}

	// Memory should be preserved
	hasMemory := false
	for _, e := range c.Entries {
		if e.Type == "memory" {
			hasMemory = true
			break
		}
	}
	if !hasMemory {
		t.Error("memory entry should be preserved during trim")
	}
}

func TestTrimMemories(t *testing.T) {
	c := NewConversation()
	c.maxMemories = 3

	for i := 0; i < 5; i++ {
		c.Entries = append(c.Entries, Entry{Type: "memory", Content: "fact"})
	}
	c.Entries = append(c.Entries, Entry{Type: "shell", Content: "cmd"})

	c.trimMemories()

	memCount := 0
	for _, e := range c.Entries {
		if e.Type == "memory" {
			memCount++
		}
	}
	if memCount != 3 {
		t.Errorf("expected 3 memories after trim, got %d", memCount)
	}

	// Shell entry should still be there
	hasShell := false
	for _, e := range c.Entries {
		if e.Type == "shell" {
			hasShell = true
		}
	}
	if !hasShell {
		t.Error("non-memory entries should be preserved")
	}
}

func TestLastIndexByte(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		c    byte
		want int
	}{
		{"found at end", []byte("hello\n"), '\n', 5},
		{"found at start", []byte("\nhello"), '\n', 0},
		{"multiple", []byte("a\nb\nc"), '\n', 3},
		{"not found", []byte("hello"), '\n', -1},
		{"empty", []byte{}, '\n', -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastIndexByte(tt.b, tt.c)
			if got != tt.want {
				t.Errorf("lastIndexByte() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStripShellEscapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single quote escape", "it\\'s", "it's"},
		{"double quote escape", "say\\\"hi\\\"", `say"hi"`},
		{"backslash escape", "path\\\\file", "path\\file"},
		{"no escapes", "hello world", "hello world"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripShellEscapes(tt.input)
			if got != tt.want {
				t.Errorf("stripShellEscapes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Tier 2: Filesystem tests

func TestAppendEntry(t *testing.T) {
	dir := t.TempDir()
	e := Entry{Type: "shell", Content: "ls -la", Time: 1000}

	if err := AppendEntry(dir, e); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Entry
	if err := json.Unmarshal(raw[:len(raw)-1], &got); err != nil { // trim trailing newline
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Type != "shell" || got.Content != "ls -la" {
		t.Errorf("expected shell/ls -la, got %s/%s", got.Type, got.Content)
	}
}

func TestLoadConversation(t *testing.T) {
	dir := t.TempDir()

	// Write some JSONL entries
	entries := []Entry{
		{Type: "memory", Content: "User likes Go", Time: 1},
		{Type: "query", Content: "list files", Time: 2},
		{Type: "command", Content: "ls -la", Action: "accept", Time: 3},
	}
	var b strings.Builder
	for _, e := range entries {
		data, _ := json.Marshal(e)
		b.Write(data)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(filepath.Join(dir, stateFile), []byte(b.String()), 0644)

	convo, err := LoadConversation(dir, 50)
	if err != nil {
		t.Fatalf("LoadConversation: %v", err)
	}

	if len(convo.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(convo.Entries))
	}
	if convo.Entries[0].Type != "memory" {
		t.Errorf("expected memory first, got %q", convo.Entries[0].Type)
	}
}

func TestLoadConversation_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadConversation(dir, 50)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSaveConversation(t *testing.T) {
	dir := t.TempDir()

	convo := NewConversation()
	convo.AddQuery("test query")
	convo.AddCommandResponse("echo hi", "accept", "abc123", 100, 50)

	if err := convo.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load it back
	loaded, err := LoadConversation(dir, 50)
	if err != nil {
		t.Fatalf("LoadConversation: %v", err)
	}

	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Type != "query" {
		t.Errorf("expected query, got %q", loaded.Entries[0].Type)
	}
	if loaded.Entries[1].Type != "command" {
		t.Errorf("expected command, got %q", loaded.Entries[1].Type)
	}
}

func TestSaveConversation_OnlyNewEntries(t *testing.T) {
	dir := t.TempDir()

	convo := NewConversation()
	convo.AddQuery("first")
	_ = convo.Save(dir)

	convo.AddQuery("second")
	_ = convo.Save(dir)

	// Load and verify only 2 entries total (not duplicated)
	loaded, _ := LoadConversation(dir, 50)
	if len(loaded.Entries) != 2 {
		t.Errorf("expected 2 entries (no duplicates), got %d", len(loaded.Entries))
	}
}

func TestResetConversation(t *testing.T) {
	dir := t.TempDir()

	// Write mixed entries
	entries := []Entry{
		{Type: "memory", Content: "fact1", Time: 1},
		{Type: "query", Content: "question", Time: 2},
		{Type: "memory", Content: "fact2", Time: 3},
		{Type: "shell", Content: "ls", Time: 4},
	}
	var b strings.Builder
	for _, e := range entries {
		data, _ := json.Marshal(e)
		b.Write(data)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(filepath.Join(dir, stateFile), []byte(b.String()), 0644)

	if err := ResetConversation(dir); err != nil {
		t.Fatalf("ResetConversation: %v", err)
	}

	// Only memories should remain
	convo, err := LoadConversation(dir, 50)
	if err != nil {
		t.Fatalf("LoadConversation after reset: %v", err)
	}

	for _, e := range convo.Entries {
		if e.Type != "memory" {
			t.Errorf("expected only memories, got type %q", e.Type)
		}
	}
	if len(convo.Entries) != 2 {
		t.Errorf("expected 2 memories, got %d", len(convo.Entries))
	}
}

func TestClearConversation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, stateFile)
	_ = os.WriteFile(path, []byte(`{"type":"query","content":"test","time":1}`+"\n"), 0644)

	if err := ClearConversation(dir); err != nil {
		t.Fatalf("ClearConversation: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be removed")
	}
}

func TestClearConversation_NoFile(t *testing.T) {
	dir := t.TempDir()
	if err := ClearConversation(dir); err != nil {
		t.Errorf("ClearConversation should not error on missing file: %v", err)
	}
}

func TestReadLinesReverse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// Write 10 lines
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString(`{"n":` + strings.Repeat("x", 0) + `}` + "\n")
	}
	_ = b.String() // just to use b

	var content strings.Builder
	for i := 0; i < 10; i++ {
		e := Entry{Type: "shell", Content: "cmd" + string(rune('0'+i)), Time: int64(i)}
		data, _ := json.Marshal(e)
		content.Write(data)
		content.WriteByte('\n')
	}
	_ = os.WriteFile(path, []byte(content.String()), 0644)

	lines, err := readLinesReverse(path, 5)
	if err != nil {
		t.Fatalf("readLinesReverse: %v", err)
	}

	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}

	// Should be in chronological order (last 5 entries)
	var first Entry
	_ = json.Unmarshal(lines[0], &first)
	if first.Content != "cmd5" {
		t.Errorf("expected first line to be cmd5, got %q", first.Content)
	}
}

func TestReadLinesReverse_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	_ = os.WriteFile(path, []byte{}, 0644)

	lines, err := readLinesReverse(path, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}
