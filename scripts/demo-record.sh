#!/usr/bin/env bash
# Re-shoot the README demo GIF. Scripted, not hand-recorded, so it regenerates
# verbatim when behavior changes. Nothing is typed by hand: every frame is real
# dfinstall output.
#
#   scripts/demo-record.sh            # writes assets/demo.gif
#
# Needs: asciinema, agg. Records dfinstall against a throwaway $HOME so the real
# machine is never touched; status/doctor/diff only read, install uses --dry-run.
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cast="$(mktemp -t dfinstall-demo.XXXXXX.cast)"
out="${1:-$repo/assets/demo.gif}"

command -v asciinema >/dev/null || { echo "need asciinema" >&2; exit 1; }
command -v agg >/dev/null || { echo "need agg (asciinema gif generator)" >&2; exit 1; }
[ -x "$repo/bin/dfinstall" ] || { echo "build first: make build" >&2; exit 1; }

# The driver runs inside the recording. It prints a styled prompt and each
# command, then runs the real binary, against a sandbox HOME wired up here.
driver="$(mktemp -t dfinstall-driver.XXXXXX.sh)"
# Pin DOTFILES to a pristine clone at the latest tag so the recording is stable
# across re-shoots: doctor's clone-freshness then reads a clean, upstream-tracked
# checkout instead of whatever branch happens to be checked out here.
# Release tags sit on main's merge commits, which develop never contains, so a
# describe from develop's HEAD would find a tag many releases old.
tag="$(git -C "$repo" describe --tags --abbrev=0 origin/main 2>/dev/null || echo main)"
clone="$(mktemp -d -t dfinstall-src.XXXXXX)"
git clone --quiet --no-hardlinks "$repo" "$clone"
git -C "$clone" checkout --quiet "$tag"
git -C "$clone" remote set-url origin https://github.com/sresarehumantoo/dotfiles.git

cat >"$driver" <<DRIVER
set -e
export DOTFILES="$clone"
H="\$(mktemp -d -t dfinstall-home.XXXXXX)"
export HOME="\$H"
export XDG_CONFIG_HOME="\$H/.config" XDG_DATA_HOME="\$H/.local/share"
export XDG_STATE_HOME="\$H/.local/state" XDG_CACHE_HOME="\$H/.cache"
unset TMUX_PLUGIN_MANAGER_PATH
mkdir -p "\$XDG_CONFIG_HOME" "\$XDG_DATA_HOME" "\$XDG_STATE_HOME" "\$XDG_CACHE_HOME"

df="$repo/bin/dfinstall"
P='\033[38;5;39m❯\033[0m'          # blue prompt arrow
C='\033[38;5;250m'                  # command text
R='\033[0m'

run() {
  printf "\$P \${C}%s\${R}\n" "\$1"; sleep 0.7
  eval "\$2"; echo; sleep "\${3:-1.2}"
}

sleep 0.6
run "dfinstall status"            "\$df status" 4
run "dfinstall doctor"            "\$df doctor"
run "dfinstall install all --dry-run | head -20"  "\$df install all --dry-run 2>&1 | head -20"
sleep 1.2
rm -rf "\$H"
DRIVER

cleanup() { rm -rf "$clone" "$driver" "$cast"; }
trap cleanup EXIT

asciinema rec --overwrite \
  --cols 92 --rows 51 \
  -c "bash $driver" \
  "$cast"

agg --cols 92 --rows 51 --font-size 16 --theme dracula "$cast" "$out"
echo "wrote $out"
rm -f "$cast" "$driver"
