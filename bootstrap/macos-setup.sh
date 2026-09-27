#!/usr/bin/env bash
# macOS bootstrap: Xcode Command Line Tools, Homebrew, Go and Ghostty, then
# `make install` (dfinstall install all). Must stay bash 3.2 compatible: on a
# fresh Mac /bin/bash 3.2 is the only bash there is.
set -euo pipefail

_BOLD='\033[1m'  _RESET='\033[0m'
_BLUE='\033[34m' _GREEN='\033[32m' _YELLOW='\033[33m'
_RED='\033[31m'  _CYAN='\033[36m'

info()   { printf "${_BLUE}${_BOLD}  [>]${_RESET} %s\n" "$*"; }
ok()     { printf "${_GREEN}${_BOLD}  [+]${_RESET} %s\n" "$*"; }
warn()   { printf "${_YELLOW}${_BOLD}  [!]${_RESET} %s\n" "$*" >&2; }
die()    { printf "${_RED}${_BOLD}  [x]${_RESET} %s\n" "$*" >&2; exit 1; }
header() { printf "\n${_BOLD}${_CYAN}== %s ==${_RESET}\n\n" "$*"; }

REPO_URL="https://github.com/sresarehumantoo/dotfiles.git"

show_help() {
    cat <<'HELP'
Usage: macos-setup.sh [options]

Installs the Xcode Command Line Tools, Homebrew, Go and Ghostty, then clones
(or reuses) the dotfiles and runs `make install`.

Options:
  --branch <branch>   Branch to clone (default: develop). An existing clone is
                      used as it is, on whatever branch it has checked out.
  --dir <path>        Where the clone lives (default: the clone this script is
                      in, else ~/dotfiles)
  --skip-ghostty      Don't install Ghostty
  --skip-dotfiles     Stop after the prerequisites
  -h, --help          Show this help

Examples:
  ./bootstrap/macos-setup.sh
  curl -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/develop/bootstrap/macos-setup.sh | bash
HELP
}

# The clone this script sits in, when it is run from one rather than piped in.
default_dir() {
    local src="${BASH_SOURCE[0]:-}" here=""
    if [[ -n "$src" && -f "$src" ]]; then
        here="$(cd "$(dirname "$src")/.." && pwd)"
    fi
    if [[ -n "$here" && -d "$here/.git" && -f "$here/go.mod" ]]; then
        echo "$here"
    else
        echo "$HOME/dotfiles"
    fi
}

# Homebrew is off PATH until something runs `brew shellenv`: /opt/homebrew on
# Apple Silicon, /usr/local on Intel.
brew_bin() {
    local b
    for b in /opt/homebrew/bin/brew /usr/local/bin/brew; do
        if [[ -x "$b" ]]; then
            echo "$b"
            return 0
        fi
    done
    return 1
}

install_clt() {
    header "Xcode Command Line Tools"
    if xcode-select -p &>/dev/null; then
        ok "Already installed"
        return
    fi
    # --install only opens a GUI dialog and returns immediately.
    xcode-select --install 2>/dev/null || true
    info "Accept the installer dialog. Waiting for it to finish (Ctrl-C to abort)..."
    until xcode-select -p &>/dev/null; do
        sleep 5
    done
    ok "Installed"
}

install_brew() {
    header "Homebrew"
    local brew
    if ! brew="$(brew_bin)"; then
        info "Running the official installer (it asks for your password)..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        brew="$(brew_bin)" || die "Homebrew installed, but brew is in neither /opt/homebrew nor /usr/local"
    fi
    eval "$("$brew" shellenv)"
    ok "$(brew --version | head -1)"
}

install_go() {
    header "Go"
    if command -v go &>/dev/null; then
        ok "Already installed: $(go version)"
        return
    fi
    brew install go
    ok "$(go version)"
}

install_ghostty() {
    header "Ghostty"
    if [[ -d /Applications/Ghostty.app || -d "$HOME/Applications/Ghostty.app" ]]; then
        ok "Already installed"
        return
    fi
    brew install --cask ghostty
    ok "Installed"
}

install_dotfiles() {
    local branch="$1" dir="$2"
    header "Dotfiles"
    if [[ -d "$dir/.git" ]]; then
        info "Using the existing clone at ${dir} ($(git -C "$dir" branch --show-current))"
    elif [[ -e "$dir" ]]; then
        die "${dir} exists but is not a git clone; move it or pass --dir"
    else
        git clone --branch "$branch" "$REPO_URL" "$dir"
    fi
    make -C "$dir" install
}

main() {
    local branch="develop" dir="" do_ghostty=true do_dotfiles=true
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --branch)        branch="${2:?--branch needs a value}"; shift 2 ;;
            --dir)           dir="${2:?--dir needs a value}"; shift 2 ;;
            --skip-ghostty)  do_ghostty=false; shift ;;
            --skip-dotfiles) do_dotfiles=false; shift ;;
            -h|--help)       show_help; exit 0 ;;
            *)               show_help >&2; die "Unknown option: $1" ;;
        esac
    done

    [[ "$(uname -s)" == Darwin ]] || die "This script is for macOS. On WSL use bootstrap/wsl-setup.sh."
    [[ "$(id -u)" -ne 0 ]] || die "Run as your normal user, not root. Homebrew refuses to run as root."
    [[ -n "$dir" ]] || dir="$(default_dir)"

    install_clt
    install_brew
    install_go
    if [[ "$do_ghostty" == true ]]; then
        install_ghostty
    fi
    if [[ "$do_dotfiles" == true ]]; then
        install_dotfiles "$branch" "$dir"
    fi

    header "Done"
    ok "Setup complete. Open a new terminal (Ghostty) to pick up the new shell."
}

main "$@"
