package tui

import (
	"fmt"
	"math"
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds the resolved colors for the spinner and greet.
type Theme struct {
	Base     [3]float64 // RGB 0..1 for scanner trail
	Accent   lipgloss.Color
	TrailHex [trailLen]string
}

// preset holds the base RGB and accent hex for a theme.
type preset struct {
	base   [3]float64
	accent string
}

// Themes use the active tab/frame green from well-known terminal color schemes.
// Colors sourced from zellij ribbon_selected.background values —
// these are the accent colors each scheme uses for active UI elements.
var presets = map[string]preset{
	// Solarized — green (133 153 0)
	"solarized": {[3]float64{0.522, 0.600, 0.0}, "#859900"},
	// Gruvbox dark — yellow-green (152 151 26)
	"gruvbox": {[3]float64{0.596, 0.592, 0.102}, "#98971a"},
	// Nord — aurora green (163 190 140)
	"nord": {[3]float64{0.639, 0.745, 0.549}, "#a3be8c"},
	// Dracula — green (80 250 123)
	"dracula": {[3]float64{0.314, 0.980, 0.482}, "#50fa7b"},
	// Monokai — green (0 140 0)
	"monokai": {[3]float64{0.0, 0.549, 0.0}, "#008c00"},
	// Catppuccin Mocha — green (166 227 161)
	"catppuccin": {[3]float64{0.651, 0.890, 0.631}, "#a6e3a1"},
	// Tokyo Night — green (158 206 106)
	"tokyo-night": {[3]float64{0.620, 0.808, 0.416}, "#9ece6a"},
	// Rose Pine — pine green (49 116 143)
	"rose-pine": {[3]float64{0.192, 0.455, 0.561}, "#31748f"},
	// Kanagawa — green (118 148 106)
	"kanagawa": {[3]float64{0.463, 0.580, 0.416}, "#76946a"},
	// Everforest — green (167 192 128)
	"everforest": {[3]float64{0.655, 0.753, 0.502}, "#a7c080"},
	// One Dark — green (152 195 121)
	"onedark": {[3]float64{0.596, 0.765, 0.475}, "#98c379"},
	// Nightfox — green (129 178 154)
	"nightfox": {[3]float64{0.506, 0.698, 0.604}, "#81b29a"},
}

// defaultTheme is the theme name used when none is configured.
const defaultTheme = "solarized"

// activeTheme is set once at startup by ApplyTheme.
var activeTheme = resolveTheme(defaultTheme, "")

// ActiveTheme returns the current theme.
func ActiveTheme() *Theme {
	return activeTheme
}

// ApplyTheme resolves the theme from config and updates all shared styles.
func ApplyTheme(name string, custom string) {
	activeTheme = resolveTheme(name, custom)
	recomputeStyles()
}

func resolveTheme(name string, custom string) *Theme {
	t := &Theme{}

	if custom != "" {
		r, g, b := parseHex(custom)
		t.Base = [3]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255}
		t.Accent = lipgloss.Color(custom)
	} else if p, ok := presets[name]; ok {
		t.Base = p.base
		t.Accent = lipgloss.Color(p.accent)
	} else {
		p := presets[defaultTheme]
		t.Base = p.base
		t.Accent = lipgloss.Color(p.accent)
	}

	computeTrail(t)
	return t
}

func computeTrail(t *Theme) {
	for i := 0; i < trailLen; i++ {
		r, g, b := t.Base[0], t.Base[1], t.Base[2]
		var alpha float64

		switch i {
		case 0:
			alpha = 1.0
		case 1:
			alpha = 0.9
			r = math.Min(1.0, r*1.15)
			g = math.Min(1.0, g*1.15)
			b = math.Min(1.0, b*1.15)
		default:
			alpha = math.Pow(0.65, float64(i-1))
		}

		fr := clamp8(r * alpha * 255)
		fg := clamp8(g * alpha * 255)
		fb := clamp8(b * alpha * 255)
		t.TrailHex[i] = rgbHex(fr, fg, fb)
	}
}

// defaultBase is the fallback RGB for invalid hex parsing.
var defaultBase = presets[defaultTheme].base

func parseHex(hex string) (int, int, int) {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	fb := defaultBase
	if len(hex) != 6 {
		return int(fb[0] * 255), int(fb[1] * 255), int(fb[2] * 255)
	}
	r, err := strconv.ParseInt(hex[0:2], 16, 32)
	if err != nil {
		return int(fb[0] * 255), int(fb[1] * 255), int(fb[2] * 255)
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 32)
	if err != nil {
		return int(fb[0] * 255), int(fb[1] * 255), int(fb[2] * 255)
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 32)
	if err != nil {
		return int(fb[0] * 255), int(fb[1] * 255), int(fb[2] * 255)
	}
	return int(r), int(g), int(b)
}

func recomputeStyles() {
	t := activeTheme
	faceStyle = stderrRenderer.NewStyle().Foreground(t.Accent)
	phaseStyle = stderrRenderer.NewStyle().Bold(true).Foreground(t.Accent)
}

// ThemeInactiveHex returns the inactive dot color for the current theme.
func ThemeInactiveHex(fadeFactor float64) string {
	t := activeTheme
	r, g, b := t.Base[0], t.Base[1], t.Base[2]
	alpha := 0.6 * fadeFactor
	return rgbHex(clamp8(r*alpha*255), clamp8(g*alpha*255), clamp8(b*alpha*255))
}

// ThemeNames returns the available preset theme names.
func ThemeNames() []string {
	return []string{
		"solarized", "gruvbox", "nord", "dracula",
		"monokai", "catppuccin", "tokyo-night", "rose-pine", "kanagawa",
		"everforest", "onedark", "nightfox",
	}
}

// FormatThemeList returns a formatted string of available themes for display.
func FormatThemeList() string {
	return fmt.Sprintf("available themes: %v", ThemeNames())
}
