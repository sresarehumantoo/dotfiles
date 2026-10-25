#!/usr/bin/env bash
# Install a snapshot release's Homebrew formula with this Mac's real Homebrew,
# the way `brew install sresarehumantoo/tap/dfinstall` will after a release.
#
# Usage: scripts/macos-ci/formula.sh <goreleaser-dist-dir>
#
# The formula comes from scripts/release/brew-formula.sh, pointed at the local
# archives, and is installed from a throwaway tap.
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

case "${1:-}" in -h | --help | "") sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;; esac
dist="$(cd "$1" && pwd -P)"

case "$(uname -m)" in arm64) arch=arm64 ;; *) arch=amd64 ;; esac
archive="$(find "$dist" -maxdepth 1 -name "dfinstall_*_darwin_${arch}.tar.gz" | head -1)"
[ -n "$archive" ] || { echo "no darwin_${arch} archive in $dist" >&2; exit 1; }
version="$(basename "$archive" | sed -E "s/^dfinstall_(.*)_darwin_${arch}\.tar\.gz$/\1/")"

export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_ENV_HINTS=1
tap=dfinstall/ci
brew tap-new --no-git "$tap" >/dev/null
formula="$(brew --repository "$tap")/Formula/dfinstall.rb"
"$repo/scripts/release/brew-formula.sh" "v$version" "$dist/checksums.txt" "file://$dist" >"$formula"

echo "== install"
check "brew install $tap/dfinstall" brew install "$tap/dfinstall"
bin="$(brew --prefix)/bin"
check "dfinstall and dfinstall-mcp are linked into $bin" test -x "$bin/dfinstall" -a -x "$bin/dfinstall-mcp"
check_output "--version reports the brew channel" '\(brew\)$' "$bin/dfinstall" --version
check_output "the binary is the one from the Cellar" '/Cellar/dfinstall/' readlink -f "$bin/dfinstall"
check "brew test" brew test "$tap/dfinstall"
check "brew audit --strict" brew audit --strict --formula "$tap/dfinstall"
check "brew style" brew style "$formula"

echo "== update defers to Homebrew"
check "dfinstall init adopts the checkout" "$bin/dfinstall" init --dir "$repo"
check_output "update names brew upgrade instead of replacing the binary" 'brew upgrade dfinstall' \
    "$bin/dfinstall" update htop --dry-run

echo "== uninstall"
check "brew uninstall" brew uninstall "$tap/dfinstall"
check "no dfinstall left in $bin" test ! -e "$bin/dfinstall"
brew untap "$tap" >/dev/null

finish
