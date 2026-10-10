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
header() { STEP="$*"; printf "\n${_BOLD}${_CYAN}== %s ==${_RESET}\n\n" "$*"; }

STEP="startup"

# Whatever stops the run, say where, so a failure on a managed Mac can be
# reported without re-running with tracing.
on_exit() {
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        printf "\n${_RED}${_BOLD}  [x]${_RESET} Bootstrap stopped during: %s (exit %s)\n" "$STEP" "$rc" >&2
        printf "      Once dfinstall is built, 'dfinstall doctor' reports admin, Homebrew, MDM, Santa, TLS trust and Gatekeeper state.\n" >&2
    fi
}
trap on_exit EXIT

REPO_URL="https://github.com/sresarehumantoo/dotfiles.git"

show_help() {
    cat <<'HELP'
Usage: macos-setup.sh [options]

Installs the Xcode Command Line Tools, Homebrew, Go and Ghostty, then clones
(or reuses) the dotfiles and runs `make install`.

Options:
  --branch <branch>   Branch to clone (default: main). An existing clone is
                      used as it is, on whatever branch it has checked out.
  --dir <path>        Where the clone lives (default: the clone this script is
                      in, else ~/dotfiles)
  --skip-ghostty      Don't install Ghostty
  --skip-dotfiles     Stop after the prerequisites
  -h, --help          Show this help

Examples:
  ./bootstrap/macos-setup.sh
  curl -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
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
# Apple Silicon, /usr/local on Intel, ~/homebrew for an account that cannot
# create either.
brew_bin() {
    local b
    for b in /opt/homebrew/bin/brew /usr/local/bin/brew "$HOME/homebrew/bin/brew"; do
        if [[ -x "$b" ]]; then
            echo "$b"
            return 0
        fi
    done
    return 1
}

is_admin() {
    id -Gn | tr ' ' '\n' | grep -qx admin
}

# Everything here runs as the user. On a managed Mac the things that can stop
# it are known up front, so check them before installing anything.
preflight() {
    header "Preflight"
    if is_admin; then
        ok "Admin account"
    else
        info "Standard account: apps go to ~/Applications, and Homebrew must already exist or be installable with temporary admin"
    fi
    if profiles status -type enrollment 2>/dev/null | grep -q 'MDM enrollment: Yes'; then
        info "MDM enrolled"
    fi
    if command -v santactl &>/dev/null && santactl status 2>/dev/null | grep -Eq 'Mode +\| +Lockdown'; then
        warn "Santa is in Lockdown: the dfinstall binary 'make install' builds, and other locally built tools, stay blocked until IT allowlists them"
    fi

    local brew prefix
    if brew="$(brew_bin)"; then
        prefix="$("$brew" --prefix)"
        if [[ ! -w "$prefix/Cellar" && ! -w "$prefix" ]]; then
            die "Homebrew at ${prefix} belongs to another account, and Homebrew supports only one. Ask IT to give this account ownership of it, or to install Homebrew for this account."
        fi
        ok "Homebrew at ${prefix} is writable"
    elif ! is_admin; then
        die "Homebrew is not installed, and its installer needs admin to create /opt/homebrew. Get temporary admin (e.g. Privileges or Admin By Request) and re-run, or have IT install Homebrew for this account."
    fi
}

# A TLS-inspecting proxy re-signs every connection with a root CA that MDM
# installs into the System keychain. Go and rustup read the keychain; Apple's
# curl, git, Node, Python and cargo do not, so each is pointed at a bundle of
# Apple's public roots plus the System keychain's certificates. Homebrew wipes
# its environment, so it gets the bundle through HOMEBREW_CURLRC and git's
# http.sslCAInfo instead. SSL_CERT_FILE is deliberately not set: since Go 1.27
# it replaces Go's keychain verification, which already works.
CA_BUNDLE="$HOME/.local/share/dfinstall/ca-bundle.pem"
CA_ENV="$HOME/.config/dfinstall/ca.sh"
CA_CURLRC="$HOME/.config/dfinstall/curlrc"
PUBLIC_ROOTS=/System/Library/Keychains/SystemRootCertificates.keychain
LOCAL_CERTS=/Library/Keychains/System.keychain
# Hosts the install reaches. A proxy may decrypt some categories and not
# others, so one host is not enough to tell.
CA_PROBE_HOSTS="github.com formulae.brew.sh registry.npmjs.org pypi.org static.rust-lang.org"

# Prints the hosts that fail against public roots alone but pass once the
# System keychain's certificates are trusted too: those are being intercepted.
intercepted_hosts() {
    local public="$1" combined="$2" host
    for host in $CA_PROBE_HOSTS; do
        if ! curl -sS -o /dev/null --max-time 15 --cacert "$public" "https://$host" 2>/dev/null \
            && curl -sS -o /dev/null --max-time 15 --cacert "$combined" "https://$host" 2>/dev/null; then
            printf '%s ' "$host"
        fi
    done
}

wire_tls_trust() {
    header "TLS trust"
    local public hosts
    public="$(mktemp)"
    mkdir -p "$(dirname "$CA_BUNDLE")" "$(dirname "$CA_ENV")"
    security find-certificate -a -p "$PUBLIC_ROOTS" >"$public"
    security find-certificate -a -p "$PUBLIC_ROOTS" "$LOCAL_CERTS" >"$CA_BUNDLE.new"
    hosts="$(intercepted_hosts "$public" "$CA_BUNDLE.new")"
    rm -f "$public"

    # Rewire on every run once wired, so a rotated proxy root is picked up.
    if [[ -z "$hosts" && ! -f "$CA_ENV" ]]; then
        rm -f "$CA_BUNDLE.new"
        ok "No TLS inspection seen; nothing to wire"
        return
    fi
    mv "$CA_BUNDLE.new" "$CA_BUNDLE"
    printf 'cacert = "%s"\n' "$CA_BUNDLE" >"$CA_CURLRC"
    cat >"$CA_ENV" <<ENVFILE
# Generated by bootstrap/macos-setup.sh, which found TLS inspection; re-run it
# to refresh the bundle after the proxy's root CA changes. Sourced early by
# ~/.profile. The TLS trust block in the bootstrap says why each one is here.
export CURL_CA_BUNDLE="$CA_BUNDLE"
export GIT_SSL_CAINFO="$CA_BUNDLE"
export NODE_EXTRA_CA_CERTS="$CA_BUNDLE"
export REQUESTS_CA_BUNDLE="$CA_BUNDLE"
export PIP_CERT="$CA_BUNDLE"
export CARGO_HTTP_CAINFO="$CA_BUNDLE"
export HOMEBREW_CURLRC="$CA_CURLRC"
ENVFILE
    git config --file "$HOME/.gitconfig.local" http.sslCAInfo "$CA_BUNDLE"
    # shellcheck source=/dev/null
    . "$CA_ENV"
    if [[ -n "$hosts" ]]; then
        ok "TLS inspection on: ${hosts% }. Trusting the System keychain's certificates via ${CA_BUNDLE}"
    else
        ok "Refreshed ${CA_BUNDLE} (no inspection seen this run)"
    fi
}

# Long enough for the download on a slow link; a policy-blocked install never
# finishes, and without a limit this waited forever.
CLT_WAIT_SECS=2700

install_clt() {
    header "Xcode Command Line Tools"
    if xcode-select -p &>/dev/null; then
        ok "Already installed"
        return
    fi
    # --install only opens a GUI dialog and returns immediately.
    local out
    if ! out="$(xcode-select --install 2>&1)"; then
        warn "xcode-select --install: ${out}"
    fi
    info "Accept the installer dialog. Waiting up to $((CLT_WAIT_SECS / 60)) minutes (Ctrl-C to abort)..."
    local waited=0
    until xcode-select -p &>/dev/null; do
        if [[ $waited -ge $CLT_WAIT_SECS ]]; then
            die "The Command Line Tools did not install. On a managed Mac they usually come from IT or Self Service; install them that way and re-run."
        fi
        sleep 5
        waited=$((waited + 5))
    done
    ok "Installed"
}

install_brew() {
    header "Homebrew"
    local brew
    if ! brew="$(brew_bin)"; then
        info "Running the official installer (it asks for your password)..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" <&3
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
    # /Applications is admin-only; a standard account installs per user.
    if [[ -w /Applications ]]; then
        brew install --cask ghostty
    else
        brew install --cask --appdir="$HOME/Applications" ghostty
    fi
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
    make -C "$dir" install <&3

    # A record of the environment to share if anything on this Mac misbehaves.
    local report
    report="$HOME/Library/Logs/dotfiles-doctor-$(date +%Y%m%d-%H%M%S).log"
    mkdir -p "$HOME/Library/Logs"
    if "$dir/bin/dfinstall" doctor >"$report" 2>&1; then
        ok "Environment report: ${report}"
    else
        warn "dfinstall doctor found problems; see ${report}"
    fi
}

main() {
    local branch="main" dir="" do_ghostty=true do_dotfiles=true
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

    # Piped from curl, stdin is the script itself. Homebrew's installer then
    # goes non-interactive and aborts unless sudo is already cached, and
    # dfinstall's prompts read EOF. fd 3 is the terminal for the commands that
    # need one; stdin itself is left alone, since bash is still reading it.
    if [[ ! -t 0 ]] && { : </dev/tty; } 2>/dev/null; then
        exec 3</dev/tty
    else
        exec 3<&0
    fi

    preflight
    install_clt
    wire_tls_trust
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
