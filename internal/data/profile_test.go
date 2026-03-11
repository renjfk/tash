package data

import (
	"testing"
)

func TestProfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	content := "## System\n- OS: darwin/arm64\n- Shell: fish\n"

	if err := WriteProfile(dir, content); err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	prof, err := ReadProfile(dir)
	if err != nil {
		t.Fatalf("ReadProfile: %v", err)
	}

	if prof.Content != content {
		t.Errorf("expected %q, got %q", content, prof.Content)
	}
}

func TestReadProfile_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadProfile(dir)
	if err == nil {
		t.Error("expected error for missing profile")
	}
}

func TestWriteProfile_Overwrite(t *testing.T) {
	dir := t.TempDir()

	_ = WriteProfile(dir, "old content")
	_ = WriteProfile(dir, "new content")

	prof, _ := ReadProfile(dir)
	if prof.Content != "new content" {
		t.Errorf("expected new content, got %q", prof.Content)
	}
}
