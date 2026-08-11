# Module Reference

Most modules are a single Go file in `src/modules/`. They run in the order listed here.

---

## locale

**File:** `modules/locale.go`

Configures the system locale (`en_US.UTF-8`). Skips if the locale is already available.

If `locale-gen` is missing, installs the `locales` package first. Then uncomments `en_US.UTF-8 UTF-8` in `/etc/locale.gen` (appending it if absent), runs `sudo locale-gen` to generate the locale, and sets it as the system default with `sudo update-locale LANG=en_US.UTF-8`.

**Status:** Checks for `locale-gen` on `$PATH` and whether `en_US.UTF-8` is available (2 checks).

---

## packages

**File:** `modules/packages.go`

Installs core system packages using the detected package manager (apt-get, dnf, pacman, or brew).

**Packages:** git, zsh, curl, wget, htop, rsync, tmux, nodejs, npm, python3, golang, locales, zsh-syntax-highlighting

Neovim is deliberately **not** installed here — apt's neovim is too old (Debian stable ships 0.7–0.10, telescope.nvim needs >= 0.11), so the nvim module installs the official prebuilt tarball instead.

Skips packages that are already installed. External command output is suppressed in default mode (shown with `-v`). Spinner pauses automatically for sudo password prompts.

**Status:** Checks 11 tool binaries via `$PATH`.

---

## extras

**File:** `modules/extras.go`

Three groups of additional tooling:

**CLI Utilities:** xclip, tree, fzf, ripgrep, fd-find, bat, jq, unzip, make, build-essential, tealdeer

**Python Tooling:** python3, pip3, python3-venv, pipx

**Docker:**
- Adds Docker apt repository (signed with GPG key)
- Installs docker-ce, docker-ce-cli, containerd.io, docker-buildx-plugin, docker-compose-plugin
- Adds current user to the `docker` group

**Terraform:**
- Adds HashiCorp apt repository (signed with GPG key)
- Installs terraform

Reads `/etc/os-release` for `VERSION_CODENAME` to construct apt repo URLs.

After installing tealdeer, updates the tldr page cache (best-effort — skipped silently on network failure).

**Status:** Checks 18 binaries/packages (11 CLI utilities, 3 Python binaries, the `python3-venv` package, docker binary + group membership, and terraform).

---

## toolkit

**File:** `modules/toolkit.go`, `modules/toolkit_menu.go`, `modules/toolkit_artifact.go` (where each method's artifact lands on disk), `modules/github_release.go` (shared GitHub-release fetch/selection)

Optional security, CTF, DFIR, development, and productivity tools. Running `dfinstall install toolkit --toolkit` or `dfinstall install all --toolkit` opens a two-level interactive menu (category picker → tool multi-select). Installed tools are shown with `✓` and pre-selected. Deselecting an installed tool marks it for removal. The Done option shows summary stats: installed, to install, to remove.

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

The registry defines 50 tools across 15 categories: Active Directory, Applications, Browsers, DFIR, Development, Forensics & Stego, Network Tools, Office, Password Cracking, Post-Exploitation, Recon & Scanning, Reverse Engineering, System, Web Testing, Wordlists. See the [registry repository](https://github.com/sresarehumantoo/dotfiles-toolkit) for the full tool list.

### Install Methods

- **apt:** Bulk-installed via the detected package manager
- **go install:** Installed to `$GOPATH/bin`, skipped if binary already in PATH
- **cargo install:** Installed to `~/.cargo/bin/`, skipped if binary already in PATH (requires Rust toolchain)
- **pipx:** Installed via `pipx install`, skipped if already in `pipx list`
- **git clone:** Shallow-cloned to `~/.local/share/toolkit/<name>`, skipped if directory exists
- **AppImage:** Downloaded from the GitHub releases API to `~/.local/bin/<binary>.AppImage`, chmod +x
- **deb:** `.deb` from a GitHub release, installed with `dpkg -i` (falls back to `apt install -f` for dependencies)
- **release_binary:** A bare binary (or one inside a `.tar.gz`) from a GitHub release, placed at `~/.local/bin/<binary>`
- **rustup:** Installs the rustup toolchain to `~/.cargo/bin/rustup`

The three GitHub-release methods (deb, AppImage, release_binary) share one
engine in `modules/github_release.go` — it fetches the latest release's assets
and picks the one matching this machine's architecture, skipping checksums,
signatures and other platforms' builds.

Selections are saved to `.config.yaml` under `toolkit_tools`. Subsequent installs (without `--toolkit`) use the saved selections. To change, re-run with `--toolkit`.

**Status:** Shows `N/M tools` when tools are configured. Shows "run --toolkit to configure" when no tools are selected. Shows "registry not fetched" when no cache exists.

**Uninstall:** Removes what dfinstall placed — AppImage and release-binary files
from `~/.local/bin/`, git clones from `~/.local/share/toolkit/` (via
`core.RemoveManagedDir`), `.deb` packages via `sudo dpkg -r`, and rustup via
`rustup self uninstall`. apt/go/cargo/pipx tools must be removed manually, since
their package manager owns the location.

Where each method's artifact lives is decided in one place —
`modules/toolkit_artifact.go` — which the installers, `Status`, `Uninstall` and
the selection menu all consult. A second copy of that mapping would mean
uninstall deleting the wrong path.

---

## delta

**File:** `modules/delta.go`

Installs [delta](https://github.com/dandavison/delta), a syntax-highlighting pager for git diffs.

Tries the latest `.deb` release from GitHub first (auto-detects architecture via `dpkg --print-architecture`). Falls back to the system package manager if the download fails.

**Status:** Reports "installed" or "not found".

---

## fonts

**File:** `modules/fonts.go`

Installs the terminal fonts into `$XDG_DATA_HOME/fonts` (not a hardcoded
`~/.local/share/fonts` — fontconfig's default config resolves its font directory
against `XDG_DATA_HOME`, so a machine that sets it elsewhere would otherwise get
fonts in a directory nothing scans).

| Font | Source | Where |
|------|--------|-------|
| IosevkaTerm Nerd Font (Regular/Bold/Italic/BoldItalic) | Downloaded from the pinned Nerd Fonts release | `$XDG_DATA_HOME/fonts/IosevkaTerm/` |
| MesloLGS NF Regular.ttf | Bundled in `config/fonts/`, symlinked | `$XDG_DATA_HOME/fonts/` |

IosevkaTerm is downloaded rather than vendored (~28 MB per version is too much
to carry in `.git`), sha256-verified against the release's `SHA-256.txt` before
a byte is extracted, and only the four canonical faces are written — never the
Mono build (clamps icons to one cell) or Propo (not monospace). MesloLGS is the
vendored offline floor, so a box that never reaches the network still renders a
working prompt. Runs `fc-cache -f` when the font set changed.

**Pinned version:** `nerdFontsTag` in `modules/fonts.go`. The installed tag is
recorded in a `.nerd-fonts-tag` stamp beside the faces, so bumping the constant
actually causes a re-download — detection is a filesystem check of the directory
the module owns, deliberately *not* an `fc-list` family query, which is
version-blind and also true of a copy hand-installed anywhere else.

**Migration:** removes the faces the pre-IosevkaTerm module installed
(`HackNerdFont*.ttf`), and the `MesloLGS NF Regular.ttf.bak` that linking
displaces — a `.bak` in a font directory is still a live font, since fontconfig
identifies files by content rather than extension.

**Status:** counts the vendored links; reports a missing or stale download, and
any legacy artifacts still to clean, in the INFO column.

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

Running `dfinstall install omz --extended` or `dfinstall install all --extended` opens a two-level interactive menu (category picker → plugin multi-select) with 22 optional OMZ plugins across 5 categories. Available plugins (OMZ directory exists) are shown with `✓`:

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
| `shell/zsh/locale.zsh` | `~/.zsh.d/locale.zsh` |

The zshrc sources p10k instant prompt, loads oh-my-zsh, then sources all `~/.zsh.d/*.zsh` files for modular configuration.

### Custom Shell File Preservation

Before linking, the shell module scans `$HOME` for custom shell files that aren't managed by dfinstall (e.g. `.companyrc`, `.work_env`, `.localrc`). If new files are found, an interactive multi-select menu lets the user choose which to keep sourcing after dfinstall replaces `~/.zshrc`.

Preserved files are written to `~/.config/dfinstall/custom-sources.zsh`, which the zshrc sources after aliases. Each entry uses a guard: `[[ -f ~/.companyrc ]] && source ~/.companyrc`.

User choices are saved to `.config.yaml` as `preserved_files` and `dismissed_files` so the menu isn't re-shown for already-classified files. Paths are validated against `^\.[a-zA-Z0-9][a-zA-Z0-9._-]*$` before writing to the shell-sourced file.

The scan filters out: managed shell destinations (`.zshrc`, `.aliases`, etc.), known non-shell dotfiles (`.vimrc`, `.npmrc`, `.netrc`, etc.), symlinks, directories, and files over 1MB.

After linking, the shell module auto-generates zsh completions for dfinstall and writes them to `~/.zsh.d/_dfinstall.zsh`.

**Status:** Checks 11 symlinks. Shows `+N preserved` when preserved files are configured.

---

## devtools

**File:** `modules/devtools.go`

Symlinks utility scripts into `~/.local/bin/`:

| Script | Purpose |
|--------|---------|
| `_lib.sh` | Shared output helpers (colors, guards, confirmation) |
| `wsl-resize-disk` | Compact WSL2 virtual disk (`--compact`, `--compact --dangerous` for in-place without backup) |
| `wsl-restart` | Restart WSL from within WSL |
| `docker-cleanup` | Full Docker system purge |
| `git-prune-branches` | Remove local branches with deleted remotes |
| `sysinfo` | System resource overview |
| `tlog-clean` | Strip ANSI escapes and powerline glyphs from tmux log captures |
| `clipboard-vm` | Diagnose and fix SPICE clipboard sharing in a QEMU/KVM guest (`--reset`) |
| `tmux-restore` | Toggle tmux session auto-restore (continuum + resurrect) on/off |
| `demorec` | Record the screen to mp4 and re-encode it small (`start`/`stop`/`status`/`render`) |
| `wsl-ffmpeg` | Install a standalone Windows ffmpeg.exe for demorec's WSL capture (`--dir`, `--yes`, `--force`) |

`tlog-clean` uses a virtual terminal line buffer to correctly resolve cursor movements and zsh line-editor edits. Detects powerlevel10k-style prompts and replaces them with a clean `directory $ command` format, dropping git info and decorations. Supports file arguments and stdin piping.

All scripts are `chmod 755` before linking. Individual failures are warned and counted rather than stopping the whole module.

See [Devtools Scripts](devtools.md) for detailed script documentation.

**Status:** Checks 9 symlinks.

---

## git

**File:** `modules/gitmod.go`

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

- **21 symlinks** covering init.lua, lazy-lock.json, .stylua.toml, lua files for custom plugins and kickstart plugins, and an `after/ftplugin/markdown.lua` editor-options file
- **Plugin sync:** Runs `nvim --headless "+Lazy! sync" "+qa"` after linking
- **Backup:** If an existing nvim config is a git repo (not symlinks), backs it up to `~/.config/nvim.bak`

Plugin output is suppressed in default mode and shown in verbose/debug mode.

**Custom plugins:** colorizer, comment, flash, harpoon, markdown, oil, smear-cursor, undotree

smear-cursor draws a cursor trail inside nvim. It disables itself when running under
Ghostty, whose `custom-shader` already draws one across the whole terminal — otherwise the
two stack into a double trail. Set `DF_SMEAR_CURSOR=1` to force it on anyway, or `0` to
force it off on a terminal without its own trail.

**Kickstart plugins:** autopairs, debug, gitsigns, indent_line, lint, neo-tree

**Status:** Checks 21 symlinks.

---

## windev

**File:** `modules/windev.go` &nbsp;·&nbsp; **See also:** [Windows Cross-Development guide](windev.md) for install + per-language usage walkthroughs.

**Opt-in.** Installs a Windows cross-development environment for building Windows software (C/C++, C#/.NET, Go, Rust) from WSL/Linux, plus the matching Neovim LSP/format/debug configs and a remote build-server helper. Excluded from `install all` until explicitly enabled with `dfinstall install windev`, which sets `windev_enabled: true` in `.config.yaml`; subsequent `install all` keeps it current. `dfinstall uninstall windev` clears the flag.

### Local cross-compilation toolchains

| Language | Installs | Builds Windows binaries via |
|----------|----------|----------------------------|
| C / C++  | `mingw-w64`, `cmake`, `ninja-build`, `clangd`, `clang-format` (apt) | `x86_64-w64-mingw32-{gcc,g++}` |
| Go       | `gopls`, `dlv` (via `go install`); Go itself via the packages module | `GOOS=windows GOARCH=amd64 go build` |
| Rust     | `rustup` (official installer if absent), target `x86_64-pc-windows-gnu`, components `rust-analyzer`, `rustfmt`, `clippy` | `cargo build --target x86_64-pc-windows-gnu` (links via MinGW) |
| C# / .NET | .NET SDK (`dotnet-install.sh` → `~/.dotnet`), OmniSharp + netcoredbg into `~/.local/share/windev/`, `csharpier` (global dotnet tool) | `dotnet publish -c Release -r win-x64` |

Each language is best-effort: a failure warns but doesn't abort the rest, so a single missing toolchain never blocks the nvim/helper wiring.

### Neovim language support

Symlinks `config/nvim/windev/windev.lua` to `~/.config/nvim/lua/custom/plugins/windev.lua` — kickstart's `{ import = 'custom.plugins' }` picks it up automatically. The file:

- Sets up the **OmniSharp** LSP for C# (init.lua's `servers` table already covers `clangd`, `gopls`, and `rust_analyzer` — they attach as soon as the binaries are on PATH).
- Adds **conform formatters**: `clang-format` (c/cpp), `csharpier` (cs), `goimports`+`gofmt` (go), `rustfmt` (rust).
- Adds **nvim-lint linters**: `cpplint` (c/cpp), `golangci-lint` (go), `clippy` (rust).
- Wires **DAP** adapters: `codelldb` for C/C++/Rust (if on PATH) and `netcoredbg` for C# (from `~/.local/share/windev/`).

The file uses dependencies + live runtime-table mutation so it extends conform/nvim-lint/nvim-dap without clobbering init.lua's existing config functions. Defers loading via `ft = { cs, c, cpp, go, rust }` to keep startup fast.

### Remote Windows build server

Links `config/devtools/winbuild` → `~/.local/bin/winbuild` — a small helper that dispatches builds to a Windows machine over SSH (rsync source up, run remote build, pull artifacts back). See [Devtools Scripts → winbuild](devtools.md#winbuild) for usage.

### Shell PATH

Writes `~/.config/dfinstall/windev.zsh` (regenerated on each install) prepending `~/.cargo/bin`, `~/.dotnet`, `~/.dotnet/tools`, and `$(go env GOPATH)/bin`, and exporting `DOTNET_ROOT`. The zshrc sources this snippet automatically.

### Caveats

- Cross-compiled Windows `.exe` debugging from WSL is limited; DAP targets are wired for the binaries' native debuggers (delve for Go, codelldb/netcoredbg for Linux-native builds). Remote-Windows debugging is out of scope.
- Downloads (dotnet-install.sh, OmniSharp, netcoredbg) need network access at install time. Failures warn rather than abort.

**Status:** Reports `disabled` when not enabled. When enabled, shows linked file counts and a `mingw+dotnet+rust+go` summary of detected toolchains.

**Uninstall:** Removes the nvim plugin file, `winbuild`, and `windev.zsh`, and clears `windev_enabled`. Language toolchains/SDKs are deliberately left installed — they may be shared with other workflows.

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

**Status bar:** 2-line layout — line 0 is a transparent spacer (`bg=terminal,fill=terminal`) creating a gap between the pane content and the status bar; line 1 is the real status bar. Two rounded pills — the left holds a badge and the window list, the right holds every readout — on a transparent strip, with one accent colour used only where something is active. The left badge is a literal glyph in `tmux.conf` (U+F120, a terminal prompt); it was previously a distro icon read from `~/.config/dfinstall/distro-icon`, and `writeDistroIcon()` still writes that file but nothing reads it.

Key config: Alt+A prefix, vi mode, mouse enabled, 50k history, vim-style pane navigation, custom 8-color powerline theme.

**Uninstall:** Removes symlinks and deletes `~/.tmux/plugins/`.

**Status:** Checks 2 symlinks. Shows `tpm +N plugins` when TPM is installed with plugins.

---

## konsole

**File:** `modules/konsole.go`

Symlinks Konsole terminal configuration:

| Source | Destination |
|--------|-------------|
| `konsole/konsolerc` | `$XDG_CONFIG_HOME/konsolerc` |
| `konsole/Dotfiles.profile` | `~/.local/share/konsole/Dotfiles.profile` |
| `konsole/Dotfiles.colorscheme` | `~/.local/share/konsole/Dotfiles.colorscheme` |

**Uninstall:** Removes all three symlinks.

**Status:** Checks 3 symlinks.

---

## ghostty

**File:** `modules/ghostty.go`

Symlinks `config/ghostty/config` to `$XDG_CONFIG_HOME/ghostty/config`, plus the three
cursor-trail shaders in `config/ghostty/shaders/*.glsl` (vendored from
[sahaj-b/ghostty-cursor-shaders](https://github.com/sahaj-b/ghostty-cursor-shaders), MIT).
The config selects one via `custom-shader`; `cursor_warp.glsl` is the active default.

The whole module no-ops when `ghostty` isn't on `PATH` — including `Status()`, which
reports nothing rather than "missing".

Because Ghostty resolves a relative `custom-shader` against the config file's *resolved*
path, a symlinked config loads the shader straight out of the repo, so editing a `.glsl` is
live on the next Ghostty restart with no `dfinstall` run. The `shaders/` symlinks are
insurance for the case where that config is ever a real file. Config changes need a
restart, not just a reload.

**Status:** Checks 4 symlinks.

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

## vmguest

**File:** `modules/vmguest.go`

Installs hypervisor guest tools when running inside a hardware VM. Skips entirely on WSL and on systems that aren't VMs. Detects the hypervisor type via `core.DetectVirt(ctx)` and installs the matching packages:

| Hypervisor | Packages |
|------------|----------|
| KVM / QEMU | qemu-guest-agent, spice-vdagent |
| VMware | open-vm-tools |
| VirtualBox | virtualbox-guest-utils |
| Hyper-V | hyperv-daemons |

After installing, enables/starts the relevant systemd units (`qemu-guest-agent`, `spice-vdagentd`, `open-vm-tools`) when `systemctl` is available — static units are started rather than enabled. Prints hypervisor-specific hints for clipboard/drag-drop support (e.g. running `clipboard-vm` on KVM/QEMU, or installing `open-vm-tools-desktop` / `virtualbox-guest-x11`).

**Status:** Reports the detected virtualization type and checks the guest packages for that hypervisor. Returns empty on WSL and non-VM systems.

---

## defaultshell

**File:** `modules/defaultshell.go`

Sets zsh as the default login shell via `chsh -s $(which zsh)`. Skips if `$SHELL` is exactly the zsh path resolved from `$PATH` — a `$SHELL` of `/usr/local/bin/zsh` against a resolved `/usr/bin/zsh` does *not* count as already set.

If no sudo password was captured at startup and the session is non-interactive (e.g. under the MCP server, where stdin is the JSON-RPC stream), it warns with the `chsh -s <path>` command to run and does nothing. When a sudo password is available it runs `chsh` through `core.SudoCmd` with no stdin wiring; only the interactive fallback attaches stdin/stdout and may prompt for a password.

**Status:** Reports "zsh" if default, or the current shell path.
