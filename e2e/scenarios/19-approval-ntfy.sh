#!/usr/bin/env bash
# Full approval workflow against a real self-hosted ntfy server: the query
# blocks, tash publishes a one-click request, an automated approver taps a
# timeframe button (POSTs the body to the button's own URL, as the ntfy app
# does), and the query then completes and persists the granted window.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_require_ntfy

e2e_write_config
# A unique topic per run keeps the scenario hermetic and free of stale backlog.
topic="tash-e2e-${RANDOM}${RANDOM}-$(date +%s)"
reply="$topic-reply"
e2e_approval_config "$topic" "$reply" 60

# Start the query; it blocks in the approval gate until the request is approved.
"$E2E_BIN" query "Give me a one-line shell command that prints exactly E2EAPPROVED" \
    >"$E2E_HOME/query.out" 2>"$E2E_HOME/query.err" &
qpid=$!
trap 'kill "$qpid" 2>/dev/null; e2e_cleanup' EXIT

e2e_ntfy_approve "$topic" 30
approve_rc=$?
case "$approve_rc" in
    0) ;;
    2) e2e_fail "approval button tap failed on $E2E_NTFY_SERVER/$topic: $(tail -3 "$E2E_HOME/query.err" 2>/dev/null)" ;;
    *) e2e_fail "no approval request arrived on $E2E_NTFY_SERVER/$topic: $(tail -3 "$E2E_HOME/query.err" 2>/dev/null)" ;;
esac

e2e_wait_until 90 sh -c "! kill -0 $qpid 2>/dev/null" \
    || e2e_fail "query did not finish after approval"
wait "$qpid"
qexit=$?
[ "$qexit" -eq 0 ] || e2e_fail "query exited $qexit after approval: $(tail -3 "$E2E_HOME/query.err")"
e2e_assert_contains "E2EAPPROVED" "$(cat "$E2E_HOME/query.out")"

# The granted window is persisted with a configured timeframe.
e2e_assert_file "$HOME/.config/tash/approval.json"
tf="$(jq -r '.timeframe' "$HOME/.config/tash/approval.json")"
case "$tf" in
    5m|15m) ;;
    *) e2e_fail "unexpected granted timeframe: '$tf'" ;;
esac

e2e_pass "one-click ntfy approval unlocked the query (granted $tf)"