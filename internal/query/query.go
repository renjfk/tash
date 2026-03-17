package query

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/renjfk/tash/internal/ai"
	"github.com/renjfk/tash/internal/data"
	"github.com/renjfk/tash/internal/tui"
)

// Version is set by the main package at startup to inject the build version into AI context.
var Version = "dev"

// Run executes the query flow. The AI decides whether to respond with
// commands or chat text. Returns the command (if any) for fish to place in the command line.
func Run(cfg *data.Config, prof *data.Profile, convo *data.Conversation, input string) (string, error) {
	if cfg.Behavior.UpdateCheck {
		if info := data.ReadUpdateAvailable(cfg.DataDir(), Version); info != nil {
			fmt.Fprintf(os.Stderr, "%s\n", data.FormatUpdateNotification(info))
		}
	}

	client := ai.NewClient(cfg)
	convo.AddQuery(input)

	spinner := tui.NewSpinner("Thinking")
	defer spinner.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchSpinnerCancel(spinner, cancel)

	// Collect context
	shellHistory := convo.RecentShellCommands()
	systemPrompt := buildSystemPrompt(cfg, prof, convo)

	slog.Debug(
		"query start",
		"input", input,
		"shell_history", len(shellHistory),
		"system_prompt_len", len(systemPrompt),
		"conversation_entries", len(convo.Entries),
	)

	// Inject initial screen context from Zellij (if available)
	var initialConstraints []string
	if screenCtx := initialScreenContext(cfg); screenCtx != "" {
		initialConstraints = append(initialConstraints, screenCtx)
		slog.Debug("initial screen context injected", "len", len(screenCtx))
	}

	// Build multi-turn messages from conversation history
	messages := buildMessages(convo, shellHistory, input, initialConstraints)

	// Attempt loop: tool calls (history, conversation, screen) don't count toward retries.
	// Only real failures (API error, parse error, no terminal response) do.
	// Tool calls are capped to prevent search loops.
	maxToolCalls := cfg.Behavior.MaxToolCalls
	var lastErr error
	constraints := append([]string{}, initialConstraints...)
	failures := 0
	toolCalls := 0
	planStep := 0
	stepsRemaining := 0
	retryReason := ""
	for attempt := 0; failures < cfg.Behavior.MaxRetries; attempt++ {
		if len(constraints) > 0 {
			messages = buildMessages(convo, shellHistory, input, constraints)
		}

		slog.Info(
			"query attempt",
			"attempt", attempt,
			"failures", failures,
			"messages", len(messages),
			"constraints", len(constraints),
		)
		for i, m := range messages {
			slog.Debug("message", "index", i, "role", m.Role, "content", m.Content)
		}

		resp, err := client.Complete(
			ctx,
			ai.Request{
				Model:    cfg.Model.Name,
				System:   systemPrompt,
				Messages: messages,
			},
		)
		if err != nil {
			if ctx.Err() != nil {
				spinner.Stop()
				data.Info("cancelled")
				return "", nil
			}
			slog.Error("API error", "attempt", attempt, "error", err)
			lastErr = err
			failures++
			spinner.SetPhase("Retrying")
			continue
		}

		if err := data.RecordUsage(
			cfg.DataDir(),
			"query",
			cfg.Model.Name,
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
		); err != nil {
			slog.Warn("record usage", "error", err)
		}

		slog.Debug("raw response", "attempt", attempt, "content", resp.Content)

		responses, err := ai.ParseResponse(resp.Content)
		if err != nil {
			slog.Error("parse error", "attempt", attempt, "error", err)
			lastErr = err
			failures++
			spinner.SetPhase("Retrying")
			continue
		}

		slog.Info("response received", "attempt", attempt, "count", len(responses))
		for i, r := range responses {
			slog.Info("response entry", "index", i, "type", r.Type)
			slog.Debug("response detail", "index", i, "type", r.Type, "message", r.Message, "commands", r.Commands)
		}

		// Stop spinner before handling if the response contains terminal output
		if hasTerminalResponse(responses) {
			spinner.Stop()
		}
		reqID := data.NewRequestID()
		result, action := handleResponses(
			responses,
			cfg,
			convo,
			shellHistory,
			&constraints,
			toolCalls >= maxToolCalls,
			&retryReason,
			&stepsRemaining,
			reqID,
			resp.Usage,
		)
		switch action {
		case actionDone:
			slog.Info("query done", "attempt", attempt)
			return result, nil
		case actionPlan:
			planStep++
			slog.Info("plan step", "attempt", attempt, "step", planStep, "remaining", stepsRemaining, "command", result)
			output, exitCode := executePlanStep(result, convo, planStep, planStep+stepsRemaining, reqID, resp.Usage)
			if output == "" && exitCode == -1 {
				// User skipped — abort the plan
				return "", nil
			}
			stepInfo := fmt.Sprintf("Output of step %d ($ %s", planStep, result)
			if exitCode != 0 {
				stepInfo += fmt.Sprintf("  # exit %d", exitCode)
			}
			stepInfo += "):\n" + output
			constraints = append(constraints, stepInfo)
			spinner = tui.NewSpinner("Planning next step")
			watchSpinnerCancel(spinner, cancel)
			continue
		case actionRetry:
			toolCalls++
			slog.Info("query tool call", "attempt", attempt, "tool_calls", toolCalls, "constraints", len(constraints))
			spinner.SetPhase(retryReason)
			continue
		default:
			slog.Warn("no terminal response", "attempt", attempt)
			if toolCalls >= maxToolCalls {
				constraints = append(
					constraints,
					"No more tool calls available (history, screen). Respond with a command or chat based on what you already know.",
				)
			}
			lastErr = fmt.Errorf("no chat or command in response")
			failures++
			spinner.SetPhase("Retrying")
		}
	}

	slog.Error("query exhausted retries", "failures", failures, "last_error", lastErr)
	spinner.Stop()
	return "", fmt.Errorf("could not get a valid response after %d attempts: %v", cfg.Behavior.MaxRetries, lastErr)
}

const (
	actionDone    = iota // terminal response handled, return result
	actionRetry          // need another attempt (history request or tool not found)
	actionNothing        // no terminal response found
	actionPlan           // plan step: run command, capture output, feed back to AI
)

func hasTerminalResponse(responses []ai.TashResponse) bool {
	for _, r := range responses {
		if r.Type == "chat" || r.Type == "command" || r.Type == "plan" {
			return true
		}
	}
	return false
}

// processSideEffects handles tool-call responses (memory, history, conversation, screen)
// and returns true if a retry is needed.
func processSideEffects(
	responses []ai.TashResponse,
	cfg *data.Config,
	convo *data.Conversation,
	shellHistory []data.ShellCommand,
	constraints *[]string,
	skipToolCalls bool,
	retryReason *string,
) bool {
	retry := false
	for _, parsed := range responses {
		switch {
		case parsed.Type == "memory" && parsed.Message != "":
			slog.Debug("storing memory", "content", parsed.Message)
			evicted := convo.AddMemory(parsed.Message)
			if len(evicted) > 0 {
				var b strings.Builder
				b.WriteString("Memory slots full — oldest memories were evicted to make room. Re-store any that are still important:\n")
				for _, m := range evicted {
					fmt.Fprintf(&b, "- %s\n", m)
				}
				*constraints = append(*constraints, b.String())
			}
		case parsed.Type == "history" && !skipToolCalls:
			slog.Debug("history request", "filter", parsed.Filter, "count", parsed.Count)
			constraint := searchContext(cfg, convo, shellHistory, parsed.Filter, parsed.Count)
			*constraints = append(*constraints, constraint)
			*retryReason = "Searching history"
			retry = true
		case parsed.Type == "conversation" && !skipToolCalls:
			slog.Debug("conversation request", "count", parsed.Count)
			constraint := loadMoreConversation(convo, parsed.Count)
			*constraints = append(*constraints, constraint)
			*retryReason = "Loading conversation"
			retry = true
		case parsed.Type == "screen" && !skipToolCalls:
			slog.Debug("screen request", "lines", parsed.Count)
			constraint := captureScreen(cfg, parsed.Count)
			*constraints = append(*constraints, constraint)
			*retryReason = "Reading terminal"
			retry = true
		}
	}
	return retry
}

func handleResponses(
	responses []ai.TashResponse,
	cfg *data.Config,
	convo *data.Conversation,
	shellHistory []data.ShellCommand,
	constraints *[]string,
	skipToolCalls bool,
	retryReason *string,
	stepsRemaining *int,
	reqID string,
	usage ai.Usage,
) (string, int) {
	// Process side-effects first
	retry := processSideEffects(responses, cfg, convo, shellHistory, constraints, skipToolCalls, retryReason)

	// Find the first terminal response
	for _, parsed := range responses {
		if parsed.Type == "chat" {
			convo.AddChatResponse(parsed.Message, reqID, usage.PromptTokens, usage.CompletionTokens)
			showChat(parsed.Message)
			return "", actionDone
		}

		if (parsed.Type == "command" || parsed.Type == "plan") && len(parsed.Commands) > 0 {
			if parsed.Message != "" {
				showChat(parsed.Message)
			}

			// Validate tools exist
			newConstraints := false
			for _, cmd := range parsed.Commands {
				tool := ExtractFirstCommand(cmd)
				slog.Debug("validating tool", "tool", tool, "command", cmd)
				if tool != "" && !CommandExists(tool) {
					slog.Warn("tool not found", "tool", tool)
					*constraints = append(*constraints, tool+" is not installed, use an alternative")
					newConstraints = true
				}
			}
			if newConstraints {
				*retryReason = "Checking alternatives"
				return "", actionRetry
			}

			// Plan type: present first command, execute, capture output
			if parsed.Type == "plan" {
				*stepsRemaining = parsed.StepsRemaining
				slog.Debug("plan step", "commands", len(parsed.Commands), "steps_remaining", parsed.StepsRemaining)
				return parsed.Commands[0], actionPlan
			}

			slog.Debug("presenting commands", "count", len(parsed.Commands))
			result := presentResults(parsed.Commands, convo, reqID, usage)
			return result, actionDone
		}
	}

	if retry {
		return "", actionRetry
	}
	return "", actionNothing
}

// loadMoreConversation loads additional older conversation entries and returns them
// formatted as a constraint string for the AI.
func loadMoreConversation(convo *data.Conversation, count int) string {
	loaded, err := convo.LoadMoreConversation(count)
	if err != nil {
		slog.Warn("load more conversation", "error", err)
		return "Could not load additional conversation entries"
	}

	if loaded == 0 {
		return "No additional conversation history available"
	}

	// Re-format older entries as context
	messages := convo.FormatForAI()
	if len(messages) == 0 {
		return "No additional conversation history available"
	}

	// Return a summary — the full entries are now in the conversation and will
	// appear in the next buildMessages call via FormatForAI()
	return fmt.Sprintf(
		"Loaded %d additional conversation entries (now %d total entries in context)",
		loaded,
		len(convo.Entries),
	)
}

// searchContext merges results from shell history and conversation entries,
// deduplicating against each other and against what's already in the prompt.
func searchContext(
	cfg *data.Config,
	convo *data.Conversation,
	shellHistory []data.ShellCommand,
	filter string,
	count int,
) string {
	historyEntries := data.SearchHistory(cfg.ResolvedHistoryPath(), filter, count, cfg.Behavior.MaxHistoryResults)
	historyFormatted := data.FormatHistory(historyEntries)
	convoResults := convo.Search(filter, count)

	// Seed seen set with content already present in prompt context:
	// shellHistory (recent commands) and conversation turns (query/command/chat).
	// Dedup by raw command to avoid duplicates regardless of timestamp formatting.
	seen := make(map[string]bool)
	for _, cmd := range shellHistory {
		seen[cmd.Command] = true
	}
	for _, e := range convo.Entries {
		if e.Type == "query" || e.Type == "command" || e.Type == "chat" {
			seen[e.Content] = true
		}
	}

	// Merge: conversation entries first (recent context), then history
	var extra []string
	for _, c := range convoResults {
		if !seen[c] {
			seen[c] = true
			extra = append(extra, c)
		}
	}
	for i, entry := range historyEntries {
		if !seen[entry.Command] {
			seen[entry.Command] = true
			extra = append(extra, historyFormatted[i])
		}
	}

	// Cap to requested count
	if len(extra) > count {
		extra = extra[len(extra)-count:]
	}

	if len(extra) == 0 {
		return "No matching history or conversation entries found"
	}

	label := "Additional context"
	if filter != "" {
		label += " (filtered: " + filter + ")"
	}
	return label + ":\n" + strings.Join(extra, "\n")
}

func buildMessages(
	convo *data.Conversation,
	shellHistory []data.ShellCommand,
	currentInput string,
	constraints []string,
) []ai.Message {
	var messages []ai.Message

	// Include prior conversation turns
	for _, m := range convo.FormatForAI() {
		messages = append(messages, ai.Message{Role: m.Role, Content: m.Content})
	}

	// Build the current user message with shell context
	userPrompt := ai.BuildQueryPrompt(currentInput, shellHistory, constraints)

	// If last message is already from user (the AddQuery call), replace it
	if len(messages) > 0 && messages[len(messages)-1].Role == "user" {
		messages[len(messages)-1].Content = userPrompt
	} else {
		messages = append(messages, ai.Message{Role: "user", Content: userPrompt})
	}

	return messages
}

func buildSystemPrompt(cfg *data.Config, prof *data.Profile, convo *data.Conversation) string {
	var b strings.Builder

	b.WriteString(ai.SystemPrompt)

	b.WriteString("\n\n--- Session ---\n")
	fmt.Fprintf(&b, "tash version: %s\n", Version)
	now := time.Now()
	fmt.Fprintf(&b, "Current time: %s (%s)\n", data.FormatTimestamp(now.Unix()), now.Format("MST"))
	if cfg.Terminal.ASCII {
		b.WriteString("Terminal: ASCII-only (TTY/framebuffer console). Do NOT use emojis or Unicode symbols — they will render as garbage. ASCII art is fine.\n")
	}

	if prof != nil {
		b.WriteString("\n--- User Profile ---\n")
		b.WriteString(prof.Content)
	}

	memories := convo.Memories()
	if memories != "" {
		b.WriteString("\n--- Memories ---\n")
		fmt.Fprintf(&b, "Things you've learned about the user from past conversations (%d/%d slots used):\n", convo.MemoryCount(), convo.MaxMemories())
		b.WriteString(memories)
	}

	return b.String()
}

// presentResults returns the command for fish to place in the command line buffer.
// Single commands are returned directly without a prompt.
// Multi-step sequences prompt for intermediate commands, executing them inline,
// and return the last command to fish.
// Token usage is attached to the first command entry only; all entries share reqID.
func presentResults(commands []string, convo *data.Conversation, reqID string, usage ai.Usage) string {
	if len(commands) == 1 {
		// Single command: send straight to fish command line
		convo.AddCommandResponse(commands[0], "accept", reqID, usage.PromptTokens, usage.CompletionTokens)
		return commands[0]
	}

	// Multi-step: prompt for intermediate commands, send last to fish
	for i, cmd := range commands {
		// Token usage only on the first command entry
		pt, ct := 0, 0
		if i == 0 {
			pt, ct = usage.PromptTokens, usage.CompletionTokens
		}

		// Last command: send to fish command line
		if i == len(commands)-1 {
			convo.AddCommandResponse(cmd, "accept", reqID, pt, ct)
			return cmd
		}

		// Intermediate commands: prompt and execute inline
		action := tui.ShowSuggestion(
			&tui.Suggestion{
				Command: cmd,
				StepNum: i + 1,
				Total:   len(commands),
			},
		)

		switch action {
		case tui.ActionAccept:
			traceAction("accept", cmd)
		default:
			traceAction("skip", cmd)
			convo.AddCommandResponse(cmd, "skip", reqID, pt, ct)
			return ""
		}

		convo.AddCommandResponse(cmd, "accept", reqID, pt, ct)
		exitCode := execCommand(cmd)
		convo.AddShellCommand(cmd, exitCode)

		if exitCode != 0 {
			fmt.Fprintf(os.Stderr, "  %s\n", tui.FailStyle.Render(fmt.Sprintf("exit %d", exitCode)))
			cont := tui.ShowSuggestion(
				&tui.Suggestion{
					Command: "continue with remaining steps?",
					StepNum: i + 1,
					Total:   len(commands),
				},
			)
			if cont != tui.ActionAccept {
				return ""
			}
		}
	}

	return ""
}

func traceAction(action string, cmd string) {
	var prefix string
	switch action {
	case "accept":
		prefix = tui.CmdStyle.Render(">")
	case "skip":
		prefix = tui.TraceStyle.Render("x")
	}
	fmt.Fprintf(os.Stderr, "  %s %s\n", prefix, tui.TraceStyle.Render(cmd))
}

// executePlanStep presents a plan command for approval, executes it, and captures output.
// Returns the captured output and exit code. Exit code -1 means user skipped.
// Token usage and reqID are attached to the command entry.
func executePlanStep(
	command string,
	convo *data.Conversation,
	step int,
	total int,
	reqID string,
	usage ai.Usage,
) (string, int) {
	action := tui.ShowSuggestion(
		&tui.Suggestion{
			Command: command,
			StepNum: step,
			Total:   total,
		},
	)

	switch action {
	case tui.ActionAccept:
		traceAction("accept", command)
	default:
		traceAction("skip", command)
		convo.AddCommandResponse(command, "skip", reqID, usage.PromptTokens, usage.CompletionTokens)
		return "", -1
	}

	convo.AddCommandResponse(command, "accept", reqID, usage.PromptTokens, usage.CompletionTokens)
	output, exitCode := execCommandCapture(command)
	convo.AddShellCommand(command, exitCode)

	// Show output to user
	if output != "" {
		fmt.Fprint(os.Stderr, output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	if exitCode != 0 {
		fmt.Fprintf(os.Stderr, "  %s\n", tui.FailStyle.Render(fmt.Sprintf("exit %d", exitCode)))
	}

	slog.Debug("plan step output", "command", command, "exit_code", exitCode, "output_len", len(output))

	// Cap output to avoid blowing up the prompt
	const maxOutputLen = 4096
	if len(output) > maxOutputLen {
		output = output[:maxOutputLen] + "\n... (truncated)"
	}

	return output, exitCode
}

// execCommandCapture runs a command and captures its combined stdout+stderr output.
func execCommandCapture(command string) (string, int) {
	cmd := exec.Command("fish", "-c", command)
	cmd.Stdin = os.Stdin

	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode()
		}
		return string(out), 1
	}
	return string(out), 0
}

// execCommand runs intermediate multi-step commands inline and returns the exit code.
// Stdout goes to stderr so output reaches the terminal when tash's stdout
// is captured by fish command substitution.
func execCommand(command string) int {
	cmd := exec.Command("fish", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// watchSpinnerCancel starts a goroutine that calls cancel when the spinner is
// cancelled by the user (Esc or Ctrl+C). The goroutine exits when either the
// spinner signals cancellation or the cancel function has already been called.
func watchSpinnerCancel(spinner *tui.Spinner, cancel context.CancelFunc) {
	go func() {
		select {
		case <-spinner.Cancelled():
			cancel()
		case <-spinner.Done():
		}
	}()
}

// showChat displays a chat message on stderr.
func showChat(message string) {
	fmt.Fprintf(os.Stderr, "\n  %s\n\n", message)
}
