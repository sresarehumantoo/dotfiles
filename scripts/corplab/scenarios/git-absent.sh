#!/usr/bin/env bash
# No git on the workstation and the internet blocked: init uses the release's
# content archive, fetched through Nexus, and the modules that need git say so
# and are skipped.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

trust_ca
policy '*'
stage_noclone
use_nexus
without_git
check "git is not on PATH" bash -c '! command -v git'

out="$("$DFINSTALL" init 2>&1)"
rc=$?
check "init succeeds" test "$rc" -eq 0
check "it says why it did not clone" grep -q "not cloning .*git is not on PATH; using the content packaged" <<<"$out"
check "the store is the dotfiles content" \
    test "$(cat "$XDG_CONFIG_HOME/dfinstall/dotfiles-dir")" = "$HOME/.local/share/dfinstall/content/current"

out="$("$DFINSTALL" install all 2>&1)"
rc=$?
check "install all succeeds" test "$rc" -eq 0
check "oh-my-zsh is skipped for want of git" grep -q "oh-my-zsh needs git: git is not on PATH" <<<"$out"
check "tmux plugins are skipped for want of git" grep -q "skipped TPM and tmux plugins, which need git: git is not on PATH" <<<"$out"
check "the zshrc links into the store" test "$(readlink "$HOME/.zshrc")" = "$HOME/.local/share/dfinstall/content/current/config/shell/zshrc"

mark="$(requests_mark)"
out="$("$DFINSTALL" doctor 2>&1)"
check "doctor reports git absent" grep -q "git access: absent: git is not on PATH" <<<"$out"
check "doctor makes no request" test -z "$(requests_since "$mark")"

finish
