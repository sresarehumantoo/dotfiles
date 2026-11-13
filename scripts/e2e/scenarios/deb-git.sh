#!/usr/bin/env bash
# git moving from Depends to Recommends: where git came in only as a dfinstall
# dependency, upgrading to a package that recommends it must not let
# apt-get autoremove take it. A package with no git relation at all is the
# control, so a pass cannot come from something else keeping git.
# e2e: packages
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

content=/e2e/pkg/1/content
build_deb 1 $content /tmp/depends.deb $'Depends: git, curl'
build_deb 2 $content /tmp/recommends.deb
build_deb 1 $content /tmp/none.deb $'Depends: curl'
apt_install() { apt-get install -y -q --allow-downgrades "$1" >/dev/null 2>&1; }
# autoremoves PKG: a simulated autoremove would remove PKG.
autoremoves() {
    local plan
    plan="$(apt-get -s autoremove 2>/dev/null)"
    grep -q "^Remv $1 " <<<"$plan"
}
keeps() { ! autoremoves "$1"; }
lacks_git() { ! grep -Eq '^(Depends|Recommends):.*\bgit\b' <<<"$(dpkg -s dfinstall)"; }

apt-mark auto git >/dev/null
check "2.x package (Depends: git) installs" apt_install /tmp/depends.deb
check "it keeps git from autoremove" keeps git

check "3.0 package (Recommends: git) upgrades it" apt_install /tmp/recommends.deb
check "the installed package recommends git" grep -qx 'Recommends: git' <<<"$(dpkg -s dfinstall)"
check "Recommends keeps git from autoremove" keeps git

# Downgraded so the version differs from the installed one.
check "control package (no git) installs" apt_install /tmp/none.deb
check "the installed package has no git relation" lacks_git
check "without it, autoremove would take git" autoremoves git

finish
