#!/usr/bin/env bash
# Bring the lab's Nexus CE to a usable state: community EULA, admin password,
# anonymous read, and proxies of the public registries. Idempotent.
set -euo pipefail
cd "$(dirname "$0")"

N=http://127.0.0.1:8081
# Lab only: Nexus listens on 127.0.0.1 and holds nothing but caches.
PASS=labadmin123
api() { curl -fsS -u "admin:$PASS" -H 'Content-Type: application/json' "$@"; }

until curl -fs "$N/service/rest/v1/status/writable" >/dev/null; do sleep 5; done

if docker compose exec -T nexus test -f /nexus-data/admin.password; then
    first="$(docker compose exec -T nexus cat /nexus-data/admin.password)"
    curl -fsS -u "admin:$first" -X PUT -H 'Content-Type: text/plain' \
        --data "$PASS" "$N/service/rest/v1/security/users/admin/change-password"
fi

eula="$(api "$N/service/rest/v1/system/eula" || true)"
if [[ -n "$eula" ]] && ! grep -q '"accepted" *: *true' <<<"$eula"; then
    api -X POST "$N/service/rest/v1/system/eula" --data "$(python3 -I -c '
import json, sys
print(json.dumps({"accepted": True, "disclaimer": json.loads(sys.argv[1])["disclaimer"]}))' "$eula")"
fi

api -X PUT "$N/service/rest/v1/security/anonymous" \
    --data '{"enabled":true,"userId":"anonymous","realmName":"NexusAuthorizingRealm"}' >/dev/null

# proxy FORMAT NAME REMOTE
proxy() {
    api "$N/service/rest/v1/repositories/$2" >/dev/null 2>&1 && return
    api -X POST "$N/service/rest/v1/repositories/$1/proxy" --data @- <<JSON
{"name":"$2","online":true,
 "storage":{"blobStoreName":"default","strictContentTypeValidation":false},
 "proxy":{"remoteUrl":"$3","contentMaxAge":1440,"metadataMaxAge":5},
 "negativeCache":{"enabled":true,"timeToLive":5},
 "httpClient":{"blocked":false,"autoBlock":true}}
JSON
    echo "nexus: created $2 -> $3"
}
proxy npm npm-proxy https://registry.npmjs.org
proxy pypi pypi-proxy https://pypi.org
proxy go go-proxy https://proxy.golang.org
proxy raw github https://github.com
proxy raw github-api https://api.github.com
