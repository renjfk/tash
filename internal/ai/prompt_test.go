package ai

import (
	"strings"
	"testing"

	"github.com/renjfk/tash/internal/data"
)

func TestBuildQueryPrompt_InputOnly(t *testing.T) {
	got := BuildQueryPrompt("list files", nil, nil)
	if got != "list files" {
		t.Errorf("expected just input, got %q", got)
	}
}

func TestBuildQueryPrompt_WithHistory(t *testing.T) {
	history := []data.ShellCommand{
		{Command: "ls -la", ExitCode: 0},
		{Command: "cd /tmp", ExitCode: 0},
	}
	got := BuildQueryPrompt("show files", history, nil)

	if !strings.Contains(got, "Recent shell activity:") {
		t.Error("expected shell activity header")
	}
	if !strings.Contains(got, "$ ls -la") {
		t.Error("expected ls -la in history")
	}
	if !strings.Contains(got, "$ cd /tmp") {
		t.Error("expected cd /tmp in history")
	}
	if !strings.HasSuffix(got, "show files") {
		t.Error("expected input at the end")
	}
}

func TestBuildQueryPrompt_FailedCommand(t *testing.T) {
	history := []data.ShellCommand{
		{Command: "make build", ExitCode: 2},
	}
	got := BuildQueryPrompt("fix it", history, nil)

	if !strings.Contains(got, "# exit 2") {
		t.Error("expected exit code annotation for failed command")
	}
}

func TestBuildQueryPrompt_WithConstraints(t *testing.T) {
	constraints := []string{"fzf is not installed, use an alternative"}
	got := BuildQueryPrompt("search", nil, constraints)

	if !strings.Contains(got, "Constraints:") {
		t.Error("expected constraints header")
	}
	if !strings.Contains(got, "- fzf is not installed") {
		t.Error("expected constraint text")
	}
}

func TestBuildQueryPrompt_AllSections(t *testing.T) {
	history := []data.ShellCommand{{Command: "git status", ExitCode: 0}}
	constraints := []string{"rg is not installed"}
	got := BuildQueryPrompt("find errors", history, constraints)

	activityIdx := strings.Index(got, "Recent shell activity:")
	constraintsIdx := strings.Index(got, "Constraints:")
	inputIdx := strings.Index(got, "find errors")

	if activityIdx < 0 || constraintsIdx < 0 || inputIdx < 0 {
		t.Fatalf("missing section: activity=%d constraints=%d input=%d", activityIdx, constraintsIdx, inputIdx)
	}
	if activityIdx >= constraintsIdx || constraintsIdx >= inputIdx {
		t.Error("expected order: activity, constraints, input")
	}
}

func TestSystemPrompt_ContainsKeyPhrases(t *testing.T) {
	phrases := []string{
		"tash",
		"fish shell",
		"command",
		"chat",
		"JSONL",
		"history",
		"memory",
		"plan",
		"screen",
	}
	for _, phrase := range phrases {
		if !strings.Contains(SystemPrompt, phrase) {
			t.Errorf("SystemPrompt missing expected phrase: %q", phrase)
		}
	}
}
