#!/usr/bin/env bash
# Shared output helpers for devtools scripts.
# Source this at the top of each script:
#   source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

# ── Colors & symbols ─────────────────────────────────────────────
_BOLD='\033[1m'
_DIM='\033[2m'
_RESET='\033[0m'
_BLUE='\033[34m'
_GREEN='\033[32m'
_YELLOW='\033[33m'
_RED='\033[31m'
_CYAN='\033[36m'

info()   { printf "${_BLUE}${_BOLD}  ▸${_RESET} %s\n" "$*"; }
ok()     { printf "${_GREEN}${_BOLD}  ✓${_RESET} %s\n" "$*"; }
warn()   { printf "${_YELLOW}${_BOLD}  ⚠${_RESET} %s\n" "$*" >&2; }
err()    { printf "${_RED}${_BOLD}  ✗${_RESET} %s\n" "$*" >&2; }
die()    { err "$@"; exit 1; }
header() { printf "\n${_BOLD}${_CYAN}── %s ──${_RESET}\n\n" "$*"; }
step()   { printf "${_DIM}  …${_RESET} %s\n" "$*"; }

# ── Guard helpers ────────────────────────────────────────────────
require_wsl() {
    [[ -f /proc/sys/fs/binfmt_misc/WSLInterop ]] || die "Not running inside WSL."
}

require_cmd() {
    command -v "$1" &>/dev/null || die "$1 is not installed."
}

require_git_repo() {
    git rev-parse --is-inside-work-tree &>/dev/null || die "Not inside a git repository."
}

# ── Confirmation prompt ──────────────────────────────────────────
confirm() {
    local msg="${1:-Continue?}"
    printf "${_YELLOW}${_BOLD}  ? ${_RESET}%s [y/N] " "$msg"
    read -r answer
    [[ "${answer,,}" == "y" ]]
}
