#!/usr/bin/env bash
# Auto-intercept: a failing invocation of a real binary is caught by `tash tick`
# (exit code 7), which re-invokes `tash query`.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init
e2e_tt_open_fish

# `git` exists but the subcommand does not -> failures trigger the intercept path.
e2e_tt_type_enter "git definitely-not-a-subcommand"

e2e_wait_until 30 e2e_query_was_called \
    || e2e_fail "auto-intercept did not invoke tash query (query.calls=$(e2e_query_calls))"

e2e_tt_close
e2e_pass "failed command was auto-intercepted"