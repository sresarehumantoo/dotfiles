# Module Reference

Every module is a single Go file in `src/modules/`. They run in the order listed here.

---

## packages

**File:** `modules/packages.go`

Installs core system packages using the detected package manager (apt-get, dnf, pacman, or brew).

**Packages:** git, zsh, curl, wget, htop, neovim, tmux, nodejs, npm, python3, golang, zsh-syntax-highlighting

Skips packages that are already installed. External command output is suppressed (stdout/stderr nil).

**Status:** Checks 10 tool binaries via `$PATH`.

---

## extras

**File:** `modules/extras.go`

Three groups of additional tooling:

**CLI Utilities:** xclip, tree, fzf, ripgrep, fd-find, bat, jq, unzip, make, build-essential

**Python Tooling:** python3, pip3, python3-venv, pipx

**Docker:**
- Adds Docker apt repository (signed with GPG key)
- Installs docker-ce, docker-ce-cli, containerd.io, docker-buildx-plugin, docker-compose-plugin
- Adds current user to the `docker` group

**Terraform:**
- Adds HashiCorp apt repository (signed with GPG key)
- Installs terraform

Reads `/etc/os-release` for `VERSION_CODENAME` to construct apt repo URLs.

**Status:** Checks 18 binaries/packages.

---

## delta

**File:** `modules/delta.go`

Installs [delta](https://github.com/dandavid/delta), a syntax-highlighting pager for git diffs.

Tries the latest `.deb` release from GitHub first (auto-detects architecture via `dpkg --print-architecture`). Falls back to the system package manager if the download fails.

**Status:** Reports "installed" or "not found".

---

## fonts

**File:** `modules/fonts.go`

Installs two fonts to `~/.local/share/fonts/`:

| Font | Source |
|------|--------|
| HackNerdFont-Regular.ttf | Bundled in `config/fonts/` |
| MesloLGS NF Regular.ttf | Bundled in `config/fonts/` |

If a bundled font is missing, falls back to downloading the Hack Nerd Font zip from GitHub. Runs `fc-cache -f` after installation.

**Status:** Checks existence and SHA-256 hash of both font files.

---

## omz

**File:** `modules/omz.go`

Sets up Oh My Zsh and two plugins:

| Component | Install method | Destination |
|-----------|---------------|-------------|
| Oh My Zsh | curl installer (`RUNZSH=no CHSH=no`) | `~/.oh-my-zsh/` |
| zsh-autosuggestions | git clone | `$ZSH_CUSTOM/plugins/zsh-autosuggestions` |
| powerlevel10k | git clone --depth=1 | `$ZSH_CUSTOM/themes/powerlevel10k` |

Skips anything already installed.

**Status:** Checks 3 directories.

---

## shell

**File:** `modules/shell.go`

Symlinks shell configuration files:

| Source | Destination |
|--------|-------------|
| `shell/zshrc` | `~/.zshrc` |
| `shell/aliases` | `~/.aliases` |
| `shell/p10k.zsh` | `~/.p10k.zsh` |
| `shell/bashrc` | `~/.bashrc` |
| `shell/profile` | `~/.profile` |
| `shell/zsh/options.zsh` | `~/.zsh.d/options.zsh` |
| `shell/zsh/keybinds.zsh` | `~/.zsh.d/keybinds.zsh` |
| `shell/zsh/path.zsh` | `~/.zsh.d/path.zsh` |
| `shell/zsh/exports.zsh` | `~/.zsh.d/exports.zsh` |

The zshrc sources p10k instant prompt, loads oh-my-zsh, then sources all `~/.zsh.d/*.zsh` files for modular configuration.

**Status:** Checks 9 symlinks.

---

## devtools

**File:** `modules/devtools.go`

Symlinks utility scripts into `~/.local/bin/`:

| Script | Purpose |
|--------|---------|
| `_lib.sh` | Shared output helpers (colors, guards, confirmation) |
| `wsl-resize-disk` | Compact WSL2 virtual disk |
| `wsl-restart` | Restart WSL from within WSL |
| `docker-cleanup` | Full Docker system purge |
| `git-prune-branches` | Remove local branches with deleted remotes |
| `sysinfo` | System resource overview |

All scripts are `chmod 755` before linking. Individual failures are warned and counted rather than stopping the whole module.

See [Devtools Scripts](devtools.md) for detailed script documentation.

**Status:** Checks 6 symlinks.

---

## git

**File:** `modules/git.go`

Symlinks `config/git/gitconfig` to `~/.gitconfig`.

Key settings:
- Default branch: `main`
- Push: `autoSetupRemote = true`
- Diff algorithm: `histogram`
- Merge conflict style: `zdiff3`
- Pager: `delta` with line numbers
- Aliases: `st`, `co`, `br`, `ci`, `lg`, `last`, `unstage`, `amend`

**Status:** Checks 1 symlink.

---

## nvim

**File:** `modules/nvim.go`

Sets up a full Neovim configuration under `~/.config/nvim/`:

- **16 symlinks** covering init.lua, lazy-lock.json, .stylua.toml, and lua files for custom plugins and kickstart plugins
- **Plugin sync:** Runs `nvim --headless "+Lazy! sync" "+qa"` after linking
- **Backup:** If an existing nvim config is a git repo (not symlinks), backs it up to `~/.config/nvim.bak`

Plugin output is suppressed in default mode and shown in verbose/debug mode.

**Custom plugins:** colorizer, comment, flash, harpoon, oil, undotree

**Kickstart plugins:** autopairs, debug, gitsigns, indent_line, lint, neo-tree

**Status:** Checks 16 symlinks.

---

## tmux

**File:** `modules/tmux.go`

Symlinks tmux configuration:

| Source | Destination |
|--------|-------------|
| `tmux/tmux.conf` | `~/.config/tmux/tmux.conf` |
| (legacy symlink) | `~/.tmux.conf` -> `~/.config/tmux/tmux.conf` |

Also cleans up old oh-my-tmux artifacts (`.tmux.conf.local`).

Key config: Alt+A prefix, vi mode, mouse enabled, 50k history, vim-style pane navigation, custom 8-color theme.

**Status:** Checks 2 symlinks.

---

## ghostty

**File:** `modules/ghostty.go`

Symlinks `config/ghostty/config` to `$XDG_CONFIG_HOME/ghostty/config`.

**Status:** Checks 1 symlink.

---

## htop

**File:** `modules/htop.go`

Symlinks `config/htop/htoprc` to `$XDG_CONFIG_HOME/htop/htoprc`.

**Status:** Checks 1 symlink.

---

## wsl

**File:** `modules/wsl.go`

WSL-specific setup. Skips entirely on non-WSL systems.

**Tasks:**

| Task | Target | Method |
|------|--------|--------|
| wsl.conf | `/etc/wsl.conf` | sudo copy (hash-checked) |
| sysctl | `/etc/sysctl.d/99-wsl.conf` | sudo copy (hash-checked), applied with `sysctl -p` |
| .wslconfig | `C:\Users\<user>\.wslconfig` | copy via Windows interop |
| Windows home link | `~/owen` -> `/mnt/c/Users/owen` | symlink |
| Git fsmonitor | global git config | `git config --global` |

Uses `cmd.exe` and `wslpath` for Windows path resolution. Prompts the user to restart WSL (`wsl --shutdown`) after changes.

**Status:** Checks wsl.conf, sysctl conf, and Windows home symlink. Reports "not WSL" on non-WSL systems.

---

## defaultshell

**File:** `modules/defaultshell.go`

Sets zsh as the default login shell via `chsh -s $(which zsh)`. Skips if `$SHELL` already ends with `zsh`.

Runs with stdin/stdout attached (may prompt for password).

**Status:** Reports "zsh" if default, or the current shell path.
