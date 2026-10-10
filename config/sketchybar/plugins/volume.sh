#!/bin/sh
# volume_change passes the level in $INFO; mute is not part of the event.
vol="${INFO:-$(osascript -e 'output volume of (get volume settings)')}"
muted="$(osascript -e 'output muted of (get volume settings)')"

if [ "$muted" = true ] || [ "$vol" = 0 ]; then
  icon=󰖁
elif [ "$vol" -ge 60 ]; then
  icon=󰕾
elif [ "$vol" -ge 30 ]; then
  icon=󰖀
else
  icon=󰕿
fi
sketchybar --set "$NAME" icon="$icon"
