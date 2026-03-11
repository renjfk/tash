package tui

import (
	"testing"
)

func TestFaceHash_Deterministic(t *testing.T) {
	a := FaceHash(42)
	b := FaceHash(42)
	if a != b {
		t.Errorf("FaceHash should be deterministic: %d != %d", a, b)
	}
}

func TestFaceHash_NonNegative(t *testing.T) {
	for i := -100; i < 100; i++ {
		if FaceHash(i) < 0 {
			t.Errorf("FaceHash(%d) returned negative: %d", i, FaceHash(i))
		}
	}
}

func TestFaceHash_DifferentInputs(t *testing.T) {
	// Not a strict requirement, but different inputs should usually produce different outputs
	a := FaceHash(1)
	b := FaceHash(2)
	if a == b {
		t.Log("warning: FaceHash(1) == FaceHash(2), possible but unlikely")
	}
}

func TestTotalFrames(t *testing.T) {
	expected := dotCount + holdEnd + (dotCount - 1) + holdStart
	got := totalFrames()
	if got != expected {
		t.Errorf("totalFrames() = %d, want %d", got, expected)
	}
}

func TestGetScannerState_Forward(t *testing.T) {
	s := getScannerState(0)
	if s.activePos != 0 {
		t.Errorf("frame 0: expected activePos 0, got %d", s.activePos)
	}
	if !s.forward {
		t.Error("frame 0: expected forward=true")
	}
	if s.isHolding {
		t.Error("frame 0: expected isHolding=false")
	}
}

func TestGetScannerState_RightEnd(t *testing.T) {
	// Last forward frame
	s := getScannerState(dotCount - 1)
	if s.activePos != dotCount-1 {
		t.Errorf("expected activePos %d, got %d", dotCount-1, s.activePos)
	}
}

func TestGetScannerState_HoldRight(t *testing.T) {
	s := getScannerState(dotCount)
	if !s.isHolding {
		t.Error("expected isHolding=true at right hold")
	}
	if s.activePos != dotCount-1 {
		t.Errorf("expected activePos %d during right hold, got %d", dotCount-1, s.activePos)
	}
	if !s.forward {
		t.Error("expected forward=true during right hold")
	}
}

func TestGetScannerState_Backward(t *testing.T) {
	// First backward frame
	frame := dotCount + holdEnd
	s := getScannerState(frame)
	if s.forward {
		t.Error("expected forward=false during backward")
	}
	if s.activePos != dotCount-2 {
		t.Errorf("expected activePos %d, got %d", dotCount-2, s.activePos)
	}
}

func TestGetScannerState_HoldLeft(t *testing.T) {
	frame := dotCount + holdEnd + (dotCount - 1)
	s := getScannerState(frame)
	if !s.isHolding {
		t.Error("expected isHolding=true at left hold")
	}
	if s.activePos != 0 {
		t.Errorf("expected activePos 0 during left hold, got %d", s.activePos)
	}
	if s.forward {
		t.Error("expected forward=false during left hold")
	}
}

func TestColorIndex_Head(t *testing.T) {
	s := scannerState{activePos: 3, forward: true}
	idx := colorIndex(3, s)
	if idx != 0 {
		t.Errorf("expected head index 0, got %d", idx)
	}
}

func TestColorIndex_Trail(t *testing.T) {
	s := scannerState{activePos: 5, forward: true}
	idx := colorIndex(4, s) // 1 behind head
	if idx != 1 {
		t.Errorf("expected trail index 1, got %d", idx)
	}
}

func TestColorIndex_Inactive(t *testing.T) {
	s := scannerState{activePos: 7, forward: true}
	idx := colorIndex(0, s) // distance 7, beyond trailLen (6)
	if idx != -1 {
		t.Errorf("expected inactive -1, got %d", idx)
	}
}

func TestColorIndex_Hold(t *testing.T) {
	s := scannerState{activePos: 7, forward: true, isHolding: true, holdProgress: 3}
	idx := colorIndex(7, s) // at head during hold
	if idx != 3 {           // dist(0) + holdProgress(3)
		t.Errorf("expected 3 during hold, got %d", idx)
	}
}

func TestColorIndex_Backward(t *testing.T) {
	s := scannerState{activePos: 3, forward: false}
	idx := colorIndex(4, s) // 1 behind head in reverse direction
	if idx != 1 {
		t.Errorf("expected trail index 1 backward, got %d", idx)
	}
}

func TestClamp8(t *testing.T) {
	tests := []struct {
		input float64
		want  int
	}{
		{0, 0},
		{255, 255},
		{128.5, 128},
		{-10, 0},
		{300, 255},
	}

	for _, tt := range tests {
		got := clamp8(tt.input)
		if got != tt.want {
			t.Errorf("clamp8(%f) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRgbHex(t *testing.T) {
	tests := []struct {
		r, g, b int
		want    string
	}{
		{0, 0, 0, "#000000"},
		{255, 255, 255, "#ffffff"},
		{255, 0, 128, "#ff0080"},
		{16, 32, 48, "#102030"},
	}

	for _, tt := range tests {
		got := rgbHex(tt.r, tt.g, tt.b)
		if got != tt.want {
			t.Errorf("rgbHex(%d,%d,%d) = %q, want %q", tt.r, tt.g, tt.b, got, tt.want)
		}
	}
}
