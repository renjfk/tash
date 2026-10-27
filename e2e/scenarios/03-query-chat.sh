#!/usr/bin/env bash
# A conversational query prints chat text to stderr and returns no command.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init

e2e_query "Reply with exactly: E2E_CHAT_MARKER"
e2e_assert_contains "E2E_CHAT_MARKER" "$E2E_ERR"
[ -z "$E2E_OUT" ] || e2e_fail "expected no command on stdout for a chat query, got: $E2E_OUT"

e2e_pass "chat query printed the reply"