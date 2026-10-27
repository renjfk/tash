#!/usr/bin/env bash
# End-to-end through the fish hook: typing an unknown command triggers
# fish_command_not_found, tash queries the model, and the suggested command is
# placed into the fish command line buffer via `commandline -r`. Running it must
# produce the marker on its own line (proving the buffer was a real command).
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init
e2e_tt_open_fish

e2e_tt_type_enter "give me a shell command that prints exactly E2EFISHMARKER"
e2e_wait_until 30 e2e_query_was_called || e2e_fail "fish-hook query did not complete"
sleep 1
e2e_tt_enter                 # run the suggested command now in the buffer
e2e_tt_expect_output "E2EFISHMARKER"

e2e_tt_close
e2e_pass "fish hook placed and ran the suggested command"