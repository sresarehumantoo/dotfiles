#!/usr/bin/env bash
# Run an end-to-end scenario against the working tree in a throwaway Debian
# container. The repo is mounted read-only; scenarios copy it before use.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"

usage() {
    cat <<EOF
Usage: $(basename "$0") <scenario>

Scenarios:
$(for s in "$here"/scenarios/*.sh; do printf '  %s\n' "$(basename "$s" .sh)"; done)

Each scenario prints PASS/FAIL per check and exits non-zero on any failure.
Requires docker. Takes several minutes: scenarios install real packages.
EOF
}

case "${1:-}" in
    -h|--help) usage; exit 0 ;;
    "") usage >&2; exit 2 ;;
esac
scenario="$1"
[[ -f "$here/scenarios/$scenario.sh" ]] || { echo "unknown scenario: $scenario" >&2; usage >&2; exit 2; }
command -v docker >/dev/null || { echo "docker not found" >&2; exit 1; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
chmod 755 "$work"

echo "Building dfinstall..."
(cd "$repo" && CGO_ENABLED=0 go build -o "$work/dfinstall" ./src/cmd/dfinstall)

echo "Building image..."
docker build -q -t dfinstall-e2e "$here" >/dev/null

docker run --rm \
    -v "$repo:/src:ro" \
    -v "$work:/e2e:ro" \
    dfinstall-e2e bash "/src/scripts/e2e/scenarios/$scenario.sh"
