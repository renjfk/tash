#!/usr/bin/env bash
# Cancelling an in-flight request: pressing Esc during the spinner aborts the
# request and returns to the shell without placing a command.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init
e2e_tt_open_fish

e2e_tt_type_enter "list every single file in this directory right now"
sleep 0.5
e2e_tt key press Escape >/dev/null 2>&1 || e2e_fail "could not press Escape"
e2e_tt_expect "cancelled"

# Buffer must be empty and the shell still usable.
e2e_tt_clear_line
e2e_tt_type_enter "echo CANCEL_ALIVE"
e2e_tt_expect "CANCEL_ALIVE"

e2e_tt_close
e2e_pass "Esc cancelled the in-flight request"