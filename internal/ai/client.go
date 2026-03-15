package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/renjfk/tash/internal/data"
)

// Client handles communication with AI providers.
type Client struct {
	cfg        *data.Config
	httpClient *http.Client
}

// Message represents a single message in a conversation.
type Message struct {
	Role    string
	Content string
}

// Request represents an AI completion request.
type Request struct {
	Model    string
	System   string
	Messages []Message
}

// Usage holds token counts from a completion request.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// Response represents an AI completion response.
type Response struct {
	Content string
	Usage   Usage
}

// TashResponse is the structured JSON response from the AI.
type TashResponse struct {
	Type           string   `json:"type"`                      // "command", "chat", "history", "memory", "plan", "screen", "context"
	Commands       []string `json:"commands,omitempty"`        // for type "command" or "plan"
	Message        string   `json:"message,omitempty"`         // explanation, chat text, or memory fact
	Count          int      `json:"count,omitempty"`           // for type "history" or "context": how many entries (default 50)
	Filter         string   `json:"filter,omitempty"`          // for type "history": grep filter
	StepsRemaining int      `json:"steps_remaining,omitempty"` // for type "plan": estimated remaining steps
	Lines          int      `json:"lines,omitempty"`           // for type "screen": how many lines to capture (default 20)
}

// NewClient creates an AI client from config.
func NewClient(cfg *data.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Complete sends a request to the AI provider and returns the response.
// Uses the OpenAI-compatible chat completions API, which works with Anthropic's
// compatibility layer and any other OpenAI-compatible endpoint (OpenAI, Ollama, etc.).
func (c *Client) Complete(req Request) (*Response, error) {
	apiKey := c.cfg.APIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("API key not set (env: %s)", c.cfg.Model.APIKeyEnv)
	}

	// Build OpenAI-compatible messages: system prompt as first message, then conversation
	var messages []chatMessage
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		messages = append(messages, chatMessage(m))
	}

	body := chatRequest{
		Model:     req.Model,
		MaxTokens: c.cfg.Model.MaxTokens,
		Messages:  messages,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("API request",
		"model", req.Model,
		"endpoint", c.cfg.Model.Endpoint,
		"system_len", len(req.System),
		"messages", len(messages),
		"body", string(jsonBody),
	)

	endpoint := strings.TrimRight(c.cfg.Model.Endpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	elapsed := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	slog.Debug("API response",
		"status", resp.StatusCode,
		"elapsed_ms", elapsed.Milliseconds(),
		"body", string(respBody),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	slog.Debug("API parsed",
		"choices", len(apiResp.Choices),
		"finish_reason", apiResp.Choices[0].FinishReason,
		"usage_prompt", apiResp.Usage.PromptTokens,
		"usage_completion", apiResp.Usage.CompletionTokens,
	)

	return &Response{
		Content: apiResp.Choices[0].Message.Content,
		Usage: Usage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
		},
	}, nil
}

// ParseResponse parses the structured JSON response from the AI.
// Supports JSONL: multiple JSON objects separated by newlines.
// Non-JSON lines preceding a command/chat are captured as context text.
// Returns all parsed responses in order.
func ParseResponse(raw string) ([]TashResponse, error) {
	raw = stripCodeFences(raw)

	var responses []TashResponse
	var preamble []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		resp := TashResponse{Count: 50, Lines: 20}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// Retry with sanitized JSON escapes if it looks like JSON
			if len(line) > 0 && line[0] == '{' {
				if err2 := json.Unmarshal([]byte(sanitizeJSON(line)), &resp); err2 != nil {
					preamble = append(preamble, line)
					continue
				}
			} else {
				preamble = append(preamble, line)
				continue
			}
		}
		if resp.Type == "" {
			continue
		}
		// Attach preamble text as message if the response doesn't have one
		if len(preamble) > 0 && resp.Message == "" {
			resp.Message = strings.Join(preamble, "\n")
		}
		preamble = nil
		responses = append(responses, resp)
	}

	if len(responses) == 0 {
		// Not valid JSON at all — treat the whole thing as chat
		return []TashResponse{{Type: "chat", Message: raw}}, nil
	}

	return responses, nil
}

// sanitizeJSON fixes invalid escape sequences that the AI sometimes produces
// inside JSON strings (e.g. \| instead of \\| in regex patterns).
func sanitizeJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		}
		if inString && ch == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
				// valid JSON escape — keep as-is
			default:
				// invalid escape like \| — double the backslash
				b.WriteByte('\\')
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func stripCodeFences(s string) string {
	s = trimPrefixAny(s, "```json\n", "```\n")
	if idx := lastIndex(s, "\n```"); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func trimPrefixAny(s string, prefixes ...string) string {
	for _, p := range prefixes {
		if len(s) > len(p) && s[:len(p)] == p {
			return s[len(p):]
		}
	}
	return s
}

func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// OpenAI-compatible API types

type chatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
