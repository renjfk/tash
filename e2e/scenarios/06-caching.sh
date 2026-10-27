#!/usr/bin/env bash
# Prompt caching: with a large, stable prefix the provider serves part of the
# prompt from its cache on a later request. This guards the invariant that
# volatile data (the current time) stays out of the system prompt.
#
# The first request populates the provider cache; usage is then reset so the
# second request's numbers are measured alone. If the timestamp were back in the
# system prompt, the shared prefix would collapse and the cache hit would vanish.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_write_config

# A large stable profile pushes the request past the provider cache threshold
# (~1024 tokens) so results are observable.
{
    echo "# Shell Environment"
    echo "## System"
    echo "- OS: linux/amd64"
    for i in $(seq 1 160); do
        echo "- known tool number $i available in the user workspace environment"
    done
} > "$HOME/.config/tash/profile.md"

e2e_query "Reply with exactly: CACHE_PROBE_ONE"
"$E2E_BIN" usage --reset >/dev/null 2>&1
e2e_query "Reply with exactly: CACHE_PROBE_TWO"

usage="$HOME/.config/tash/usage.json"
e2e_assert_file "$usage"
cached="$(jq -r '.total_cached // 0' "$usage")"
prompt="$(jq -r '.total_prompt // 0' "$usage")"

[ "${cached:-0}" -gt 0 ] || e2e_fail "expected a cache hit on the second request, got $cached cached of $prompt prompt tokens"
# The shared prefix should dominate the prompt. The fixed layout measures ~98%;
# a comfortable threshold catches the timestamp moving back without flapping.
[ $((cached * 100)) -ge $((prompt * 80)) ] \
    || e2e_fail "cache hit too small: $cached of $prompt prompt tokens (<80%)"

e2e_pass "cache served $cached of $prompt prompt tokens on the second request"