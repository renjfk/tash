#!/usr/bin/env bash
# Screen context: outside Zellij, tash must degrade gracefully (no crash) and
# still answer. Exercises the screen-context plumbing and its fallback.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init
e2e_tt_open_fish

e2e_tt_type_enter "is there an error on my screen"
e2e_wait_until 30 e2e_query_was_called \
    || e2e_fail "screen-context query did not complete"

# The model may have left a suggested command in the buffer; clear it, then
# confirm the session is still responsive.
e2e_tt_clear_line
e2e_tt_type_enter "echo SCREEN_ALIVE"
e2e_tt_expect "SCREEN_ALIVE"

e2e_tt_close
e2e_pass "screen-context query handled without Zellij"