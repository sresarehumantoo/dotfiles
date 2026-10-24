# shellcheck shell=bash
# Shared by scripts/macos-ci/*.sh, which run on GitHub's macOS runners: a
# throwaway Mac with passwordless sudo, so these checks do for real what
# scripts/macos-harness can only record on a Mac in daily use.

fails=0

pass() { printf 'PASS  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1"; fails=$((fails + 1)); }

# check DESC CMD...: pass when CMD succeeds.
check() {
    local desc="$1"
    shift
    if "$@"; then pass "$desc"; else fail "$desc"; fi
}

# check_output DESC PATTERN CMD...: pass when CMD's output matches PATTERN
# (an extended regex), whatever its exit status.
check_output() {
    local desc="$1" pattern="$2" out
    shift 2
    out="$("$@" 2>&1 || true)"
    if printf '%s\n' "$out" | grep -Eq -- "$pattern"; then
        pass "$desc"
    else
        fail "$desc"
        printf '%s\n' "$out" | tail -15 | sed 's/^/      /'
    fi
}

finish() {
    echo
    if [ "$fails" -gt 0 ]; then
        echo "${fails} check(s) failed"
        exit 1
    fi
    echo "all checks passed"
}

# shellcheck disable=SC2034  # used by the scripts that source this
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
