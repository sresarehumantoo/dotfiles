#!/usr/bin/env bash
# The locked-down case: the firewall blocks the internet outright and every
# package comes from the internal Nexus, which proxies npm, PyPI, Go, and
# GitHub's site and API as raw repositories.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

# github_proxy_online true|false: Nexus has no on/off endpoint; a proxy's
# status is part of its definition.
github_proxy_online() {
    curl -fsS -u admin:labadmin123 -X PUT -H 'Content-Type: application/json' \
        http://nexus:8081/service/rest/v1/repositories/raw/proxy/github --data @- <<JSON
{"name":"github","online":$1,
 "storage":{"blobStoreName":"default","strictContentTypeValidation":false},
 "proxy":{"remoteUrl":"https://github.com","contentMaxAge":1440,"metadataMaxAge":5},
 "negativeCache":{"enabled":true,"timeToLive":5},
 "httpClient":{"blocked":false,"autoBlock":true},
 "raw":{"contentDisposition":"ATTACHMENT"}}
JSON
}

trust_ca
policy '*'
stage
use_nexus
check "github.com is blocked" test "$(http_code https://github.com)" = 403

doctor="$("$DFINSTALL" doctor 2>&1)"
check "doctor names the npm mirror" grep -q 'package mirrors: npm http://nexus:8081/repository/npm-proxy/' <<<"$doctor"

out="$("$DFINSTALL" install opencode -v 2>&1)"
check "opencode comes from npm through Nexus" grep -q 'installed opencode .* from npm' <<<"$out"
check "opencode runs" quietly opencode --version

out="$("$DFINSTALL" install delta -v 2>&1)"
check "delta comes from the GitHub release through Nexus" test -x "$HOME/.local/bin/delta"
check "no GitHub fallback was needed" lacks "GitHub mirror failed" "$out"

# Nexus's GitHub proxy down: the fetch falls back to GitHub, which the
# firewall also blocks, so the install must fail rather than hang or lie.
github_proxy_online false
rm -f "$HOME/.local/bin/delta"
if out="$("$DFINSTALL" install delta -v 2>&1)" && test -x "$HOME/.local/bin/delta"; then
    fail "delta install fails when neither Nexus nor GitHub can serve it"
else
    pass "delta install fails when neither Nexus nor GitHub can serve it"
fi
check "the mirror failure is reported" grep -q 'GitHub mirror failed, trying GitHub' <<<"$out"
github_proxy_online true

finish
