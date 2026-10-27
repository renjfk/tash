#!/usr/bin/env bash
# A query asking for a command returns it on stdout (the fish buffer payload).
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init

e2e_query "Give me a one-line shell command that prints exactly E2E_MARKER_CMD"
[ "$E2E_EXIT" -eq 0 ] || e2e_fail "query exited $E2E_EXIT: $E2E_ERR"
e2e_assert_contains "E2E_MARKER_CMD" "$E2E_OUT"

e2e_pass "command query returned: $(printf '%s' "$E2E_OUT" | head -c 120)"