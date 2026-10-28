package query

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/renjfk/tash/internal/data"
	"github.com/renjfk/tash/internal/ntfy"
	"github.com/renjfk/tash/internal/tui"
)

const (
	// ntfy refills requests at roughly one per 5s, so polling any faster risks 429.
	approvalPollInterval  = 5 * time.Second
	approvalMaxPollErrors = 3
	approvalBodyPrefix    = "approve"
	approvalPreviewMax    = 200
)

// ErrApprovalCancelled signals that the user cancelled the approval wait.
var ErrApprovalCancelled = fmt.Errorf("approval cancelled")

// EnsureApproval enforces the Screen Time-style usage gate. When no approval
// window is active it pushes an ntfy notification with one-click timeframe
// buttons and waits for a tap, granting the chosen timeframe on approval.
func EnsureApproval(cfg *data.Config, input string) error {
	if !cfg.Approval.Enabled {
		return nil
	}

	state, err := data.LoadApprovalState(cfg.DataDir())
	if err != nil {
		slog.Warn("load approval state", "error", err)
	}
	now := time.Now()
	if state.Active(now) {
		slog.Debug("approval window active", "remaining", state.Remaining(now))
		return nil
	}

	return requestApproval(cfg, input)
}

// requestApproval sends the ntfy request and blocks until an approver taps a
// timeframe, the wait timeout elapses, or the user cancels.
func requestApproval(cfg *data.Config, input string) error {
	ac := cfg.Approval

	client := ntfy.New(ntfy.Config{
		Server:       ac.Ntfy.Server,
		Topic:        ac.Ntfy.Topic,
		ReplyTopic:   ac.Ntfy.ReplyTopic,
		Token:        envValue(ac.Ntfy.TokenEnv),
		ActionsToken: envValue(ac.Ntfy.ActionsTokenEnv),
	})

	nonce := data.NewRequestID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Send(ctx, "tash usage request", approvalMessage(input), 4, buildActions(ac.Timeframes, nonce)); err != nil {
		return fmt.Errorf("request approval: %w", err)
	}
	data.Info(fmt.Sprintf("approval requested on ntfy topic %q", ac.Ntfy.Topic))

	spinner := tui.NewSpinner("Waiting for approval")
	watchSpinnerCancel(spinner, cancel)
	timeframe, active, err := waitForApproval(ctx, client, cfg, nonce, time.Now(), approvalPollInterval)
	spinner.Stop()

	switch {
	case err != nil:
		return err
	case active:
		slog.Info("approval window opened elsewhere")
		return nil
	default:
		return grantTimeframe(cfg, timeframe)
	}
}

// waitForApproval polls the reply topic until it sees an approval carrying the
// request nonce, observes a window granted elsewhere, hits the wait deadline,
// or the context is cancelled.
func waitForApproval(
	ctx context.Context,
	client *ntfy.Client,
	cfg *data.Config,
	nonce string,
	start time.Time,
	interval time.Duration,
) (string, bool, error) {
	deadline := start.Add(time.Duration(cfg.Approval.WaitTimeout) * time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	since := ""
	pollErrors := 0
	for {
		messages, last, err := client.Poll(ctx, since)
		if err != nil {
			if ctx.Err() != nil {
				return "", false, ErrApprovalCancelled
			}
			pollErrors++
			slog.Warn("approval poll", "error", err, "consecutive", pollErrors)
			if pollErrors >= approvalMaxPollErrors {
				return "", false, fmt.Errorf("approval poll failed: %w", err)
			}
		} else {
			pollErrors = 0
			since = last
			if tf, ok := matchApproval(messages, nonce, cfg.Approval.Timeframes); ok {
				return tf, false, nil
			}
			if state, err := data.LoadApprovalState(cfg.DataDir()); err == nil && state.Active(time.Now()) {
				return "", true, nil
			}
		}

		if !time.Now().Before(deadline) {
			return "", false, fmt.Errorf("approval not granted within %ds", cfg.Approval.WaitTimeout)
		}

		select {
		case <-ctx.Done():
			return "", false, ErrApprovalCancelled
		case <-ticker.C:
		}
	}
}

// buildActions creates one-click timeframe buttons, each carrying a body the
// reply poll can attribute to this request through the nonce.
func buildActions(timeframes []string, nonce string) []ntfy.Action {
	actions := make([]ntfy.Action, 0, len(timeframes))
	for _, tf := range timeframes {
		actions = append(actions, ntfy.Action{
			Label: tf,
			Body:  approvalBodyPrefix + ":" + nonce + ":" + tf,
		})
	}
	return actions
}

// grantTimeframe parses and persists an approved timeframe. The caller must
// stop any active spinner first so the message is not interleaved.
func grantTimeframe(cfg *data.Config, timeframe string) error {
	d, err := time.ParseDuration(timeframe)
	if err != nil || d <= 0 {
		return fmt.Errorf("invalid approved timeframe %q", timeframe)
	}
	state, err := data.GrantApproval(cfg.DataDir(), timeframe, d, time.Now())
	if err != nil {
		return fmt.Errorf("grant approval: %w", err)
	}
	slog.Info("usage approved", "timeframe", timeframe, "until", state.AllowedUntil)
	data.Info(fmt.Sprintf("usage approved for %s (until %s)", timeframe, time.Unix(state.AllowedUntil, 0).Format("15:04")))
	return nil
}

// matchApproval returns the timeframe from a reply whose nonce matches and
// whose timeframe is one of the configured options.
func matchApproval(messages []ntfy.Message, nonce string, timeframes []string) (string, bool) {
	for _, msg := range messages {
		parts := strings.SplitN(strings.TrimSpace(msg.Message), ":", 3)
		if len(parts) != 3 || parts[0] != approvalBodyPrefix || parts[1] != nonce {
			continue
		}
		for _, tf := range timeframes {
			if parts[2] == tf {
				return tf, true
			}
		}
	}
	return "", false
}

// approvalMessage builds the notification body shown to approvers.
func approvalMessage(input string) string {
	var b strings.Builder
	b.WriteString("tash usage requested")
	if who := requestor(); who != "" {
		b.WriteString(" by " + who)
	}
	b.WriteString(".\n\n")

	if preview := strings.TrimSpace(input); preview != "" {
		b.WriteString("$ " + truncateRunes(preview, approvalPreviewMax))
		b.WriteString("\n\n")
	}

	b.WriteString("Tap a timeframe to allow usage.")
	return b.String()
}

// requestor returns a best-effort user@host label for the notification.
func requestor() string {
	user := os.Getenv("USER")
	host, _ := os.Hostname()
	switch {
	case user != "" && host != "":
		return user + "@" + host
	case host != "":
		return host
	default:
		return user
	}
}

// truncateRunes shortens s to at most max runes, appending an ellipsis.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// envValue reads an environment variable, tolerating an empty name.
func envValue(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}
