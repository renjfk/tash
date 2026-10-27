#!/usr/bin/env bash
# Real Zellij screen capture: run fish inside a Zellij pane, put a marker on the
# screen, then ask tash to read it. tash captures the pane via
# `zellij action dump-screen`; the marker must appear in the injected screen
# context.
#
# Container-only for safety: driving Zellij from a host with a live session can
# read or kill the developer's sessions. This runs only inside a container with
# E2E_ALLOW_ZELLIJ=1 (set by the Docker image), and it isolates Zellij via
# ZELLIJ_SOCKET_DIR so it can never reach a host session.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

[ "${E2E_ALLOW_ZELLIJ:-0}" = "1" ] || e2e_skip "zellij capture is opt-in (set E2E_ALLOW_ZELLIJ=1)"
{ [ -f /.dockerenv ] || [ -f /run/.containerenv ]; } || e2e_skip "zellij capture is container-only"

e2e_require_cmd zellij
e2e_require_cmd tui-test
e2e_setup
e2e_init

# Dedicated socket directory: every zellij command below only sees our session.
export ZELLIJ_SOCKET_DIR="$E2E_HOME/zellij"
mkdir -p "$ZELLIJ_SOCKET_DIR"

layout="$E2E_HOME/layout.kdl"
cat > "$layout" <<'KDL'
layout {
    pane command="fish" {
    }
}
KDL

E2E_INTERACTIVE_SESSION="tash-e2e-zellij-$RANDOM$RANDOM"

# A real client lets Zellij start the pane (background sessions keep panes
# suspended) so tash's dump-screen has content to read.
tui-test --session "$E2E_INTERACTIVE_SESSION" run --no-wait-ready --cols 120 --rows 40 \
    --env "HOME=$E2E_HOME" --env "TERM=dumb" --env "NO_COLOR=1" \
    --env "ZELLIJ_SOCKET_DIR=$ZELLIJ_SOCKET_DIR" \
    -- zellij --layout "$layout" >/dev/null 2>&1 \
    || e2e_fail "could not start zellij session"

sleep 3
e2e_tt key press Enter >/dev/null 2>&1 || true   # dismiss the release-notes pane
sleep 1

e2e_tt submit "printf 'SCREENSEEK_MARK\n'" >/dev/null 2>&1
sleep 1
e2e_tt submit "please tell me what is visible on my screen right now" >/dev/null 2>&1

e2e_wait_until 40 e2e_query_was_called || e2e_fail "zellij query did not complete"
sleep 1

log="$E2E_HOME/.config/tash/tash.log"
e2e_assert_grep "$log" "initial screen context injected"

# The injected context must contain the on-screen marker, proving the capture
# read the real pane rather than falling back.
grep -q "Terminal screen" "$log" || e2e_fail "screen context was not injected"
grep -q "SCREENSEEK_MARK" "$log" || e2e_fail "screen context did not contain the on-screen marker"

e2e_tt_close
# Isolated socket dir: only our session is visible, so cleaning it is safe.
zsess="$(zellij list-sessions 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' | awk 'NR==1{print $1}')"
[ -n "$zsess" ] && zellij kill-session "$zsess" >/dev/null 2>&1

e2e_pass "zellij screen capture included the on-screen marker"