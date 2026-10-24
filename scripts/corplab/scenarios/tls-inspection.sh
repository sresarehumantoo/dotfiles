#!/usr/bin/env bash
# A TLS-inspecting firewall with no internal mirror: every HTTPS connection is
# re-signed by the corporate root CA. Trust comes either from the system store
# (bootstrap/wsl-setup.sh install-cert, root) or from per-user env vars (what
# bootstrap/macos-setup.sh writes, no root).
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

untrust_ca
/src/bootstrap/wsl-setup.sh check-tls >/dev/null 2>&1
check "check-tls exits 2 (corp CA needed) before install-cert" test $? -eq 2

trust_ca
/src/bootstrap/wsl-setup.sh check-tls >/dev/null 2>&1
check "check-tls passes after install-cert of the DER .cer" test $? -eq 0

tools_work() {
    check "$1: curl" test "$(http_code https://github.com)" = 200
    check "$1: git over https" git ls-remote -q https://github.com/sresarehumantoo/dotfiles HEAD
    check "$1: node fetch" node -e 'fetch("https://registry.npmjs.org/").then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))'
    check "$1: npm" npm view --silent opencode-linux-x64 version
    check "$1: pip" pip3 download -q --no-deps -d "$(mktemp -d)" six
}
tools_work "system store"

stage
mkdir -p "$HOME/p" && git -C "$HOME/p" init -q
check "dfinstall installs delta from GitHub" installs delta
check "dfinstall installs opencode from GitHub" installs opencode
# A dummy key gets past TLS to Anthropic's own rejection.
# Output is captured before grep: grep -q exits on the first match, and under
# pipefail the SIGPIPE that then kills opencode would fail the check.
opencode_tls() {
    local out
    out="$(cd "$HOME/p" && ANTHROPIC_API_KEY=x timeout 120 opencode run -m anthropic/claude-haiku-5-5 hi </dev/null 2>&1)"
    grep -q 'invalid x-api-key' <<<"$out" && return
    printf '      opencode: %s\n' "$(tail -n 1 <<<"$out")" >&2
    return 1
}
check "opencode reaches Anthropic through the proxy" opencode_tls

untrust_ca
if opencode_tls 2>/dev/null; then fail "without the CA, opencode fails verification"; else pass "without the CA, opencode fails verification"; fi
bundle="$HOME/ca-bundle.pem"
cat /etc/ssl/certs/ca-certificates.crt "$CA_PEM" > "$bundle"
export CURL_CA_BUNDLE="$bundle" GIT_SSL_CAINFO="$bundle" NODE_EXTRA_CA_CERTS="$bundle" \
    REQUESTS_CA_BUNDLE="$bundle" PIP_CERT="$bundle"
tools_work "per-user env, no root"
check "per-user env: opencode" opencode_tls
check "per-user env: dfinstall update opencode" quietly "$DFINSTALL" update opencode

finish
