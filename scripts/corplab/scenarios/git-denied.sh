#!/usr/bin/env bash
# git present but its https transport denied by the system gitconfig, and the
# internet blocked: init uses the release's content archive and names the
# setting, and the modules that need git are skipped.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

trust_ca
policy '*'
stage_noclone
use_nexus
system_git_config protocol.https.allow never

out="$("$DFINSTALL" init 2>&1)"
rc=$?
check "init succeeds" test "$rc" -eq 0
check "it names the setting that stopped the clone" grep -q "not cloning .*protocol.https.allow=never.*using the content packaged" <<<"$out"
check "the store is the dotfiles content" \
    test "$(cat "$XDG_CONFIG_HOME/dfinstall/dotfiles-dir")" = "$HOME/.local/share/dfinstall/content/current"

out="$("$DFINSTALL" install all 2>&1)"
rc=$?
check "install all succeeds" test "$rc" -eq 0
check "oh-my-zsh is skipped, naming the setting" grep -q "oh-my-zsh needs git: .*protocol.https.allow=never" <<<"$out"
check "tmux plugins are skipped, naming the setting" grep -q "skipped TPM and tmux plugins, which need git: .*protocol.https.allow=never" <<<"$out"

mark="$(requests_mark)"
out="$("$DFINSTALL" doctor 2>&1)"
check "doctor reports the policy" grep -q "git access: denied-by-policy: .*protocol.https.allow=never" <<<"$out"
check "doctor makes no request" test -z "$(requests_since "$mark")"

finish
