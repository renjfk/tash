#!/usr/bin/env bash
# tash init writes config, generates the profile via the model, and installs
# the fish hook.
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

e2e_setup
e2e_init

e2e_assert_file "$HOME/.config/tash/config.yaml"
e2e_assert_file "$HOME/.config/tash/profile.md"
e2e_assert_file "$HOME/.config/fish/conf.d/tash.fish"
e2e_assert_grep "$HOME/.config/fish/conf.d/tash.fish" "fish_command_not_found"
e2e_assert_grep "$HOME/.config/fish/conf.d/tash.fish" "commandline -r"

# Profile must be non-empty (the model produced something).
[ -s "$HOME/.config/tash/profile.md" ] || e2e_fail "profile.md is empty"

e2e_pass "init created config, profile, and fish integration"