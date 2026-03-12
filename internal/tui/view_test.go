package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFace_DefaultExpression(t *testing.T) {
	// frame 0 with a known seed should produce a valid face
	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 0

	f := face(0)
	if f == "" {
		t.Error("face(0) returned empty string")
	}
	// Should be one of the known faces
	knownFaces := map[string]bool{
		"(◕‿◕)": true,
		"(◐‿◐)": true,
		"(◑‿◑)": true,
		"(◔‿◔)": true,
		"(-‿-)": true,
		"(◡‿◡)": true,
	}
	if !knownFaces[f] {
		t.Errorf("face(0) returned unknown face: %q", f)
	}
}

func TestFace_SleepyAfter200(t *testing.T) {
	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	f := face(201)
	// After frame 200, should return sleepy face unless in a blink
	if f != "(◡‿◡)" && f != "(-‿-)" {
		t.Errorf("face(201) = %q, expected sleepy or blink", f)
	}
}

func TestFace_BlinkDetection(t *testing.T) {
	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 12345

	// Scan first 200 frames, should find at least one blink
	blinks := 0
	for i := 0; i < 200; i++ {
		if face(i) == "(-‿-)" {
			blinks++
		}
	}
	if blinks == 0 {
		t.Error("expected at least one blink in first 200 frames")
	}
}

func TestFace_VariesOverTime(t *testing.T) {
	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 99

	// Collect faces over range of frames, should see variety
	faces := make(map[string]bool)
	for i := 0; i < 150; i++ {
		faces[face(i)] = true
	}
	if len(faces) < 2 {
		t.Errorf("expected face variety over 150 frames, got %d unique faces", len(faces))
	}
}

func TestSpinnerModel_Update_Tick(t *testing.T) {
	m := spinnerModel{phase: "Thinking", start: time.Now()}

	updated, cmd := m.Update(tickMsg(time.Now()))
	model := updated.(spinnerModel)

	if model.frame != 1 {
		t.Errorf("expected frame 1 after tick, got %d", model.frame)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (next tick)")
	}
}

func TestSpinnerModel_Update_TickWraps(t *testing.T) {
	m := spinnerModel{frame: totalFrames() - 1, start: time.Now()}

	updated, _ := m.Update(tickMsg(time.Now()))
	model := updated.(spinnerModel)

	if model.frame != 0 {
		t.Errorf("expected frame to wrap to 0, got %d", model.frame)
	}
	if model.cycle != 1 {
		t.Errorf("expected cycle 1, got %d", model.cycle)
	}
}

func TestSpinnerModel_Update_Phase(t *testing.T) {
	m := spinnerModel{phase: "Thinking", start: time.Now()}

	updated, cmd := m.Update(phaseMsg("Searching history"))
	model := updated.(spinnerModel)

	if model.phase != "Searching history" {
		t.Errorf("expected phase 'Searching history', got %q", model.phase)
	}
	if cmd != nil {
		t.Error("expected nil cmd from phase update")
	}
}

func TestSpinnerModel_Update_Done(t *testing.T) {
	m := spinnerModel{phase: "Thinking", start: time.Now()}

	updated, cmd := m.Update(doneMsg{})
	model := updated.(spinnerModel)

	if !model.quitting {
		t.Error("expected quitting=true")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestSpinnerModel_Update_CtrlC(t *testing.T) {
	m := spinnerModel{phase: "Thinking", start: time.Now()}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(spinnerModel)

	if !model.quitting {
		t.Error("expected quitting=true on ctrl+c")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestSpinnerModel_Update_OtherKey(t *testing.T) {
	m := spinnerModel{phase: "Thinking", start: time.Now()}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(spinnerModel)

	if model.quitting {
		t.Error("should not quit on regular key")
	}
	if cmd != nil {
		t.Error("expected nil cmd for unrecognized key")
	}
}

func TestSpinnerModel_View_Quitting(t *testing.T) {
	m := spinnerModel{quitting: true}
	if m.View() != "" {
		t.Error("expected empty view when quitting")
	}
}

func TestSpinnerModel_View_Normal(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("solarized", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	m := spinnerModel{
		phase: "Thinking",
		frame: 0,
		start: time.Now(),
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
	// Should contain the phase text
	if !strings.Contains(view, "Thinking") {
		t.Error("view should contain phase text")
	}
}

func TestSpinnerModel_View_WithElapsed(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("solarized", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	m := spinnerModel{
		phase: "Thinking",
		frame: 0,
		start: time.Now().Add(-2 * time.Second), // 2s ago
	}

	view := m.View()
	// Should contain elapsed time (> 1s triggers display)
	if !strings.Contains(view, "s") {
		t.Error("view should contain elapsed time for >1s durations")
	}
}

func TestSpinnerModel_Init(t *testing.T) {
	m := spinnerModel{phase: "test"}
	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init (tick)")
	}
}

func TestPromptModel_View_SingleStep(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{Command: "ls -la"},
	}

	view := m.View()
	if !strings.Contains(view, "ls -la") {
		t.Error("view should contain command")
	}
	if !strings.Contains(view, "Accept") {
		t.Error("view should contain Accept hint")
	}
	if !strings.Contains(view, "Cancel") {
		t.Error("view should contain Cancel hint")
	}
}

func TestPromptModel_View_MultiStep(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{
			Command: "mkdir -p src",
			StepNum: 1,
			Total:   3,
		},
	}

	view := m.View()
	if !strings.Contains(view, "1/3") {
		t.Error("view should contain step counter")
	}
	if !strings.Contains(view, "mkdir -p src") {
		t.Error("view should contain command")
	}
}

func TestPromptModel_View_Done(t *testing.T) {
	m := promptModel{
		suggestion: &Suggestion{Command: "test"},
		done:       true,
	}

	view := m.View()
	if view != "" {
		t.Error("expected empty view when done")
	}
}

func TestSpinnerModel_View_DuringHold(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("solarized", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	// During right hold (dotCount frames into animation)
	m := spinnerModel{
		phase: "Working",
		frame: dotCount + 3, // during holdEnd
		start: time.Now(),
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view during hold")
	}
}

func TestSpinnerModel_View_DuringBackward(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("solarized", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	// During backward movement
	m := spinnerModel{
		phase: "Working",
		frame: dotCount + holdEnd + 2, // backward movement
		start: time.Now(),
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view during backward")
	}
}

func TestSpinnerModel_View_DuringLeftHold(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("solarized", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	// During left hold
	m := spinnerModel{
		phase: "Working",
		frame: dotCount + holdEnd + (dotCount - 1) + 5,
		start: time.Now(),
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view during left hold")
	}
}

func TestSpinnerModel_View_MultipleCycles(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("dracula", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	// Multiple cycles - absFrame should be cycle*totalFrames + frame
	m := spinnerModel{
		phase: "Still working",
		frame: 3,
		cycle: 5,
		start: time.Now().Add(-5 * time.Second),
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for multi-cycle")
	}
	if !strings.Contains(view, "Still working") {
		t.Error("view should contain phase text")
	}
}

func TestSpinnerModel_View_MoveTotal1(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()
	activeTheme = resolveTheme("solarized", "")

	old := faceSeed
	defer func() { faceSeed = old }()
	faceSeed = 42

	// Frame 1 - during forward movement, moveTotal > 1
	m := spinnerModel{
		phase: "Thinking",
		frame: 1,
		start: time.Now(),
	}

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
