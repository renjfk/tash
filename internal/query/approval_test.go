package query

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renjfk/tash/internal/data"
	"github.com/renjfk/tash/internal/ntfy"
)

func TestEnsureApproval_Disabled(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	cfg.Approval.Enabled = false

	if err := EnsureApproval(cfg, "list files"); err != nil {
		t.Fatalf("disabled gate should allow usage: %v", err)
	}
}

func TestEnsureApproval_ActiveWindow(t *testing.T) {
	dir := t.TempDir()

	// A server that fails the test if the gate ever reaches the network.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("active window must not touch the network, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)
	cfg.Approval.Enabled = true
	cfg.Approval.Ntfy.Server = srv.URL
	cfg.Approval.Ntfy.Topic = "tash"

	if _, err := data.GrantApproval(dir, "15m", 15*time.Minute, time.Now()); err != nil {
		t.Fatalf("GrantApproval: %v", err)
	}

	if err := EnsureApproval(cfg, "list files"); err != nil {
		t.Fatalf("active window should allow usage without network: %v", err)
	}
}

func TestWaitForApproval_MatchesNonce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"1","time":1,"event":"message","message":"approve:nonce1:30m"}` + "\n"))
	}))
	defer srv.Close()

	cfg := approvalTestConfig(t)
	client := ntfy.New(ntfy.Config{Server: srv.URL, Topic: "tash"})

	tf, active, err := waitForApproval(context.Background(), client, cfg, "nonce1", time.Now(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForApproval: %v", err)
	}
	if active {
		t.Error("expected a matched timeframe, not an external grant")
	}
	if tf != "30m" {
		t.Errorf("expected 30m, got %q", tf)
	}
}

func TestWaitForApproval_AlreadyActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := approvalTestConfig(t)
	if _, err := data.GrantApproval(cfg.DataDir(), "1h", time.Hour, time.Now()); err != nil {
		t.Fatalf("GrantApproval: %v", err)
	}
	client := ntfy.New(ntfy.Config{Server: srv.URL, Topic: "tash"})

	_, active, err := waitForApproval(context.Background(), client, cfg, "nonce1", time.Now(), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForApproval: %v", err)
	}
	if !active {
		t.Error("expected an externally granted window to be detected")
	}
}

func TestWaitForApproval_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := approvalTestConfig(t)
	cfg.Approval.WaitTimeout = 1
	client := ntfy.New(ntfy.Config{Server: srv.URL, Topic: "tash"})

	_, _, err := waitForApproval(context.Background(), client, cfg, "nonce1", time.Now(), 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !strings.Contains(err.Error(), "not granted") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWaitForApproval_AbortsOnRepeatedPollErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := approvalTestConfig(t)
	client := ntfy.New(ntfy.Config{Server: srv.URL, Topic: "tash"})

	_, _, err := waitForApproval(context.Background(), client, cfg, "nonce1", time.Now(), 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected an error after repeated poll failures")
	}
	if !strings.Contains(err.Error(), "approval poll failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWaitForApproval_Cancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := approvalTestConfig(t)
	client := ntfy.New(ntfy.Config{Server: srv.URL, Topic: "tash"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := waitForApproval(ctx, client, cfg, "nonce1", time.Now(), time.Hour)
	if !errors.Is(err, ErrApprovalCancelled) {
		t.Fatalf("expected ErrApprovalCancelled, got %v", err)
	}
}

func TestMatchApproval(t *testing.T) {
	timeframes := []string{"15m", "30m", "1h"}
	messages := []ntfy.Message{
		{Message: "hello"},
		{Message: "approve:wrong:15m"},
		{Message: "approve:abc:2h"},
		{Message: "approve:abc:30m"},
	}

	tf, ok := matchApproval(messages, "abc", timeframes)
	if !ok {
		t.Fatal("expected to match a configured timeframe")
	}
	if tf != "30m" {
		t.Errorf("expected 30m, got %q", tf)
	}

	if _, ok := matchApproval(messages, "nope", timeframes); ok {
		t.Error("wrong nonce should not match")
	}
}

func TestBuildActions(t *testing.T) {
	actions := buildActions([]string{"15m", "1h"}, "nonce1")
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].Label != "15m" || actions[0].Body != "approve:nonce1:15m" {
		t.Errorf("unexpected action: %+v", actions[0])
	}
}

func TestApprovalMessage_IncludesPreview(t *testing.T) {
	msg := approvalMessage("find large files in /var")
	if !strings.Contains(msg, "$ find large files in /var") {
		t.Errorf("expected preview in message, got %q", msg)
	}
	if !strings.Contains(msg, "Tap a timeframe to allow usage.") {
		t.Errorf("expected instruction in message, got %q", msg)
	}
}

func TestApprovalMessage_NoPreview(t *testing.T) {
	msg := approvalMessage("   ")
	if strings.Contains(msg, "$") {
		t.Errorf("expected no preview line for empty input, got %q", msg)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("short", 10); got != "short" {
		t.Errorf("expected unchanged, got %q", got)
	}
	if got := truncateRunes("abcdefghij", 5); got != "abcde..." {
		t.Errorf("expected truncation, got %q", got)
	}
}

func TestGrantTimeframe(t *testing.T) {
	dir := t.TempDir()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(dir)

	if err := grantTimeframe(cfg, "30m"); err != nil {
		t.Fatalf("grantTimeframe: %v", err)
	}
	state, err := data.LoadApprovalState(dir)
	if err != nil {
		t.Fatalf("LoadApprovalState: %v", err)
	}
	if !state.Active(time.Now()) {
		t.Error("expected granted window to be active")
	}

	if err := grantTimeframe(cfg, "banana"); err == nil {
		t.Error("expected error for invalid timeframe")
	}
}

// approvalTestConfig returns an enabled approval config with a temp data dir.
func approvalTestConfig(t *testing.T) *data.Config {
	t.Helper()
	cfg := data.DefaultConfig()
	cfg.SetDataDir(t.TempDir())
	cfg.Approval.Enabled = true
	cfg.Approval.Timeframes = []string{"15m", "30m", "1h"}
	cfg.Approval.WaitTimeout = 60
	cfg.Approval.Ntfy.Topic = "tash"
	return cfg
}
