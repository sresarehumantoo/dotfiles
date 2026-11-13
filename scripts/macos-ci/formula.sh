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
share="$(brew --prefix dfinstall)/share/dfinstall"
check "the content stamp is installed to $share" test -f "$share/.dfinstall-content.json"
check "a config file and an executable script are installed" \
    test -f "$share/config/shell/zshrc" -a -x "$share/config/devtools/_lib.sh"
check "brew test" brew test "$tap/dfinstall"
check "brew audit --strict" brew audit --strict --formula "$tap/dfinstall"
check "brew style" brew style "$formula"

echo "== packaged content"
# A fresh home, so nothing the runner already has decides where links point.
home="$(mktemp -d)"
home="$(cd "$home" && pwd -P)"
store="$home/.local/share/dfinstall/content"
in_home() {
    (cd "$home" && env -u DOTFILES HOME="$home" XDG_CONFIG_HOME="$home/.config" XDG_DATA_HOME="$home/.local/share" \
        XDG_STATE_HOME="$home/.local/state" XDG_CACHE_HOME="$home/.cache" "$@")
}
mkdir -p "$home/.config/dfinstall"
printf 'skip_backup: true\n' >"$home/.config/dfinstall/config.yaml"
# Every symlink under the home and its target, leaving out the store's own
# current link.
links() { find "$home" -path "$store" -prune -o -type l -print | while read -r l; do printf '%s -> %s\n' "$l" "$(readlink "$l")"; done; }
dangling() { find "$home" -type l | while read -r l; do [ -e "$l" ] || echo "$l"; done; }

check "init --packaged" in_home "$bin/dfinstall" init --packaged
check "the store is the dotfiles content" \
    test "$(cat "$home/.config/dfinstall/dotfiles-dir")" = "$store/current"
check "the store holds the snapshot's version" test -f "$store/v$version/.dfinstall-content.json"
check "install shell htop" in_home "$bin/dfinstall" install shell htop
check "the zshrc links into the store" test "$(readlink "$home/.zshrc")" = "$store/current/config/shell/zshrc"
check "htoprc is a seeded copy" test -f "$home/.config/htop/htoprc" -a ! -L "$home/.config/htop/htoprc"
check "the install made links" test "$(links | grep -c "$store/current/")" -gt 3
check "no link points into the Cellar" test -z "$(links | grep /Cellar/)"
links | grep /Cellar/ || true
check_output "doctor names the packaged content" "packaged content v$version " in_home "$bin/dfinstall" doctor

echo "== update defers to Homebrew"
check "dfinstall init adopts the checkout" "$bin/dfinstall" init --dir "$repo"
# A snapshot build has the latest tag's version, so update reports it current.
check_output "update names brew upgrade, or reports it current, instead of replacing the binary" \
    'brew upgrade dfinstall|is the latest release' \
    "$bin/dfinstall" update htop --dry-run

echo "== uninstall"
check "brew uninstall" brew uninstall "$tap/dfinstall"
check "no dfinstall left in $bin" test ! -e "$bin/dfinstall"
check "the packaged links outlive the keg" test -z "$(dangling)"
dangling
brew untap "$tap" >/dev/null

finish
