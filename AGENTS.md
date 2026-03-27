# AGENTS.md — tash

Guidelines for AI agents working in this repository. Keep this file concise —
only document constraints and rules an agent would get wrong without being told.
Implementation details belong in code comments, not here.

## Project Overview

**tash** (Terminal Assistant Shell) is a Go CLI that acts as fish shell middleware.
Intercepts unknown commands via `fish_command_not_found`, queries an AI, and places
accepted commands into the fish command line buffer via `commandline -r`.

## Architecture

Four packages under `internal/` — nothing exported for external consumption.

- `cmd/tash/` — thin CLI entry (`main.go`), post-exec hook (`tick.go`), profile rebuild (`rebuild.go`)
- `internal/ai/` — OpenAI-compatible API client, system prompt
- `internal/data/` — all persistent state: config, conversation, history, profile, usage, logging
- `internal/query/` — query orchestration: AI call, response parsing, tool validation, screen capture
- `internal/tui/` — spinner + suggestion prompt (bubbletea)

### Key Invariants

- TUI goes to stderr, command result to stdout. Fish captures stdout.
- AI decides response type: `command`, `chat`, `history`, `conversation`, `memory`, `screen`, `plan`
- Screen capture uses Zellij >= 0.44.0 (`--path` and `--pane-id` flags). Detects via `$ZELLIJ` env var,
  targets current pane via `$ZELLIJ_PANE_ID`. Quiet failure — no version check.
- `conversation.jsonl` is append-only, read from tail. Initial load: 20 entries + memories.
- `tash tick` must return instantly.

## Build Commands

```bash
make build          # lint + fmt + build bin/tash
make install        # build + copy to ~/.local/bin/
make check          # lint + fmt only (no build)
make test           # go test ./...
make lint           # golangci-lint run ./...
make fmt            # golangci-lint fmt ./... --diff
```

Verify changes: `make build` + `make test` with zero failures.

The binary outputs to `bin/tash`. Fish hooks point at `~/tash/bin/tash` for
development — no install step needed.

CI runs on every PR and push to main (lint, test, build). PRs require all
checks to pass.

## Dependencies

- `gopkg.in/yaml.v3` — config parsing
- `github.com/charmbracelet/bubbletea` + `lipgloss` — TUI

No cobra, no logging framework, no testing libraries. Manual `os.Args` parsing.
stdlib `net/http` for HTTP.

## Code Style

- **Imports**: three groups (stdlib, third-party, internal) separated by blank lines
- **Errors**: always `fmt.Errorf` with `%w`, never `errors.New`. Non-critical: `data.Warn()`/`data.Error()`
- **Naming**: PascalCase exported, camelCase unexported. Single-letter receivers matching type initial
- **Functions**: `cfg *data.Config` always first parameter when present. No named returns
- **Output**: stderr for all UI (`tash:` prefix), stdout only for fish buffer commands
- **Logging**: stdlib `log/slog` to `tash.log`. User-facing: `data.Info/Warn/Error`
- **No interfaces** — concrete types throughout
- **Formatting**: enforced by `golangci-lint fmt`
