package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestResolveTheme_KnownPreset(t *testing.T) {
	for name, p := range presets {
		theme := resolveTheme(name, "")
		if theme.Base != p.base {
			t.Errorf("resolveTheme(%q): base mismatch", name)
		}
		if theme.Accent != lipgloss.Color(p.accent) {
			t.Errorf("resolveTheme(%q): accent mismatch", name)
		}
		// TrailHex should be populated
		if theme.TrailHex[0] == "" {
			t.Errorf("resolveTheme(%q): TrailHex[0] empty", name)
		}
	}
}

func TestResolveTheme_UnknownFallsToDefault(t *testing.T) {
	theme := resolveTheme("nonexistent", "")
	def := presets[defaultTheme]
	if theme.Base != def.base {
		t.Error("unknown theme should fall back to default base")
	}
}

func TestResolveTheme_CustomHex(t *testing.T) {
	theme := resolveTheme("", "#FF0000")
	if theme.Base[0] != 1.0 || theme.Base[1] != 0.0 || theme.Base[2] != 0.0 {
		t.Errorf("custom #FF0000: expected base [1,0,0], got %v", theme.Base)
	}
	if theme.Accent != lipgloss.Color("#FF0000") {
		t.Errorf("custom hex: accent mismatch")
	}
}

func TestResolveTheme_CustomOverridesName(t *testing.T) {
	theme := resolveTheme("gruvbox", "#00FF00")
	// Custom should take precedence over name
	if theme.Base[0] != 0.0 || theme.Base[2] != 0.0 {
		t.Error("custom hex should override preset name")
	}
}

func TestComputeTrail(t *testing.T) {
	theme := &Theme{
		Base: [3]float64{1.0, 0.0, 0.0},
	}
	computeTrail(theme)

	// Head (index 0) should be brightest
	if theme.TrailHex[0] == "" {
		t.Error("TrailHex[0] should not be empty")
	}
	// Head is full brightness: rgb(255, 0, 0) -> #ff0000
	if theme.TrailHex[0] != "#ff0000" {
		t.Errorf("TrailHex[0] = %q, want #ff0000", theme.TrailHex[0])
	}

	// Index 1 has 0.9 alpha with 1.15x boost, but clamped to 1.0
	// So R = min(1.0, 1.0*1.15) = 1.0, then *0.9*255 = 229
	if theme.TrailHex[1] == "" {
		t.Error("TrailHex[1] should not be empty")
	}

	// Trailing colors should fade (lower hex values for R)
	for i := 2; i < trailLen; i++ {
		if theme.TrailHex[i] == "" {
			t.Errorf("TrailHex[%d] should not be empty", i)
		}
	}
}

func TestParseHex_Valid(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b int
	}{
		{"#FF0000", 255, 0, 0},
		{"#00ff00", 0, 255, 0},
		{"#0000FF", 0, 0, 255},
		{"FF8040", 255, 128, 64},
		{"#102030", 16, 32, 48},
	}
	for _, tt := range tests {
		r, g, b := parseHex(tt.hex)
		if r != tt.r || g != tt.g || b != tt.b {
			t.Errorf("parseHex(%q) = (%d,%d,%d), want (%d,%d,%d)", tt.hex, r, g, b, tt.r, tt.g, tt.b)
		}
	}
}

func TestParseHex_Invalid(t *testing.T) {
	fb := presets[defaultTheme].base
	wantR := int(fb[0] * 255)
	wantG := int(fb[1] * 255)
	wantB := int(fb[2] * 255)

	tests := []string{
		"",
		"#",
		"#GG0000",
		"too-short",
		"#12345",
		"#1234567",
	}
	for _, hex := range tests {
		r, g, b := parseHex(hex)
		if r != wantR || g != wantG || b != wantB {
			t.Errorf("parseHex(%q) = (%d,%d,%d), want fallback (%d,%d,%d)", hex, r, g, b, wantR, wantG, wantB)
		}
	}
}

func TestParseHex_PartialInvalidDigits(t *testing.T) {
	// Valid R, invalid G
	fb := presets[defaultTheme].base
	wantR := int(fb[0] * 255)
	r, g, b := parseHex("#FFGG00")
	if r != wantR {
		t.Errorf("parseHex with invalid G: expected fallback, got (%d,%d,%d)", r, g, b)
	}

	// Valid R+G, invalid B
	r2, g2, b2 := parseHex("#FF00ZZ")
	if r2 != wantR {
		t.Errorf("parseHex with invalid B: expected fallback, got (%d,%d,%d)", r2, g2, b2)
	}
}

func TestThemeInactiveHex(t *testing.T) {
	// Save and restore
	saved := activeTheme
	defer func() { activeTheme = saved }()

	activeTheme = resolveTheme("solarized", "")

	hex := ThemeInactiveHex(1.0)
	if hex == "" || hex[0] != '#' || len(hex) != 7 {
		t.Errorf("ThemeInactiveHex(1.0) = %q, expected valid hex", hex)
	}

	hex0 := ThemeInactiveHex(0.0)
	if hex0 != "#000000" {
		t.Errorf("ThemeInactiveHex(0.0) = %q, want #000000", hex0)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) != 12 {
		t.Errorf("expected 12 theme names, got %d", len(names))
	}

	// All names should exist in presets
	for _, name := range names {
		if _, ok := presets[name]; !ok {
			t.Errorf("theme name %q not found in presets", name)
		}
	}
}

func TestFormatThemeList(t *testing.T) {
	got := FormatThemeList()
	if !strings.HasPrefix(got, "available themes:") {
		t.Errorf("FormatThemeList() = %q, expected prefix 'available themes:'", got)
	}
	if !strings.Contains(got, "solarized") {
		t.Error("FormatThemeList should contain solarized")
	}
}

func TestActiveTheme(t *testing.T) {
	theme := ActiveTheme()
	if theme == nil {
		t.Fatal("ActiveTheme() returned nil")
	}
	if theme.TrailHex[0] == "" {
		t.Error("ActiveTheme should have populated TrailHex")
	}
}

func TestApplyTheme(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()

	ApplyTheme("dracula", "")
	theme := ActiveTheme()
	if theme.Base != presets["dracula"].base {
		t.Error("ApplyTheme(dracula) did not set correct base")
	}
}

func TestApplyTheme_Custom(t *testing.T) {
	saved := activeTheme
	defer func() { activeTheme = saved }()

	ApplyTheme("", "#0000FF")
	theme := ActiveTheme()
	if theme.Base[2] != 1.0 {
		t.Errorf("ApplyTheme custom blue: expected Base[2]=1.0, got %f", theme.Base[2])
	}
}
