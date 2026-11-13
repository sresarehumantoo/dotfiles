#!/usr/bin/env bash
# git allowed but github.com dropped at the firewall, with no mirror: a plain
# init fails and suggests --packaged rather than falling back, since an
# unreachable host may only be down for now.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

trust_ca
policy drop:github.com
stage_noclone
use_nexus

out="$("$DFINSTALL" init 2>&1)"
rc=$?
check "init fails" test "$rc" -ne 0
check "it suggests --packaged" grep -q "cannot reach https://github.com/sresarehumantoo/dotfiles.git: .*run: dfinstall init --packaged" <<<"$out"
check "nothing is recorded as the dotfiles content" test ! -e "$XDG_CONFIG_HOME/dfinstall/dotfiles-dir"
check "no packaged content was installed" test ! -e "$HOME/.local/share/dfinstall/content"

mark="$(requests_mark)"
out="$("$DFINSTALL" doctor 2>&1)"
check "doctor finds git usable" grep -q "git access: git .*https allowed" <<<"$out"
check "doctor makes no request" test -z "$(requests_since "$mark")"

finish
