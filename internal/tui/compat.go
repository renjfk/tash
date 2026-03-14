package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// charSet holds the character mappings for spinner dots and companion faces.
// The active set is selected at startup based on terminal.ascii config.
type charSet struct {
	ActiveDot   string
	InactiveDot string
	Faces       []string
	Glances     []string
	Blink       string
	Sleepy      string
	Default     string
}

var unicodeChars = charSet{
	ActiveDot:   "■",
	InactiveDot: "⬝",
	Faces: []string{
		"(◕‿◕)",
		"(◐‿◐)",
		"(◑‿◑)",
		"(◔‿◔)",
	},
	Glances: []string{
		"(◐‿◐)",
		"(◑‿◑)",
		"(◔‿◔)",
	},
	Blink:   "(-‿-)",
	Sleepy:  "(◡‿◡)",
	Default: "(◕‿◕)",
}

var asciiChars = charSet{
	ActiveDot:   "#",
	InactiveDot: ".",
	Faces: []string{
		"(o_o)",
		"(>_>)",
		"(<_<)",
		"(^_^)",
	},
	Glances: []string{
		"(>_>)",
		"(<_<)",
		"(^_^)",
	},
	Blink:   "(-_-)",
	Sleepy:  "(~_~)",
	Default: "(o_o)",
}

// activeChars is set once at startup by ApplyCompat.
var activeChars = &unicodeChars

// ActiveChars returns the current character set.
func ActiveChars() *charSet {
	return activeChars
}

// ApplyCompat configures terminal compatibility: ASCII character set and
// lipgloss color profile override. Call once at startup after loading config.
func ApplyCompat(ascii bool, colorMode string) {
	if ascii {
		activeChars = &asciiChars
	} else {
		activeChars = &unicodeChars
	}

	applyColorProfile(colorMode)
}

func applyColorProfile(mode string) {
	var profile termenv.Profile

	switch mode {
	case "256":
		profile = termenv.ANSI256
	case "16":
		profile = termenv.ANSI
	case "none":
		profile = termenv.Ascii
	default:
		// "auto" or unrecognized: leave lipgloss auto-detection in place
		return
	}

	stderrRenderer.SetColorProfile(profile)
	lipgloss.DefaultRenderer().SetColorProfile(profile)

	// Re-render any styles that were already created so they pick up the new profile.
	// faceStyle and phaseStyle are recomputed by ApplyTheme which runs after ApplyCompat,
	// but the prompt styles (CmdStyle, etc.) were set at package init and use stderrRenderer.
	// Re-creating them ensures they honour the forced profile.
	CmdStyle = stderrRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	FailStyle = stderrRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	TraceStyle = stderrRenderer.NewStyle().Faint(true)
	hintStyle = stderrRenderer.NewStyle().Faint(true)
	stepStyle = stderrRenderer.NewStyle().Faint(true)
	elapsedStyle = stderrRenderer.NewStyle().Faint(true)

	// Also update the default renderer for styles created outside tui package
	// (e.g. greet in main.go uses lipgloss.NewStyle())
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr, termenv.WithProfile(profile)))
}
