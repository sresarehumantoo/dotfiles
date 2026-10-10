#!/bin/sh
# Shown while a tunnel interface carries an IPv4 address. macOS keeps several
# utun interfaces up for its own use, but those only have IPv6 link-locals.
if ifconfig | awk '/^utun/ { tun = 1; next } /^[a-z]/ { tun = 0 } tun && /inet / { found = 1 } END { exit !found }'; then
  sketchybar --set "$NAME" drawing=on
else
  sketchybar --set "$NAME" drawing=off
fi
