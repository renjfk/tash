#!/usr/bin/env bash
# tash usage reports recorded token usage, including cache hits.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init              # records a rebuild call
e2e_query "say hi"    # records a query call

out="$("$E2E_BIN" usage 2>&1)"
e2e_assert_contains "Token usage" "$out"
e2e_assert_contains "query" "$out"
e2e_assert_contains "Cached" "$out"

# Reset clears the counters.
"$E2E_BIN" usage --reset >/dev/null 2>&1
after="$("$E2E_BIN" usage 2>&1)"
e2e_assert_contains "no usage recorded" "$after"

e2e_pass "usage reports counters and resets"