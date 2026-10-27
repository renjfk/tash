#!/usr/bin/env bash
# Memory: asking the model to remember something stores a durable memory entry
# in the append-only conversation log.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init

e2e_query "Please remember that my favorite editor is vim, then reply ok" >/dev/null 2>&1

conf="$HOME/.config/tash/conversation.jsonl"
e2e_assert_file "$conf"
e2e_assert_grep "$conf" "favorite editor is vim"

e2e_pass "memory was stored to the conversation log"