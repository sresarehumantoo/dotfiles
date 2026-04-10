#!/usr/bin/env bash
set -euo pipefail

# Suppress noisy warnings on minimal systems (fixed once packages are installed)
export DEBIAN_FRONTEND=noninteractive
export PERL_BADLANG=0
export LC_ALL=C

# ── Colors & symbols ─────────────────────────────────────────────
_BOLD='\033[1m'  _DIM='\033[2m'  _RESET='\033[0m'
_BLUE='\033[34m' _GREEN='\033[32m' _YELLOW='\033[33m'
_RED='\033[31m'  _CYAN='\033[36m'

info()   { printf "${_BLUE}${_BOLD}  ▸${_RESET} %s\n" "$*"; }
ok()     { printf "${_GREEN}${_BOLD}  ✓${_RESET} %s\n" "$*"; }
warn()   { printf "${_YELLOW}${_BOLD}  ⚠${_RESET} %s\n" "$*" >&2; }
err()    { printf "${_RED}${_BOLD}  ✗${_RESET} %s\n" "$*" >&2; }
die()    { err "$@"; exit 1; }
header() {
    local label="── $* ──"
    local width=60
    local pad=$(( width - ${#label} ))
    (( pad < 0 )) && pad=0
    local trail=""
    for (( i=0; i<pad; i++ )); do trail+="─"; done
    printf "\n${_BOLD}${_CYAN}%s%s${_RESET}\n\n" "$label" "$trail"
}
step() { printf "${_DIM}  …${_RESET} %s\n" "$*"; }

# ── Spinner for long-running commands ────────────────────────────
_spin_pid=""
_spin_frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
trap 'spin_stop' EXIT

spin_start() {
    local msg="$1"
    (
        local i=0
        while true; do
            printf "\r${_CYAN}${_BOLD}  %s${_RESET} %s" "${_spin_frames[$((i % 10))]}" "$msg"
            i=$((i + 1))
            sleep 0.1
        done
    ) &
    _spin_pid=$!
}

spin_stop() {
    if [[ -n "$_spin_pid" ]]; then
        kill "$_spin_pid" 2>/dev/null
        wait "$_spin_pid" 2>/dev/null || true
        _spin_pid=""
        printf "\r\033[K"
    fi
}

# ── Helpers ──────────────────────────────────────────────────────
is_wsl() { [[ -f /proc/sys/fs/binfmt_misc/WSLInterop ]] || grep -qi microsoft /proc/version 2>/dev/null; }

run_hook() {
    local label="$1" path="$2"
    [[ -z "$path" ]] && return 0

    if [[ ! -f "$path" ]]; then
        die "${label}: file not found: ${path}"
    fi
    if [[ ! -x "$path" ]]; then
        chmod +x "$path"
    fi

    header "Running ${label}: $(basename "$path")"
    if "$path"; then
        ok "${label} completed"
    else
        die "${label} failed (exit $?): ${path}"
    fi
}

# ── Phase: Root Setup ────────────────────────────────────────────
setup_root() {
    local username="${1:?usage: setup-root <username>}"

    header "Updating system packages"
    spin_start "Updating package lists..."
    apt-get update -y > /dev/null 2>&1
    spin_stop
    ok "Package lists updated"

    spin_start "Upgrading installed packages..."
    apt-get full-upgrade -y > /dev/null 2>&1
    spin_stop
    ok "System packages upgraded"

    header "Installing base packages"
    spin_start "Installing debconf prerequisites..."
    apt-get install -y dialog perl > /dev/null 2>&1
    spin_stop
    ok "debconf ready"

    spin_start "Installing core packages (this may take a few minutes)..."
    apt-get install -y \
        sudo vim nano curl wget git make \
        build-essential cmake ninja-build gettext \
        golang python3 python3-pip python3-venv pipx \
        zsh htop rsync locales ca-certificates gnupg \
        iputils-ping dnsutils traceroute net-tools \
        dbus-x11 gdebi-core unzip tar jq lsb-release \
        > /dev/null 2>&1
    spin_stop
    ok "Core packages installed"

    header "Creating user: ${username}"
    if id "$username" &>/dev/null; then
        ok "User ${username} already exists"
    else
        useradd -m -s /bin/bash "$username"
        ok "Created user ${username}"
    fi

    # Ensure user is in sudo group
    usermod -aG sudo "$username"
    ok "Added ${username} to sudo group"

    # Set initial password
    echo "${username}:root" | chpasswd
    warn "Password set to 'root' - change it after setup with: passwd"

    if is_wsl; then
        header "Writing /etc/wsl.conf"
        cat > /etc/wsl.conf <<WSLCONF
[boot]
systemd=true

[automount]
enabled=true
root=/mnt/
options="metadata,umask=22,fmask=11"
mountFsTab=true

[network]
generateHosts=true
generateResolvConf=true

[interop]
enabled=true
appendWindowsPath=true

[user]
default=${username}

[time]
useWindowsTimezone=true
WSLCONF
        ok "/etc/wsl.conf written (default user: ${username})"
    else
        info "Not running in WSL - skipping wsl.conf"
    fi

    header "Configuring locale"
    local gen_path="/etc/locale.gen"
    local target_locale="en_US.UTF-8"
    if grep -q "^# *${target_locale}" "$gen_path" 2>/dev/null; then
        sed -i "s/^# *${target_locale}/${target_locale}/" "$gen_path"
    elif ! grep -q "^${target_locale}" "$gen_path" 2>/dev/null; then
        echo "${target_locale} UTF-8" >> "$gen_path"
    fi
    locale-gen > /dev/null 2>&1
    update-locale LANG="${target_locale}" LC_ALL="${target_locale}" 2>/dev/null
    ok "Locale set to ${target_locale}"

    ok "Root setup complete"
}

# ── Phase: Build Neovim ──────────────────────────────────────────
build_neovim() {
    header "Building Neovim from source"

    if command -v nvim &>/dev/null; then
        local current
        current="$(nvim --version | head -1)"
        warn "Neovim already installed: ${current}"
        info "Rebuilding from source..."
    fi

    local build_dir="/tmp/neovim-build"
    rm -rf "$build_dir"

    spin_start "Cloning neovim repository..."
    git clone --depth 1 https://github.com/neovim/neovim.git "$build_dir" > /dev/null 2>&1
    spin_stop
    ok "Repository cloned"

    cd "$build_dir"
    spin_start "Building neovim (this may take a few minutes)..."
    make -j"$(nproc)" CMAKE_BUILD_TYPE=RelWithDebInfo > /dev/null 2>&1
    spin_stop
    ok "Build complete"

    spin_start "Installing neovim..."
    make install > /dev/null 2>&1
    spin_stop

    rm -rf "$build_dir"

    local version
    version="$(nvim --version | head -1)"
    ok "Neovim installed: ${version}"
}

# ── Phase: Install Ghostty ───────────────────────────────────────
install_ghostty() {
    header "Installing Ghostty (latest .deb release)"

    local api_url="https://api.github.com/repos/dariogriffo/ghostty-debian/releases"
    local arch codename
    arch="$(dpkg --print-architecture)"
    codename="$(. /etc/os-release && echo "${VERSION_CODENAME:-bookworm}")"

    step "Detecting distro: ${codename} (${arch})"
    step "Fetching releases..."
    local releases_json
    releases_json="$(curl -sL "$api_url")"

    # Search all releases (not just latest) for a .deb matching our codename + arch
    local deb_url
    deb_url="$(echo "$releases_json" | jq -r \
        --arg codename "$codename" \
        --arg arch "$arch" \
        '[ .[] | .assets[] |
           select(.name | endswith(".deb")) |
           select(.name | contains($codename)) |
           select(.name | contains($arch))
         ] | first | .browser_download_url // empty')"

    # Fallback: try matching just the codename (any arch)
    if [[ -z "$deb_url" ]]; then
        deb_url="$(echo "$releases_json" | jq -r \
            --arg codename "$codename" \
            '[ .[] | .assets[] |
               select(.name | endswith(".deb")) |
               select(.name | contains($codename))
             ] | first | .browser_download_url // empty')"
    fi

    if [[ -z "$deb_url" ]]; then
        die "No Ghostty .deb found for ${codename}/${arch}. Check https://github.com/dariogriffo/ghostty-debian/releases"
    fi

    local deb_file="/tmp/ghostty-latest.deb"
    step "Downloading: $(basename "$deb_url")..."
    curl -sL -o "$deb_file" "$deb_url"

    step "Installing..."
    gdebi -n "$deb_file"
    rm -f "$deb_file"

    ok "Ghostty installed"
}

# ── Phase: Install Dotfiles ──────────────────────────────────────
install_dotfiles() {
    local branch="${1:-develop}"
    local source_dir="${2:-}"

    header "Setting up dotfiles"

    local target="$HOME/dotfiles"

    if [[ -n "$source_dir" && -d "$source_dir/.git" ]]; then
        # Copy from Windows-side clone already placed inside WSL
        if [[ "$source_dir" != "$target" ]]; then
            info "Copying dotfiles from ${source_dir}..."
            rm -rf "$target"
            cp -a "$source_dir" "$target"
        fi
        cd "$target"
        git checkout "$branch" 2>/dev/null || true
    elif [[ -d "$target/.git" ]]; then
        info "Dotfiles already present at ${target}"
        cd "$target"
        git fetch origin
        git checkout "$branch"
        git pull origin "$branch" || true
    else
        step "Cloning dotfiles repository..."
        rm -rf "$target"
        git clone https://github.com/sresarehumantoo/dotfiles.git "$target"
        cd "$target"
        git checkout "$branch"
    fi

    step "Building dfinstall..."
    make build

    ok "Dotfiles built at ${target}"

    header "Running dfinstall install all"
    ./bin/dfinstall install all
}

# ── Full standalone setup ────────────────────────────────────────
setup() {
    local username="" branch="develop"
    local do_neovim=true do_ghostty=true do_dotfiles=true
    local pre_hook="" post_hook=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --username)      username="$2"; shift 2 ;;
            --branch)        branch="$2"; shift 2 ;;
            --skip-neovim)   do_neovim=false; shift ;;
            --skip-ghostty)  do_ghostty=false; shift ;;
            --skip-dotfiles) do_dotfiles=false; shift ;;
            --pre-hook)      pre_hook="$2"; shift 2 ;;
            --post-hook)     post_hook="$2"; shift 2 ;;
            *) die "Unknown option: $1" ;;
        esac
    done

    # Prompt for username if not provided
    if [[ -z "$username" ]]; then
        if [[ "$(id -u)" -eq 0 ]]; then
            printf "${_YELLOW}${_BOLD}  ? ${_RESET}%s " "Linux username to create:"
            read -r username
            [[ -z "$username" ]] && die "Username required"
        else
            username="$(whoami)"
            info "Running as ${username}"
        fi
    fi

    # Root setup (needs root)
    if [[ "$(id -u)" -eq 0 ]]; then
        setup_root "$username"
    else
        info "Not running as root - skipping system setup"
        info "Run as root first for full setup: sudo $0 setup --username $username"
    fi

    # Run pre-hook (e.g. proxy setup, custom repos)
    run_hook "pre-hook" "$pre_hook"

    # These can run as root
    if [[ "$(id -u)" -eq 0 ]]; then
        [[ "$do_neovim" == true ]] && build_neovim
        [[ "$do_ghostty" == true ]] && install_ghostty
    fi

    # Dotfiles should run as the target user
    if [[ "$do_dotfiles" == true ]]; then
        if [[ "$(id -u)" -eq 0 ]]; then
            info "Switching to ${username} for dotfiles install..."
            su - "$username" -c "DFINSTALL_SUDO_PASS=root $(readlink -f "$0") install-dotfiles '$branch'"
        else
            install_dotfiles "$branch"
        fi
    fi

    # Run post-hook (e.g. custom config, additional tools)
    run_hook "post-hook" "$post_hook"

    header "Done"
    ok "Setup complete"
    warn "Password is 'root' - change it with: passwd"
    info "Open a new terminal or run: exec zsh"
}

show_help() {
    cat <<'HELP'
Usage: wsl-setup.sh <command> [options]

Commands:
  setup               Full interactive setup (standalone mode)
  setup-root <user>   System packages, user creation, locale, wsl.conf
  build-neovim        Build and install Neovim from source
  install-ghostty     Install latest Ghostty .deb for this distro
  install-dotfiles    Clone, build, and run dfinstall install all

Setup options:
  --username <name>   Linux username (prompted if omitted)
  --branch <branch>   Dotfiles branch (default: develop)
  --skip-neovim       Skip building Neovim
  --skip-ghostty      Skip installing Ghostty
  --skip-dotfiles     Skip dotfiles clone and install
  --pre-hook <path>   Script to run after root setup (e.g. proxy/repo config)
  --post-hook <path>  Script to run after dotfiles install (e.g. extra tools)

Examples:
  sudo ./wsl-setup.sh setup --username owen
  sudo ./wsl-setup.sh setup --skip-dotfiles --skip-ghostty
  sudo ./wsl-setup.sh setup --username owen --pre-hook ./setup-proxy.sh
  ./wsl-setup.sh install-dotfiles main
HELP
}

# ── Dispatch ─────────────────────────────────────────────────────
case "${1:-}" in
    setup)            shift; setup "$@" ;;
    setup-root)       shift; setup_root "$@" ;;
    build-neovim)     build_neovim ;;
    install-ghostty)  install_ghostty ;;
    install-dotfiles) shift; install_dotfiles "$@" ;;
    -h|--help|help)   show_help ;;
    *)
        if [[ -n "${1:-}" ]]; then
            err "Unknown command: $1"
            echo ""
        fi
        show_help
        exit 1
        ;;
esac
