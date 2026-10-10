#!/bin/sh
# Workspace chip $1: lit when focused, text when it holds windows, dim when
# empty. FOCUSED_WORKSPACE comes with the aerospace_workspace_change trigger;
# every other event has to ask.
. "$CONFIG_DIR/colors.sh"

focused="${FOCUSED_WORKSPACE:-$(aerospace list-workspaces --focused)}"
if [ "$1" = "$focused" ]; then
  sketchybar --set "$NAME" background.drawing=on label.color="$TEXT"
elif [ -n "$(aerospace list-windows --workspace "$1" 2>/dev/null)" ]; then
  sketchybar --set "$NAME" background.drawing=off label.color="$SUBTEXT"
else
  sketchybar --set "$NAME" background.drawing=off label.color="$OVERLAY"
fi
