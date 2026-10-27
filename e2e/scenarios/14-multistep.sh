#!/usr/bin/env bash
# Multi-step commands prompt for interactive approval of intermediate steps and
# leave the final command in the fish buffer.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init
e2e_tt_open_fish

e2e_tt_type_enter "Give me a sequence of exactly two commands: the first prints MULTI_ONE, the second prints MULTI_TWO"
e2e_wait_until 30 e2e_query_was_called || e2e_fail "multi-step query did not complete"
sleep 1

# The first (intermediate) step is prompted and, once accepted, runs inline.
e2e_tt_enter
e2e_tt_expect_line "MULTI_ONE"

# The last command is placed in the buffer; running it prints the second marker.
e2e_tt_enter
e2e_tt_expect_line "MULTI_TWO"

e2e_tt_close
e2e_pass "multi-step flow prompted and queued the final command"