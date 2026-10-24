# shellcheck shell=bash
# Shared by the corplab scenarios, which run as "dev" inside the dev box.
# shellcheck source=../e2e/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/../e2e/lib.sh"

export CA_PEM=/lab/ca/corp-ca.crt CA_DER=/lab/ca/corp-ca.cer DFINSTALL=/lab/bin/dfinstall

# policy RULE...: replace the firewall's URL filter (see proxy/filter.py).
policy() { printf '%s\n' "$@" > /lab/policy/blocklist; }

# stage: a fresh home holding a copy of the working tree as its dotfiles clone,
# so scenarios do not see each other's state.
stage() {
    local home
    home="$(mktemp -d /tmp/home.XXXXXX)"
    cp -a /src "$home/dotfiles"
    rm -rf "$home/dotfiles/bin" "$home/dotfiles/scripts/corplab/.state"
    mkdir -p "$home/.config/dfinstall"
    printf 'skip_backup: true\n' > "$home/.config/dfinstall/config.yaml"
    export HOME="$home" DOTFILES="$home/dotfiles"
    export XDG_CONFIG_HOME="$home/.config" XDG_DATA_HOME="$home/.local/share"
    export XDG_STATE_HOME="$home/.local/state" XDG_CACHE_HOME="$home/.cache"
    export PATH="$home/.local/bin:$PATH"
}

# use_nexus: point npm and pip at Nexus the way a managed machine's config does.
use_nexus() {
    printf 'registry=http://nexus:8081/repository/npm-proxy/\n' > "$HOME/.npmrc"
    mkdir -p "$HOME/.config/pip"
    printf '[global]\nindex-url = http://nexus:8081/repository/pypi-proxy/simple\n' > "$HOME/.config/pip/pip.conf"
}

# trust_ca: what bootstrap/wsl-setup.sh install-cert does, from the DER file.
trust_ca() { sudo /src/bootstrap/wsl-setup.sh install-cert "$CA_DER" >/dev/null; }

# untrust_ca: back to the image's stock trust store.
untrust_ca() {
    sudo rm -f /usr/local/share/ca-certificates/corp-ca.crt
    sudo update-ca-certificates --fresh >/dev/null 2>&1
}

# http_code URL [CURL ARGS...]: the status curl saw, or "curl:<exit>" when the
# transfer failed before one.
http_code() {
    local url="$1" code rc
    shift
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 30 "$@" "$url" 2>/dev/null)" && rc=0 || rc=$?
    if [[ $rc -ne 0 && "$code" == 000 ]]; then echo "curl:$rc"; else echo "$code"; fi
}

is_curl_failure() { [[ "$1" == curl:* ]]; }

# installs MODULE [BINARY]: dfinstall installs MODULE and BINARY (default: the
# module's name) lands in ~/.local/bin.
installs() { "$DFINSTALL" install "$1" >/dev/null 2>&1 && test -x "$HOME/.local/bin/${2:-$1}"; }

# lacks TEXT HAYSTACK: HAYSTACK does not contain TEXT.
lacks() { ! grep -q "$1" <<<"$2"; }

# quietly CMD...: run CMD with its output discarded.
quietly() { "$@" >/dev/null 2>&1; }
