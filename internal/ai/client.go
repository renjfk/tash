package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	aitools "github.com/renjfk/tash/internal/ai/tools"
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
	Tools    []aitools.Definition
}

// ToolCall represents one tool call emitted by the model.
type ToolCall struct {
	ID       string
	Type     string
	Function FunctionCall
}

// FunctionCall carries the tool name and JSON arguments.
type FunctionCall struct {
	Name      string
	Arguments string
}

// Usage holds token counts from a completion request.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
}

// APIError represents a non-200 response from the provider.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Body)
}

// IsToolRejection reports whether the error is an API response that typically
// means the request shape was rejected, such as an endpoint that does not
// support the `tools` parameter. Deliberately excludes 401/403/429, which are
// auth or throttling failures that tools do not cause and must not mask.
func IsToolRejection(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}

// Response represents an AI completion response.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
}

// TashResponse is the structured JSON response from the AI.
type TashResponse struct {
	Type           string   `json:"type"`                      // "command", "chat", "history", "memory", "plan", "screen", "conversation"
	Commands       []string `json:"commands,omitempty"`        // for type "command" or "plan"
	Message        string   `json:"message,omitempty"`         // explanation, chat text, or memory fact
	Count          int      `json:"count,omitempty"`           // quantity: entries for history/context, lines for screen (default 50)
	Filter         string   `json:"filter,omitempty"`          // for type "history": grep filter
	StepsRemaining int      `json:"steps_remaining,omitempty"` // for type "plan": estimated remaining steps
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
// The context can be used to cancel in-flight requests (e.g. when the user presses Esc).
func (c *Client) Complete(ctx context.Context, req Request) (*Response, error) {
	apiKey := c.cfg.APIKey()

	// Build OpenAI-compatible messages: system prompt as first message, then conversation
	var messages []chatMessage
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		messages = append(messages, chatMessage{Role: m.Role, Content: m.Content})
	}

	body := chatRequest{
		Model:              req.Model,
		Messages:           messages,
		Tools:              toChatTools(req.Tools),
		ChatTemplateKwargs: c.cfg.Model.ChatTemplateKwargs,
		ReasoningEffort:    strings.TrimSpace(c.cfg.Model.ReasoningEffort),
	}
	if c.cfg.Model.MaxTokensParam == "max_completion_tokens" {
		body.MaxCompletionTokens = c.cfg.Model.MaxTokens
	} else {
		body.MaxTokens = c.cfg.Model.MaxTokens
	}
	if len(body.Tools) > 0 {
		body.ToolChoice = "auto"
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
		"tools", len(body.Tools),
		"tool_names", aitools.Names(req.Tools),
		"tool_choice", body.ToolChoice,
		"body", string(jsonBody),
	)

	endpoint := strings.TrimRight(c.cfg.Model.Endpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

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
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
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
		Content:   apiResp.Choices[0].Message.Content,
		ToolCalls: fromChatToolCalls(apiResp.Choices[0].Message.ToolCalls),
		Usage: Usage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			CachedTokens:     apiResp.Usage.cachedTokens(),
		},
	}, nil
}

// ParseToolCalls maps standard tool calls into tash's internal response format.
func ParseToolCalls(toolCalls []ToolCall) ([]TashResponse, error) {
	responses := make([]TashResponse, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		resp, err := parseToolCall(toolCall)
		if err != nil {
			return nil, err
		}
		if resp.Count <= 0 {
			resp.Count = 50
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

func parseToolCall(toolCall ToolCall) (TashResponse, error) {
	var args struct {
		Commands       []string `json:"commands"`
		Message        string   `json:"message"`
		Count          int      `json:"count"`
		Filter         string   `json:"filter"`
		StepsRemaining int      `json:"steps_remaining"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return TashResponse{}, fmt.Errorf("parse tool arguments for %s: %w", toolCall.Function.Name, err)
	}

	switch toolCall.Function.Name {
	case "command":
		return TashResponse{Type: "command", Commands: args.Commands, Message: args.Message}, nil
	case "chat":
		return TashResponse{Type: "chat", Message: args.Message}, nil
	case "plan":
		return TashResponse{Type: "plan", Commands: args.Commands, Message: args.Message, StepsRemaining: args.StepsRemaining}, nil
	case "history":
		return TashResponse{Type: "history", Filter: args.Filter, Count: args.Count}, nil
	case "conversation":
		return TashResponse{Type: "conversation", Count: args.Count}, nil
	case "screen":
		return TashResponse{Type: "screen", Count: args.Count}, nil
	case "memory":
		return TashResponse{Type: "memory", Message: args.Message}, nil
	default:
		return TashResponse{}, fmt.Errorf("unknown tool call: %s", toolCall.Function.Name)
	}
}

// ParseResponse parses the structured JSON response from the AI.
// Supports JSONL: multiple JSON objects separated by newlines.
// Non-JSON lines preceding a command/chat are captured as context text.
// Returns all parsed responses in order.
func ParseResponse(raw string) ([]TashResponse, error) {
	raw = stripThinkingContent(raw)
	raw = stripCodeFences(raw)

	var responses []TashResponse
	var preamble []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp TashResponse
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
		if resp.Count <= 0 {
			resp.Count = 50
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

func stripThinkingContent(s string) string {
	stripped := strings.TrimSpace(s)
	for _, marker := range []struct{ open, close string }{
		{open: "<think>", close: "</think>"},
		{open: "<thinking>", close: "</thinking>"},
	} {
		for {
			lower := strings.ToLower(stripped)
			start := strings.Index(lower, marker.open)
			if start < 0 {
				break
			}
			end := strings.Index(lower[start+len(marker.open):], marker.close)
			if end < 0 {
				stripped = strings.TrimSpace(stripped[:start])
				break
			}
			end += start + len(marker.open) + len(marker.close)
			stripped = strings.TrimSpace(stripped[:start] + "\n" + stripped[end:])
		}
	}

	lower := strings.ToLower(strings.TrimSpace(stripped))
	for _, prefix := range []string{
		"thinking process:",
		"thought process:",
		"reasoning:",
		"chain of thought:",
		"here's a thinking process",
		"here is a thinking process",
	} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		for _, marker := range []string{"final answer:", "final:", "summary:", "overview:", "answer:"} {
			idx := strings.Index(lower, marker)
			if idx >= 0 {
				return strings.TrimSpace(stripped[idx+len(marker):])
			}
		}
		return ""
	}

	return stripped
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
	Model               string         `json:"model"`
	MaxTokens           int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens int            `json:"max_completion_tokens,omitempty"`
	Messages            []chatMessage  `json:"messages"`
	Tools               []chatTool     `json:"tools,omitempty"`
	ToolChoice          string         `json:"tool_choice,omitempty"`
	ChatTemplateKwargs  map[string]any `json:"chat_template_kwargs,omitempty"`
	ReasoningEffort     string         `json:"reasoning_effort,omitempty"`
}

type chatMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatFunctionCall `json:"function"`
}

type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatUsage struct {
	PromptTokens         int `json:"prompt_tokens"`
	CompletionTokens     int `json:"completion_tokens"`
	PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"` // DeepSeek
	CacheReadInputTokens int `json:"cache_read_input_tokens"` // proxies (e.g. LiteLLM)
	PromptTokensDetails  struct {
		CachedTokens int `json:"cached_tokens"` // OpenAI-compatible
	} `json:"prompt_tokens_details"`
}

// cachedTokens returns the number of prompt tokens served from the provider's
// prompt cache. It honors the field names used by OpenAI-compatible endpoints,
// DeepSeek, and caching proxies.
func (u chatUsage) cachedTokens() int {
	return max(u.PromptTokensDetails.CachedTokens, u.PromptCacheHitTokens, u.CacheReadInputTokens)
}

func toChatTools(tools []aitools.Definition) []chatTool {
	chatTools := make([]chatTool, 0, len(tools))
	for _, tool := range tools {
		chatTools = append(chatTools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters.ToMap(),
			},
		})
	}
	return chatTools
}

func fromChatToolCalls(toolCalls []chatToolCall) []ToolCall {
	parsed := make([]ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		parsed = append(parsed, ToolCall{
			ID:   toolCall.ID,
			Type: toolCall.Type,
			Function: FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		})
	}
	return parsed
}
