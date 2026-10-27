#!/usr/bin/env bash
# Conversation through the fish hook: an unknown "command" that is really a
# question returns chat text instead of a command.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init
e2e_tt_open_fish

e2e_tt_type_enter "what is the capital of France"
e2e_tt_expect "Paris"

e2e_tt_close
e2e_pass "fish hook returned a chat reply"