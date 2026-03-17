package tui

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Action represents the user's choice in the suggestion prompt.
type Action int

const (
	ActionAccept Action = iota // Enter - accept
	ActionQuit                 // Esc/q/Ctrl+C - quit
)

// Suggestion represents a command suggestion for multi-step approval prompts.
type Suggestion struct {
	Command string
	StepNum int // step number within multi-step sequence
	Total   int // total steps
}

// stderrRenderer is used for all TUI styles so colors work when stdout
// is captured by fish command substitution.
var stderrRenderer = lipgloss.NewRenderer(os.Stderr)

// Styles shared across TUI components.
var (
	CmdStyle   = stderrRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	WarnStyle  = stderrRenderer.NewStyle().Foreground(lipgloss.Color("3"))
	FailStyle  = stderrRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	TraceStyle = stderrRenderer.NewStyle().Faint(true)
	hintStyle  = stderrRenderer.NewStyle().Faint(true)
	stepStyle  = stderrRenderer.NewStyle().Faint(true)
)

type promptModel struct {
	suggestion *Suggestion
	action     Action
	done       bool
}

func (m promptModel) Init() tea.Cmd {
	return nil
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.action = ActionAccept
			m.done = true
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.action = ActionQuit
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m promptModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	if m.suggestion.StepNum > 0 {
		b.WriteString(stepStyle.Render(fmt.Sprintf("  (%d/%d)", m.suggestion.StepNum, m.suggestion.Total)))
		b.WriteString("\n")
	}

	b.WriteString("  ")
	b.WriteString(CmdStyle.Render(m.suggestion.Command))
	b.WriteString("\n\n")

	b.WriteString(hintStyle.Render("  [Enter] Accept  [Esc] Cancel"))

	return b.String()
}

// ShowSuggestion displays a command suggestion and waits for user input.
// Used for multi-step intermediate commands and plan steps. All UI goes to stderr.
func ShowSuggestion(suggestion *Suggestion) Action {
	m := promptModel{suggestion: suggestion, action: ActionQuit}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	result, err := p.Run()
	drainStdin()
	if err != nil {
		return ActionQuit
	}

	return result.(promptModel).action
}

// drainStdin reads and discards any stale bytes from stdin.
// Bubbletea/lipgloss queries the terminal for background color (OSC 11) via
// stdin, and the response may arrive after the TUI exits. Without draining,
// the leftover escape sequence leaks into the shell prompt.
func drainStdin() {
	// Use syscall-level non-blocking read to drain without hanging
	if err := syscall.SetNonblock(int(os.Stdin.Fd()), true); err != nil {
		return
	}
	buf := make([]byte, 256)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
	}
	_ = syscall.SetNonblock(int(os.Stdin.Fd()), false)
}
