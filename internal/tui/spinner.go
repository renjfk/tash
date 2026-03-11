package tui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	dotCount  = 8
	trailLen  = 6
	holdEnd   = 9
	holdStart = 30
	interval  = 40 * time.Millisecond

	activeChar   = "■"
	inactiveChar = "⬝"
)

var (
	faceStyle    = stderrRenderer.NewStyle().Foreground(lipgloss.Color("#859900"))
	phaseStyle   = stderrRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("#859900"))
	elapsedStyle = stderrRenderer.NewStyle().Faint(true)
)

// Faces is the shared set of tash companion faces used in greetings and spinner.
var Faces = []string{
	"(◕‿◕)", // default
	"(◐‿◐)", // look right
	"(◑‿◑)", // look left
	"(◔‿◔)", // look up
}

var glances = []string{
	"(◐‿◐)", // look right
	"(◑‿◑)", // look left
	"(◔‿◔)", // look up
}

// faceSeed is set at process start so each invocation produces a different sequence.
var faceSeed = int(time.Now().UnixNano() & 0x7fffffff)

// FaceHash returns a cheap pseudo-random number from an integer value.
func FaceHash(n int) int {
	n = ((n >> 16) ^ n) * 0x45d9f3b
	n = ((n >> 16) ^ n) * 0x45d9f3b
	n = (n >> 16) ^ n
	if n < 0 {
		return -n
	}
	return n
}

// face returns the tash companion face based on a global frame counter.
// Blinks at irregular intervals, changes expression every 1-3s, sleepy after ~8s.
func face(frame int) string {
	// Blink check — walk through variable-length blink intervals.
	// Each interval is 30-80 frames (1.2-3.2s), blink lasts 3 frames (~120ms).
	blinkPos := 0
	blinkSeg := 0
	for blinkPos <= frame {
		h := FaceHash(blinkSeg*7 + faceSeed)
		gap := 25 + h%25 // 25..49 frames between blinks (1-2s)
		if frame >= blinkPos+gap && frame < blinkPos+gap+3 {
			return "(-‿-)"
		}
		blinkPos += gap + 3
		blinkSeg++
	}

	// After ~8s go sleepy
	if frame > 200 {
		return "(◡‿◡)"
	}

	// Expression changes — walk through variable-length segments.
	// Each segment is 25-75 frames (1-3s), picks a glance or default.
	pos := 0
	seg := 0
	for pos <= frame {
		h := FaceHash(seg + faceSeed)
		segLen := 25 + h%50
		if frame < pos+segLen {
			idx := (h / 50) % (len(glances) + 1)
			if idx < len(glances) {
				return glances[idx]
			}
			return "(◕‿◕)"
		}
		pos += segLen
		seg++
	}

	return "(◕‿◕)"
}

// phaseMsg updates the spinner phase label.
type phaseMsg string

// doneMsg tells the spinner to quit.
type doneMsg struct{}

// tickMsg drives the animation forward.
type tickMsg time.Time

// scannerState holds the computed state for a single frame.
// holdProgress and holdTotal are integer frame counts (not normalized),
// matching the opencode implementation where holdProgress is used as a
// direct additive shift to fade the trail off-screen during holds.
type scannerState struct {
	activePos    int
	isHolding    bool
	holdProgress int
	holdTotal    int
	moveProgress int
	moveTotal    int
	forward      bool
}

// totalFrames calculates the full bidirectional cycle length.
// Forward (dotCount) + holdEnd + backward (dotCount-1) + holdStart
func totalFrames() int {
	return dotCount + holdEnd + (dotCount - 1) + holdStart
}

// getScannerState returns the scanner position and direction for a given frame.
func getScannerState(frame int) scannerState {
	fwd := dotCount
	bwd := dotCount - 1

	// Moving forward: head goes 0..dotCount-1
	if frame < fwd {
		return scannerState{
			activePos:    frame,
			forward:      true,
			moveProgress: frame,
			moveTotal:    fwd,
		}
	}
	frame -= fwd

	// Holding at right end
	if frame < holdEnd {
		return scannerState{
			activePos:    dotCount - 1,
			isHolding:    true,
			holdProgress: frame,
			holdTotal:    holdEnd,
			forward:      true,
		}
	}
	frame -= holdEnd

	// Moving backward: head goes dotCount-2..0
	if frame < bwd {
		return scannerState{
			activePos:    dotCount - 2 - frame,
			forward:      false,
			moveProgress: frame,
			moveTotal:    bwd,
		}
	}
	frame -= bwd

	// Holding at left end
	return scannerState{
		activePos:    0,
		isHolding:    true,
		holdProgress: frame,
		holdTotal:    holdStart,
		forward:      false,
	}
}

// colorIndex returns the trail color index for a character at charIdx.
// 0 = head (brightest), 1..trailLen-1 = trail, -1 = inactive.
// During holds the index is shifted by holdProgress so the trail fades
// off-screen one step per hold frame -- exactly matching the opencode behavior.
func colorIndex(charIdx int, s scannerState) int {
	var dist int
	if s.forward {
		dist = s.activePos - charIdx
	} else {
		dist = charIdx - s.activePos
	}

	if s.isHolding {
		return dist + s.holdProgress
	}

	if dist == 0 {
		return 0
	}
	if dist > 0 && dist < trailLen {
		return dist
	}
	return -1
}

func clamp8(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v)
}

func rgbHex(r, g, b int) string {
	const hex = "0123456789abcdef"
	return "#" + string(
		[]byte{
			hex[r>>4], hex[r&0x0f],
			hex[g>>4], hex[g&0x0f],
			hex[b>>4], hex[b&0x0f],
		},
	)
}

type spinnerModel struct {
	phase    string
	frame    int
	cycle    int
	start    time.Time
	quitting bool
}

func (m spinnerModel) Init() tea.Cmd {
	return tick()
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.frame++
		if m.frame >= totalFrames() {
			m.frame = 0
			m.cycle++
		}
		return m, tick()
	case phaseMsg:
		m.phase = string(msg)
		return m, nil
	case doneMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}

	s := getScannerState(m.frame)

	// Inactive dot fading: fade out during hold, fade in during movement
	const minAlpha = 0.3
	var fadeFactor float64
	if s.isHolding && s.holdTotal > 0 {
		progress := math.Min(float64(s.holdProgress)/float64(s.holdTotal), 1.0)
		fadeFactor = math.Max(minAlpha, 1.0-progress*(1.0-minAlpha))
	} else if !s.isHolding && s.moveTotal > 0 {
		denom := s.moveTotal - 1
		if denom < 1 {
			denom = 1
		}
		progress := math.Min(float64(s.moveProgress)/float64(denom), 1.0)
		fadeFactor = minAlpha + progress*(1.0-minAlpha)
	} else {
		fadeFactor = 1.0
	}

	inactiveCol := lipgloss.Color(ThemeInactiveHex(fadeFactor))

	var b strings.Builder
	absFrame := m.cycle*totalFrames() + m.frame
	b.WriteString(faceStyle.Render(face(absFrame)))
	b.WriteString(" ")
	for i := 0; i < dotCount; i++ {
		idx := colorIndex(i, s)
		if idx >= 0 && idx < trailLen {
			b.WriteString(stderrRenderer.NewStyle().Foreground(lipgloss.Color(activeTheme.TrailHex[idx])).Render(activeChar))
		} else {
			b.WriteString(stderrRenderer.NewStyle().Foreground(inactiveCol).Render(inactiveChar))
		}
	}

	b.WriteString(" ")
	b.WriteString(phaseStyle.Render(m.phase))

	elapsed := time.Since(m.start)
	if elapsed > time.Second {
		b.WriteString(elapsedStyle.Render(fmt.Sprintf(" %.1fs", elapsed.Seconds())))
	}

	return b.String()
}

func tick() tea.Cmd {
	return tea.Tick(
		interval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		},
	)
}

// Spinner wraps a bubbletea program driving a Knight Rider style animation on stderr.
type Spinner struct {
	prog *tea.Program
	done chan struct{}
}

// NewSpinner creates and starts a spinner with the given initial phase.
func NewSpinner(phase string) *Spinner {
	m := spinnerModel{
		phase: phase,
		start: time.Now(),
	}

	prog := tea.NewProgram(
		m,
		tea.WithOutput(os.Stderr),
		tea.WithoutSignalHandler(),
	)

	sp := &Spinner{
		prog: prog,
		done: make(chan struct{}),
	}

	go func() {
		defer close(sp.done)
		_, _ = prog.Run()
	}()

	return sp
}

// SetPhase updates the displayed phase label.
func (s *Spinner) SetPhase(phase string) {
	s.prog.Send(phaseMsg(phase))
}

// Stop halts the spinner and clears output.
func (s *Spinner) Stop() {
	s.prog.Send(doneMsg{})
	<-s.done
}
