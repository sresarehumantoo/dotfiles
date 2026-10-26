#!/usr/bin/env bash
# A release binary updating itself on the locked-down network: the firewall
# blocks the internet, so finding the latest release and downloading it both
# go through Nexus's GitHub proxies. Starts from the published v2.0.1 archive,
# fetched through Nexus as well.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

old=v2.0.1
nexus=http://nexus:8081/repository

# raw_proxy_online NAME REMOTE true|false: see nexus.sh.
raw_proxy_online() {
    curl -fsS -u admin:labadmin123 -X PUT -H 'Content-Type: application/json' \
        "http://nexus:8081/service/rest/v1/repositories/raw/proxy/$1" --data @- <<JSON
{"name":"$1","online":$3,
 "storage":{"blobStoreName":"default","strictContentTypeValidation":false},
 "proxy":{"remoteUrl":"$2","contentMaxAge":1440,"metadataMaxAge":5},
 "negativeCache":{"enabled":true,"timeToLive":5},
 "httpClient":{"blocked":false,"autoBlock":true},
 "raw":{"contentDisposition":"ATTACHMENT"}}
JSON
}

# install_old: the old release's binaries in ~/.local/bin, as the release
# tarball install in the README puts them.
install_old() {
    local archive
    archive="dfinstall_${old#v}_linux_$(dpkg --print-architecture).tar.gz"
    mkdir -p "$HOME/.local/bin"
    curl -fsSL --max-time 120 "$nexus/github/sresarehumantoo/dotfiles/releases/download/$old/$archive" |
        tar -xz -C "$HOME/.local/bin" dfinstall dfinstall-mcp
}

trust_ca
policy '*'
stage
use_nexus
check "github.com is blocked" test "$(http_code https://github.com)" = 403

bin="$HOME/.local/bin/dfinstall"
check "the $old archive comes through Nexus" install_old
check "it is the $old release build" grep -qx "dfinstall version $old (release)" <<<"$("$bin" --version)"
mcp_before="$(sha256sum < "$HOME/.local/bin/dfinstall-mcp")"

out="$("$bin" update htop -v 2>&1)"
version="$("$bin" --version)"
check "update replaces it with a newer release" grep -q "updated dfinstall $old to v" <<<"$out"
check "the new binary is a release build" grep -Eq '^dfinstall version v[0-9.]+ \(release\)$' <<<"$version"
check "and is not $old" lacks "$old" "$version"
check "dfinstall-mcp beside it is replaced too" test "$(sha256sum < "$HOME/.local/bin/dfinstall-mcp")" != "$mcp_before"
check "no GitHub fallback was needed" lacks "GitHub mirror failed" "$out"
check "and goes on to install the module" test -L "$XDG_CONFIG_HOME/htop/htoprc"

out="$("$bin" update htop 2>&1)"
check "a second update finds nothing newer" grep -q "is the latest release" <<<"$out"

# Both GitHub proxies down: GitHub itself is blocked too, so the update has
# to say so and carry on with the binary it has.
install_old
raw_proxy_online github-api https://api.github.com false
raw_proxy_online github https://github.com false
out="$("$bin" update htop 2>&1)"
rc=$?
raw_proxy_online github-api https://api.github.com true
raw_proxy_online github https://github.com true
check "with no way to GitHub, update warns" grep -q "dfinstall not updated, continuing with $old" <<<"$out"
check "and keeps $old" grep -qx "dfinstall version $old (release)" <<<"$("$bin" --version)"
check "and still exits cleanly" test "$rc" -eq 0

finish
