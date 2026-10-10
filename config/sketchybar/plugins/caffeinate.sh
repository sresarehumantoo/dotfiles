#!/bin/sh
# waybar's idle_inhibitor: a background `caffeinate -dims` while switched on.
# `toggle` is the click; with no argument it only redraws.
. "$CONFIG_DIR/colors.sh"
pidfile="${TMPDIR:-/tmp}/sketchybar-caffeinate.pid"

running() { [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; }

if [ "$1" = toggle ]; then
  if running; then
    kill "$(cat "$pidfile")"
    rm -f "$pidfile"
  else
    nohup caffeinate -dims >/dev/null 2>&1 &
    echo $! > "$pidfile"
  fi
fi

if running; then
  sketchybar --set caffeinate icon=󰅶 icon.color="$YELLOW"
else
  sketchybar --set caffeinate icon=󰾪 icon.color="$TEXT"
fi
