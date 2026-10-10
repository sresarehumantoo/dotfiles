#!/bin/sh
# Hidden on a desktop Mac, which reports no battery.
. "$CONFIG_DIR/colors.sh"

batt="$(pmset -g batt)"
pct="$(printf '%s\n' "$batt" | grep -Eo '[0-9]+%' | head -n1 | tr -d %)"
if [ -z "$pct" ]; then
  sketchybar --set "$NAME" drawing=off
  exit 0
fi

case "$pct" in
  100|9[0-9]) icon=󰁹 ;;
  [7-8][0-9]) icon=󰂀 ;;
  [5-6][0-9]) icon=󰁾 ;;
  [3-4][0-9]) icon=󰁼 ;;
  *)          icon=󰁺 ;;
esac
color="$TEXT"
[ "$pct" -le 25 ] && color="$YELLOW"
[ "$pct" -le 10 ] && color="$RED"
if printf '%s\n' "$batt" | grep -q 'AC Power'; then
  icon=󰂄
  color="$TEXT"
fi
sketchybar --set "$NAME" drawing=on icon="$icon" icon.color="$color" label="${pct}%"
