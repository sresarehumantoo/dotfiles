<p align="center">
  <img src="assets/logo.svg?v=3" alt="dfinstall logo" width="820">
</p>

<p align="center">
  <a href="https://github.com/sresarehumantoo/dotfiles/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/sresarehumantoo/dotfiles/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/sresarehumantoo/dotfiles/releases/latest"><img src="https://img.shields.io/github/v/release/sresarehumantoo/dotfiles?style=flat-square&color=blue" alt="Latest release"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/sresarehumantoo/dotfiles?style=flat-square" alt="Go version">
  <img src="https://img.shields.io/badge/platform-Linux%20%C2%B7%20WSL%20%C2%B7%20macOS-73DACA?style=flat-square" alt="Platforms: Linux, WSL, macOS">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT License"></a>
</p>

<p align="center">
  <b>One <code>dfinstall</code> command to symlink your configs, install your tools, and keep every machine identical.</b>
</p>

<p align="center">
  One static Go binary for <b>Linux</b>, <b>WSL2</b>, and <b>macOS</b>. Packages via apt, dnf, pacman, or Homebrew; no runtime dependencies.
</p>

<p align="center">
  <img src="assets/demo.gif?v=2" alt="dfinstall status, doctor, and a dry-run install" width="760">
</p>

---

## Why

Most dotfiles repos are a pile of symlinks and a `bootstrap.sh` that nobody dares re-run. `dfinstall` is a real CLI instead:

- **Idempotent.** Run `install all` as often as you like: it converges, it doesn't duplicate.
- **Reversible.** Every run can snapshot what it's about to touch, and `dfinstall restore` puts it back.
- **Previewable.** `--dry-run` shows every change without writing a byte.
- **Honest.** `status`, `diff` and `doctor` tell you what drifted, not what you hoped.
- **Multi-machine safe.** One clone is recorded as canonical, so two checkouts can't fight over your symlinks.
- **Cross-platform.** The same binary, modules, and config run on Linux, WSL2, and macOS; platform-specific modules skip themselves where they don't apply.

## Contents

- [Install](#install) · [Usage](#usage) · [Upgrading](#upgrading) · [Backup and Restore](#backup-and-restore)
- [Configuration](#configuration) · [Modules](#modules) · [MCP Server](#mcp-server)
- [Development](#development) · [Documentation](#documentation)

## Install

### From a package

Every release publishes `dfinstall` and `dfinstall-mcp` for Linux and macOS on amd64 and arm64, as archives, a Homebrew formula and a `.deb`:

```bash
# Homebrew (macOS or Linux)
brew install sresarehumantoo/tap/dfinstall

# Debian, Ubuntu
sudo install -d /etc/apt/keyrings
curl -fsSL https://sresarehumantoo.github.io/dotfiles/apt/dfinstall-archive-keyring.gpg | sudo tee /etc/apt/keyrings/dfinstall.gpg >/dev/null
echo "deb [signed-by=/etc/apt/keyrings/dfinstall.gpg] https://sresarehumantoo.github.io/dotfiles/apt stable main" | sudo tee /etc/apt/sources.list.d/dfinstall.list
sudo apt-get update && sudo apt-get install dfinstall
# the keyring's fingerprint is 68923E9D2B5D025E4EB9FBCE48E2D3B9E6EE8E62:
#   gpg --show-keys /etc/apt/keyrings/dfinstall.gpg

# With Go: builds through GOPROXY, so a corporate Go mirror serves it
go install github.com/sresarehumantoo/dotfiles/v2/cmd/dfinstall@latest
go install github.com/sresarehumantoo/dotfiles/v2/cmd/dfinstall-mcp@latest

# Anywhere else: unpack dfinstall_<version>_<os>_<arch>.tar.gz from the release onto your PATH
```

A packaged binary has no configs to link until it has a checkout. `dfinstall init` clones one to `~/.local/share/dfinstall/dotfiles` and records it as the machine's checkout; every command that links or inspects managed files tells you to run it when none is found.

```bash
dfinstall init                          # clone this repo's default branch
dfinstall init --repo <url> --ref <ref> # another remote, branch or tag
dfinstall init --dir ~/dotfiles         # another location, or adopt a checkout already there
dfinstall install all
```

`init` is a no-op once a checkout is recorded. It defaults to the default branch rather than a tag because `update` fast-forwards a branch, and a tag checkout has none to follow.

`dfinstall --version` names the channel the binary came from (`release`, `brew`, `deb`, `go`, or `source` for a `make` build), which decides how `update` replaces it.

### On a fresh machine

The bootstrap scripts start from nothing and end with `dfinstall install all`. [The bootstrap guide](docs/bootstrap.md) covers their flags, mirrors, company-managed Macs and TLS-inspecting proxies.

**WSL**, from PowerShell: creates a Debian distro and user, builds Neovim, installs Ghostty and dfinstall (from its apt repository), then the dotfiles.

```powershell
git clone https://github.com/sresarehumantoo/dotfiles
cd dotfiles
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1
```

`-ExecutionPolicy Bypass` is needed because Windows refuses to run scripts by default; it applies to that one PowerShell process and changes no setting on the machine.

**macOS**: installs the Xcode Command Line Tools, Homebrew, Ghostty and dfinstall (from the Homebrew tap), clones to `~/dotfiles`, then installs the dotfiles. It asks before every step that changes the machine, and on a company-managed Mac nothing after it needs admin rights.

```bash
curl -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
```

### From a clone

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make install
```

`make install` compiles the CLI and runs `dfinstall install all`, walking every module in dependency order: system packages first, then the shell, dev tools, the editor and the desktop, with platform tuning last. Building needs Go 1.26+; see [Development](#development).

> [!TIP]
> Not sure what it'll do to your machine? Run `dfinstall install all --dry-run` first (`./bin/dfinstall` from a clone). It prints every change and writes nothing.

## Usage

**Installing and updating**

| Command | What it does |
|---------|--------------|
| `dfinstall init` | Clone the dotfiles checkout, for a binary installed without the repo |
| `dfinstall install all` | Install every module (except opt-in ones) |
| `dfinstall install shell git` | Install the named modules, run in dependency order whatever order you name them |
| `dfinstall update all` | Pull the clone, then install and update what dfinstall installed earlier (`install all --upgrade`) |
| `dfinstall update all --no-upgrade` | Re-apply all modules without pulling or updating (same as `install all`) |
| `dfinstall uninstall shell git` | Remove what the named modules installed |
| `dfinstall uninstall all` | Remove all managed symlinks |

**Inspecting**

| Command | What it does |
|---------|--------------|
| `dfinstall status` | Every module grouped as installed, partial, not installed, skipped, or not available on this platform |
| `dfinstall diff` | Drift between config and filesystem, including managed keys in `~/.claude/settings.json` |
| `dfinstall diff --brief` | Only what changed since install, one line each; silent when clean |
| `dfinstall doctor` | Environment health checks, plus whether the clone itself is clean, pushed and level with its upstream |
| `dfinstall --version` | Version stamped in at build time, and the install channel |

**Backups**

| Command | What it does |
|---------|--------------|
| `dfinstall restore --list` | List available snapshots |
| `dfinstall restore <timestamp>` | Restore a specific snapshot |
| `dfinstall restore` | Restore the latest snapshot |

**Other**

| Command | What it does |
|---------|--------------|
| `dfinstall root` | Apply a curated subset of configs to `/root` via sudo |
| `dfinstall registry validate <src>` | Validate a toolkit registry file (a CI helper); read-only, never touches the cache |

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
| `--upgrade` | `install` | Pull the clone and update tools and plugins first ([details](#upgrading)); always on for `update` |
| `--no-upgrade` | `update` | Skip the pull and tool updates, making `update` a plain `install` |

`install`, `update` and `uninstall` take one or more module names, or `all` on its own.

```bash
dfinstall install omz --extended     # pick extended OMZ plugins
dfinstall install all --toolkit      # pick toolkit tools
dfinstall install windev             # opt-in Windows cross-dev module
dfinstall install all --dry-run      # preview everything
```

Output falls back to ASCII symbols and drops the spinner on terminals that cannot show them: `TERM=dumb`, the Linux text console (`TERM=linux`), and non-UTF-8 locales. Set `DF_ASCII=1` to force ASCII (when the terminal font lacks the symbols, for example) or `DF_ASCII=0` to force Unicode. Color follows `NO_COLOR` and is off when output is not a terminal.

## Upgrading

`install` adds what is missing and leaves what is already there. `update`, or `install --upgrade`, also brings everything dfinstall owns up to date:

1. **The clone and dfinstall itself.** The clone is fetched and fast-forwarded to its upstream branch. What happens to the binary depends on how it was installed:

   | Installed by | `update` |
   |---|---|
   | `make` in a clone | rebuilds it when the pull changed dfinstall's source, then re-runs it |
   | a release archive | downloads the newer release for this platform, checks its sha256 against the release listing, swaps `dfinstall` (and `dfinstall-mcp` if it is installed beside it) in place, then re-runs it |
   | Homebrew, a `.deb`, `go install` | leaves the binary alone and prints the command that updates it |

   Release downloads go through a Nexus GitHub mirror when npm points at one, falling back to GitHub.
2. **Release binaries in `~/.local`** (nvim, tree-sitter, delta, terraform, opencode) are replaced when a newer release exists, and offline doc sets are refetched when their source changed.
3. **Cloned plugins** (oh-my-zsh, powerlevel10k, zsh-autosuggestions, TPM and the tmux plugins) are fast-forwarded.
4. Then the normal install runs, asking for sudo only if something is missing.

Packages from apt or Homebrew are left to `sudo apt upgrade` or `brew upgrade`, and fonts stay on their pinned release.

```bash
dfinstall update all
dfinstall update all --dry-run   # what would change, without fetching
```

### Local edits survive the pull

Managed configs are symlinks into the clone, so editing `~/.config/ghostty/config` on one machine edits the clone. Before pulling, `--upgrade` sorts those edits out:

- **Moved to an override file.** When the edit only adds lines or sets keys, it offers to move it into a file that loads after the managed one but lives outside the clone. That puts the clone back to upstream's version, and the setting keeps applying.
- **Kept in the clone.** Other edits stay as uncommitted changes and are carried over the pull, provided they merge cleanly with what upstream changed. If one would conflict, or the clone has unpushed commits, it skips the pull, says why, and the rest of the install still runs.
- **A link replaced by a file.** Some tools save by writing a new file over the symlink (`sed -i` does). It shows the difference and offers to copy that file into the clone, rather than letting the install move it to `.bak`. Without a terminal it copies it.

| Managed file | Override file |
|--------------|---------------|
| `~/.config/ghostty/config` | `~/.config/ghostty/ghostty.user` |
| `~/.zshrc`, `~/.zsh.d/*.zsh` | `~/.zshrc.local` |
| `~/.aliases` | `~/.aliases.local` |
| `~/.gitconfig` | `~/.gitconfig.user` (included last, so its values win; a removed key, or a rewritten multi-value key such as `credential.helper`, cannot move) |

Anything the override file already holds is not written again, so a repeated move, or a setting you had already put there, does not duplicate. You can write to these files directly too.

For git, `~/.gitconfig.local` stays the place for identity and site settings, since the tracked `gitconfig` sets no identity. `~/.gitconfig.user` is included after it, so a name there wins.

`~/.zshenv`, extra files in `~/.zsh.d/`, and anything else dfinstall does not link are never touched.

### Drift warnings

A new zsh checks, in the background, whether something changed a managed file behind dfinstall's back:

- a link replaced by a file, or pointing at another clone;
- managed settings edited, for example Claude Code's `/config` rewriting `tui` in `~/.claude/settings.json`;
- a linked file edited through its link, such as a script running `git config --global`. The link stays intact and the change lands in the clone as an uncommitted edit.

The result shows once, at a later prompt:

```
⚠ dfinstall: managed files changed outside dfinstall
  ~/.claude/settings.json: managed keys differ: tui
  what to do about each: dfinstall diff
```

It adds nothing to startup, runs at most every ten minutes, prints each finding once rather than in every pane, and never fixes anything itself. Links that were never installed are not reported. `DF_NO_DRIFT_CHECK=1` turns it off.

<details>
<summary><b>From 1.x to 2.0</b></summary>

### From 1.x to 2.0

No re-clone: 2.0 finds the existing clone through the pointer 1.x already recorded (`~/.config/dfinstall/dotfiles-dir`), keeps linking from it, and reads the clone's old `.config.yaml` until it first saves a setting to `~/.config/dfinstall/config.yaml`. Two things need doing by hand, in this order:

1. **Bring the clone up to date first**: `git -C <clone> pull`. A 2.0 binary run against a 1.x clone skips the files the clone does not have yet (`not linked …: … is not in the dotfiles checkout`) rather than leaving dangling links, but they stay unlinked until it is pulled.
2. **Install 2.0, and retire the 1.x binary.** Staying on a source build, `make -C <clone> install-bin` replaces it. Moving to a package (`brew install sresarehumantoo/tap/dfinstall`, or apt), delete `~/.local/bin/dfinstall` and `~/.local/bin/dfinstall-mcp` as well, since `~/.local/bin` comes first on `PATH` and the old build would keep running. If Claude Code's MCP server was registered by path (`claude mcp list`), point it at the new `dfinstall-mcp`.

Then run `dfinstall install all` and `dfinstall doctor`. Doctor's `dfinstall on PATH`, `checkout files` and `settings` rows report, respectively, an older binary that still runs first, a clone missing files this dfinstall links, and a pre-2.0 `.config.yaml` that can be deleted once nothing older uses the clone.

</details>

## Backup and Restore

The **first** `install` run (when no `config.yaml` exists yet) automatically snapshots everything it's about to touch. After a successful run it writes `config.yaml` with `skip_backup: true`, so later runs don't snapshot by default.

> [!IMPORTANT]
> A **failed** first run stays a first run. If any module errors, the config isn't written, so the automatic backup is still armed next time. `--dry-run` never consumes the first run either.

To snapshot more often:

- **`--backup`**: always snapshot, whatever the config says.
- **`skip_backup: false`** in `config.yaml`: snapshot on every install.
- **`backup_dir`** in `config.yaml`: a custom location (default `~/.local/share/dfinstall/backups/`).

```bash
dfinstall install all --backup    # force a snapshot
dfinstall restore --list          # see what's available
dfinstall restore <timestamp>     # revert to that snapshot
```

Each entry records the original state (missing, symlink, or regular file), so restore undoes precisely what dfinstall changed.

> [!NOTE]
> System paths under `/etc/` are deliberately excluded from snapshots, so changes a module makes there are not restorable.

## Configuration

Config lives at `~/.config/dfinstall/config.yaml` (under `$XDG_CONFIG_HOME` when set) and is generated on first run. [`.config.yaml.example`](.config.yaml.example) documents every key; it's a reference, not a template that gets copied.

| Key | Default | Purpose |
|-----|---------|---------|
| `skip_backup` | `true` after first run | Skip automatic backups on install |
| `backup_dir` | `~/.local/share/dfinstall/backups` | Where snapshots are written |
| `skip_modules` | none | Modules to skip during `install all` |
| `extended_plugins` | none | Extra Oh My Zsh plugins (set via `--extended`) |
| `toolkit_tools` | none | Toolkit tools to keep installed (set via `--toolkit`) |
| `toolkit_registry_url` | upstream registry | Override the toolkit registry location |
| `preserved_files` | none | Shell files to keep sourcing after `~/.zshrc` is replaced |
| `dismissed_files` | none | Shell files declined, so the prompt doesn't return |
| `windev_enabled` | `false` | Opt-in for the `windev` module |
| `opencode_enabled` | `false` | Opt-in for the `opencode` module |
| `doc_sets` | python, bash, git, go, docker, kubernetes, node | Offline doc sets the `docs` module fetches: DevDocs slugs, or `gitlab` |

Releases before 2.0 kept the config in the clone as `.config.yaml`. That file is read until the new one exists and is then left alone, so delete it once nothing older uses the clone.

## Modules

Modules run in this order, dependencies first. Each is documented in the [module reference](docs/modules.md).

| # | Module | What it does |
|---|--------|--------------|
| 1 | **locale** | Reports the UTF-8 locale shells get (`en_US.UTF-8`, else built-in `C.UTF-8`); no root |
| 2 | **packages** | Core system packages via apt/dnf/pacman/brew (only what's missing) |
| 3 | **extras** | CLI utilities (fzf, ripgrep, bat, jq, fd), Python tooling, Docker, Terraform |
| 4 | **toolkit** | External registry of security/CTF/dev tools (`--toolkit`) |
| 5 | **delta** | [delta](https://github.com/dandavison/delta) git diff viewer |
| 6 | **fonts** | IosevkaTerm Nerd Font (downloaded, pinned) + MesloLGS NF offline floor (bundled) |
| 7 | **omz** | Oh My Zsh + zsh-autosuggestions + powerlevel10k (`--extended` for more) |
| 8 | **shell** | zshrc, bashrc, aliases, p10k config, modular `zsh.d` files |
| 9 | **devtools** | Utility scripts into `~/.local/bin/` (sysinfo, docker-cleanup, …) |
| 10 | **git** | gitconfig: delta pager, histogram diff, aliases |
| 11 | **nvim** | Neovim config with Lazy.nvim + headless sync |
| 12 | **windev** | _Opt-in._ Windows cross-dev toolchains + `winbuild` ([guide](docs/windev.md)) |
| 13 | **tmux** | Tmux config (Alt+A prefix, vi mode, custom theme) |
| 14 | **konsole** | Konsole profile and color scheme |
| 15 | **ghostty** | Ghostty terminal config |
| 16 | **sway** | Sway + waybar + swaync desktop ([guide](docs/sway.md)). No-ops without `sway` |
| 17 | **aerospace** | macOS tiling window manager (AeroSpace). No-ops off macOS |
| 18 | **sketchybar** | macOS status bar (SketchyBar). No-ops off macOS |
| 19 | **mackeys** | macOS keyboard like a Windows laptop: modifier swap, Ctrl shortcuts in every app, key repeat, no Accessibility needed. No-ops off macOS |
| 20 | **alttab** | macOS Option-Tab app switcher that looks like Command-Tab, and Win+L to lock, no Accessibility needed. Only `install alttab` sets it up |
| 21 | **htop** | htop config |
| 22 | **newsboat** | Feed reader with a curated security feed set; images inline via chafa, links in w3m |
| 23 | **docs** | Offline usage docs from DevDocs and GitLab (`doc_sets`), read with the `docs` script, which also reads these docs live |
| 24 | **claude** | Claude Code global `CLAUDE.md` + managed keys merged into `settings.json` |
| 25 | **opencode** | _Opt-in._ opencode for Anthropic API / Bedrock users, locked to those providers with Claude Code-style permissions ([reference](docs/modules.md#opencode)) |
| 26 | **wsl** | wsl.conf, sysctl tuning, .wslconfig, Windows home symlink, git fsmonitor |
| 27 | **vmguest** | Hypervisor guest tools when running in a VM; no-op on bare metal or WSL |
| 28 | **defaultshell** | Sets zsh as the default login shell |

> [!NOTE]
> **Opt-in modules** (**windev** and **opencode**) are skipped by `install all` until you enable them with `dfinstall install <name>`. The choice then persists in `config.yaml`, and later `install all` runs keep them current.

mackeys and alttab rest on undocumented macOS behavior and have been tested only on macOS 26.7.1; the module reference says what to check on another version.

## MCP Server

dfinstall ships an [MCP](https://modelcontextprotocol.io/) server so an AI assistant can drive the same operations as the CLI, over stdio JSON-RPC.

```bash
make build-mcp    # compile to bin/dfinstall-mcp
make install-mcp  # register with Claude Code (user scope, any session)
```

There are two ways to wire it up:

- **In the repo:** the committed `.mcp.json` points at `./bin/dfinstall-mcp`, which Claude Code picks up when launched from the repo (after `make build-mcp`).
- **Anywhere:** `make install-mcp` registers it user-scoped with an absolute path. `make uninstall-mcp` removes it.

<details>
<summary><b>Available tools (10)</b></summary>

`dfinstall_status` · `dfinstall_install` · `dfinstall_uninstall` · `dfinstall_diff` · `dfinstall_doctor` · `dfinstall_list_modules` · `dfinstall_list_backups` · `dfinstall_restore` · `dfinstall_config` · `dfinstall_registry_validate`

`diff` and `doctor` report multi-clone symlink drift, and `install` with module `all` records the canonical clone, matching the CLI exactly because both go through the same code. `install` takes `upgrade: true` to do what `update` does. It returns the pull's report, and when the pull changes dfinstall's source it rebuilds the binaries and asks for a reconnect, since the running server keeps its old code. Moving local edits into override files needs a terminal, so here it only reports them. `restore` lists snapshots by default and restores only with `action: "restore"` and an explicit timestamp.

</details>

## Development

Building needs Go 1.26+ ([install](https://go.dev/doc/install)), Git, and optionally Make.

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make build    # bin/dfinstall
make ci       # everything CI's Go job runs
make          # list every target
```

`make ci` does not run ShellCheck, which CI also does over `config/devtools/`, `bootstrap/` and `scripts/`. `main` is protected, and pull requests go to `develop`.

The [build guide](docs/building.md) covers building without Make, version stamping, releases, testing (including the Docker e2e scenarios and the corporate-network lab), the project layout, and CI. The [contributing guide](docs/contributing.md) covers adding a module or script and the conventions they follow.

## Documentation

| Guide | Covers |
|-------|--------|
| [Bootstrapping](docs/bootstrap.md) | The WSL and macOS bootstrap scripts: flags, mirrors, managed Macs, TLS-inspecting proxies |
| [Architecture](docs/architecture.md) | Core systems, module interface, output pipeline, linking |
| [Module Reference](docs/modules.md) | Detailed breakdown of every module |
| [Building from Source](docs/building.md) | Requirements, releases, testing, project layout, CI |
| [Devtools Scripts](docs/devtools.md) | Utility scripts and shared helpers |
| [Keybindings](docs/keybindings.md) | Custom bindings across tmux, nvim, zsh, and the menus |
| [Neovim Tutorial](docs/nvim-tutorial/README.md) | Hands-on chapters to work through in nvim: editing, search, LSP, debugging, notes |
| [Sway Desktop](docs/sway.md) | The `sway` module: waybar, swaync, outputs, and the Alt reservation |
| [Windows Cross-Development](docs/windev.md) | The opt-in `windev` module: toolchains, nvim, `winbuild` |
| [Contributing](docs/contributing.md) | Adding modules, conventions, testing |

With the devtools installed, `docs dfinstall <heading>` opens any heading of this README or `docs/` in the terminal (`docs dfinstall Configuration`, say), read live from the checkout.

## License

[MIT](LICENSE) © Owen Pierce
