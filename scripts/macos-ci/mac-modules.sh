#!/usr/bin/env bash
# Install and uninstall the macOS-only modules for real, as the runner's
# logged-in user, and check what each leaves behind: the defaults keys,
# launch agents, Homebrew services and links that scripts/macos-harness can
# only record. Run on a throwaway runner: this changes the account's settings.
#
# Usage: scripts/macos-ci/mac-modules.sh [log-dir]
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

case "${1:-}" in -h | --help) sed -n '2,7p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;; esac
logs="$(mkdir -p "${1:-/tmp/dfinstall-mac-modules}" && cd "${1:-/tmp/dfinstall-mac-modules}" && pwd -P)"

export DOTFILES="$repo"
(cd "$repo" && make build -s)
df="$repo/bin/dfinstall"
gui="gui/$(id -u)"

agent_loaded() { launchctl print "$gui/$1" >/dev/null 2>&1; }
pref() { defaults read "$1" "$2" 2>/dev/null; }

run() {
    local name="$1"
    shift
    if "$df" "$@" </dev/null >"$logs/$name.log" 2>&1; then
        pass "dfinstall $*"
    else
        fail "dfinstall $*"
        sed 's/\x1b\[[0-9;]*m//g' "$logs/$name.log" | tail -25 | sed 's/^/      /'
    fi
}

echo "== mackeys"
run mackeys-install install mackeys
check_output "KeyRepeat is 2" '^2$' pref NSGlobalDomain KeyRepeat
check_output "press-and-hold is off" '^0$' pref NSGlobalDomain ApplePressAndHoldEnabled
check_output "Globe does nothing" '^0$' pref com.apple.HIToolbox AppleFnUsageType
check_output "app shortcuts are set in NSGlobalDomain" 'NSUserKeyEquivalents' defaults read NSGlobalDomain
check "the login helper agent is loaded" agent_loaded io.github.sresarehumantoo.mackeys
check "the helper's plist is in ~/Library/LaunchAgents" test -f "$HOME/Library/LaunchAgents/io.github.sresarehumantoo.mackeys.plist"
run mackeys-again install mackeys
check_output "a second install mackeys changes nothing" 'Unchanged' cat "$logs/mackeys-again.log"
run mackeys-uninstall uninstall mackeys
check "KeyRepeat is removed" sh -c '! defaults read NSGlobalDomain KeyRepeat >/dev/null 2>&1'
check "the app shortcuts are removed" sh -c '! defaults read NSGlobalDomain NSUserKeyEquivalents >/dev/null 2>&1'
check "the login helper agent is gone" sh -c "! launchctl print $gui/io.github.sresarehumantoo.mackeys >/dev/null 2>&1"
check "the helper's plist is gone" test ! -e "$HOME/Library/LaunchAgents/io.github.sresarehumantoo.mackeys.plist"

echo "== alttab"
run alttab-install install alttab
check "the alttab agent is loaded" agent_loaded io.github.sresarehumantoo.alttab
run alttab-uninstall uninstall alttab
check "the alttab agent is gone" sh -c "! launchctl print $gui/io.github.sresarehumantoo.alttab >/dev/null 2>&1"

echo "== sketchybar"
run sketchybar-install install sketchybar
check_output "the sketchybar service is started" '^sketchybar +started' brew services list
check "sketchybarrc links into the checkout" sh -c "readlink '$HOME/.config/sketchybar/sketchybarrc' | grep -q '^$repo/config/'"
run sketchybar-uninstall uninstall sketchybar
check_output "the sketchybar service is stopped" '^sketchybar +(none|stopped)' brew services list
check "sketchybarrc is unlinked" test ! -L "$HOME/.config/sketchybar/sketchybarrc"

echo "== aerospace"
run aerospace-install install aerospace
check "AeroSpace.app is installed" sh -c "test -d /Applications/AeroSpace.app || test -d '$HOME/Applications/AeroSpace.app'"
check "aerospace.toml links into the checkout" sh -c "readlink '$HOME/.config/aerospace/aerospace.toml' | grep -q '^$repo/config/'"
run aerospace-uninstall uninstall aerospace
check "aerospace.toml is unlinked" test ! -L "$HOME/.config/aerospace/aerospace.toml"

finish
