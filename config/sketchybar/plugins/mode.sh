#!/bin/sh
# Shows the AeroSpace binding mode while it is anything but main, like sway/mode.
mode="$(aerospace list-modes --current)"
if [ "$mode" = main ] || [ -z "$mode" ]; then
  sketchybar --set "$NAME" drawing=off
else
  sketchybar --set "$NAME" drawing=on label="$mode"
fi
