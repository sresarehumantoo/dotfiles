#!/usr/bin/env bash
# The macOS bootstrap's TLS-inspection handling on a real Mac: mitmproxy
# re-signs every connection, and its CA is trusted in the System keychain,
# the way MDM installs a corporate proxy's root. The bootstrap's own detection
# and wiring then run, and each tool is checked through the proxy.
#
# Usage: scripts/macos-ci/tls-inspection.sh [log-dir]
#
# Needs sudo (for the System keychain) and Homebrew (for mitmproxy); run it on
# a throwaway runner.
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

case "${1:-}" in -h | --help) sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;; esac
logs="$(mkdir -p "${1:-/tmp/dfinstall-tls}" && cd "${1:-/tmp/dfinstall-tls}" && pwd -P)"

export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1
brew install --quiet mitmproxy >/dev/null
mitmdump --listen-host 127.0.0.1 --listen-port 8080 >"$logs/mitmdump.log" 2>&1 &
proxy_pid=$!
trap 'kill "$proxy_pid" 2>/dev/null || true' EXIT
ca="$HOME/.mitmproxy/mitmproxy-ca-cert.pem"
for _ in $(seq 60); do [ -s "$ca" ] && break; sleep 1; done
[ -s "$ca" ] || { echo "mitmproxy never wrote its CA" >&2; exit 1; }
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$ca"

export http_proxy=http://127.0.0.1:8080 https_proxy=http://127.0.0.1:8080
export HTTP_PROXY="$http_proxy" HTTPS_PROXY="$https_proxy"

echo "== before wiring"
rc=0
/usr/bin/curl -sS -o /dev/null --max-time 20 https://github.com 2>/dev/null || rc=$?
check "Apple's curl rejects the inspected chain (exit $rc, want 60)" test "$rc" -eq 60

echo "== the bootstrap's TLS step"
sandbox="$(mktemp -d)"
HOME="$sandbox" /bin/bash -c '
    source "$1"
    trap - EXIT
    ASSUME_YES=true
    exec 3</dev/null
    wire_tls_trust
' _ "$repo/bootstrap/macos-setup.sh" >"$logs/wire.log" 2>&1 || true
sed 's/\x1b\[[0-9;]*m//g' "$logs/wire.log" | sed 's/^/      /'
check_output "it detects the inspection on github.com" 'TLS inspection on:.*github\.com' cat "$logs/wire.log"
env_file="$sandbox/.config/dfinstall/ca.sh"
bundle="$sandbox/.local/share/dfinstall/ca-bundle.pem"
check "it writes ca.sh" test -f "$env_file"
check "the bundle carries the proxy's root" grep -qF "$(sed -n 2p "$ca")" "$bundle"
check_output "git is pointed at the bundle" "$bundle" git config --file "$sandbox/.gitconfig.local" http.sslCAInfo

echo "== through the proxy, with ca.sh"
# shellcheck source=/dev/null
. "$env_file"
check "curl" /usr/bin/curl -sS -o /dev/null --max-time 20 https://github.com
check "git" git ls-remote --heads https://github.com/sresarehumantoo/dotfiles main
if command -v node >/dev/null; then
    check "node fetch" node -e 'fetch("https://registry.npmjs.org/").then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))'
fi
if python3 -m pip --version >/dev/null 2>&1; then
    check "pip" python3 -m pip download --quiet --no-deps --dest "$sandbox/pip" six
fi
check "Homebrew (HOMEBREW_CURLRC)" brew fetch --quiet --force --formula hello
# Go verifies with the keychain itself, which is why the bootstrap leaves
# SSL_CERT_FILE alone; this is the check that it does.
check "Go, from the keychain alone" env -u SSL_CERT_FILE GOMODCACHE="$sandbox/gomod" GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org \
    go mod download -json golang.org/x/text@v0.20.0

echo "== dfinstall doctor"
(cd "$repo" && make build -s)
HOME="$sandbox" DOTFILES="$repo" "$repo/bin/dfinstall" doctor >"$logs/doctor.log" 2>&1 || true
check_output "doctor's tls trust row passes" '^ *(✓|\+) tls trust' cat "$logs/doctor.log"
grep -i 'tls trust' "$logs/doctor.log" | sed 's/^/      /' || true

sudo security delete-certificate -c mitmproxy /Library/Keychains/System.keychain >/dev/null 2>&1 || true
finish
