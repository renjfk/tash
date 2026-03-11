package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptModel_Enter(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{Command: "ls -la"},
		action:     ActionQuit, // default
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(promptModel)

	if model.action != ActionAccept {
		t.Errorf("expected ActionAccept, got %d", model.action)
	}
	if !model.done {
		t.Error("expected done=true")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestPromptModel_Esc(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{Command: "rm -rf /"},
		action:     ActionAccept,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := updated.(promptModel)

	if model.action != ActionQuit {
		t.Errorf("expected ActionQuit, got %d", model.action)
	}
	if !model.done {
		t.Error("expected done=true")
	}
}

func TestPromptModel_OtherKey(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{Command: "echo hi"},
		action:     ActionQuit,
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(promptModel)

	if model.done {
		t.Error("expected done=false for unrecognized key")
	}
	if cmd != nil {
		t.Error("expected nil command for unrecognized key")
	}
}

func TestPromptModel_CtrlC(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{Command: "test"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(promptModel)

	if model.action != ActionQuit {
		t.Errorf("expected ActionQuit on ctrl+c, got %d", model.action)
	}
}

func TestPromptModel_Init(t *testing.T) {
	m := promptModel{suggestion: &Suggestion{Command: "test"}}
	cmd := m.Init()
	if cmd != nil {
		t.Error("expected nil from Init()")
	}
}
