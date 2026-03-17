package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/renjfk/tash/internal/ai"
	"github.com/renjfk/tash/internal/data"
)

// RebuildInput contains data needed to generate or update a profile.
type RebuildInput struct {
	CurrentProfile string
	HistoryStats   string
	InstalledTools []string
	IsIncremental  bool
}

func rebuildProfile(cfg *data.Config, input *RebuildInput) (string, error) {
	client := ai.NewClient(cfg)

	var systemPrompt string
	if input.IsIncremental {
		systemPrompt = incrementalSystemPrompt()
	} else {
		systemPrompt = fullRebuildSystemPrompt()
	}

	userPrompt := buildRebuildUserPrompt(input)

	resp, err := client.Complete(
		context.Background(),
		ai.Request{
			Model:  cfg.Model.Name,
			System: systemPrompt,
			Messages: []ai.Message{
				{Role: "user", Content: userPrompt},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("profile rebuild: %w", err)
	}

	if err := data.RecordUsage(cfg.DataDir(), "rebuild", cfg.Model.Name, resp.Usage.PromptTokens, resp.Usage.CompletionTokens); err != nil {
		data.Warn(fmt.Sprintf("record usage: %v", err))
	}

	return systemSection() + "\n" + resp.Content, nil
}

func fullRebuildSystemPrompt() string {
	return `You are generating a shell environment profile for an AI assistant called tash.
Analyze the user's shell history statistics and installed tools to create a concise profile.

IMPORTANT:
- Only include what is directly observable from the data provided.
- Do NOT include a System section (that is added separately).
- Do NOT guess the user's name or personal details.

Output ONLY the profile sections below in markdown, 20-40 lines, ~200 tokens:

## Installed Tools
- Categorized list of CLI tools (group by purpose: containers, cloud, build, search, etc.)
- Only include notable tools, not every binary in PATH

## Frequently Used Commands
- Top commands with approximate counts`
}

func incrementalSystemPrompt() string {
	return `You are updating a shell environment profile for an AI assistant called tash.
Compare the new activity against the current profile and update ONLY if meaningful changes exist.

IMPORTANT:
- Do NOT include a System section.
- Do NOT guess the user's name or personal details.

If nothing meaningful changed, respond with exactly: NO_CHANGE

Otherwise, output the Installed Tools and Frequently Used Commands sections.
Keep it 20-40 lines, ~200 tokens.`
}

func buildRebuildUserPrompt(input *RebuildInput) string {
	var b strings.Builder

	if input.CurrentProfile != "" {
		b.WriteString("Current profile:\n```\n")
		b.WriteString(input.CurrentProfile)
		b.WriteString("\n```\n\n")
	}

	b.WriteString("Shell history statistics:\n```\n")
	b.WriteString(input.HistoryStats)
	b.WriteString("\n```\n\n")

	fmt.Fprintf(&b, "Installed tools (%d total):\n", len(input.InstalledTools))
	b.WriteString(strings.Join(input.InstalledTools, ", "))

	return b.String()
}

func systemSection() string {
	var b strings.Builder

	b.WriteString("# Shell Environment\n\n")
	b.WriteString("## System\n")
	fmt.Fprintf(&b, "- OS: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	b.WriteString("- Shell: fish\n")

	if home, err := os.UserHomeDir(); err == nil {
		fmt.Fprintf(&b, "- Home: %s\n", home)
	}

	return b.String()
}
