package data

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	stateFile          = "conversation.jsonl"
	defaultMaxEntries  = 250
	defaultMaxMemories = 50
)

// Entry represents a single event in the conversation timeline.
type Entry struct {
	Type             string `json:"type"` // "shell", "query", "command", "chat", "memory"
	Content          string `json:"content"`
	Session          string `json:"session,omitempty"`           // terminal session ID (fish PID)
	ExitCode         int    `json:"exit_code,omitempty"`         // for shell commands
	Action           string `json:"action,omitempty"`            // for commands: "accept", "skip"
	RequestID        string `json:"request_id,omitempty"`        // links entries from the same AI response
	PromptTokens     int    `json:"prompt_tokens,omitempty"`     // token usage from AI response (first entry only)
	CompletionTokens int    `json:"completion_tokens,omitempty"` // token usage from AI response (first entry only)
	Time             int64  `json:"time"`
}

// NewRequestID generates a short random ID to link entries from a single AI response.
func NewRequestID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Conversation holds the conversation context across shell invocations.
type Conversation struct {
	Entries     []Entry
	session     string
	dataDir     string // for loading more context on demand
	maxEntries  int
	maxMemories int
	maxLoaded   int // total lines loaded from disk so far
	savedCount  int // number of entries already persisted on disk
}

// NewConversation creates an empty conversation state.
func NewConversation() *Conversation {
	return &Conversation{
		Entries:     []Entry{},
		maxEntries:  defaultMaxEntries,
		maxMemories: defaultMaxMemories,
	}
}

// SetSession sets the terminal session ID for new entries.
func (s *Conversation) SetSession(session string) {
	s.session = session
}

// LoadConversation reads conversation state from disk as JSONL.
// Reads the file backwards to avoid loading the entire file into memory,
// keeping only the last maxEntries ephemeral entries plus up to maxMemories memories.
func LoadConversation(dataDir string, maxEntries int, maxMemories int) (*Conversation, error) {
	path := filepath.Join(dataDir, stateFile)

	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	if maxMemories <= 0 {
		maxMemories = defaultMaxMemories
	}
	state := &Conversation{maxEntries: maxEntries, maxMemories: maxMemories, dataDir: dataDir}

	loadCount := maxEntries + maxMemories
	lines, err := readLinesReverse(path, loadCount)
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		state.Entries = append(state.Entries, e)
	}

	state.maxLoaded = loadCount
	state.trim()
	state.savedCount = len(state.Entries)
	return state, nil
}

// LoadMoreContext loads additional older conversation entries from disk.
// Returns the number of new entries loaded. The maxTotal parameter caps the
// total conversation size to prevent unbounded growth (scroll buffer).
func (s *Conversation) LoadMoreContext(count int, maxTotal int) (int, error) {
	if s.dataDir == "" {
		return 0, fmt.Errorf("no data directory set")
	}

	path := filepath.Join(s.dataDir, stateFile)

	// Load more lines than currently loaded
	newLoadCount := s.maxLoaded + count
	if maxTotal > 0 && newLoadCount > maxTotal {
		newLoadCount = maxTotal
	}
	if newLoadCount <= s.maxLoaded {
		return 0, nil // already at max
	}

	lines, err := readLinesReverse(path, newLoadCount)
	if err != nil {
		return 0, fmt.Errorf("load more context: %w", err)
	}

	// Parse all loaded lines
	var allEntries []Entry
	for _, line := range lines {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		allEntries = append(allEntries, e)
	}

	// Find entries added during this session (after savedCount)
	var newSessionEntries []Entry
	if s.savedCount < len(s.Entries) {
		newSessionEntries = s.Entries[s.savedCount:]
	}

	// Replace entries with the expanded set, then re-append session entries
	s.Entries = allEntries
	s.maxLoaded = newLoadCount
	oldLen := len(s.Entries)
	s.Entries = append(s.Entries, newSessionEntries...)
	s.savedCount = oldLen

	return len(allEntries) - (oldLen - len(newSessionEntries)), nil
}

// readLinesReverse reads up to maxLines non-empty lines from the end of a file.
// Returns lines in chronological (original) order.
func readLinesReverse(path string, maxLines int) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}

	const chunkSize = 8192
	var lines [][]byte
	var trailing []byte // leftover bytes from previous chunk (toward start of file)
	pos := size

	for pos > 0 && len(lines) < maxLines {
		readSize := int64(chunkSize)
		if readSize > pos {
			readSize = pos
		}
		pos -= readSize

		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, pos); err != nil {
			return nil, err
		}

		// Prepend trailing bytes from previous chunk
		if len(trailing) > 0 {
			buf = append(buf, trailing...)
			trailing = nil
		}

		// Split into lines, scan from end
		for len(buf) > 0 && len(lines) < maxLines {
			idx := lastIndexByte(buf, '\n')
			if idx == -1 {
				// No newline found — this is a partial line from a chunk boundary
				trailing = make([]byte, len(buf))
				copy(trailing, buf)
				break
			}

			line := buf[idx+1:]
			buf = buf[:idx]

			if len(line) > 0 {
				lines = append(lines, line)
			}
		}
	}

	// Handle remaining partial line (first line of file)
	if len(trailing) > 0 && len(lines) < maxLines {
		if len(trailing) > 0 {
			lines = append(lines, trailing)
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines, nil
}

func lastIndexByte(b []byte, c byte) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// ResetConversation clears ephemeral entries but preserves memories.
// Rewrites the file with only memory entries.
func ResetConversation(dataDir string) error {
	path := filepath.Join(dataDir, stateFile)

	lines, err := readLinesReverse(path, defaultMaxEntries+defaultMaxMemories)
	if err != nil {
		return nil //nolint:nilerr // file doesn't exist — nothing to reset
	}

	var memories []Entry
	for _, line := range lines {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if e.Type == "memory" {
			memories = append(memories, e)
		}
	}

	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, e := range memories {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("reset conversation: %w", err)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ClearConversation removes the conversation file entirely.
func ClearConversation(dataDir string) error {
	path := filepath.Join(dataDir, stateFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear conversation: %w", err)
	}
	return nil
}

// Save appends new entries to the conversation file on disk.
// Only entries added after the last load/save are written.
func (s *Conversation) Save(dataDir string) error {
	if s.savedCount >= len(s.Entries) {
		return nil
	}

	path := filepath.Join(dataDir, stateFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	defer f.Close() //nolint:errcheck

	enc := json.NewEncoder(f)
	for _, e := range s.Entries[s.savedCount:] {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("save conversation entry: %w", err)
		}
	}

	s.savedCount = len(s.Entries)
	return nil
}

// AppendEntry writes a single entry to the end of the file without rewriting.
// Used by tick for fast single-entry writes.
func AppendEntry(dataDir string, e Entry) error {
	path := filepath.Join(dataDir, stateFile)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("append conversation: %w", err)
	}
	defer f.Close() //nolint:errcheck

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal conversation entry: %w", err)
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// AddMemory stores a durable fact the LLM decided is worth remembering.
// Oldest memories are dropped when the cap is exceeded.
func (s *Conversation) AddMemory(content string) {
	s.Entries = append(
		s.Entries, Entry{
			Type:    "memory",
			Content: content,
			Session: s.session,
			Time:    time.Now().Unix(),
		},
	)
	s.trimMemories()
}

// AddShellCommand records a command the user ran in the shell.
func (s *Conversation) AddShellCommand(command string, exitCode int) {
	s.Entries = append(
		s.Entries, Entry{
			Type:     "shell",
			Content:  command,
			Session:  s.session,
			ExitCode: exitCode,
			Time:     time.Now().Unix(),
		},
	)
	s.trim()
}

// AddQuery records a tash query from the user.
func (s *Conversation) AddQuery(query string) {
	s.Entries = append(
		s.Entries, Entry{
			Type:    "query",
			Content: query,
			Session: s.session,
			Time:    time.Now().Unix(),
		},
	)
	s.trim()
}

// AddCommandResponse records a command suggestion from tash with the user's action.
// action is "accept" or "skip". requestID links entries from the same AI response.
// Token usage should only be set on the first entry per request.
func (s *Conversation) AddCommandResponse(
	command string,
	action string,
	requestID string,
	promptTokens int,
	completionTokens int,
) {
	s.Entries = append(
		s.Entries, Entry{
			Type:             "command",
			Content:          command,
			Action:           action,
			RequestID:        requestID,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			Session:          s.session,
			Time:             time.Now().Unix(),
		},
	)
	s.trim()
}

// AddChatResponse records a chat message from tash.
// requestID links entries from the same AI response.
func (s *Conversation) AddChatResponse(message string, requestID string, promptTokens int, completionTokens int) {
	s.Entries = append(
		s.Entries, Entry{
			Type:             "chat",
			Content:          message,
			RequestID:        requestID,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			Session:          s.session,
			Time:             time.Now().Unix(),
		},
	)
	s.trim()
}

// ShellCommand represents a command with its exit code for context injection.
type ShellCommand struct {
	Command  string
	ExitCode int
	Time     int64
}

// RecentShellCommands returns the last N shell commands with exit codes.
// Shell entries that match a query entry (tash-routed commands from
// fish_command_not_found) are excluded — they're not real shell commands.
func (s *Conversation) RecentShellCommands(n int) []ShellCommand {
	// Build set of query contents to filter against.
	// Include shell-unescaped forms so fish-escaped commands match.
	queries := make(map[string]bool)
	for _, e := range s.Entries {
		if e.Type == "query" {
			queries[e.Content] = true
			queries[stripShellEscapes(e.Content)] = true
		}
	}

	var cmds []ShellCommand
	for i := len(s.Entries) - 1; i >= 0 && len(cmds) < n; i-- {
		e := s.Entries[i]
		if e.Type != "shell" {
			continue
		}
		// Skip shell entries that are tash queries routed via command_not_found
		if queries[e.Content] || queries[stripShellEscapes(e.Content)] {
			continue
		}
		cmds = append(
			cmds, ShellCommand{
				Command:  e.Content,
				ExitCode: e.ExitCode,
				Time:     e.Time,
			},
		)
	}
	for i, j := 0, len(cmds)-1; i < j; i, j = i+1, j-1 {
		cmds[i], cmds[j] = cmds[j], cmds[i]
	}
	return cmds
}

// Memories returns all stored memory entries as formatted text.
func (s *Conversation) Memories() string {
	var b strings.Builder
	for _, e := range s.Entries {
		if e.Type == "memory" {
			fmt.Fprintf(&b, "- %s\n", e.Content)
		}
	}
	return b.String()
}

// stripShellEscapes removes common fish shell escape sequences so that
// fish-escaped strings (e.g. it\'s, say\"hi\") match their unescaped forms.
func stripShellEscapes(s string) string {
	r := strings.NewReplacer(
		"\\'", "'",
		"\\\"", "\"",
		"\\\\", "\\",
	)
	return r.Replace(s)
}

// FormatForAI returns the conversation formatted as AI message history.
// Only commands the user accepted (action="accept") are included.
// Skipped and edited suggestions are omitted entirely to avoid confusing
// the model with rejected suggestions.
// Failed shell commands (non-zero exit) are included as user context so
// the AI knows which commands didn't work.
// Each entry includes a timestamp prefix so the AI is aware of when events occurred.
func (s *Conversation) FormatForAI() []AIMessage {
	// Build set of query contents to skip matching shell entries.
	// Store both raw and shell-unescaped forms so that fish-escaped
	// commands (e.g. it\'s) match their unescaped query counterparts.
	queries := make(map[string]bool)
	for _, e := range s.Entries {
		if e.Type == "query" {
			queries[e.Content] = true
			queries[stripShellEscapes(e.Content)] = true
		}
	}

	var messages []AIMessage

	for _, e := range s.Entries {
		ts := FormatTimestamp(e.Time)
		switch e.Type {
		case "memory":
			continue
		case "shell":
			// Include failed commands (non-zero exit) that aren't tash queries
			if e.ExitCode != 0 && !queries[e.Content] && !queries[stripShellEscapes(e.Content)] {
				messages = append(
					messages, AIMessage{
						Role:    "user",
						Content: fmt.Sprintf("[%s] $ %s  # exit %d", ts, e.Content, e.ExitCode),
					},
				)
			}
		case "query":
			messages = append(messages, AIMessage{Role: "user", Content: fmt.Sprintf("[%s] %s", ts, e.Content)})
		case "command":
			if e.Action == "accept" || e.Action == "" {
				messages = append(messages, AIMessage{Role: "assistant", Content: e.Content})
			}
		case "chat":
			messages = append(messages, AIMessage{Role: "assistant", Content: e.Content})
		}
	}

	return messages
}

// FormatTimestamp converts a unix timestamp to a human-readable format.
// Returns "unknown" for zero timestamps.
func FormatTimestamp(unix int64) string {
	if unix == 0 {
		return "unknown"
	}
	return time.Unix(unix, 0).Format("2006-01-02T15:04:05")
}

// AIMessage is a role+content pair for multi-turn conversation.
type AIMessage struct {
	Role    string
	Content string
}

// Search finds conversation entries matching a filter (regex or substring).
func (s *Conversation) Search(filter string, count int) []string {
	if count <= 0 {
		count = 50
	}

	var results []string
	var re *regexp.Regexp
	if filter != "" {
		re, _ = regexp.Compile(filter)
	}

	for _, e := range s.Entries {
		if e.Type == "shell" {
			continue
		}
		if filter == "" {
			results = append(results, e.Content)
			continue
		}
		if re != nil && re.MatchString(e.Content) {
			results = append(results, e.Content)
		} else if re == nil && strings.Contains(e.Content, filter) {
			results = append(results, e.Content)
		}
	}

	if len(results) > count {
		results = results[len(results)-count:]
	}
	return results
}

func (s *Conversation) trim() {
	s.trimMemories()

	if len(s.Entries) > s.maxEntries {
		before := len(s.Entries)
		var memories []Entry
		var ephemeral []Entry
		for _, e := range s.Entries {
			if e.Type == "memory" {
				memories = append(memories, e)
			} else {
				ephemeral = append(ephemeral, e)
			}
		}
		keep := s.maxEntries - len(memories)
		if keep < 0 {
			keep = 0
		}
		if len(ephemeral) > keep {
			ephemeral = ephemeral[len(ephemeral)-keep:]
		}
		s.Entries = append(memories, ephemeral...)
		dropped := before - len(s.Entries)
		s.savedCount -= dropped
		if s.savedCount < 0 {
			s.savedCount = 0
		}
	}
}

func (s *Conversation) trimMemories() {
	var count int
	for _, e := range s.Entries {
		if e.Type == "memory" {
			count++
		}
	}
	if count <= s.maxMemories {
		return
	}

	dropped := count - s.maxMemories
	filtered := make([]Entry, 0, len(s.Entries)-dropped)
	for _, e := range s.Entries {
		if e.Type == "memory" && dropped > 0 {
			dropped--
			continue
		}
		filtered = append(filtered, e)
	}
	s.Entries = filtered
	s.savedCount -= count - s.maxMemories
	if s.savedCount < 0 {
		s.savedCount = 0
	}
}
