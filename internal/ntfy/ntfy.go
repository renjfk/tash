// Package ntfy is a minimal client for ntfy (https://ntfy.sh or any
// self-hosted instance). It publishes approval requests and polls a reply
// topic for one-click approval taps.
package ntfy

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultServer = "https://ntfy.sh"
	replySuffix   = "-reply"
	maxTopicLen   = 64
)

// Config holds the connection settings for an ntfy client.
type Config struct {
	Server       string // ntfy server base URL; defaults to https://ntfy.sh
	Topic        string // topic approval requests are published to
	ReplyTopic   string // topic button taps publish to; defaults to <topic>-reply
	Token        string // access token used to publish and poll, optional
	ActionsToken string // restricted token embedded in action buttons; defaults to Token
}

// Action is a single ntfy action button.
type Action struct {
	Label string
	Body  string
}

// Message is one entry returned by Poll.
type Message struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"`
	Event   string `json:"event"`
	Message string `json:"message"`
}

// Client publishes approval requests and polls for button taps.
type Client struct {
	cfg  Config
	http *http.Client
}

// New creates a client, normalizing the server URL and filling defaults.
func New(cfg Config) *Client {
	if strings.TrimSpace(cfg.Server) == "" {
		cfg.Server = defaultServer
	}
	cfg.Server = strings.TrimRight(cfg.Server, "/")
	return &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
}

// ReplyURL returns the web URL of the reply topic, used as the notification click target.
func (c *Client) ReplyURL() string {
	return c.cfg.Server + "/" + c.replyTopic()
}

// Send publishes a notification to the request topic. When actions are given,
// each becomes a one-click button that POSTs its body to the reply topic.
func (c *Client) Send(ctx context.Context, title, message string, priority int, actions []Action) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.Server+"/"+c.cfg.Topic, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("ntfy request: %w", err)
	}
	req.Header.Set("Title", encodeHeaderValue(title))
	if priority > 0 {
		req.Header.Set("Priority", strconv.Itoa(priority))
	}
	if len(actions) > 0 {
		req.Header.Set("Actions", encodeHeaderValue(c.actionsHeader(actions)))
	}
	req.Header.Set("Click", encodeHeaderValue(c.ReplyURL()))
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy send: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Poll fetches messages published to the reply topic after the given cursor and
// returns them with the newest cursor. An empty cursor fetches the backlog.
func (c *Client) Poll(ctx context.Context, since string) ([]Message, string, error) {
	target := c.cfg.Server + "/" + c.replyTopic() + "/json?poll=1"
	if since != "" {
		target += "&since=" + url.QueryEscape(since)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, since, fmt.Errorf("ntfy poll request: %w", err)
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, since, fmt.Errorf("ntfy poll: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, since, fmt.Errorf("ntfy poll: HTTP %d", resp.StatusCode)
	}

	var messages []Message
	last := since
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID != "" {
			last = msg.ID
		}
		if msg.Event == "message" {
			messages = append(messages, msg)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, last, fmt.Errorf("ntfy poll: %w", err)
	}
	return messages, last, nil
}

// replyTopic returns the configured reply topic, or a derived <topic>-reply.
func (c *Client) replyTopic() string {
	if c.cfg.ReplyTopic != "" {
		return c.cfg.ReplyTopic
	}
	return deriveReplyTopic(c.cfg.Topic)
}

func (c *Client) actionsToken() string {
	if c.cfg.ActionsToken != "" {
		return c.cfg.ActionsToken
	}
	return c.cfg.Token
}

// actionsHeader serializes action buttons per the ntfy Actions header format.
// Each button POSTs its body to the reply topic and clears the notification.
func (c *Client) actionsHeader(actions []Action) string {
	target := c.cfg.Server + "/" + c.replyTopic()
	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		fields := []string{
			"http",
			quoteActionValue(action.Label),
			target,
			"body=" + quoteActionValue(action.Body),
			"clear=true",
		}
		if token := c.actionsToken(); token != "" {
			fields = append(fields, "headers.Authorization=Bearer "+token)
		}
		parts = append(parts, strings.Join(fields, ", "))
	}
	return strings.Join(parts, "; ")
}

// deriveReplyTopic builds a companion topic for button taps, truncated to the
// ntfy topic length limit.
func deriveReplyTopic(topic string) string {
	if len(topic)+len(replySuffix) > maxTopicLen {
		topic = topic[:maxTopicLen-len(replySuffix)]
	}
	return topic + replySuffix
}

// quoteActionValue strips quotes and wraps the value when it contains the
// action-list separators, per the ntfy docs.
func quoteActionValue(value string) string {
	text := strings.ReplaceAll(value, "\"", "")
	if strings.ContainsAny(text, ",;") {
		return "\"" + text + "\""
	}
	return text
}

// encodeHeaderValue encodes non-ASCII header values as RFC 2047 encoded words,
// which ntfy understands. Plain ASCII is returned unchanged.
func encodeHeaderValue(value string) string {
	for i := 0; i < len(value); i++ {
		if value[i] > 127 {
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(value)) + "?="
		}
	}
	return value
}
