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
# Keep the script's own exit status: a trap ending in `|| true` would turn a
# failed run into a pass.
# shellcheck disable=SC2329  # run by the EXIT trap
stop_proxy() {
    local rc=$?
    kill "$proxy_pid" 2>/dev/null
    exit "$rc"
}
trap stop_proxy EXIT
ca="$HOME/.mitmproxy/mitmproxy-ca-cert.pem"
for _ in $(seq 60); do [ -s "$ca" ] && break; sleep 1; done
[ -s "$ca" ] || { echo "mitmproxy never wrote its CA" >&2; exit 1; }
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$ca"

export http_proxy=http://127.0.0.1:8080 https_proxy=http://127.0.0.1:8080
export HTTP_PROXY="$http_proxy" HTTPS_PROXY="$https_proxy"

# Which clients honor the System keychain, before anything is wired. Recorded,
# not judged: this is what the bootstrap's detection has to work with.
public="$(mktemp)"
security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain >"$public"
probe() {
    local name="$1" result=trusts
    shift
    "$@" >/dev/null 2>&1 || result=REJECTS
    printf '  %-60s %s\n' "$name" "$result" | tee -a "$logs/facts.txt"
}
pyget='import ssl, sys, urllib.request as u
ctx = ssl.create_default_context(cafile=sys.argv[1]) if len(sys.argv) > 1 else ssl.create_default_context()
u.urlopen("https://github.com", context=ctx, timeout=20)'
echo "== who accepts the proxy's certificate, unwired"
/usr/bin/curl -V | head -1 | tee "$logs/facts.txt"
probe "Apple curl" /usr/bin/curl -sS -o /dev/null --max-time 20 https://github.com
probe "Apple curl --cacert <Apple's public roots>" /usr/bin/curl -sS -o /dev/null --max-time 20 --cacert "$public" https://github.com
probe "Apple curl, CURL_SSL_BACKEND=openssl, --cacert <public roots>" \
    env CURL_SSL_BACKEND=openssl /usr/bin/curl -sS -o /dev/null --max-time 20 --cacert "$public" https://github.com
probe "Apple curl, CURL_SSL_BACKEND=securetransport" \
    env CURL_SSL_BACKEND=securetransport /usr/bin/curl -sS -o /dev/null --max-time 20 https://github.com
probe "/usr/bin/python3 urllib, cafile=<public roots>" /usr/bin/python3 -c "$pyget" "$public"
probe "/usr/bin/python3 urllib, default context" /usr/bin/python3 -c "$pyget"
combined="$(mktemp)"
security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain /Library/Keychains/System.keychain >"$combined"
# s_client exits 1 after any handshake with stdin closed; the verify line is
# the result.
sclient() {
    local out
    out="$(/usr/bin/openssl s_client -proxy 127.0.0.1:8080 -connect github.com:443 -servername github.com \
        -CAfile "$1" </dev/null 2>&1 || true)"
    [[ "$out" == *"Verify return code: 0 (ok)"* ]]
}
probe "/usr/bin/openssl s_client -proxy, -CAfile <public roots>" sclient "$public"
probe "/usr/bin/openssl s_client -proxy, -CAfile <public + System keychain>" sclient "$combined"
{
    /usr/bin/openssl version
    echo "-- s_client, CAfile=public + System keychain:"
    { /usr/bin/openssl s_client -proxy 127.0.0.1:8080 -connect github.com:443 -servername github.com \
        -CAfile "$combined" -showcerts </dev/null 2>&1 || true; } | grep -E 'Verify return|depth=|issuer=' | head -20 || true
    echo "-- the proxy root, as the keychain holds it:"
    /usr/bin/openssl x509 -in "$ca" -noout -subject -issuer -dates -ext basicConstraints,keyUsage 2>&1
    echo "-- certificates in the combined bundle: $(grep -c 'BEGIN CERTIFICATE' "$combined")"
} 2>&1 | tee "$logs/openssl.txt" | sed 's/^/      /'
probe "Apple git (/usr/bin/git)" /usr/bin/git ls-remote --heads https://github.com/sresarehumantoo/dotfiles main
probe "Homebrew's git" "$(brew --prefix)/bin/git" ls-remote --heads https://github.com/sresarehumantoo/dotfiles main
if command -v node >/dev/null; then
    # fetch ignores HTTPS_PROXY unless NODE_USE_ENV_PROXY is set (Node 24+).
    probe "node $(node --version) fetch, NODE_USE_ENV_PROXY=1" env NODE_USE_ENV_PROXY=1 \
        node -e 'fetch("https://registry.npmjs.org/").then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))'
    probe "npm view" npm view --registry https://registry.npmjs.org/ six version
fi
if python3 -m pip --version >/dev/null 2>&1; then
    probe "pip ($(command -v python3))" python3 -m pip download --quiet --no-deps --dest "$(mktemp -d)" six
fi
probe "brew fetch" brew fetch --quiet --force --formula hello
probe "Go" env -u SSL_CERT_FILE GOMODCACHE="$(mktemp -d)" GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org \
    go mod download -json golang.org/x/text@v0.20.0

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

# Any probe above that never reached the proxy says nothing; this is the record.
grep -E 'client connect|server connect' "$logs/mitmdump.log" | sed -E 's/^\[[^]]*\]//' | awk '{print $NF, $(NF-1)}' | sort | uniq -c >"$logs/proxied.txt" || true

echo "== through the proxy, with ca.sh"
# Guarded: on macOS's bash 3.2 a failed `.` ends the script, and its status was
# lost, so the remaining checks would neither run nor fail the job.
if [ -f "$env_file" ]; then
    # shellcheck source=/dev/null
    . "$env_file"
fi
check "curl" /usr/bin/curl -sS -o /dev/null --max-time 20 https://github.com
check "git" git ls-remote --heads https://github.com/sresarehumantoo/dotfiles main
if command -v node >/dev/null; then
    check "node fetch (NODE_EXTRA_CA_CERTS)" env NODE_USE_ENV_PROXY=1 \
        node -e 'fetch("https://registry.npmjs.org/").then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))'
    check "npm view" npm view --registry https://registry.npmjs.org/ six version
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
