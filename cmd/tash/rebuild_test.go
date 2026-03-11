package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildRebuildUserPrompt_Full(t *testing.T) {
	input := &RebuildInput{
		HistoryStats:   "git(50)\ndocker(30)\n",
		InstalledTools: []string{"git", "docker", "kubectl"},
		IsIncremental:  false,
	}

	got := buildRebuildUserPrompt(input)

	if !strings.Contains(got, "Shell history statistics:") {
		t.Error("expected history stats section")
	}
	if !strings.Contains(got, "git(50)") {
		t.Error("expected history stats content")
	}
	if !strings.Contains(got, "Installed tools (3 total):") {
		t.Error("expected installed tools header with count")
	}
	if !strings.Contains(got, "git, docker, kubectl") {
		t.Error("expected tool list")
	}
	if strings.Contains(got, "Current profile:") {
		t.Error("should not have current profile section for full rebuild")
	}
}

func TestBuildRebuildUserPrompt_Incremental(t *testing.T) {
	input := &RebuildInput{
		CurrentProfile: "## Tools\n- docker",
		HistoryStats:   "docker(10)\n",
		InstalledTools: []string{"docker"},
		IsIncremental:  true,
	}

	got := buildRebuildUserPrompt(input)

	if !strings.Contains(got, "Current profile:") {
		t.Error("expected current profile section for incremental")
	}
	if !strings.Contains(got, "## Tools") {
		t.Error("expected current profile content")
	}
}

func TestBuildRebuildUserPrompt_EmptyTools(t *testing.T) {
	input := &RebuildInput{
		HistoryStats:   "git(1)\n",
		InstalledTools: nil,
		IsIncremental:  false,
	}

	got := buildRebuildUserPrompt(input)
	if !strings.Contains(got, "Installed tools (0 total):") {
		t.Error("expected 0 total tools")
	}
}

func TestFullRebuildSystemPrompt(t *testing.T) {
	got := fullRebuildSystemPrompt()
	phrases := []string{
		"shell environment profile",
		"Installed Tools",
		"Frequently Used Commands",
	}
	for _, p := range phrases {
		if !strings.Contains(got, p) {
			t.Errorf("fullRebuildSystemPrompt missing phrase: %q", p)
		}
	}
}

func TestIncrementalSystemPrompt(t *testing.T) {
	got := incrementalSystemPrompt()
	if !strings.Contains(got, "NO_CHANGE") {
		t.Error("expected NO_CHANGE instruction")
	}
	if !strings.Contains(got, "updating") {
		t.Error("expected incremental context")
	}
}

func TestSystemSection(t *testing.T) {
	got := systemSection()

	expected := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(got, expected) {
		t.Errorf("expected OS %q in system section", expected)
	}
	if !strings.Contains(got, "fish") {
		t.Error("expected fish shell in system section")
	}
	if !strings.Contains(got, "Home:") {
		t.Error("expected Home: in system section")
	}
}
