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

script="/src/scripts/e2e/scenarios/$scenario.sh"
mounts=(-v "$repo:/src:ro" -v "$work:/e2e:ro")

if ! grep -qx '# e2e: systemd' "$here/scenarios/$scenario.sh"; then
    docker run --rm "${mounts[@]}" dfinstall-e2e bash "$script"
    exit
fi

# systemd as PID 1 needs a privileged container. The volumes are where a
# Docker daemon keeps its layers, which overlayfs cannot stack on the
# container's own overlayfs.
cid="$(docker run -d --privileged --cgroupns=private \
    -v /home -v /var/lib/docker -v /var/lib/containerd \
    "${mounts[@]}" dfinstall-e2e /sbin/init)"
trap 'docker rm -f -v "$cid" >/dev/null; rm -rf "$work"' EXIT
for _ in $(seq 60); do
    case "$(docker exec "$cid" systemctl is-system-running --wait 2>/dev/null)" in
        running|degraded) break ;;
    esac
    sleep 1
done
docker exec "$cid" bash "$script"
