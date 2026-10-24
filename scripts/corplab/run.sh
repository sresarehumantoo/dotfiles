#!/usr/bin/env bash
# Run dfinstall and the bootstraps on a simulated corporate network: an inline
# TLS-inspecting firewall with URL filtering, and a Nexus proxying the public
# registries. See compose.yaml for the topology.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
state="$here/.state"

usage() {
    cat <<USAGE
Usage: $(basename "$0") [--down] [scenario...]

Brings the lab up (first Nexus start takes 2-3 minutes; it is left running for
the next run), then runs each scenario, or all of them, inside the dev box.

Scenarios:
$(for s in "$here"/scenarios/*.sh; do printf '  %s\n' "$(basename "$s" .sh)"; done)

  --down   stop the lab afterwards (Nexus keeps its cache in a volume)

By hand: docker compose -f $here/compose.yaml exec -u dev dev bash
Nexus UI: http://127.0.0.1:8081 (admin / see setup-nexus.sh)
Remove everything: docker compose -f $here/compose.yaml down -v
USAGE
}

down=false
scenarios=()
for arg; do
    case "$arg" in
        -h|--help) usage; exit 0 ;;
        --down) down=true ;;
        *)
            [[ -f "$here/scenarios/$arg.sh" ]] || { echo "unknown scenario: $arg" >&2; usage >&2; exit 2; }
            scenarios+=("$arg")
            ;;
    esac
done
if [[ ${#scenarios[@]} -eq 0 ]]; then
    for s in "$here"/scenarios/*.sh; do scenarios+=("$(basename "$s" .sh)"); done
fi
command -v docker >/dev/null || { echo "docker not found" >&2; exit 1; }

mkdir -p "$state/ca" "$state/bin" "$state/policy"
if [[ ! -f "$state/ca/corp-ca.crt" ]]; then
    # The shape corporate IT hands out: a root CA as PEM .crt, and as DER .cer.
    openssl req -x509 -newkey rsa:2048 -nodes -days 365 -sha256 \
        -subj "/O=Example Corp/CN=Example Corp Forward Trust CA" \
        -addext "basicConstraints=critical,CA:TRUE" \
        -addext "keyUsage=critical,keyCertSign,cRLSign" \
        -keyout "$state/ca/corp-ca.key" -out "$state/ca/corp-ca.crt" 2>/dev/null
    openssl x509 -in "$state/ca/corp-ca.crt" -outform DER -out "$state/ca/corp-ca.cer"
    chmod 644 "$state/ca/corp-ca.key"
fi
: > "$state/policy/blocklist"

echo "Building dfinstall..."
(cd "$repo" && CGO_ENABLED=0 go build -o "$state/bin/dfinstall" ./src/cmd/dfinstall)

export REPO="$repo"
compose=(docker compose -f "$here/compose.yaml")
echo "Starting the lab..."
"${compose[@]}" up -d --build --wait >/dev/null
"$here/setup-nexus.sh"

failed=()
for s in "${scenarios[@]}"; do
    printf '\n== %s\n' "$s"
    : > "$state/policy/blocklist"
    if ! "${compose[@]}" exec -T -u dev dev bash "/src/scripts/corplab/scenarios/$s.sh"; then
        failed+=("$s")
    fi
done
: > "$state/policy/blocklist"

if [[ "$down" == true ]]; then
    "${compose[@]}" down
fi
echo
if [[ ${#failed[@]} -gt 0 ]]; then
    echo "failed: ${failed[*]}"
    exit 1
fi
echo "all scenarios passed"
