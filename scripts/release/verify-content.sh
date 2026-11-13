#!/usr/bin/env bash
# Check a GoReleaser dist/ directory's packaged content: the content tarball
# and every .deb must hold exactly the files their stamp lists, with the
# recorded digests and modes, owned by root. Each .deb must recommend git
# rather than depend on it.
#
# Usage: verify-content.sh <dist-dir>
set -euo pipefail
shopt -s inherit_errexit

usage() { sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'; }
case "${1:-}" in -h | --help | "") usage; exit 0 ;; esac
dist="$1"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# owned_by_root WHAT LISTING: every entry of a tar -tv style listing is root's.
# The listing is captured first: piping it into grep -q under pipefail would
# fail on the SIGPIPE grep causes and pass a bad archive.
owned_by_root() {
    local bad
    bad="$(printf '%s\n' "$2" | awk '$2 != "root/root"')"
    if [[ -n "$bad" ]]; then
        echo "verify-content: $1 has entries not owned by root:" >&2
        printf '%s\n' "$bad" | head -5 >&2
        exit 1
    fi
}

# lists PKG FIELD: a control field's comma-separated list names PKG.
lists() { [[ ",${2// /}," == *",$1,"* ]]; }

# restamp DIR: regenerate DIR's stamp and compare it with the shipped one.
restamp() {
    local dir="$1" version
    version="$(sed -n 's/^  "version": "\(.*\)",$/\1/p' "$dir/.dfinstall-content.json")"
    go run ./internal/tools/contentstamp -root "$dir" -version "$version" -o "$work/restamp.json"
    cmp "$work/restamp.json" "$dir/.dfinstall-content.json"
}

tarball="$(find "$dist" -maxdepth 1 -name 'dfinstall-content_*.tar.gz')"
[[ -n "$tarball" ]] || { echo "verify-content: no content tarball in $dist" >&2; exit 1; }
owned_by_root "$tarball" "$(tar -tzvf "$tarball")"
mkdir "$work/tar"
tar -xpzf "$tarball" -C "$work/tar" --strip-components=1
restamp "$work/tar"
echo "ok $(basename "$tarball")"

for deb in "$dist"/*.deb; do
    owned_by_root "$deb" "$(dpkg-deb -c "$deb")"
    depends="$(dpkg-deb -f "$deb" Depends)"
    recommends="$(dpkg-deb -f "$deb" Recommends)"
    if lists git "$depends" || ! lists git "$recommends"; then
        echo "verify-content: $(basename "$deb") must recommend git, not depend on it (Depends: $depends; Recommends: $recommends)" >&2
        exit 1
    fi
    rm -rf "$work/deb"
    dpkg-deb -x "$deb" "$work/deb"
    restamp "$work/deb/usr/share/dfinstall"
    cmp "$work/deb/usr/share/dfinstall/.dfinstall-content.json" "$work/tar/.dfinstall-content.json"
    echo "ok $(basename "$deb")"
done
