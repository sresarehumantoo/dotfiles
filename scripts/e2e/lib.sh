# shellcheck shell=bash
# Shared by scripts/e2e/scenarios/*.sh, which run as root inside the container.

fails=0

pass() { printf 'PASS  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1"; fails=$((fails + 1)); }

# check DESC CMD...: pass when CMD succeeds.
check() {
    local desc="$1"
    shift
    if "$@"; then pass "$desc"; else fail "$desc"; fi
}

# expect_step RESULTS LABEL PROMPTS RC [AUTH]: the step ran with exactly that
# many sudo prompts, AUTH other password prompts (default 0) and that exit code.
expect_step() {
    local results="$1" label="$2" prompts="$3" rc="$4" auth="${5:-0}" got want
    # Fixed string up to "prompts=", so "extras" does not also match "extras again".
    got="$(grep -F "STEP ${label} prompts=" "$results" || true)"
    want="prompts=${prompts} auth=${auth} rc=${rc}"
    if [[ "${got#STEP "${label}" }" == "$want" ]]; then
        pass "${label}: ${want}"
    else
        fail "${label}: want ${want}, got '${got#STEP "${label}" }'"
    fi
}

# owned_by USER PATH: PATH (not following a symlink) belongs to USER.
owned_by() { [[ "$(stat -c %U "$2")" == "$1" ]]; }

# stage_repo USER: copy the read-only /src mount into USER's home as their
# dotfiles clone, with a dfinstall config that skips the first-run backup.
stage_repo() {
    local user="$1" home
    home="$(getent passwd "$user" | cut -d: -f6)"
    cp -a /src "$home/dotfiles"
    rm -rf "$home/dotfiles/bin"
    mkdir -p "$home/.config/dfinstall"
    printf 'skip_backup: true\n' > "$home/.config/dfinstall/config.yaml"
    chown -R "$user:" "$home/dotfiles" "$home/.config"
}

# build_deb N CONTENT OUT: a .deb of /e2e/pkg/N/dfinstall laid out as the
# release's nfpms config lays it out, with CONTENT as /usr/share/dfinstall.
build_deb() {
    local n="$1" content="$2" out="$3" root
    root="$(mktemp -d)"
    mkdir -p "$root/DEBIAN" "$root/usr/bin" "$root/usr/share/dfinstall"
    install -m 755 "/e2e/pkg/$n/dfinstall" "$root/usr/bin/dfinstall"
    cp -a "$content/." "$root/usr/share/dfinstall/"
    cat > "$root/DEBIAN/control" <<EOF
Package: dfinstall
Version: 3.0.0~e2e.$n
Architecture: $(dpkg --print-architecture)
Maintainer: e2e <e2e@example.invalid>
Depends: git, curl
Description: dfinstall e2e build $n
EOF
    dpkg-deb --root-owner-group -Zgzip --build "$root" "$out" >/dev/null
    rm -rf "$root"
}

# content_v2 OUT: the first build's content with the changes a second release
# brings: a linked file, an untouched seeded file and an edited one.
content_v2() {
    local out="$1"
    cp -a /e2e/pkg/1/content "$out"
    chmod -R u+w "$out"
    echo '# v2' >> "$out/config/tmux/tmux.conf"
    echo '# v2' >> "$out/config/htop/htoprc"
    echo '# v2' >> "$out/config/shell/p10k.zsh"
    /e2e/contentstamp -root "$out" -version v3.0.0-e2e.2
}

# links_under HOME: every symlink under HOME with its target, sorted, leaving
# out dfinstall's own data (backups, the store's current link).
links_under() {
    find "$1" -path "$1/.local/share/dfinstall" -prune -o -type l -printf '%p -> %l\n' | sort
}

# tree_sum DIR: one digest of every path, mode and file content under DIR.
tree_sum() {
    (cd "$1" && find . -printf '%p %m %y\n' | sort && find . -type f -exec sha256sum {} + | sort) | sha256sum
}

finish() {
    echo
    if ((fails)); then
        echo "${fails} check(s) failed"
        exit 1
    fi
    echo "all checks passed"
}
