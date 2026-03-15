package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renjfk/tash/internal/data"
)

func TestNewClient(t *testing.T) {
	cfg := data.DefaultConfig()
	client := NewClient(cfg)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.cfg != cfg {
		t.Error("client.cfg does not match provided config")
	}
	if client.httpClient == nil {
		t.Error("client.httpClient is nil")
	}
}

func TestComplete_Success(t *testing.T) {
	apiResp := chatResponse{
		Choices: []chatChoice{
			{
				Message:      chatMessage{Role: "assistant", Content: `{"type":"command","commands":["ls -la"]}`},
				FinishReason: "stop",
			},
		},
		Usage: chatUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Verify body structure
		var body chatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", body.Model)
		}
		if len(body.Messages) != 2 {
			t.Errorf("expected 2 messages (system + user), got %d", len(body.Messages))
		}
		if body.Messages[0].Role != "system" {
			t.Errorf("expected first message role system, got %s", body.Messages[0].Role)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiResp)
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	resp, err := client.Complete(Request{
		Model:  "test-model",
		System: "You are a helper.",
		Messages: []Message{
			{Role: "user", Content: "list files"},
		},
	})

	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != `{"type":"command","commands":["ls -la"]}` {
		t.Errorf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 50 {
		t.Errorf("expected 50 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
}

func TestComplete_NoAPIKey(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Model.APIKeyEnv = "NONEXISTENT_KEY_FOR_TEST"
	t.Setenv("NONEXISTENT_KEY_FOR_TEST", "")

	client := NewClient(cfg)
	_, err := client.Complete(Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	if err == nil {
		t.Error("expected error when API key is empty")
	}
}

func TestComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	_, err := client.Complete(Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	if err == nil {
		t.Error("expected error on 429 response")
	}
}

func TestComplete_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(chatResponse{Choices: []chatChoice{}})
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	_, err := client.Complete(Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	if err == nil {
		t.Error("expected error on empty choices")
	}
}

func TestComplete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	_, err := client.Complete(Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	if err == nil {
		t.Error("expected error on invalid JSON response")
	}
}

func TestComplete_UsesConfigMaxTokens(t *testing.T) {
	var receivedBody chatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Content: "ok"}}},
		})
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	cfg.Model.MaxTokens = 4096
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	_, _ = client.Complete(Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	if receivedBody.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096 from config, got %d", receivedBody.MaxTokens)
	}
}

func TestComplete_NoSystemPrompt(t *testing.T) {
	var receivedBody chatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Content: "ok"}}},
		})
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	_, _ = client.Complete(Request{
		Model:    "test",
		System:   "", // no system prompt
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	// Should only have user message, no system
	for _, m := range receivedBody.Messages {
		if m.Role == "system" {
			t.Error("should not include system message when System is empty")
		}
	}
}

func TestComplete_TrailingSlashEndpoint(t *testing.T) {
	var requestPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []chatChoice{{Message: chatMessage{Content: "ok"}}},
		})
	}))
	defer server.Close()

	cfg := data.DefaultConfig()
	cfg.Model.Endpoint = server.URL + "/" // trailing slash
	cfg.Model.APIKeyEnv = "TEST_TASH_API_KEY"
	t.Setenv("TEST_TASH_API_KEY", "test-key")

	client := NewClient(cfg)
	_, _ = client.Complete(Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})

	if requestPath != "/chat/completions" {
		t.Errorf("expected /chat/completions, got %s (trailing slash not stripped)", requestPath)
	}
}
