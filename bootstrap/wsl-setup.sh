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

# ── Phase: Root Setup ────────────────────────────────────────────
setup_root() {
    local username="${1:?usage: setup-root <username>}"

    header "Updating system packages"
    apt-get update -y
    apt-get full-upgrade -y

    header "Installing base packages"
    # dialog + readline first so debconf works for subsequent installs
    apt-get install -y dialog perl
    apt-get install -y \
        sudo vim nano curl wget git make \
        build-essential cmake ninja-build gettext \
        golang python3 python3-pip python3-venv pipx \
        zsh htop rsync locales ca-certificates gnupg \
        iputils-ping dnsutils traceroute net-tools \
        dbus-x11 gdebi-core unzip tar jq lsb-release

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

    header "Configuring locale"
    local gen_path="/etc/locale.gen"
    local target_locale="en_US.UTF-8"
    if grep -q "^# *${target_locale}" "$gen_path" 2>/dev/null; then
        sed -i "s/^# *${target_locale}/${target_locale}/" "$gen_path"
    elif ! grep -q "^${target_locale}" "$gen_path" 2>/dev/null; then
        echo "${target_locale} UTF-8" >> "$gen_path"
    fi
    locale-gen
    update-locale LANG="${target_locale}" LC_ALL="${target_locale}"
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

    step "Cloning neovim repository..."
    git clone --depth 1 https://github.com/neovim/neovim.git "$build_dir"

    step "Building (RelWithDebInfo)..."
    cd "$build_dir"
    make -j"$(nproc)" CMAKE_BUILD_TYPE=RelWithDebInfo

    step "Installing..."
    make install

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

# ── Dispatch ─────────────────────────────────────────────────────
case "${1:-}" in
    setup-root)      shift; setup_root "$@" ;;
    build-neovim)    build_neovim ;;
    install-ghostty) install_ghostty ;;
    install-dotfiles) shift; install_dotfiles "$@" ;;
    *)
        err "Unknown command: ${1:-}"
        echo "Usage: $0 {setup-root|build-neovim|install-ghostty|install-dotfiles}"
        exit 1
        ;;
esac
