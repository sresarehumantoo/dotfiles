#!/usr/bin/env bash
# An insteadOf rule in the system gitconfig sends github.com to an internal
# mirror, and github.com itself is blocked: init --clone clones through the mirror,
# and doctor shows it without its credentials.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

trust_ca
policy '*'
stage_noclone
use_nexus
system_git_config url.http://mirror-user:s3cret@gitmirror:8000/github/.insteadOf https://github.com/

out="$("$DFINSTALL" init --clone 2>&1)"
rc=$?
clone="$HOME/.local/share/dfinstall/dotfiles"
check "init --clone succeeds" test "$rc" -eq 0
check "it cloned" test -d "$clone/config"
check "the clone is the dotfiles content" test "$(cat "$XDG_CONFIG_HOME/dfinstall/dotfiles-dir")" = "$clone"
check "nothing was fetched from github.com" lacks "github.com" "$(cat /lab/log/requests)"
check "no packaged content was installed" test ! -e "$HOME/.local/share/dfinstall/content"

mark="$(requests_mark)"
out="$("$DFINSTALL" doctor 2>&1)"
check "doctor shows the mirror" grep -q "git access: git .*rewritten by insteadOf to http://.*gitmirror:8000/github/" <<<"$out"
check "without its password" lacks "s3cret" "$out"
check "doctor makes no request" test -z "$(requests_since "$mark")"

finish
