#!/bin/sh
# Wi-Fi, wired or offline, from the interface carrying the default route. No
# signal strength: macOS removed the airport tool that reported it.
iface="$(route -n get default 2>/dev/null | awk '/interface:/ {print $2}')"
if [ -z "$iface" ]; then
  icon=󰤮
elif networksetup -listallhardwareports | awk -v d="$iface" '
    /^Hardware Port:/ { port = $0 }
    $0 == "Device: " d { found = (port ~ /Wi-Fi/) }
    END { exit !found }'; then
  icon=󰤨
else
  icon=󰈀
fi
sketchybar --set "$NAME" icon="$icon"
