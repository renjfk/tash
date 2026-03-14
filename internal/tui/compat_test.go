package tui

import (
	"testing"
)

func TestApplyCompat_Unicode(t *testing.T) {
	saved := activeChars
	defer func() { activeChars = saved }()

	ApplyCompat(false, "auto")

	chars := ActiveChars()
	if chars.ActiveDot != "■" {
		t.Errorf("expected Unicode active dot, got %q", chars.ActiveDot)
	}
	if chars.InactiveDot != "⬝" {
		t.Errorf("expected Unicode inactive dot, got %q", chars.InactiveDot)
	}
	if chars.Default != "(◕‿◕)" {
		t.Errorf("expected Unicode default face, got %q", chars.Default)
	}
}

func TestApplyCompat_ASCII(t *testing.T) {
	saved := activeChars
	defer func() { activeChars = saved }()

	ApplyCompat(true, "auto")

	chars := ActiveChars()
	if chars.ActiveDot != "#" {
		t.Errorf("expected ASCII active dot, got %q", chars.ActiveDot)
	}
	if chars.InactiveDot != "." {
		t.Errorf("expected ASCII inactive dot, got %q", chars.InactiveDot)
	}
	if chars.Default != "(o_o)" {
		t.Errorf("expected ASCII default face, got %q", chars.Default)
	}
	if chars.Blink != "(-_-)" {
		t.Errorf("expected ASCII blink face, got %q", chars.Blink)
	}
	if chars.Sleepy != "(~_~)" {
		t.Errorf("expected ASCII sleepy face, got %q", chars.Sleepy)
	}
}

func TestApplyCompat_FacesMatchCharSet(t *testing.T) {
	saved := activeChars
	defer func() { activeChars = saved }()

	ApplyCompat(false, "auto")
	unicodeFaces := Faces()
	if len(unicodeFaces) != 4 {
		t.Errorf("expected 4 unicode faces, got %d", len(unicodeFaces))
	}

	ApplyCompat(true, "auto")
	asciiFaces := Faces()
	if len(asciiFaces) != 4 {
		t.Errorf("expected 4 ascii faces, got %d", len(asciiFaces))
	}

	// ASCII faces should differ from unicode faces
	if unicodeFaces[0] == asciiFaces[0] {
		t.Error("expected different faces between unicode and ascii sets")
	}
}

func TestApplyCompat_GlancesCount(t *testing.T) {
	saved := activeChars
	defer func() { activeChars = saved }()

	ApplyCompat(true, "auto")
	chars := ActiveChars()
	if len(chars.Glances) != 3 {
		t.Errorf("expected 3 ascii glances, got %d", len(chars.Glances))
	}

	ApplyCompat(false, "auto")
	chars = ActiveChars()
	if len(chars.Glances) != 3 {
		t.Errorf("expected 3 unicode glances, got %d", len(chars.Glances))
	}
}

func TestFace_ASCIIMode(t *testing.T) {
	savedChars := activeChars
	savedSeed := faceSeed
	defer func() {
		activeChars = savedChars
		faceSeed = savedSeed
	}()

	ApplyCompat(true, "auto")
	faceSeed = 42

	asciiFaceSet := map[string]bool{
		"(o_o)": true,
		"(>_>)": true,
		"(<_<)": true,
		"(^_^)": true,
		"(-_-)": true,
		"(~_~)": true,
	}

	for i := 0; i < 250; i++ {
		f := face(i)
		if !asciiFaceSet[f] {
			t.Errorf("face(%d) = %q, not in ASCII face set", i, f)
		}
	}
}
