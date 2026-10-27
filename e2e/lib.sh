#!/usr/bin/env bash
# Shared helpers for tash e2e scenarios.
#
# A scenario is a bash script that sources this file and exits non-zero on
# failure. Scenarios exercise the real tash binary against a real
# OpenAI-compatible endpoint; nothing is mocked, so assertions steer the model
# with explicit prompts and check for stable markers and observable side effects.

set -uo pipefail

E2E_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_REPO="$(cd "$E2E_ROOT/.." && pwd)"
E2E_BIN="${E2E_TASH_BIN:-$E2E_REPO/bin/tash}"

# Endpoint and model are required and have no committed defaults, so the suite
# never hardcodes an internal host. Provide them via the environment, e.g.
#   export E2E_ENDPOINT=https://your-endpoint/v1
#   export E2E_MODEL=your-model
#   export E2E_API_KEY=...            # or set E2E_API_KEY_ENV to your var name
E2E_ENDPOINT="${E2E_ENDPOINT:-}"
E2E_MODEL="${E2E_MODEL:-}"
E2E_API_KEY_ENV="${E2E_API_KEY_ENV:-E2E_API_KEY}"
E2E_MAX_TOKENS_PARAM="${E2E_MAX_TOKENS_PARAM:-}"
E2E_REASONING_EFFORT="${E2E_REASONING_EFFORT:-}"

E2E_TIMEOUT="${E2E_TIMEOUT:-30000}"
E2E_KEEP="${E2E_KEEP:-0}"

# Point tui-test at the suite's config regardless of the current directory.
export TUI_TEST_CONFIG="$E2E_ROOT/tui-test.toml"

# --- logging -----------------------------------------------------------------
if [ -t 2 ] && [ "${E2E_COLOR:-auto}" != "0" ]; then
    C_RESET=$'\033[0m'; C_RED=$'\033[31m'; C_GRN=$'\033[32m'; C_YLW=$'\033[33m'; C_DIM=$'\033[2m'
else
    C_RESET=""; C_RED=""; C_GRN=""; C_YLW=""; C_DIM=""
fi

e2e_log()  { printf '%s\n' "$*" >&2; }
e2e_step() { printf "  ${C_DIM}- %s${C_RESET}\n" "$*" >&2; }
e2e_pass() { printf "  ${C_GRN}PASS${C_RESET} %s\n" "$*" >&2; }
e2e_skip() { printf "  ${C_YLW}SKIP${C_RESET} %s\n" "$*" >&2; exit 0; }
e2e_fail() {
    printf "  ${C_RED}FAIL${C_RESET} %s\n" "$*" >&2
    e2e_debug
    exit 1
}

e2e_debug() {
    [ "${E2E_DEBUG:-0}" = "1" ] || return 0
    [ -n "${E2E_HOME:-}" ] || return 0
    e2e_log "----- debug: ${E2E_HOME} -----"
    for f in config.yaml usage.json profile.md conversation.jsonl; do
        [ -f "$E2E_HOME/.config/tash/$f" ] || continue
        e2e_log "== $f =="
        head -c 3000 "$E2E_HOME/.config/tash/$f" >&2 || true
        e2e_log ""
    done
}

# --- preconditions -----------------------------------------------------------
e2e_require_cmd() { command -v "$1" >/dev/null 2>&1 || e2e_fail "required command not found: $1"; }
e2e_require_bin() { [ -x "$E2E_BIN" ] || e2e_fail "tash binary not found at $E2E_BIN (run: make build)"; }
e2e_require_env() {
    [ -n "${E2E_ENDPOINT:-}" ] || e2e_skip "E2E_ENDPOINT is not set"
    [ -n "${E2E_MODEL:-}" ] || e2e_skip "E2E_MODEL is not set"
    [ -n "${!E2E_API_KEY_ENV:-}" ] || e2e_skip "env $E2E_API_KEY_ENV is not set"
}

# --- isolated HOME -----------------------------------------------------------
# Both tash and fish resolve config from $HOME/.config (XDG_CONFIG_HOME unset),
# so the hook tash init installs is picked up by the fish session we start.
e2e_setup() {
    e2e_require_bin
    e2e_require_env
    E2E_HOME="$(mktemp -d "${TMPDIR:-/tmp}/tash-e2e-XXXXXX")"
    export HOME="$E2E_HOME"
    # Isolate everything tash/fish/tui-test/zellij resolve from the environment,
    # so a run never reads or writes the developer's real state. In particular,
    # ZELLIJ* must be cleared or tash's screen capture would read the developer's
    # live pane (the zellij socket is found via ZELLIJ_SESSION_NAME, not HOME).
    unset XDG_CONFIG_HOME XDG_DATA_HOME
    unset ZELLIJ ZELLIJ_PANE_ID ZELLIJ_SESSION_NAME ZELLIJ_SOCKET_DIR
    mkdir -p "$HOME/.config/tash" "$HOME/.config/fish" "$HOME/.local/share/fish"
    E2E_HISTORY="$HOME/.local/share/fish/fish_history"
    : > "$E2E_HISTORY"
    E2E_INTERACTIVE_SESSION=""
    trap e2e_cleanup EXIT
}

e2e_cleanup() {
    [ -n "${E2E_HOME:-}" ] || return 0
    # Close any interactive session before removing HOME, since the tui-test
    # daemon keeps its reachable state under $HOME/.tui-test.
    e2e_tt_close
    if [ "${E2E_KEEP:-0}" = "1" ]; then
        e2e_log "kept HOME: $E2E_HOME"
    else
        rm -rf "$E2E_HOME"
    fi
}

e2e_history_add() {
    printf -- '- cmd: %s\n  when: %s\n' "$1" "$(date +%s)" >> "$E2E_HISTORY"
}

e2e_write_config() {
    {
        printf 'model:\n'
        printf '  name: %s\n' "$E2E_MODEL"
        printf '  endpoint: %s\n' "$E2E_ENDPOINT"
        printf '  api_key_env: %s\n' "$E2E_API_KEY_ENV"
        printf '  max_tokens: 512\n'
        [ -n "$E2E_MAX_TOKENS_PARAM" ] && printf '  max_tokens_param: %s\n' "$E2E_MAX_TOKENS_PARAM"
        [ -n "$E2E_REASONING_EFFORT" ] && printf '  reasoning_effort: %s\n' "$E2E_REASONING_EFFORT"
        cat <<YAML
behavior:
  max_retries: 3
  max_tool_calls: 5
  auto_intercept: true
  screen_capture: true
  screen_capture_max_lines: 200
  update_check: false
profile:
  rebuild_interval: 31536000
  history_path: $E2E_HISTORY
terminal:
  color: none
log_level: debug
YAML
    } > "$HOME/.config/tash/config.yaml"
}

e2e_init() {
    e2e_history_add "git status"
    e2e_history_add "git log --oneline"
    e2e_history_add "ls -la"
    e2e_write_config
    "$E2E_BIN" init >"$E2E_HOME/init.out" 2>"$E2E_HOME/init.err" \
        || e2e_fail "tash init failed: $(tail -3 "$E2E_HOME/init.err")"
}

# --- tash CLI ----------------------------------------------------------------
# Runs `tash query "$@"`; sets E2E_OUT (stdout), E2E_ERR (stderr), E2E_EXIT.
e2e_query() {
    "$E2E_BIN" query "$@" >"$E2E_HOME/q.out" 2>"$E2E_HOME/q.err"
    E2E_EXIT=$?
    E2E_OUT="$(cat "$E2E_HOME/q.out")"
    E2E_ERR="$(cat "$E2E_HOME/q.err")"
}

# --- assertions --------------------------------------------------------------
e2e_assert_contains() {
    case "$2" in
        *"$1"*) ;;
        *) e2e_fail "expected to find '$1' in: $(printf '%s' "$2" | head -c 400)" ;;
    esac
}

e2e_assert_file() { [ -f "$1" ] || e2e_fail "expected file: $1"; }

e2e_assert_grep() {
    grep -qF -- "$2" "$1" || e2e_fail "expected '$2' in $1"
}

# --- tui-test ----------------------------------------------------------------
e2e_tt() { tui-test --session "$E2E_INTERACTIVE_SESSION" "$@"; }

e2e_tt_open_fish() {
    e2e_require_cmd tui-test
    E2E_INTERACTIVE_SESSION="tash-e2e-$RANDOM$RANDOM"
    # TERM=dumb + NO_COLOR prevent tash/lipgloss from emitting a background-color
    # probe whose async response otherwise leaks into the shell as typed input.
    # ZELLIJ is emptied so tash never tries to capture a real pane. The API key
    # is inherited from the environment, not passed on the command line.
    tui-test --session "$E2E_INTERACTIVE_SESSION" open --shell fish \
        --env "HOME=$E2E_HOME" \
        --env "TERM=dumb" \
        --env "NO_COLOR=1" \
        --env "ZELLIJ=" \
        --cols 120 --rows 40 --restart --wait-ready --timeout-ready "$E2E_TIMEOUT" \
        >/dev/null 2>&1 || e2e_fail "could not open interactive fish session"
}

e2e_tt_expect() {
    e2e_tt expect text "$1" --timeout "$E2E_TIMEOUT" >/dev/null 2>&1 \
        || e2e_fail "expected text not on screen: $1"
}

e2e_tt_type_enter() {
    e2e_tt submit "$1" >/dev/null 2>&1 || e2e_fail "could not submit: $1"
}

e2e_tt_enter() {
    e2e_tt key press Enter >/dev/null 2>&1 || e2e_fail "could not press Enter"
}

# Clears whatever may be sitting in the fish command line buffer.
e2e_tt_clear_line() {
    e2e_tt key press Ctrl+c >/dev/null 2>&1 || true
    sleep 1
}

e2e_tt_screen() { e2e_tt text --full 2>/dev/null || true; }

# Asserts a screen line equals needle exactly. The distinct output line proves a
# suggested command actually ran, rather than matching the echoed input line.
e2e_tt_expect_line() {
    local needle="$1" i=0
    while [ "$i" -lt 30 ]; do
        if e2e_tt_screen | grep -qxF -- "$needle"; then return 0; fi
        sleep 1
        i=$((i + 1))
    done
    e2e_fail "expected a screen line exactly '$needle'"
}

# Asserts needle appears in command output, ignoring echoed input lines (which
# start with the fish prompt). Tolerates missing trailing newlines.
e2e_tt_expect_output() {
    local needle="$1" i=0
    while [ "$i" -lt 30 ]; do
        if e2e_tt_screen | grep -v '^>' | grep -qF -- "$needle"; then return 0; fi
        sleep 1
        i=$((i + 1))
    done
    e2e_fail "expected '$needle' in command output"
}

e2e_tt_close() {
    [ -n "$E2E_INTERACTIVE_SESSION" ] && tui-test --session "$E2E_INTERACTIVE_SESSION" close >/dev/null 2>&1
    E2E_INTERACTIVE_SESSION=""
}

# --- polling -----------------------------------------------------------------
# e2e_wait_until <seconds> <cmd...> ; succeeds as soon as cmd exits 0.
e2e_wait_until() {
    local secs="$1"; shift
    local i=0
    while [ "$i" -lt "$secs" ]; do
        if "$@" >/dev/null 2>&1; then return 0; fi
        sleep 1
        i=$((i + 1))
    done
    return 1
}

e2e_query_calls() {
    jq -r '.query.calls // 0' "$HOME/.config/tash/usage.json" 2>/dev/null || echo 0
}

e2e_query_was_called() { [ "$(e2e_query_calls)" -ge 1 ]; }