# AGENTS.md - Coding Agent Guidelines for tash

## Project overview

tash is a Go CLI binary that acts as fish shell middleware. It intercepts unknown commands
via `fish_command_not_found`, queries an AI for command suggestions or chat responses,
and validates commands locally. Accepted commands are placed directly into the fish command
line buffer via `commandline -r`, giving the user final control to review, edit, or run.
It maintains a persistent user profile derived from shell history and feeds recent shell
activity into every AI conversation for context continuity.

## Build / Test / Lint

```bash
make build          # lint + fmt + build bin/tash
make install        # build + copy to ~/.local/bin/
make check          # lint + fmt only (no build)
make lint           # golangci-lint run ./...
make fmt            # golangci-lint fmt ./... --diff
make test           # go test ./...
make dist           # goreleaser release --clean
make snapshot       # goreleaser release --clean --snapshot
make clean          # rm -rf bin/
```

Run a single test:

```bash
go test ./internal/data/ -run TestSomething -v
```

The binary outputs to `bin/tash`. Fish shell hooks point directly at `~/tash/bin/tash`
for development -- no install step needed, just `make build`.

CI runs on every PR and push to main via `.github/workflows/ci.yml` (lint, test, build).
PRs require all checks to pass before merging. Coverage is reported to Codecov on main.

## Project structure

```
cmd/tash/main.go           # thin CLI entry, os.Args switch, delegates to internal/
cmd/tash/tick.go            # post-exec hook: bg profile update, auto-intercept, PATH scan
cmd/tash/rebuild.go         # profile rebuild orchestration (calls AI)
internal/ai/               # OpenAI-compatible API client (client.go), prompt builder (prompt.go)
internal/data/              # all persistent state: config, conversation, history, profile, usage, logging
internal/query/             # query orchestration: AI call -> parse response -> execute + tool validation
internal/tui/               # Knight Rider style spinner + suggestion prompt (bubbletea)
```

Everything is under `internal/` -- nothing is exported for external consumption. The `data`
package owns all persistent state (config.yaml, conversation.jsonl, fish history, profile.md).
The `ai` package handles external API communication. Orchestrator packages (`query`, `cmd/tash`)
are thin and delegate to services. Each package has one primary file matching the package name
plus supporting files named after their concern (e.g. `query/validator.go`, `tui/prompt.go`,
`data/history.go`).

## Dependencies

- `gopkg.in/yaml.v3` -- config.yaml parsing
- `github.com/charmbracelet/bubbletea` + `lipgloss` -- TUI (spinner, multi-step approval prompt)

No cobra, no logging framework, no testing libraries. CLI parsing is manual `os.Args`.
HTTP uses `net/http` stdlib directly.

## Code style

### Imports

Three groups separated by blank lines: stdlib, third-party, then internal. Sorted
alphabetically within each group.

```go
import (
"fmt"
"os"
"strings"

"github.com/charmbracelet/lipgloss"

"github.com/renjfk/tash/internal/ai"
"github.com/renjfk/tash/internal/data"
)
```

When there are no third-party imports, two groups (stdlib + internal) is fine.

### Error handling

- Always use `fmt.Errorf` with `%w` for wrapping: `fmt.Errorf("planning: %w", err)`
- Error prefixes are short, lowercase, describing the operation
- `errors.New` is not used anywhere -- use `fmt.Errorf` even for static messages
- No custom error types or sentinel errors
- Non-critical errors: use `data.Warn()` or `data.Error()` (writes to both stderr and slog)
- Return pattern: `(value, error)` universally

```go
// Good
return nil, fmt.Errorf("parse plan response: %w (raw: %s)", err, resp.Content)

// Good - non-critical (dual output: stderr + log file)
data.Warn("no profile found, run 'tash init' first")

// Bad - don't use errors.New
return nil, errors.New("empty response")

// Bad - don't use raw fmt.Fprintf for warnings, use data.Warn instead
fmt.Fprintf(os.Stderr, "tash: warning: something\n")
```

### Naming

- **Types**: PascalCase, full words, no abbreviations: `HistoryEntry`, `RebuildInput`
- **Unexported types**: camelCase, full words: `anthropicRequest`, `phaseMsg`
- **Receivers**: single letter matching type initial: `(c *Config)`, `(s *State)`
- **Variables**: camelCase, descriptive: `systemPrompt`, `historyPath`, `dataDir`
- **Short vars** only in tight scopes: `b` for Builder, `f` for file, `e` for entry
- **Abbreviations** only for locals: `cfg`, `convo`, `prof`, `resp`, `req`, `ts`
- **Constants**: PascalCase exported, camelCase unexported (`stateFile`)
- **Constructors**: `NewX()` returning pointer: `NewClient(cfg) *Client`

### Functions

- `cfg *data.Config` is always the first parameter when present
- Pointer receivers for all mutable state
- Value receivers only for bubbletea models (framework convention)
- No named returns

### Comments

- Godoc on all exported symbols: `// TypeName verb-phrase describing purpose.`
- No godoc on unexported functions
- Inline comments only for non-obvious logic
- No block comments (`/* */`)
- Section headers: `// Anthropic API types`

### Strings and output

- `strings.Builder` for concatenation, never `+` in loops
- stderr for all UI: `fmt.Fprintf(os.Stderr, ...)`
- stdout only for the command to place in fish buffer: `fmt.Print(result)`
- All stderr messages prefixed with `tash:`
- lipgloss for all TUI styling

### Struct layout

- Exported fields first, then unexported
- YAML tags use `snake_case`: `yaml:"api_key_env"`
- JSON tags use `snake_case`: `json:"last_activity"`
- All serialized fields must have tags

### Package conventions

- No `context.Context` passing -- HTTP client uses flat 30s timeout
- Logging via stdlib `log/slog` -- file output to `tash.log`, levels: debug/info/warn/error
- User-facing messages: `data.Info/Warn/Error` (writes to both stderr and slog)
- Formatting enforced by `golangci-lint fmt`
- No interfaces defined -- concrete types throughout

## Architecture invariants

- The binary owns stdin/stdout during `tash query`. TUI goes to stderr, command result
  to stdout. Fish captures stdout via command substitution and places it in the command
  line buffer via `commandline -r`. Single commands skip the TUI prompt entirely.
- AI decides response type: `command` (with optional explanation), `chat`, `history` request, `memory` (store a fact),
  or `plan` (iterative step-by-step execution).
- All input goes to the AI -- no local typo detection or filtering.
- `tash tick` records failed shell commands into conversation state (successful commands skip tick entirely). Must
  return instantly.
- `profile.md` System section is hardcoded facts (OS, arch, shell), not AI-generated.
- `conversation.jsonl` is append-only. Reads from tail via reverse reader, capped at 250 entries + 50 memories in
  memory.
- Retry loop uses the same model for all attempts (single model config). Tool calls (history, memory) don't count as
  failures. Capped at 2 tool calls per query.
- Multi-step commands execute inline sequentially, not batched to fish. The last command is returned to fish's command
  line buffer.
- Plan mode: AI returns one command at a time with `steps_remaining`; command is executed, output captured (capped at
  4KB), and fed back as context for the next AI call. The AI dynamically decides the next step based on real output.
- AI can request more context (filtered by regex) via `{"type": "history"}` response; searches both fish history and
  conversation entries, deduplicated against what's already in the prompt.

## Roadmap

### Done

- [x] CLI skeleton with query, tick, init subcommands
- [x] profile.md read/write + AI-driven rebuild (facts-only: system, tools, commands)
- [x] Anthropic API client (model name passed directly from config)
- [x] Unified query flow: AI decides command vs chat vs history request
- [x] fish_command_not_found + fish_postexec integration
- [x] Conversation state tracks shell commands + tash interactions
- [x] Recent shell commands fed into every AI query for context
- [x] Agentic history tool: AI can request more context with regex filter
- [x] Command responses can include chat explanation
- [x] Tool validation via exec.LookPath with constraint accumulation across retries
- [x] Retry loop with single model (no escalation)
- [x] Interactive TUI approval for multi-step intermediate commands (Enter/Esc) via bubbletea
- [x] Single/last commands returned to fish command line buffer (no prompt, user has final control)
- [x] Multi-step sequential execution (approve + run each step inline)
- [x] Knight Rider style spinner animation during AI calls (Claude KIT style)
- [x] Fish history parser (line-by-line, handles YAML-special chars in commands)
- [x] PATH binary scanner
- [x] Watermark-based incremental tick with background fork + PID file
- [x] conversation.jsonl with TTL and cross-invocation context
- [x] Auto-intercept: failed natural language commands invoke tash query directly
- [x] config.yaml support with defaults and bootstrap on init
- [x] golangci-lint integration (lint + fmt gate the build)
- [x] System prompt with tash personality/character
- [x] Migrated config from TOML to YAML (gopkg.in/yaml.v3)
- [x] Removed BurntSushi/toml dependency
- [x] Lipgloss default renderer set to stderr (fixes color in fish command substitution)
- [x] Conversation state migrated from JSON to JSONL
- [x] Fish history parser replaced yaml.Unmarshal with line-by-line (handles colons in commands)
- [x] Fish shell auto-integration via conf.d/tash.fish (generated by tash init)
- [x] Spinner during tash init
- [x] `--bg-update` flag routed in main.go (tick background profile update)
- [x] Conversation JSONL entries tagged with session ID (fish PID) for multi-terminal support
- [x] `--session` flag on query and tick, passed from fish hooks via `$fish_pid`
- [x] `--output` flag on query for auto-intercept (avoids stdout capture of sub-commands)
- [x] Trace lines after TUI actions (>, x) so suggestions stay visible on screen
- [x] Companion face character (◕‿◕) in spinner and greeting with animated expressions
- [x] `tash greet` subcommand with randomized face + message (called from fish_greeting)
- [x] Agentic history tool searches both fish history and conversation entries (queries, commands, chat), deduplicated
  against prompt context
- [x] Memory cap: configurable max_memories (default 50), oldest dropped when exceeded
- [x] Conversation file: append-only, no TTL, reverse reader for efficient tail loading
- [x] SearchHistory streams with ring buffer (no full file allocation)
- [x] JSONL response parsing: supports multi-line responses (memory + chat in one response)
- [x] ParseResponse handles preamble text before JSON, sanitizes invalid JSON escapes
- [x] Structured logging via stdlib log/slog with file rotation (5MB, single .old backup)
- [x] Full API request/response debug logging (system prompt, messages, raw response, token usage)
- [x] Fish hook guard: `__tash_handled` prevents double invocation from command_not_found + auto-intercept
- [x] `tash reset` (clear conversation, keep memories) and `tash clear` (wipe everything)
- [x] Spinner phases: Thinking, Searching history, Checking alternatives, Retrying
- [x] TUI styles use explicit stderr renderer (fixes colors in fish command substitution)
- [x] Stdin drain after TUI exit (prevents OSC escape sequence leak to shell)
- [x] Tool call cap (max 2) with forced constraint when exhausted
- [x] Switch API client to generic OpenAI-compatible endpoint (use Anthropic compatibility layer, configurable endpoint
  in config.yaml)
- [x] Track token usage in state file per action for cost/usage calculations
- [x] `tash version` subcommand (prints version from build-time ldflags)
- [x] `tash usage` subcommand with `--reset` flag (shows token usage stats per action)
- [x] CI workflow: lint, test, build on PRs and main pushes (`.github/workflows/ci.yml`)
- [x] Codecov integration for coverage reporting on main
- [x] Test suite: pure function tests, filesystem tests, env-dependent tests across all packages
- [x] Fish short aliases: `t` (tash) and `q` (tash query) as fish functions in tash.fish
- [x] Tick skipped on successful commands (exit 0) — zero tash overhead on happy path
- [x] `__tash_handled` tick branch runs in background (`&`) to avoid blocking prompt
- [x] Color theme system: 12 presets (solarized, gruvbox, nord, dracula, monokai, catppuccin, tokyo-night, rose-pine,
  kanagawa, everforest, onedark, nightfox) + custom hex via config
- [x] Theme colors sourced from zellij `ribbon_selected.background` (active tab accent)
- [x] `tash init` prints stats: history entries analyzed, unique commands, binaries in PATH
- [x] Production builds (`make dist` / goreleaser) strip symbols and debug info (`-s -w`)
- [x] Plan mode: AI returns one command at a time with `steps_remaining`, executes it, captures output (4KB cap), feeds
  back for dynamic next-step planning

- [x] GoReleaser integration (.goreleaser.yaml) with macOS signing and notarization
- [x] GitHub Actions release workflow (manual dispatch with release tag, notes, draft/prerelease)
- [x] Homebrew formula via GoReleaser brews section (same repo, `brew install renjfk/tash/tash`)
- [x] Fish install script (`install.fish`) for curl-pipe installation
- [x] README.md, LICENSE, RELEASE_PROCESS.md
- [x] Uninstall instructions in README

