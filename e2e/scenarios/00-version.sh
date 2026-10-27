#!/usr/bin/env bash
# tash version prints a version string.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

out="$("$E2E_BIN" version 2>&1)"
e2e_assert_contains "tash" "$out"
e2e_pass "version: $out"