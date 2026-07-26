<p align="center">
  <img src="assets/logo.svg?v=2" alt="dfinstall logo" width="800">
</p>

<p align="center">
  <a href="https://github.com/sresarehumantoo/dotfiles/actions/workflows/ci.yml"><img src="https://github.com/sresarehumantoo/dotfiles/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
</p>

Personal dotfiles manager built in Go. A single `dfinstall` CLI symlinks config files into place, installs packages and tools, and keeps everything reproducible across machines.

Built for WSL2 (Debian/Ubuntu) but works on native Linux with apt, dnf, pacman, or brew.

## Quick Start

### Fresh WSL Setup (from PowerShell)

Bootstrap a fresh WSL Debian distro from scratch -- creates a user, installs packages, builds Neovim, installs Ghostty, and runs the full dotfiles install:

```powershell
git clone https://github.com/sresarehumantoo/dotfiles
.\dotfiles\bootstrap\wsl-bootstrap.ps1
```

The wizard prompts for distro and username, then handles everything end-to-end. Optional flags:

```powershell
.\bootstrap\wsl-bootstrap.ps1 -Distro Debian -Username owen   # skip prompts
.\bootstrap\wsl-bootstrap.ps1 -SkipNeovim                     # skip neovim build
.\bootstrap\wsl-bootstrap.ps1 -SkipGhostty                    # skip ghostty install
.\bootstrap\wsl-bootstrap.ps1 -SkipDotfiles                   # core tools only, no dotfiles
.\bootstrap\wsl-bootstrap.ps1 -SkipDotfiles -SkipNeovim -SkipGhostty  # bare minimum
```

### Existing Linux System

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make install
```

`make install` compiles the CLI and runs `dfinstall install all`, which walks through every module in dependency order: system packages, shell setup, editor configs, dev tools, and WSL-specific tuning.

## Usage

```bash
dfinstall install all             # install everything
dfinstall install shell           # install a single module
dfinstall install all -v          # verbose output (detailed logs)
dfinstall install all --debug     # debug output (verbose + internals)
dfinstall install all --dry-run   # show what would change without modifying anything
dfinstall install all --backup    # snapshot targets before modifying (restorable)
dfinstall install omz --extended  # interactive menu to select extended OMZ plugins
dfinstall install all --toolkit   # interactive menu to select toolkit tools
dfinstall install windev          # opt-in Windows cross-dev module (see docs/windev.md)
dfinstall update all              # re-apply all modules (alias for install)
dfinstall update omz --extended   # update and select extended OMZ plugins
dfinstall uninstall shell         # remove symlinks for a module
dfinstall uninstall all           # remove all managed symlinks
dfinstall status                  # show link status for all modules
dfinstall diff                    # show drift between config and filesystem
dfinstall doctor                  # run environment health checks
dfinstall root                    # apply a curated subset of configs to /root via sudo
dfinstall restore                 # restore latest backup
dfinstall restore <timestamp>     # restore a specific backup
dfinstall restore --list          # list available backups
dfinstall registry validate <src> # validate a toolkit registry file (CI helper)
```

By default the CLI shows an animated spinner. Pass `-v` for the full log output, `--debug` for additional detail, or `--dry-run` to preview changes without touching the filesystem.

### Backup & Restore

On the very first `install` run, dfinstall automatically creates a backup before modifying anything. After that first run, a `.config.yaml` is saved in the dotfiles root with `skip_backup: true`, so subsequent runs skip backups by default.

You can override this behavior:

- **`--backup` flag** — always creates a backup, regardless of config
- **`skip_backup: false`** in `.config.yaml` — backup on every install
- **`backup_dir`** in `.config.yaml` — custom backup location (default: `~/.local/share/dfinstall/backups/`)

See `.config.yaml.example` for all available options.

```bash
dfinstall install all --backup    # force a backup
dfinstall restore --list          # see available snapshots
dfinstall restore                 # revert to latest snapshot
```

Each entry records the original state (missing, symlink, or regular file) so restore can precisely undo what dfinstall changed.

## Modules

Modules run in this order (dependencies first):

| Module | What it does |
|--------|-------------|
| **locale** | Ensures en_US.UTF-8 locale is generated and set |
| **packages** | Core system packages via apt/dnf/pacman/brew (only installs what's missing) |
| **extras** | CLI utilities (fzf, ripgrep, bat, jq, fd), Python tooling, Docker, Terraform |
| **toolkit** | External tool registry for security/CTF/dev tools (`--toolkit` flag) |
| **delta** | Installs [delta](https://github.com/dandavison/delta) git diff viewer |
| **fonts** | Hack Nerd Font and MesloLGS NF (bundled or downloaded) |
| **omz** | Oh My Zsh + zsh-autosuggestions + powerlevel10k + extended plugin support (`--extended`) |
| **shell** | Symlinks zshrc, bashrc, aliases, p10k config, and modular zsh.d files |
| **devtools** | Utility scripts to `~/.local/bin/` (sysinfo, docker-cleanup, git-prune-branches, etc.) |
| **git** | Symlinks gitconfig (delta pager, histogram diff, aliases) |
| **nvim** | Neovim config with Lazy.nvim plugin manager + headless sync |
| **windev** | _Opt-in._ Windows cross-dev toolchains (MinGW-w64 / .NET / Rust windows-gnu / Go), nvim LSP+format+debug for C/C++/C#/Go/Rust, and `winbuild` SSH dispatch to a Windows build server. Enable with `dfinstall install windev` — see [Windows cross-development](docs/windev.md). |
| **tmux** | Tmux config (Alt+A prefix, vi mode, custom theme) |
| **konsole** | Konsole terminal profile and color scheme |
| **ghostty** | Ghostty terminal emulator config |
| **htop** | htop config |
| **wsl** | WSL-specific: wsl.conf, sysctl tuning, .wslconfig, Windows home symlink, git fsmonitor |
| **vmguest** | Installs hypervisor guest tools when running in a VM (qemu-guest-agent/spice for KVM/QEMU, open-vm-tools for VMware, VirtualBox/Hyper-V daemons); no-op on bare metal or WSL |
| **defaultshell** | Sets zsh as the default login shell |

> **Opt-in modules.** `install all` walks every module above _except_ ones flagged `_Opt-in._` — those need an explicit `dfinstall install <name>` to enable. Currently that's just **windev** (Windows cross-development). Once enabled, the opt-in persists in `.config.yaml` and future `install all` keeps it current.

## Project Layout

```
.config.yaml.example     # Example dfinstall config (copied on first run)
assets/                  # Logo SVG and generator script
bootstrap/               # WSL bootstrap wizard (PowerShell + bash)
config/                  # Config files symlinked into ~
  shell/                 #   zsh/bash dotfiles
  devtools/              #   utility scripts -> ~/.local/bin/
  git/ nvim/ tmux/       #   tool configs
  ghostty/ htop/ wsl/    #   more tool configs
  konsole/ fonts/        #   terminal + font configs
src/
  cmd/dfinstall/         # CLI entry point (Cobra)
  cmd/mcp/               # MCP server (stdio JSON-RPC)
  core/                  # Module interface + LinkSet, linking, backup/restore, output, spinner,
                       # env detection, install session, subprocess timeouts, toolkit registry
  modules/               # One file per module
tests/                   # Unit tests
docs/                    # In-depth documentation
```

## Make Targets

Run `make` (or `make help`) to list all targets.

```
make build          # compile the CLI to bin/dfinstall
make build-mcp      # compile the MCP server to bin/dfinstall-mcp
make build-all      # compile both binaries
make test           # go test ./src/... ./tests/...
make lint           # go vet
make fmt            # gofmt -s -w (in place)
make fmt-check      # gofmt -s check (non-mutating, same as CI)
make test-race      # run the test suite under the race detector
make ci             # run every check CI runs (fmt-check + lint + build-all + test + test-race)
make install        # build + dfinstall install all
make uninstall      # build + dfinstall uninstall all
make install-bin    # install both binaries onto PATH (~/.local/bin)
make install-mcp    # register the MCP server with Claude Code (user scope, works anywhere)
make uninstall-mcp  # unregister the MCP server from Claude Code
make clean          # rm -rf bin/
```

## Building from Source

**Requirements:**

- Go 1.25+ ([install](https://go.dev/doc/install))
- Git
- Make (optional, for convenience targets)

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make build          # compiles to bin/dfinstall
```

Or without Make:

```bash
go build -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(pwd)" \
  -o bin/dfinstall ./src/cmd/dfinstall
```

The `-ldflags` flag bakes the dotfiles directory path into the binary so it can find config files regardless of where it's run from. At runtime this baked path is the fallback: `dfinstall` resolves its dotfiles root as `$DOTFILES` → a machine-global canonical pointer (`~/.config/dfinstall/dotfiles-dir`) → the baked path. `install all` records the clone it runs from as canonical, so if you have more than one clone on a host, symlinks all point at a single source instead of drifting between clones (check with `dfinstall doctor` / `dfinstall diff`). Dependencies are vendored via `go.sum` and fetched automatically on first build.

See [Building from Source](docs/building.md) for more detail on dependencies, cross-compilation, and development setup.

## Continuous Integration

GitHub Actions ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs on every push and pull request to `main` and `develop`:

- **Go** -- `gofmt -s` check, `go vet`, builds both binaries (`dfinstall` + MCP), `go test`, and the suite again under `go test -race`. The Go version tracks `go.mod`.
- **ShellCheck** -- lints the bash scripts in `config/devtools/` and `bootstrap/`.

`main` is protected: both checks must pass before a pull request can be merged.

[Dependabot](.github/dependabot.yml) opens weekly PRs against `develop` for Go modules and GitHub Actions version bumps. Minor/patch updates are grouped into one PR per ecosystem; major updates land as individual PRs because they tend to need attention. The same CI checks gate the bump PRs.

## MCP Server

dfinstall includes an [MCP](https://modelcontextprotocol.io/) server for AI-assisted dotfiles management. It exposes the same operations as the CLI over a stdio JSON-RPC transport.

```bash
make build-mcp    # compile to bin/dfinstall-mcp
make install-mcp  # register with Claude Code (user scope) for use in any session
```

Two ways to wire it up:

- **In the repo** — the committed `.mcp.json` points at `./bin/dfinstall-mcp`, so Claude Code picks it up automatically when launched from the repo (after `make build-mcp`).
- **Anywhere** — `make install-mcp` registers the server user-scoped with an absolute path so it's available from any Claude session. `make uninstall-mcp` removes it. (When both are active in-repo, Claude notes the server is defined in two scopes; it's harmless for this local stdio server.)

Available tools: `dfinstall_status`, `dfinstall_install`, `dfinstall_uninstall`, `dfinstall_diff`, `dfinstall_doctor`, `dfinstall_list_modules`, `dfinstall_list_backups`, `dfinstall_restore`, `dfinstall_config`, `dfinstall_registry_validate`. `dfinstall_diff` and `dfinstall_doctor` report multi-clone symlink drift, and `install` with module `all` records the canonical clone — matching the CLI.

## Documentation

- [Architecture](docs/architecture.md) -- core systems, module interface, output pipeline, linking
- [Building from Source](docs/building.md) -- requirements, dependencies, cross-compilation
- [Module Reference](docs/modules.md) -- detailed breakdown of every module
- [Devtools Scripts](docs/devtools.md) -- utility scripts and shared helpers
- [Windows Cross-Development](docs/windev.md) -- install + use the opt-in `windev` module (toolchains, nvim, `winbuild`)
- [Contributing](docs/contributing.md) -- adding modules, conventions, testing
