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
# dotfiles clone, with a config that skips the first-run backup prompt.
stage_repo() {
    local user="$1" home
    home="$(getent passwd "$user" | cut -d: -f6)"
    cp -a /src "$home/dotfiles"
    rm -rf "$home/dotfiles/bin"
    printf 'skip_backup: true\n' > "$home/dotfiles/.config.yaml"
    chown -R "$user:" "$home/dotfiles"
}

finish() {
    echo
    if ((fails)); then
        echo "${fails} check(s) failed"
        exit 1
    fi
    echo "all checks passed"
}
