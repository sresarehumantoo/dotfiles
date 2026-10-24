#!/usr/bin/env bash
# The lab itself: every HTTPS connection is re-signed, the URL filter answers
# with a block page or a reset, and Nexus is reachable directly.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

untrust_ca
check "stock trust store rejects the inspected chain (curl exit 60)" \
    test "$(http_code https://github.com)" = curl:60
check "the lab CA verifies it" test "$(http_code https://github.com --cacert "$CA_PEM")" = 200
check "nexus answers on the corp network" \
    test "$(http_code http://nexus:8081/service/rest/v1/status)" = 200

policy github.com
check "a blocked host gets the policy page (403)" \
    test "$(http_code https://api.github.com --cacert "$CA_PEM")" = 403
policy drop:github.com
check "a dropped host is reset" is_curl_failure "$(http_code https://github.com --cacert "$CA_PEM")"
policy '*' '!pypi.org'
check "an exemption passes a block-all policy" \
    test "$(http_code https://pypi.org/simple/ --cacert "$CA_PEM")" = 200

finish
