#!/usr/bin/env bash
# Temporary: collects what is needed to debug the mackeys command layout on a
# Mac. Remove once the layout works.
#
#   scripts/mac-layout-debug.sh [JIRA_URL]       report, with Chrome restarted
#   scripts/mac-layout-debug.sh --manual          force the command layout on every
#                                                 built-in HID service, by hand
#   scripts/mac-layout-debug.sh --restore         undo --manual
#
# The report is also saved to $TMPDIR/mackeys-debug.txt.
set -uo pipefail

LABEL=io.github.sresarehumantoo.mackeys-layout
DOMAIN="gui/$(id -u)"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
BIN="$HOME/.local/bin/mackeys-layout"
CONF="$HOME/Library/Application Support/dfinstall/mackeys-layout.json"
LOG="$HOME/Library/Logs/mackeys-layout.log"
MATCH='{"PrimaryUsagePage":1,"PrimaryUsage":6,"Built-In":true}'
# Every built-in HID service: the Globe key may come from a vendor service
# rather than the keyboard one MATCH selects.
ALL='{"Built-In":true}'
GLOBE=1095216660483
CMD=30064771299
CTRL=30064771296
OUT="${TMPDIR:-/tmp}/mackeys-debug.txt"

usage() {
	sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'
}

section() { printf '\n== %s\n' "$1"; }

mapping() {
	hidutil property --matching "$MATCH" --get HIDKeyboardModifierMappingPairs 2>&1
}

# Which modifier the Globe key (the Ctrl position) sends right now.
globe_sends() {
	local m dst
	m=$(mapping | tr -d '\n')
	dst=$(printf '%s' "$m" | grep -oE "Dst = [0-9]+; HIDKeyboardModifierMappingSrc = $GLOBE;" | grep -oE '[0-9]+' | head -1)
	case $dst in
	"$CMD") echo "Command (command layout)" ;;
	"$CTRL") echo "Control (standard layout)" ;;
	"") echo "unknown: no Globe entry parsed, see the raw mapping" ;;
	*) echo "unexpected Dst $dst" ;;
	esac
}

layout() {
	plutil -extract "$1" raw -o - "$CONF"
}

case ${1:-} in
-h | --help)
	usage
	exit 0
	;;
--manual)
	launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null
	hidutil property --matching "$ALL" --set "$(layout command)" >/dev/null || exit 1
	echo "Command layout forced on every built-in service, agent stopped."
	echo "If it took, the Ctrl key now sends Cmd everywhere, this terminal included."
	hidutil property --matching "$ALL" --get HIDKeyboardModifierMappingPairs
	echo
	echo "1. Here in Ghostty: Ctrl key + C should now act as Cmd+C (copy), not interrupt."
	echo "2. In Jira: Ctrl key + Z with the console trace open; look for meta true."
	echo "Then run: $0 --restore"
	exit 0
	;;
--restore)
	hidutil property --matching "$ALL" --set "$(layout standard)" >/dev/null
	launchctl bootstrap "$DOMAIN" "$PLIST" 2>/dev/null
	echo "Standard layout restored, agent started. Globe sends: $(globe_sends)"
	exit 0
	;;
esac

URL=${1:-}

{
	section "system"
	sw_vers
	echo "uid $(id -u), swiftc: $(command -v swiftc || echo MISSING)"

	section "files"
	ls -la "$BIN" "$PLIST" "$CONF" "$LOG" 2>&1
	echo "LaunchAgents:"
	ls "$HOME/Library/LaunchAgents/" 2>&1

	section "built-in HID services"
	hidutil list --matching "$ALL" 2>&1

	section "agent"
	launchctl print "$DOMAIN/$LABEL" 2>&1 | grep -E 'state|pid =|last exit|program' || echo "not loaded"

	section "config"
	cat "$CONF" 2>&1

	section "chrome"
	echo "bundle id: $(osascript -e 'id of app "Google Chrome"' 2>&1)"
	echo "Copy shortcut in Chrome's table: $(defaults read com.google.Chrome NSUserKeyEquivalents 2>/dev/null | grep -E '^ *Copy =' || echo none)"

	section "mapping with this terminal in front"
	echo "Globe sends: $(globe_sends)"

	section "restarting Chrome"
	osascript -e 'tell application "Google Chrome" to quit' 2>&1
	for _ in 1 2 3 4 5 6 7 8 9 10; do
		pgrep -x "Google Chrome" >/dev/null || break
		sleep 1
	done
	if [ -n "$URL" ]; then
		open -a "Google Chrome" "$URL"
	else
		open -a "Google Chrome"
	fi
	sleep 4
	echo "frontmost: $(lsappinfo info -only bundleid "$(lsappinfo front)" 2>&1)"

	section "mapping with Chrome in front"
	echo "Globe sends: $(globe_sends)"
	mapping
	echo "every built-in service:"
	hidutil property --matching "$ALL" --get HIDKeyboardModifierMappingPairs 2>&1

	section "agent log (last 30 lines)"
	tail -n 30 "$LOG" 2>&1
} 2>&1 | tee "$OUT"

open -a "$TERM_PROGRAM" 2>/dev/null || true
printf '\nSaved to %s\n' "$OUT"
