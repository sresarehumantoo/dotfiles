#!/usr/bin/env bash
# Stage the packaged content for a release: config/ as committed at HEAD, plus
# its content stamp.
#
# Usage: content.sh <tag> <out-dir>
#
# The tree comes from git rather than the working directory, so gitignored
# machine-local files under config/ never reach a package. <out-dir> is
# replaced.
set -euo pipefail

usage() { sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; }
case "${1:-}" in -h | --help) usage; exit 0 ;; esac
if [[ $# -ne 2 ]]; then
    usage >&2
    exit 1
fi

tag="$1"
out="$2"
rm -rf "$out"
mkdir -p "$out"
# tar.umask and -p fix the modes at 0644 and 0755 whatever the local umask is.
git -c tar.umask=0022 archive --format=tar HEAD config | tar -xp -C "$out"
go run ./internal/tools/contentstamp -root "$out" -version "$tag"
