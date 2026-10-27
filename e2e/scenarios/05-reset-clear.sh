#!/usr/bin/env bash
# tash reset clears the conversation but keeps memories; tash clear removes all.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init

# Seed a memory + a conversation entry.
e2e_query "Please remember that my favorite number is 42, then reply ok" >/dev/null 2>&1
conf="$HOME/.config/tash/conversation.jsonl"
e2e_assert_file "$conf"
e2e_assert_grep "$conf" "42"

"$E2E_BIN" reset >/dev/null 2>&1
after_reset="$(cat "$conf")"
e2e_assert_contains "42" "$after_reset"   # memory preserved

"$E2E_BIN" clear >/dev/null 2>&1
after_clear="$(cat "$conf" 2>/dev/null || true)"
case "$after_clear" in
    *42*) e2e_fail "clear should remove memories too" ;;
esac

e2e_pass "reset keeps memories, clear removes everything"