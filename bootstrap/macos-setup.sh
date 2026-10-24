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
BREW_INSTALL_URL="https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

ASSUME_YES=false
BREW_PREFIX_REQ=""

show_help() {
    cat <<'HELP'
Usage: macos-setup.sh [options]

Installs the Xcode Command Line Tools, Homebrew, Go and Ghostty, then clones
(or reuses) the dotfiles and runs `make install`. Asks before each step that
changes the machine; declining one stops there and changes nothing further.

Options:
  --branch <branch>     Branch to clone (default: main). An existing clone is
                        used as it is, on whatever branch it has checked out.
  --dir <path>          Where the clone lives (default: the clone this script
                        is in, else ~/dotfiles)
  --brew-prefix <path>  Use, or install, a Homebrew at this path, for when the
                        default one cannot be written by this account. At most
                        13 characters (as long as /opt/homebrew) and every
                        prebuilt bottle works there; longer, and the ones that
                        are not relocatable compile from source.
  --skip-ghostty        Don't install Ghostty
  --skip-dotfiles       Stop after the prerequisites
  -y, --yes             Answer yes to every confirmation
  -h, --help            Show this help

Examples:
  ./bootstrap/macos-setup.sh
  ./bootstrap/macos-setup.sh --brew-prefix ~/homebrew
  curl -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
HELP
}

# Asks before a step that changes the machine. Reads the terminal on fd 3, so
# it works when the script is piped from curl.
confirm() {
    local reply
    if [[ "$ASSUME_YES" == true ]]; then
        return 0
    fi
    if [[ ! -t 3 ]]; then
        die "No terminal to answer \"$1\". Run it from a terminal, or pass --yes."
    fi
    printf "${_YELLOW}${_BOLD}  [?]${_RESET} %s [Y/n] " "$1"
    read -r reply <&3 || reply=n
    case "$reply" in
        "" | [Yy] | [Yy][Ee][Ss]) return 0 ;;
        *) return 1 ;;
    esac
}

# A declined confirmation ends the run cleanly: nothing after it has happened,
# and re-running picks up where this left off.
stop_here() {
    info "Stopped before: ${STEP}. Nothing past this point was changed; re-run to continue."
    exit 0
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

# Which Homebrew to use is discovered, not assumed: a managed Mac may have an
# IT-owned /opt/homebrew this account cannot write to, a Homebrew handed to the
# user at enrollment, or one the user installed somewhere of their own. The
# prefix a run settles on is recorded, and ~/.profile and dfinstall read it
# back first, so a read-only Homebrew earlier on PATH never wins over it.
BREW_PREFIX_FILE="$HOME/.config/dfinstall/brew-prefix"
BREW_PREFIX=""
READONLY_BREW=""

# Prefixes in the order they are considered; may repeat.
brew_candidates() {
    if [[ -n "$BREW_PREFIX_REQ" ]]; then
        echo "$BREW_PREFIX_REQ"
    fi
    if [[ -r "$BREW_PREFIX_FILE" ]]; then
        head -n1 "$BREW_PREFIX_FILE"
    fi
    if command -v brew &>/dev/null; then
        dirname "$(dirname "$(command -v brew)")"
    fi
    printf '%s\n' /opt/homebrew /usr/local "$HOME/homebrew"
}

# Homebrew supports one owning account; anyone else gets permission errors.
brew_writable() {
    [[ -w "$1/Cellar" ]] || { [[ ! -e "$1/Cellar" ]] && [[ -w "$1" ]]; }
}

# Sets BREW_PREFIX to the first Homebrew this account can install with, and
# READONLY_BREW to the first one found that it cannot.
find_brew() {
    local p
    BREW_PREFIX="" READONLY_BREW=""
    while IFS= read -r p; do
        [[ -n "$p" && -x "$p/bin/brew" ]] || continue
        if brew_writable "$p"; then
            BREW_PREFIX="$p"
            return 0
        fi
        [[ -n "$READONLY_BREW" ]] || READONLY_BREW="$p"
    done < <(brew_candidates)
    return 1
}

is_admin() {
    id -Gn | tr ' ' '\n' | grep -qx admin
}

# Everything here runs as the user. On a managed Mac the things that can stop
# it are known up front, so report them before installing anything.
preflight() {
    header "Preflight"
    if is_admin; then
        ok "Admin account"
    else
        info "Standard account: apps go to ~/Applications when /Applications is not writable"
    fi
    if profiles status -type enrollment 2>/dev/null | grep -q 'MDM enrollment: Yes'; then
        info "MDM enrolled"
    fi
    if command -v santactl &>/dev/null && santactl status 2>/dev/null | grep -Eq 'Mode +\| +Lockdown'; then
        warn "Santa is in Lockdown: the dfinstall binary 'make install' builds, and other locally built tools, stay blocked until IT allowlists them"
    fi
    if find_brew; then
        ok "Homebrew at ${BREW_PREFIX} is writable"
    elif [[ -n "$READONLY_BREW" ]]; then
        warn "Homebrew at ${READONLY_BREW} is not writable by this account"
    else
        info "Homebrew is not installed yet"
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
    if [[ -n "$hosts" ]]; then
        info "TLS inspection on: ${hosts% }. Those fail verification in curl, git, Node, Python, cargo and Homebrew until they trust the System keychain's certificates."
    fi
    info "This writes ${CA_BUNDLE}, ${CA_ENV} (sourced by ~/.profile) and http.sslCAInfo in ~/.gitconfig.local."
    if ! confirm "Point those tools at the System keychain's certificates?"; then
        rm -f "$CA_BUNDLE.new"
        warn "Skipped. Downloads through the proxy may fail with certificate errors from here on."
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
    info "Needed for git, make and a C compiler. On a managed Mac IT or Self Service may provide them instead."
    confirm "Open Apple's Command Line Tools installer?" || stop_here
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

brew_options() {
    die "No Homebrew this account can install with. Options:
      - get temporary admin (e.g. Privileges or Admin By Request) and re-run, so the official installer can finish
      - ask IT for a Homebrew this account owns
      - re-run with --brew-prefix <path> for a separate Homebrew this account owns, e.g. --brew-prefix ~/homebrew"
}

# The official installer, at its default prefix or at $1. NONINTERACTIVE only
# under --yes: otherwise it shows its own plan and asks before acting.
run_brew_installer() {
    local script
    script="$(curl -fsSL "$BREW_INSTALL_URL")" || return 1
    (
        if [[ "$ASSUME_YES" == true ]]; then
            export NONINTERACTIVE=1
        fi
        if [[ $# -gt 0 ]]; then
            /bin/bash -c "$script" install.sh --path "$1" <&3
        else
            /bin/bash -c "$script" <&3
        fi
    )
}

# Homebrew patches /opt/homebrew inside prebuilt bottles in place, so a prefix
# no longer than that can use every bottle, and its installer only accepts
# those. A longer one needs the release tarball, and the bottles that are not
# relocatable then compile from source.
install_brew_at() {
    local prefix="${1%/}"
    [[ "$prefix" == /* ]] || die "--brew-prefix must be an absolute path"
    if [[ -e "$prefix" && -n "$(ls -A "$prefix")" ]]; then
        die "${prefix} exists, is not empty and has no Homebrew in it; pick another --brew-prefix"
    fi
    if [[ ${#prefix} -le ${#HOMEBREW_DEFAULT_PREFIX} ]]; then
        info "${prefix} is no longer than ${HOMEBREW_DEFAULT_PREFIX}, so every prebuilt bottle works there."
        confirm "Install Homebrew into ${prefix} with the official installer?" || stop_here
        run_brew_installer "$prefix" || die "The Homebrew installer failed for ${prefix}"
    else
        warn "${prefix} is longer than ${HOMEBREW_DEFAULT_PREFIX}. Bottles that are not relocatable (coreutils, gawk, make, bash, node, neovim, wget, openssl, gettext, ...) compile from source there, which can take an hour or more the first time."
        confirm "Install Homebrew into ${prefix} from its release tarball (Homebrew's unsupported alternative install)?" || stop_here
        mkdir -p "$prefix"
        curl -fsSL https://github.com/Homebrew/brew/tarball/main | tar xz --strip-components 1 -C "$prefix"
        "$prefix/bin/brew" update --force --quiet
    fi
    [[ -x "$prefix/bin/brew" ]] || die "No brew at ${prefix}/bin/brew after installing"
    brew_writable "$prefix" || die "Homebrew at ${prefix} is not writable by this account"
    BREW_PREFIX="$prefix"
}

HOMEBREW_DEFAULT_PREFIX=/opt/homebrew
if [[ "$(uname -m)" != arm64 ]]; then
    HOMEBREW_DEFAULT_PREFIX=/usr/local
fi

install_brew() {
    header "Homebrew"
    if [[ -n "$BREW_PREFIX_REQ" ]]; then
        if [[ -x "$BREW_PREFIX_REQ/bin/brew" ]]; then
            brew_writable "$BREW_PREFIX_REQ" || die "Homebrew at ${BREW_PREFIX_REQ} is not writable by this account"
            BREW_PREFIX="$BREW_PREFIX_REQ"
        else
            install_brew_at "$BREW_PREFIX_REQ"
        fi
    elif find_brew; then
        ok "Using Homebrew at ${BREW_PREFIX}"
    else
        if [[ -n "$READONLY_BREW" ]]; then
            warn "Homebrew at ${READONLY_BREW} is not writable by this account. With admin, the official installer offers to make this account its owner; if IT manages that Homebrew, that may conflict with them, and --brew-prefix avoids it."
        fi
        info "The official installer uses ${HOMEBREW_DEFAULT_PREFIX}, shows what it will do, and asks for your password if that needs admin."
        confirm "Run the official Homebrew installer?" || stop_here
        run_brew_installer || brew_options
        find_brew || brew_options
    fi
    eval "$("$BREW_PREFIX/bin/brew" shellenv)"
    mkdir -p "$(dirname "$BREW_PREFIX_FILE")"
    printf '%s\n' "$BREW_PREFIX" >"$BREW_PREFIX_FILE"
    ok "$(brew --version | head -1) at ${BREW_PREFIX}"
}

install_go() {
    header "Go"
    if command -v go &>/dev/null; then
        ok "Already installed: $(go version)"
        return
    fi
    confirm "brew install go (needed to build dfinstall)?" || stop_here
    brew install go
    ok "$(go version)"
}

# Ghostty's stable builds live only on its own download site, which the cask
# fetches from. GitHub carries just the rolling nightly ("tip"), so that is the
# fallback when the site is blocked.
GHOSTTY_TIP_API="https://api.github.com/repos/ghostty-org/ghostty/releases/tags/tip"
GHOSTTY_TIP_ZIP="ghostty-macos-universal.zip"

install_ghostty() {
    header "Ghostty"
    if [[ -d /Applications/Ghostty.app || -d "$HOME/Applications/Ghostty.app" ]]; then
        ok "Already installed"
        return
    fi
    if ! confirm "brew install --cask ghostty?"; then
        info "Skipped Ghostty"
        return
    fi
    # /Applications is admin-only; a standard account installs per user.
    local appdir=/Applications
    [[ -w /Applications ]] || appdir="$HOME/Applications"
    if brew install --cask --appdir="$appdir" ghostty; then
        ok "Installed"
        return
    fi
    warn "Homebrew could not install Ghostty; its download site may be blocked on this network."
    if ! confirm "Install Ghostty's nightly build from GitHub instead? (GitHub has no stable release of it)"; then
        info "Skipped Ghostty"
        return
    fi
    install_ghostty_tip "$appdir"
}

# The nightly macOS build from GitHub, checked against the sha256 GitHub
# recorded for it.
install_ghostty_tip() {
    local appdir="$1" tmp digest
    tmp="$(mktemp -d)"
    if ! digest="$(curl -fsSL --max-time 60 "$GHOSTTY_TIP_API" | python3 -I -c '
import json, sys
for a in json.load(sys.stdin)["assets"]:
    if a["name"] == sys.argv[1]:
        print(a.get("digest") or "")' "$GHOSTTY_TIP_ZIP")"; then
        rm -rf "$tmp"
        die "Could not reach GitHub for Ghostty either. Install it from Self Service, or download it on another network."
    fi
    if [[ "$digest" != sha256:* ]]; then
        rm -rf "$tmp"
        die "${GHOSTTY_TIP_ZIP} has no published sha256 digest; refusing to install it"
    fi
    info "Downloading ${GHOSTTY_TIP_ZIP}..."
    if ! curl -fsSL --retry 3 -o "$tmp/$GHOSTTY_TIP_ZIP" \
        "https://github.com/ghostty-org/ghostty/releases/download/tip/$GHOSTTY_TIP_ZIP"; then
        rm -rf "$tmp"
        die "Downloading ${GHOSTTY_TIP_ZIP} failed"
    fi
    if ! echo "${digest#sha256:}  $tmp/$GHOSTTY_TIP_ZIP" | shasum -a 256 -c --quiet -; then
        rm -rf "$tmp"
        die "sha256 mismatch for ${GHOSTTY_TIP_ZIP}"
    fi
    # ditto keeps the bundle's signature and extended attributes intact.
    ditto -x -k "$tmp/$GHOSTTY_TIP_ZIP" "$tmp/unzipped"
    if [[ ! -d "$tmp/unzipped/Ghostty.app" ]]; then
        rm -rf "$tmp"
        die "${GHOSTTY_TIP_ZIP} holds no Ghostty.app"
    fi
    mkdir -p "$appdir"
    ditto "$tmp/unzipped/Ghostty.app" "$appdir/Ghostty.app"
    rm -rf "$tmp"
    ok "Installed Ghostty's nightly build in ${appdir}. 'brew install --cask ghostty' moves to the stable release once the download site is reachable."
}

install_dotfiles() {
    local branch="$1" dir="$2"
    header "Dotfiles"
    if [[ -d "$dir/.git" ]]; then
        info "Using the existing clone at ${dir} ($(git -C "$dir" branch --show-current))"
    elif [[ -e "$dir" ]]; then
        die "${dir} exists but is not a git clone; move it or pass --dir"
    else
        confirm "Clone the dotfiles (${branch}) into ${dir}?" || stop_here
        git clone --branch "$branch" "$REPO_URL" "$dir"
    fi
    info "make install builds dfinstall and runs 'dfinstall install all': Homebrew packages, then symlinks for the shell, git, nvim, tmux and Ghostty configs into your home folder. 'dfinstall install all --dry-run' shows the full list first."
    confirm "Run make install?" || stop_here
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
            --brew-prefix)   BREW_PREFIX_REQ="${2:?--brew-prefix needs a value}"; BREW_PREFIX_REQ="${BREW_PREFIX_REQ%/}"; shift 2 ;;
            --skip-ghostty)  do_ghostty=false; shift ;;
            --skip-dotfiles) do_dotfiles=false; shift ;;
            -y|--yes)        ASSUME_YES=true; shift ;;
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

    header "Plan"
    info "1. Xcode Command Line Tools, if missing"
    info "2. Check for TLS inspection, and trust the System keychain's certificates if found"
    info "3. Homebrew: ${BREW_PREFIX:-${BREW_PREFIX_REQ:-install with the official installer}}"
    info "4. Go$([[ "$do_ghostty" == true ]] && printf ' and Ghostty')"
    if [[ "$do_dotfiles" == true ]]; then
        info "5. Dotfiles in ${dir}, then make install"
    fi
    info "Each step that changes something asks first."
    confirm "Continue?" || stop_here

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

# Sourced, as scripts/corplab does to exercise single steps, it only defines
# the functions. Piped into bash it is not sourced, so main runs.
(return 0 2>/dev/null) || main "$@"
