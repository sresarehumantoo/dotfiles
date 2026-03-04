# Module Reference

Every module is a single Go file in `src/modules/`. They run in the order listed here.

---

## packages

**File:** `modules/packages.go`

Installs core system packages using the detected package manager (apt-get, dnf, pacman, or brew).

**Packages:** git, zsh, curl, wget, htop, neovim, tmux, nodejs, npm, python3, golang, zsh-syntax-highlighting

Skips packages that are already installed. External command output is suppressed in default mode (shown with `-v`). Spinner pauses automatically for sudo password prompts.

**Status:** Checks 11 tool binaries via `$PATH`.

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

## toolkit

**File:** `modules/toolkit.go`, `modules/toolkit_menu.go`

Optional security, CTF, development, and productivity tools. Running `dfinstall install toolkit --toolkit` or `dfinstall install all --toolkit` opens an interactive multi-select menu.

### External Registry

Tool metadata (names, descriptions, install methods) is stored in a separate GitHub repository ([dotfiles-toolkit](https://github.com/sresarehumantoo/dotfiles-toolkit)) and fetched at runtime. This keeps the main dfinstall binary free of security tool names that might trigger EDR string-based heuristics.

**Registry URL:** `https://raw.githubusercontent.com/sresarehumantoo/dotfiles-toolkit/main/registry.json`

**Cache:** `~/.local/share/dfinstall/toolkit-registry.json`

**Fetch behavior:**
- `--toolkit` flag: always fetches the latest registry before showing the menu
- Normal install (no `--toolkit`): uses the cached registry; fetches if no cache exists
- If no cache and no `--toolkit`: warns and skips toolkit installation

**Offline / custom registries:**
- `--registry <path>` CLI flag overrides the registry URL for a single run
- `toolkit_registry_url` in `.config.yaml` sets a persistent override
- Supports `file://` paths, plain file paths, and HTTP(S) URLs

### Tool Categories

The registry defines tools across 11 categories (recon, web testing, password cracking, network tools, forensics, reverse engineering, active directory, post-exploitation, wordlists, development, applications). See the [registry repository](https://github.com/sresarehumantoo/dotfiles-toolkit) for the full tool list.

### Install Methods

- **apt:** Bulk-installed via the detected package manager
- **go install:** Installed to `$GOPATH/bin`, skipped if binary already in PATH
- **cargo install:** Installed to `~/.cargo/bin/`, skipped if binary already in PATH (requires Rust toolchain)
- **pipx:** Installed via `pipx install`, skipped if already in `pipx list`
- **git clone:** Shallow-cloned to `~/.local/share/toolkit/<name>`, skipped if directory exists
- **AppImage:** Downloaded from GitHub releases API to `~/.local/bin/`, chmod +x

Selections are saved to `.config.yaml` under `toolkit_tools`. Subsequent installs (without `--toolkit`) use the saved selections. To change, re-run with `--toolkit`.

**Status:** Shows `N/M tools` when tools are configured. Shows "run --toolkit to configure" when no tools are selected. Shows "registry not fetched" when no cache exists.

**Uninstall:** Removes AppImage files from `~/.local/bin/` and git clone directories from `~/.local/share/toolkit/`. apt/go/cargo/pipx tools must be removed manually.

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

**File:** `modules/omz.go`, `modules/omz_extended.go`

Sets up Oh My Zsh, two custom plugins, and optional extended plugins:

| Component | Install method | Destination |
|-----------|---------------|-------------|
| Oh My Zsh | curl installer (`RUNZSH=no CHSH=no`) | `~/.oh-my-zsh/` |
| zsh-autosuggestions | git clone | `$ZSH_CUSTOM/plugins/zsh-autosuggestions` |
| powerlevel10k | git clone --depth=1 | `$ZSH_CUSTOM/themes/powerlevel10k` |

Skips anything already installed. Git clone output is suppressed in default mode (shown with `-v`).

### Core Plugins

These are always loaded in the zshrc `plugins=()` array: `git`, `zsh-autosuggestions`, `docker`, `terraform`, `fzf`, `golang`.

### Extended Plugins (`--extended`)

Running `dfinstall install omz --extended` or `dfinstall install all --extended` opens an interactive multi-select menu with 22 optional OMZ plugins across 5 categories:

| Category | Plugins |
|----------|---------|
| Container & Orchestration | kubectl, helm, docker-compose |
| Cloud | aws, gcloud, azure |
| Languages & Tools | npm, yarn, pip, rust, python, ruby, dotnet |
| DevOps | ansible, vagrant |
| Utilities | sudo, rsync, systemd, encode64, jsontools, urltools, command-not-found |

Selections are saved to `.config.yaml` under `extended_plugins` and written to `~/.config/dfinstall/plugins.zsh`, which the zshrc sources before the `plugins=()` declaration. Plugin names are validated against `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` before writing to prevent shell injection.

Subsequent installs (without `--extended`) regenerate the `plugins.zsh` file from the saved config. To change selections, re-run with `--extended`.

**Status:** Checks 3 directories. Shows `+N extended` when extended plugins are configured.

---

## shell

**File:** `modules/shell.go`, `modules/shell_preserve.go`

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
| `shell/zsh/ssh.zsh` | `~/.zsh.d/ssh.zsh` |

The zshrc sources p10k instant prompt, loads oh-my-zsh, then sources all `~/.zsh.d/*.zsh` files for modular configuration.

### Custom Shell File Preservation

Before linking, the shell module scans `$HOME` for custom shell files that aren't managed by dfinstall (e.g. `.companyrc`, `.work_env`, `.localrc`). If new files are found, an interactive multi-select menu lets the user choose which to keep sourcing after dfinstall replaces `~/.zshrc`.

Preserved files are written to `~/.config/dfinstall/custom-sources.zsh`, which the zshrc sources after aliases. Each entry uses a guard: `[[ -f ~/.companyrc ]] && source ~/.companyrc`.

User choices are saved to `.config.yaml` as `preserved_files` and `dismissed_files` so the menu isn't re-shown for already-classified files. Paths are validated against `^\.[a-zA-Z0-9][a-zA-Z0-9._-]*$` before writing to the shell-sourced file.

The scan filters out: managed shell destinations (`.zshrc`, `.aliases`, etc.), known non-shell dotfiles (`.vimrc`, `.npmrc`, `.netrc`, etc.), symlinks, directories, and files over 1MB.

After linking, the shell module auto-generates zsh completions for dfinstall and writes them to `~/.zsh.d/_dfinstall.zsh`.

**Status:** Checks 10 symlinks. Shows `+N preserved` when preserved files are configured.

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

Symlinks tmux configuration and installs TPM with plugins:

| Source | Destination |
|--------|-------------|
| `tmux/tmux.conf` | `~/.config/tmux/tmux.conf` |
| (legacy symlink) | `~/.tmux.conf` -> `~/.config/tmux/tmux.conf` |

Also cleans up old oh-my-tmux artifacts (`.tmux.conf.local`).

**TPM (Tmux Plugin Manager):**
- Clones `tmux-plugins/tpm` to `~/.tmux/plugins/tpm` (skips if already present)
- Sets `TMUX_PLUGIN_MANAGER_PATH` in the tmux global environment before running the install script
- Runs `tpm/bin/install_plugins` to install declared plugins

**Plugins:**

| Plugin | Purpose |
|--------|---------|
| tmux-resurrect | Save/restore tmux sessions (prefix+Ctrl+s / prefix+Ctrl+r) |
| tmux-continuum | Automatic session save/restore on tmux start |
| tmux-yank | Clipboard copy from copy mode |
| tmux-logging | Pane logging, screen capture, history save (`~/.local/share/tmux/logs/`) |

**Status bar:** 2-line layout — line 0 is a transparent spacer (`bg=terminal,fill=terminal`) creating a gap between the pane content and the status bar; line 1 is the real status bar with a powerline theme. The left side shows a distro icon detected at install time (same Nerd Font v3 icons as powerlevel10k), read from `~/.config/dfinstall/distro-icon`.

Key config: Alt+A prefix, vi mode, mouse enabled, 50k history, vim-style pane navigation, custom 8-color powerline theme.

**Uninstall:** Removes symlinks and deletes `~/.tmux/plugins/`.

**Status:** Checks 2 symlinks. Shows `tpm +N plugins` when TPM is installed with plugins.

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
