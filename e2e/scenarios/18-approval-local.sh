#!/usr/bin/env bash
# Approval gate: with no window the gate blocks the query (no model call), and
# a locally granted window lets the query through without ever reaching ntfy.
# The ntfy server is deliberately unreachable: a granted window must be honored
# without contacting it, so any accidental request would fail the scenario.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_write_config
cat >> "$HOME/.config/tash/config.yaml" <<YAML
approval:
  enabled: true
  timeframes: ["5m", "15m"]
  wait_timeout: 5
  ntfy:
    server: http://127.0.0.1:9
    topic: tash-e2e-local
YAML

# Closed gate: the query fails before any AI work, reporting the ntfy failure.
"$E2E_BIN" query "print exactly E2ELOCAL" >"$E2E_HOME/blocked.out" 2>"$E2E_HOME/blocked.err"
blocked_exit=$?
[ "$blocked_exit" -ne 0 ] || e2e_fail "query should fail while the approval gate is closed"
e2e_assert_contains "request approval" "$(cat "$E2E_HOME/blocked.err")"
# The gate must block before any AI work.
[ "$(e2e_query_calls)" -eq 0 ] || e2e_fail "closed gate still called the model"

# Status reflects the closed gate.
out="$("$E2E_BIN" approval 2>&1)"
e2e_assert_contains "no active approval window" "$out"

# A grant is limited to the configured timeframes.
"$E2E_BIN" approval --grant 2h >"$E2E_HOME/badgrant.err" 2>&1 \
    && e2e_fail "granting an unconfigured duration should fail"
e2e_assert_contains "approval.timeframes" "$(cat "$E2E_HOME/badgrant.err")"

# Grant a configured timeframe, then the query proceeds without ntfy.
"$E2E_BIN" approval --grant 15m >/dev/null 2>&1 || e2e_fail "approval --grant failed"
out="$("$E2E_BIN" approval 2>&1)"
e2e_assert_contains "usage approved for 15m" "$out"

e2e_query "Give me a one-line shell command that prints exactly E2ELOCAL"
[ "$E2E_EXIT" -eq 0 ] || e2e_fail "query exited $E2E_EXIT with an open window: $E2E_ERR"
e2e_assert_contains "E2ELOCAL" "$E2E_OUT"

# Reset clears the window.
"$E2E_BIN" approval --reset >/dev/null 2>&1 || e2e_fail "approval --reset failed"
out="$("$E2E_BIN" approval 2>&1)"
e2e_assert_contains "no active approval window" "$out"

e2e_pass "gate blocked without a window and was bypassed by a local grant"