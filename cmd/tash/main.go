package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/renjfk/tash/internal/data"
	"github.com/renjfk/tash/internal/query"
	"github.com/renjfk/tash/internal/tui"
)

const usageText = `tash - Terminal Assistant Shell

Usage:
  tash query <input>          AI-assisted command generation
  tash init                   First-time setup (profile, fish integration)
  tash usage [--reset]        Show token usage stats (or reset them)
  tash reset                  Clear conversation history (keeps memories)
  tash clear                  Clear everything (conversation + memories)
  tash version                Print version

Just type naturally in fish — unknown commands go to AI automatically.
Failed commands that look like natural language are auto-intercepted.
Run 'tash init' to set everything up.`

var (
	version   = "dev"
	buildTime = "unknown"
)

func init() {
	// Set lipgloss default renderer to stderr so color detection works when
	// stdout is captured by fish command substitution (fish_command_not_found).
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usageText)
		os.Exit(1)
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tash: config error: %v\n", err)
		os.Exit(1)
	}

	query.Version = version
	data.RegisterThemeNames(tui.ThemeNames())
	closeLog := data.InitLogger(cfg.DataDir(), cfg.LogLevel)
	defer closeLog()

	tui.ApplyCompat(cfg.Terminal.ASCII, cfg.Terminal.Color)
	tui.ApplyTheme(cfg.Theme.Name, cfg.Theme.Color)

	switch os.Args[1] {
	case "query":
		session, args := extractFlag(os.Args[2:], "--session")
		outputFile, args := extractFlag(args, "--output")
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "tash: query requires input")
			os.Exit(1)
		}
		input := strings.Join(args, " ")
		runQuery(cfg, input, session, outputFile)

	case "tick":
		runTick(cfg)

	case "init":
		runInit(cfg)

	case "greet":
		runGreet(cfg)

	case "reset":
		if err := data.ResetConversation(cfg.DataDir()); err != nil {
			data.Error("reset: " + err.Error())
			os.Exit(1)
		}
		data.Info("conversation reset (memories preserved)")

	case "clear":
		if err := data.ClearConversation(cfg.DataDir()); err != nil {
			data.Error("clear: " + err.Error())
			os.Exit(1)
		}
		data.Info("conversation cleared")

	case "version":
		fmt.Printf("tash %s (built %s)\n", version, buildTime)

	case "usage":
		runUsage(cfg)

	default:
		fmt.Fprintf(os.Stderr, "tash: unknown command %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, usageText)
		os.Exit(1)
	}
}

func runQuery(cfg *data.Config, input string, session string, outputFile string) {
	prof, err := data.ReadProfile(cfg.DataDir())
	if err != nil {
		data.Warn("no profile found, run 'tash init' first")
	}

	convo, err := data.LoadConversation(cfg.DataDir(), cfg.Behavior.MaxMemories)
	if err != nil {
		convo = data.NewConversation()
	}
	convo.SetSession(session)

	result, err := query.Run(cfg, prof, convo, input)
	if err != nil {
		data.Error(err.Error())
		os.Exit(1)
	}

	// Save conversation state for follow-ups
	if err := convo.Save(cfg.DataDir()); err != nil {
		data.Warn("could not save conversation state: " + err.Error())
	}

	// Write result: to file if --output specified, otherwise stdout.
	// Fish captures this and places it in the command line buffer via commandline -r.
	if result != "" {
		if outputFile != "" {
			_ = os.WriteFile(outputFile, []byte(result), 0644)
		} else {
			fmt.Print(result)
		}
	}
}

func runTick(cfg *data.Config) {
	exitCode := 0
	command := ""
	session := ""
	bgUpdate := false

	// Parse flags
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--exit-code":
			if i+1 < len(os.Args) {
				exitCode, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		case "--command":
			if i+1 < len(os.Args) {
				command = os.Args[i+1]
				i++
			}
		case "--session":
			if i+1 < len(os.Args) {
				session = os.Args[i+1]
				i++
			}
		case "--bg-update":
			bgUpdate = true
		}
	}

	if bgUpdate {
		if err := tickBackgroundUpdate(cfg); err != nil {
			data.Error("bg-update: " + err.Error())
		}
		return
	}

	if err := tickRun(cfg, exitCode, command, session); err != nil {
		// tick errors are silent - never block the shell
		data.Error("tick: " + err.Error())
	}
}

func runInit(cfg *data.Config) {
	migrated, err := cfg.Migrate()
	if err != nil {
		data.Error("config migration failed: " + err.Error())
		os.Exit(1)
	}
	if migrated {
		data.Info("migrated " + cfg.DataDir() + "/config.yaml")
	}

	created, err := cfg.WriteDefault()
	if err != nil {
		data.Error("config write failed: " + err.Error())
		os.Exit(1)
	}
	if created {
		data.Info("created " + cfg.DataDir() + "/config.yaml")
	} else if !migrated {
		data.Info("config already exists at " + cfg.DataDir() + "/config.yaml")
	}

	if err := installFishIntegration(); err != nil {
		data.Warn("could not install fish integration: " + err.Error())
	}

	spinner := tui.NewSpinner("Initializing")
	stats, err := tickInit(cfg)
	spinner.Stop()

	if err != nil {
		data.Error("init failed: " + err.Error())
		os.Exit(1)
	}

	data.Info(fmt.Sprintf("analyzed %d history entries, %d unique commands, %d binaries in PATH",
		stats.HistoryEntries, stats.UniqueCommands, stats.Binaries))
	data.Info("profile created at " + cfg.DataDir() + "/profile.md")
}

// tashBinPath returns the absolute path to the tash binary.
// Uses the running binary's path so the fish hooks point at the right location.
func tashBinPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "tash"
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	return resolved
}

func installFishIntegration() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	confDir := filepath.Join(home, ".config", "fish", "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("create conf.d: %w", err)
	}

	bin := tashBinPath()

	content := fmt.Sprintf(
		`# tash - Terminal Assistant Shell
# Auto-generated by tash init

# Short aliases
function t --wraps=tash --description 'tash shortcut'
    %s $argv
end
function q --wraps='tash query' --description 'tash query shortcut'
    set -l result (%s query --session $fish_pid "$argv")
    if test -n "$result"
        commandline -r "$result"
    end
end

function fish_greeting
    %s greet
end

function fish_command_not_found
    set -g __tash_handled 1
    set -l result (%s query --session $fish_pid "$argv")
    if test -n "$result"
        commandline -r "$result"
    end
end

function tash_tick --on-event fish_postexec
    set -l last_status $status
    # Skip auto-intercept if fish_command_not_found already handled this
    if set -q __tash_handled
        set -e __tash_handled
        %s tick --exit-code $last_status --session $fish_pid --command "$argv" &
        return
    end
    # Successful commands don't need tick — fish history covers them
    if test $last_status -eq 0
        return
    end
    %s tick --exit-code $last_status --session $fish_pid --command "$argv"
    if test $status -eq 7
        set -l tmpfile (mktemp /tmp/tash.XXXXXX)
        %s query --session $fish_pid --output $tmpfile "$argv"
        set -l result (cat $tmpfile 2>/dev/null)
        rm -f $tmpfile
        if test -n "$result"
            commandline -r "$result"
        end
    end
end
`, bin, bin, bin, bin, bin, bin, bin,
	)

	path := filepath.Join(confDir, "tash.fish")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write tash.fish: %w", err)
	}

	fmt.Fprintln(os.Stderr, "tash: installed fish integration at", path)
	return nil
}

func runUsage(cfg *data.Config) {
	// Handle --reset flag
	for _, arg := range os.Args[2:] {
		if arg == "--reset" {
			if err := data.ResetUsage(cfg.DataDir()); err != nil {
				data.Error("usage reset: " + err.Error())
				os.Exit(1)
			}
			data.Info("usage stats reset")
			return
		}
	}

	stats, err := data.LoadUsage(cfg.DataDir())
	if err != nil {
		data.Error("load usage: " + err.Error())
		os.Exit(1)
	}

	if stats.TotalCalls == 0 {
		fmt.Fprintln(os.Stderr, "tash: no usage recorded yet")
		return
	}

	// Count by action
	queryCalls := 0
	queryPrompt := 0
	queryComp := 0
	rebuildCalls := 0
	rebuildPrompt := 0
	rebuildComp := 0
	for _, r := range stats.Records {
		switch r.Action {
		case "query":
			queryCalls++
			queryPrompt += r.PromptTokens
			queryComp += r.CompletionTokens
		case "rebuild":
			rebuildCalls++
			rebuildPrompt += r.PromptTokens
			rebuildComp += r.CompletionTokens
		}
	}

	first := data.FormatTimestamp(stats.FirstCall)
	last := data.FormatTimestamp(stats.LastCall)

	fmt.Fprintf(os.Stderr, "Token usage (%s to %s)\n\n", first, last)
	fmt.Fprintf(os.Stderr, "  %-12s %6s %10s %10s %10s\n", "Action", "Calls", "Prompt", "Completion", "Total")
	fmt.Fprintf(os.Stderr, "  %-12s %6s %10s %10s %10s\n", "------", "-----", "------", "----------", "-----")
	if queryCalls > 0 {
		fmt.Fprintf(os.Stderr, "  %-12s %6d %10d %10d %10d\n", "query", queryCalls, queryPrompt, queryComp, queryPrompt+queryComp)
	}
	if rebuildCalls > 0 {
		fmt.Fprintf(os.Stderr, "  %-12s %6d %10d %10d %10d\n", "rebuild", rebuildCalls, rebuildPrompt, rebuildComp, rebuildPrompt+rebuildComp)
	}
	fmt.Fprintf(os.Stderr, "  %-12s %6d %10d %10d %10d\n", "total", stats.TotalCalls, stats.TotalPrompt, stats.TotalComp, stats.TotalPrompt+stats.TotalComp)
}

var greetMessages = []string{
	"just type naturally",
	"ready when you are",
	"what are we building?",
	"at your service",
	"let's get things done",
	"standing by",
	"hey there",
}

func runGreet(cfg *data.Config) {
	seed := int(time.Now().UnixNano() & 0x7fffffff)
	h1 := tui.FaceHash(seed)
	h2 := tui.FaceHash(seed + 31)
	faces := tui.Faces()
	f := faces[h1%len(faces)]
	msg := greetMessages[h2%len(greetMessages)]
	accent := tui.ActiveTheme().Accent
	face := lipgloss.NewStyle().Foreground(accent).Render(f)
	name := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("tash")
	text := lipgloss.NewStyle().Faint(true).Render(msg)
	fmt.Fprintf(os.Stderr, "%s %s %s\n", face, name, text)

	if cfg.Behavior.UpdateCheck {
		if info := data.ReadUpdateAvailable(cfg.DataDir(), version); info != nil {
			notice := lipgloss.NewStyle().Faint(true).Render(data.FormatUpdateNotification(info))
			fmt.Fprintf(os.Stderr, "%s\n", notice)
		}
	}
}

// extractFlag removes a --flag value pair from args and returns the value and remaining args.
func extractFlag(args []string, flag string) (string, []string) {
	var value string
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			value = args[i+1]
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	return value, rest
}
