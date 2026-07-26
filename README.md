<p align="center">
  <img src="assets/logo.svg?v=2" alt="dfinstall logo" width="800">
</p>

<p align="center">
  <a href="https://github.com/sresarehumantoo/dotfiles/actions/workflows/ci.yml"><img src="https://github.com/sresarehumantoo/dotfiles/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/sresarehumantoo/dotfiles/releases/latest"><img src="https://img.shields.io/github/v/release/sresarehumantoo/dotfiles?color=blue" alt="Latest release"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/sresarehumantoo/dotfiles" alt="Go version">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT License"></a>
</p>

<p align="center">
  <b>One <code>dfinstall</code> command to symlink your configs, install your tools, and keep every machine identical.</b>
</p>

<p align="center">
  Built in Go for WSL2 (Debian/Ubuntu) — works on native Linux with apt, dnf, pacman, or brew.
</p>

---

## Why

Most dotfiles repos are a pile of symlinks and a `bootstrap.sh` that nobody dares re-run. `dfinstall` is a real CLI instead:

- **Idempotent.** Run `install all` as often as you like — it converges, it doesn't duplicate.
- **Reversible.** Every run can snapshot what it's about to touch; `dfinstall restore` puts it back.
- **Previewable.** `--dry-run` shows every change without writing a byte.
- **Honest.** `status`, `diff` and `doctor` tell you what drifted, not what you hoped.
- **Multi-machine safe.** One clone is recorded as canonical, so two checkouts can't fight over your symlinks.

## Contents

- [Quick Start](#quick-start) · [Usage](#usage) · [Backup & Restore](#backup--restore) · [Configuration](#configuration)
- [Modules](#modules) · [MCP Server](#mcp-server) · [Building from Source](#building-from-source)
- [Project Layout](#project-layout) · [Make Targets](#make-targets) · [CI](#continuous-integration) · [Documentation](#documentation)

## Quick Start

### Fresh WSL setup (from PowerShell)

Bootstraps a WSL Debian distro from nothing — creates a user, installs packages, builds Neovim, installs Ghostty, and runs the full dotfiles install:

```powershell
git clone https://github.com/sresarehumantoo/dotfiles
.\dotfiles\bootstrap\wsl-bootstrap.ps1
```

The wizard prompts for distro and username. To skip the prompts or trim the scope:

```powershell
.\bootstrap\wsl-bootstrap.ps1 -Distro Debian -Username owen   # skip prompts
.\bootstrap\wsl-bootstrap.ps1 -SkipNeovim                     # skip neovim build
.\bootstrap\wsl-bootstrap.ps1 -SkipGhostty                    # skip ghostty install
.\bootstrap\wsl-bootstrap.ps1 -SkipDotfiles                   # core tools only
.\bootstrap\wsl-bootstrap.ps1 -SkipDotfiles -SkipNeovim -SkipGhostty  # bare minimum
```

### Existing Linux system

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make install
```

`make install` compiles the CLI and runs `dfinstall install all`, walking every module in dependency order: system packages → shell → editor → dev tools → WSL tuning.

> [!TIP]
> Not sure what it'll do to your machine? Run `./bin/dfinstall install all --dry-run` first. It prints every change and writes nothing.

## Usage

**Installing and updating**

| Command | What it does |
|---------|--------------|
| `dfinstall install all` | Install every module (except opt-in ones) |
| `dfinstall install shell` | Install a single module |
| `dfinstall update all` | Re-apply all modules (alias for `install`) |
| `dfinstall uninstall shell` | Remove one module's symlinks |
| `dfinstall uninstall all` | Remove all managed symlinks |

**Inspecting**

| Command | What it does |
|---------|--------------|
| `dfinstall status` | Link status for every module |
| `dfinstall diff` | Drift between config and filesystem |
| `dfinstall doctor` | Environment health checks |
| `dfinstall --version` | Version stamped in at build time |

**Backups**

| Command | What it does |
|---------|--------------|
| `dfinstall restore` | Restore the latest snapshot |
| `dfinstall restore <timestamp>` | Restore a specific snapshot |
| `dfinstall restore --list` | List available snapshots |

**Other**

| Command | What it does |
|---------|--------------|
| `dfinstall root` | Apply a curated subset of configs to `/root` via sudo |
| `dfinstall registry validate <src>` | Validate a toolkit registry file (CI helper) — read-only, never touches the cache |

### Flags

| Flag | Applies to | Effect |
|------|------------|--------|
| `-v`, `--verbose` | all | Full log output instead of the spinner |
| `--debug` | all | Verbose plus internal detail |
| `--dry-run` | all | Preview every change, write nothing (implies `-v`) |
| `--backup` | `install`, `update` | Snapshot targets first, regardless of config |
| `--extended` | `install`, `update` | Interactive menu for extended Oh My Zsh plugins |
| `--toolkit` | `install`, `update` | Interactive menu for security/CTF/dev toolkit tools |
| `--registry <path\|url>` | `install`, `update` | Override the toolkit registry for this run |

```bash
dfinstall install omz --extended     # pick extended OMZ plugins
dfinstall install all --toolkit      # pick toolkit tools
dfinstall install windev             # opt-in Windows cross-dev module
dfinstall install all --dry-run      # preview everything
```

## Backup & Restore

On the **first** `install` run — when no `.config.yaml` exists yet — dfinstall automatically snapshots everything it's about to touch. After a successful run it writes `.config.yaml` with `skip_backup: true`, so later runs don't snapshot by default.

> [!IMPORTANT]
> A **failed** first run stays a first run. If any module errors, the config isn't written, so the automatic backup is still armed next time. `--dry-run` never consumes the first run either.

Override the default behaviour with any of:

- **`--backup`** — always snapshot, whatever the config says
- **`skip_backup: false`** in `.config.yaml` — snapshot on every install
- **`backup_dir`** in `.config.yaml` — custom location (default `~/.local/share/dfinstall/backups/`)

```bash
dfinstall install all --backup    # force a snapshot
dfinstall restore --list          # see what's available
dfinstall restore                 # revert to the latest
```

Each entry records the original state — missing, symlink, or regular file — so restore undoes precisely what dfinstall changed.

> [!NOTE]
> System paths under `/etc/` are deliberately excluded from snapshots, so changes a module makes there are not restorable.

## Configuration

Config lives at `<dotfiles>/.config.yaml`, generated on first run. [`.config.yaml.example`](.config.yaml.example) documents every key — it's a reference, not a template that gets copied.

| Key | Default | Purpose |
|-----|---------|---------|
| `skip_backup` | `true` after first run | Skip automatic backups on install |
| `backup_dir` | `~/.local/share/dfinstall/backups` | Where snapshots are written |
| `skip_modules` | — | Modules to skip during `install all` |
| `extended_plugins` | — | Extra Oh My Zsh plugins (set via `--extended`) |
| `toolkit_tools` | — | Toolkit tools to keep installed (set via `--toolkit`) |
| `toolkit_registry_url` | upstream registry | Override the toolkit registry location |
| `preserved_files` | — | Shell files to keep sourcing after `~/.zshrc` is replaced |
| `dismissed_files` | — | Shell files declined, so the prompt doesn't return |
| `windev_enabled` | `false` | Opt-in for the `windev` module |

## Modules

Modules run in this order — dependencies first:

| # | Module | What it does |
|---|--------|--------------|
| 1 | **locale** | Ensures `en_US.UTF-8` is generated and set |
| 2 | **packages** | Core system packages via apt/dnf/pacman/brew (only what's missing) |
| 3 | **extras** | CLI utilities (fzf, ripgrep, bat, jq, fd), Python tooling, Docker, Terraform |
| 4 | **toolkit** | External registry of security/CTF/dev tools (`--toolkit`) |
| 5 | **delta** | [delta](https://github.com/dandavison/delta) git diff viewer |
| 6 | **fonts** | Hack Nerd Font and MesloLGS NF (bundled or downloaded) |
| 7 | **omz** | Oh My Zsh + zsh-autosuggestions + powerlevel10k (`--extended` for more) |
| 8 | **shell** | zshrc, bashrc, aliases, p10k config, modular `zsh.d` files |
| 9 | **devtools** | Utility scripts into `~/.local/bin/` (sysinfo, docker-cleanup, …) |
| 10 | **git** | gitconfig — delta pager, histogram diff, aliases |
| 11 | **nvim** | Neovim config with Lazy.nvim + headless sync |
| 12 | **windev** | _Opt-in._ Windows cross-dev toolchains + `winbuild` — see [docs](docs/windev.md) |
| 13 | **tmux** | Tmux config (Alt+A prefix, vi mode, custom theme) |
| 14 | **konsole** | Konsole profile and colour scheme |
| 15 | **ghostty** | Ghostty terminal config |
| 16 | **htop** | htop config |
| 17 | **wsl** | wsl.conf, sysctl tuning, .wslconfig, Windows home symlink, git fsmonitor |
| 18 | **vmguest** | Hypervisor guest tools when running in a VM; no-op on bare metal or WSL |
| 19 | **defaultshell** | Sets zsh as the default login shell |

> [!NOTE]
> **Opt-in modules** are skipped by `install all` until you enable them explicitly with `dfinstall install <name>`. Currently that's just **windev**. Once enabled the choice persists in `.config.yaml`, and future `install all` runs keep it current.

## MCP Server

dfinstall ships an [MCP](https://modelcontextprotocol.io/) server so an AI assistant can drive the same operations as the CLI, over stdio JSON-RPC.

```bash
make build-mcp    # compile to bin/dfinstall-mcp
make install-mcp  # register with Claude Code (user scope, any session)
```

Two ways to wire it up:

- **In the repo** — the committed `.mcp.json` points at `./bin/dfinstall-mcp`, picked up automatically when Claude Code launches from the repo (after `make build-mcp`).
- **Anywhere** — `make install-mcp` registers it user-scoped with an absolute path. `make uninstall-mcp` removes it.

<details>
<summary><b>Available tools (10)</b></summary>

`dfinstall_status` · `dfinstall_install` · `dfinstall_uninstall` · `dfinstall_diff` · `dfinstall_doctor` · `dfinstall_list_modules` · `dfinstall_list_backups` · `dfinstall_restore` · `dfinstall_config` · `dfinstall_registry_validate`

`diff` and `doctor` report multi-clone symlink drift, and `install` with module `all` records the canonical clone — matching the CLI exactly, because both go through the same code.

</details>

## Building from Source

**Requirements:** Go 1.25.5+ ([install](https://go.dev/doc/install)), Git, and optionally Make.

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make build          # -> bin/dfinstall
```

Without Make — note both `-X` flags:

```bash
go build -ldflags "\
  -X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(pwd) \
  -X github.com/sresarehumantoo/dotfiles/src/core.Version=$(git describe --tags --always --dirty)" \
  -o bin/dfinstall ./src/cmd/dfinstall
```

`DefaultDotfilesDir` bakes in where the configs live; `Version` is what `dfinstall --version` reports (omit it and you get `dev`).

At runtime the dotfiles root resolves in order: **`$DOTFILES`** → the **machine-global canonical pointer** (`~/.config/dfinstall/dotfiles-dir`) → the **baked-in path**. `install all` records the clone it ran from as canonical, so multiple checkouts on one host all link from a single source instead of drifting — check with `dfinstall doctor` or `dfinstall diff`. (A `--dry-run` reports the change it would make without recording it.)

See [Building from Source](docs/building.md) for dependencies, cross-compilation, and development setup.

## Project Layout

<details>
<summary><b>Directory tree</b></summary>

```
.config.yaml.example     # Documented example of every config key
.github/                 # CI workflow + Dependabot config
.mcp.json                # In-repo MCP server registration
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
  core/                  # Module interface + LinkSet, linking, backup/restore,
                         #   output, spinner, env detection, install session,
                         #   canonical dir + drift, subprocess timeouts,
                         #   toolkit registry, version stamp
  modules/               # One file per module, plus shared machinery
tests/                   # Black-box tests (package tests)
                         #   in-package tests live beside the code in src/
docs/                    # In-depth documentation
```

</details>

## Make Targets

Run `make` (or `make help`) to list everything.

<details>
<summary><b>All targets</b></summary>

```
make build          # compile the CLI to bin/dfinstall
make build-mcp      # compile the MCP server to bin/dfinstall-mcp
make build-all      # compile both binaries
make test           # go test ./src/... ./tests/...
make test-race      # run the suite under the race detector
make lint           # go vet
make fmt            # gofmt -s -w (in place)
make fmt-check      # gofmt -s check (non-mutating, same as CI)
make ci             # everything CI runs (fmt-check + lint + build-all + test + test-race)
make install        # build + dfinstall install all
make uninstall      # build + dfinstall uninstall all
make install-bin    # install both binaries onto PATH (~/.local/bin)
make install-mcp    # register the MCP server with Claude Code (user scope)
make uninstall-mcp  # unregister the MCP server
make clean          # rm -rf bin/
```

</details>

## Continuous Integration

[GitHub Actions](.github/workflows/ci.yml) runs on every push and pull request to `main` and `develop`, and on manual dispatch:

- **Go** — `gofmt -s` check, `go vet`, builds both binaries, `go test`, then the suite again under `-race`. The Go version tracks `go.mod`.
- **ShellCheck** — lints `config/devtools/` and `bootstrap/`.

`main` is protected: both checks must pass before a PR can merge.

[Dependabot](.github/dependabot.yml) opens weekly PRs against `develop` for Go modules and Actions bumps — minor/patch grouped into one PR per ecosystem, majors individually since they tend to need attention. The same checks gate them.

## Documentation

| Guide | Covers |
|-------|--------|
| [Architecture](docs/architecture.md) | Core systems, module interface, output pipeline, linking |
| [Module Reference](docs/modules.md) | Detailed breakdown of every module |
| [Building from Source](docs/building.md) | Requirements, dependencies, cross-compilation |
| [Devtools Scripts](docs/devtools.md) | Utility scripts and shared helpers |
| [Keybindings](docs/keybindings.md) | Custom bindings across tmux, nvim, zsh, and the menus |
| [Windows Cross-Development](docs/windev.md) | The opt-in `windev` module — toolchains, nvim, `winbuild` |
| [Contributing](docs/contributing.md) | Adding modules, conventions, testing |

## License

[MIT](LICENSE) © Owen Pierce
