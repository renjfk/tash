package ntfy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSend_PublishesWithActions(t *testing.T) {
	var gotPath, gotTitle, gotActions, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("Title")
		gotActions = r.Header.Get("Actions")
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{
		Server: srv.URL,
		Topic:  "tash-approval",
		Token:  "secret",
	})

	err := client.Send(context.Background(), "tash usage request", "please approve", 4, []Action{
		{Label: "15m", Body: "approve:abc:15m"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotPath != "/tash-approval" {
		t.Errorf("expected path /tash-approval, got %q", gotPath)
	}
	if gotTitle != "tash usage request" {
		t.Errorf("unexpected title %q", gotTitle)
	}
	if gotBody != "please approve" {
		t.Errorf("unexpected body %q", gotBody)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("unexpected auth %q", gotAuth)
	}
	for _, want := range []string{"http", srv.URL + "/tash-approval-reply", "body=approve:abc:15m", "clear=true"} {
		if !strings.Contains(gotActions, want) {
			t.Errorf("actions header %q missing %q", gotActions, want)
		}
	}
}

func TestSend_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := New(Config{Server: srv.URL, Topic: "t"})
	if err := client.Send(context.Background(), "", "", 0, nil); err == nil {
		t.Fatal("expected error for HTTP 401")
	}
}

func TestPoll_ParsesMessagesAndCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tash-approval-reply/json" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("poll") != "1" {
			t.Errorf("expected poll=1, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(
			`{"id":"1","time":100,"event":"message","message":"approve:abc:15m"}` + "\n" +
				`{"id":"2","time":101,"event":"open","message":""}` + "\n" +
				"garbage-not-json\n",
		))
	}))
	defer srv.Close()

	client := New(Config{Server: srv.URL, Topic: "tash-approval"})
	messages, last, err := client.Poll(context.Background(), "")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Message != "approve:abc:15m" {
		t.Errorf("unexpected message %q", messages[0].Message)
	}
	if last != "2" {
		t.Errorf("expected cursor 2, got %q", last)
	}
}

func TestPoll_PassesSinceCursor(t *testing.T) {
	var gotSince string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("since")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(Config{Server: srv.URL, Topic: "t"})
	if _, _, err := client.Poll(context.Background(), "42"); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if gotSince != "42" {
		t.Errorf("expected since=42, got %q", gotSince)
	}
}

func TestActionsHeader_Multiple(t *testing.T) {
	client := New(Config{Server: "https://ntfy.sh", Topic: "tash", ActionsToken: "tok"})
	got := client.actionsHeader([]Action{
		{Label: "15m", Body: "approve:a:15m"},
		{Label: "1h", Body: "approve:a:1h"},
	})
	want := "http, 15m, https://ntfy.sh/tash-reply, body=approve:a:15m, clear=true, headers.Authorization=Bearer tok; " +
		"http, 1h, https://ntfy.sh/tash-reply, body=approve:a:1h, clear=true, headers.Authorization=Bearer tok"
	if got != want {
		t.Errorf("actions header mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestReplyTopic(t *testing.T) {
	if got := New(Config{Topic: "tash"}).replyTopic(); got != "tash-reply" {
		t.Errorf("expected tash-reply, got %q", got)
	}
	if got := New(Config{Topic: "tash", ReplyTopic: "custom"}).replyTopic(); got != "custom" {
		t.Errorf("expected custom, got %q", got)
	}
}

func TestDeriveReplyTopic_Truncates(t *testing.T) {
	long := strings.Repeat("a", 64)
	got := deriveReplyTopic(long)
	if len(got) != maxTopicLen {
		t.Errorf("expected length %d, got %d", maxTopicLen, len(got))
	}
	if !strings.HasSuffix(got, replySuffix) {
		t.Errorf("expected suffix %q, got %q", replySuffix, got)
	}
}

func TestEncodeHeaderValue(t *testing.T) {
	if got := encodeHeaderValue("plain ascii"); got != "plain ascii" {
		t.Errorf("ascii should be unchanged, got %q", got)
	}
	if got := encodeHeaderValue("héllo"); !strings.HasPrefix(got, "=?UTF-8?B?") {
		t.Errorf("non-ascii should be encoded, got %q", got)
	}
}

func TestQuoteActionValue(t *testing.T) {
	if got := quoteActionValue("15m"); got != "15m" {
		t.Errorf("expected 15m, got %q", got)
	}
	if got := quoteActionValue("a,b"); got != `"a,b"` {
		t.Errorf("expected quoted, got %q", got)
	}
	if got := quoteActionValue(`a"b`); got != "ab" {
		t.Errorf("expected quotes stripped, got %q", got)
	}
}
