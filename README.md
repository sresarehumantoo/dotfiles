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

- **Idempotent.** Run `install all` as often as you like — it converges, it doesn't duplicate.
- **Reversible.** Every run can snapshot what it's about to touch; `dfinstall restore` puts it back.
- **Previewable.** `--dry-run` shows every change without writing a byte.
- **Honest.** `status`, `diff` and `doctor` tell you what drifted, not what you hoped.
- **Multi-machine safe.** One clone is recorded as canonical, so two checkouts can't fight over your symlinks.
- **Cross-platform.** The same binary, modules, and config run on Linux, WSL2, and macOS; platform-specific modules skip themselves where they don't apply.

## Contents

- [Quick Start](#quick-start) · [Usage](#usage) · [Backup & Restore](#backup--restore) · [Configuration](#configuration)
- [Modules](#modules) · [MCP Server](#mcp-server) · [Building from Source](#building-from-source)
- [Project Layout](#project-layout) · [Make Targets](#make-targets) · [CI](#continuous-integration) · [Documentation](#documentation)

## Quick Start

### Fresh WSL setup (from PowerShell)

Bootstraps a WSL Debian distro from nothing — creates a user, installs packages, builds Neovim, installs Ghostty, installs dfinstall from its apt repository, and runs the full dotfiles install:

```powershell
git clone https://github.com/sresarehumantoo/dotfiles
cd dotfiles
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1
```

Windows refuses to run scripts by default (execution policy `Restricted`), so a plain `.\bootstrap\wsl-bootstrap.ps1` fails before doing anything. `-ExecutionPolicy Bypass` applies to that one PowerShell process and changes no setting on the machine.

The wizard prompts for distro and username. To skip the prompts or trim the scope:

```powershell
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1 -Distro Debian -Username owen   # skip prompts
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1 -SkipNeovim                     # skip neovim build
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1 -SkipGhostty                    # skip ghostty install
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1 -SkipDotfiles                   # core tools only
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1 -SkipDotfiles -SkipNeovim -SkipGhostty  # bare minimum
powershell -ExecutionPolicy Bypass -File .\bootstrap\wsl-bootstrap.ps1 -Branch develop                 # another branch: builds dfinstall from source
```

The released dfinstall is only used on `main`, which releases are cut from: another branch's configs may need code no release has yet, so `-Branch` anything else (or `-FromSource`) builds it from the clone with Go, as does an apt repository that cannot be reached. `DFINSTALL_APT_URL` in the distro points the install at a mirror of the repository instead.

### Fresh Mac

Installs the Xcode Command Line Tools, Homebrew and Ghostty, then dfinstall from the Homebrew tap, clones to `~/dotfiles` and runs `dfinstall install all`:

```bash
curl -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
```

From an existing clone, `./bootstrap/macos-setup.sh` uses that clone instead. `--skip-ghostty` and `--skip-dotfiles` trim the scope. On a branch other than `main`, with `--from-source`, or when the tap cannot be reached, it installs Go and builds dfinstall from the clone (`make install`) instead. `DFINSTALL_TAP_URL` clones the tap from a mirror, and Homebrew's own `HOMEBREW_ARTIFACT_DOMAIN` mirrors the download. Linux-only modules (konsole, sway, wsl, vmguest, windev) skip themselves on macOS. Bootstrap shows its plan and asks before every step that changes the machine; declining stops there with nothing further changed, and re-running picks up where it left off. `--yes` answers every prompt.

Which Homebrew to use is discovered, not assumed. Bootstrap uses the first one this account can write to: one recorded by an earlier run, the `brew` on `PATH`, then `/opt/homebrew`, `/usr/local` and `~/homebrew`. It records the choice in `~/.config/dfinstall/brew-prefix`, which `~/.profile` and dfinstall read first, so a read-only Homebrew (an IT-owned `/opt/homebrew`, say) never shadows yours. With none usable it offers Homebrew's official installer at its default prefix, and if that cannot finish it stops with the options rather than picking a path for you. `--brew-prefix <path>` installs a separate Homebrew this account owns: at most 13 characters (as long as `/opt/homebrew`) and every prebuilt bottle works there, through the official installer; longer, like `~/homebrew`, it comes from Homebrew's release tarball and the bottles that are not relocatable (coreutils, gawk, make, bash, node, neovim, wget, openssl and a few more) compile from source on first install.

On a company-managed Mac nothing past bootstrap needs admin. Bootstrap checks up front and says what it found: whether the account is an admin, which Homebrew is usable, and Santa in Lockdown, which blocks an unsigned `dfinstall`. Apps go to `~/Applications` when `/Applications` is not writable, casks installed by hand included (`HOMEBREW_CASK_OPTS`), `install all` creates no login items (only `install sketchybar`, `install alttab`, `install mackeys` and AeroSpace's own setting do), Rust comes from Homebrew's rustup rather than `curl | sh`, and an SSH agent that is already running is kept. Bootstrap ends by saving `dfinstall doctor` to `~/Library/Logs/`, which reports admin, Homebrew ownership, the app folder, MDM enrollment, Santa, TLS trust and Gatekeeper; run `dfinstall doctor` again any time to see the same.

Behind a TLS-inspecting proxy, bootstrap detects the interception by who issued the certificates a few hosts present (none of Apple's public roots, so something re-signed them) and points every tool at a bundle of Apple's public roots plus the System keychain's certificates, through `~/.config/dfinstall/ca.sh`, which `~/.profile` sources. On current macOS most tools already trust the keychain, where MDM installs the proxy's root: measured on macOS 26, Apple's curl, git and Python, Homebrew, pip and Go all do, and Node and npm, which carry their own CA list, are what the bundle fixes. `SSL_CERT_FILE` is left alone, because it would turn off Go's keychain verification. Re-run the bootstrap with `--skip-ghostty --skip-dotfiles` after the proxy's root CA changes; `dfinstall doctor` flags a bundle that is missing a keychain certificate. On an older macOS whose `curl` does not trust the keychain, the very first download can fail with a certificate error; fetch with the keychain's certificates instead:

```bash
security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain /Library/Keychains/System.keychain > /tmp/ca.pem
curl --cacert /tmp/ca.pem -fsSL https://raw.githubusercontent.com/sresarehumantoo/dotfiles/main/bootstrap/macos-setup.sh | bash
```

When Ghostty's download site is blocked, so `brew install --cask ghostty` fails, bootstrap offers Ghostty's nightly build from GitHub instead (GitHub carries no stable release of it), checked against GitHub's sha256.

### Existing Linux system

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make install
```

`make install` compiles the CLI and runs `dfinstall install all`, walking every module in dependency order: system packages, then shell, dev tools, git, the editor, the desktop, and platform tuning last.

> [!TIP]
> Not sure what it'll do to your machine? Run `./bin/dfinstall install all --dry-run` first. It prints every change and writes nothing.

### Without cloning first

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

`dfinstall --version` names the channel it came from (`release`, `brew`, `deb`, `go`, or `source` for a `make` build), which is what `update` goes by.

A `dfinstall` built or installed outside a clone has no configs to link until it has a checkout. `dfinstall init` clones one to `~/.local/share/dfinstall/dotfiles` and records it as the machine's checkout; every command that links or inspects managed files says to run it when none is found.

```bash
dfinstall init                          # clone this repo's default branch
dfinstall init --repo <url> --ref <ref> # another remote, branch or tag
dfinstall init --dir ~/dotfiles         # another location, or adopt a checkout already there
dfinstall install all
```

`init` is a no-op once a checkout is recorded. The default branch is the default ref because `update` fast-forwards a branch, and a tag checkout has none.

## Usage

**Installing and updating**

| Command | What it does |
|---------|--------------|
| `dfinstall init` | Clone the dotfiles checkout, for a binary installed without the repo |
| `dfinstall install all` | Install every module (except opt-in ones) |
| `dfinstall install shell` | Install a single module |
| `dfinstall update all` | Pull the clone, then install and update what dfinstall installed earlier (`install all --upgrade`) |
| `dfinstall update all --no-upgrade` | Re-apply all modules without pulling or updating (same as `install all`) |
| `dfinstall uninstall shell` | Remove one module's symlinks |
| `dfinstall uninstall all` | Remove all managed symlinks |

**Inspecting**

| Command | What it does |
|---------|--------------|
| `dfinstall status` | Every module grouped as installed, partial, not installed, skipped, or not available on this platform |
| `dfinstall diff` | Drift between config and filesystem, including managed keys in `~/.claude/settings.json` |
| `dfinstall diff --brief` | Only what changed since install, one line each; silent when clean |
| `dfinstall doctor` | Environment health checks, plus whether the clone itself is clean, pushed and level with its upstream |
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
| `--upgrade` | `install` | Pull the clone and update tools and plugins first ([details](#upgrading)); always on for `update` |
| `--no-upgrade` | `update` | Skip the pull and tool updates, making `update` a plain `install` |

```bash
dfinstall install omz --extended     # pick extended OMZ plugins
dfinstall install all --toolkit      # pick toolkit tools
dfinstall install windev             # opt-in Windows cross-dev module
dfinstall install all --dry-run      # preview everything
```

Output falls back to ASCII symbols and drops the spinner on terminals that cannot show them: `TERM=dumb`, the Linux text console (`TERM=linux`), and non-UTF-8 locales. Set `DF_ASCII=1` to force ASCII, for example when the terminal font lacks the symbols, or `DF_ASCII=0` to force Unicode. Color follows `NO_COLOR` and is off when output is not a terminal.

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

### From 1.x to 2.0

No re-clone: 2.0 finds the existing clone through the pointer 1.x already recorded (`~/.config/dfinstall/dotfiles-dir`), keeps linking from it, and reads the clone's old `.config.yaml` until it first saves a setting to `~/.config/dfinstall/config.yaml`. Two things need doing by hand, in this order:

1. **Bring the clone up to date first**: `git -C <clone> pull`. A 2.0 binary run against a 1.x clone skips the files the clone does not have yet (`not linked …: … is not in the dotfiles checkout`) rather than leaving dangling links, but they stay unlinked until it is pulled.
2. **Install 2.0, and retire the 1.x binary.** Staying on a source build, `make -C <clone> install-bin` replaces it. Moving to a package (`brew install sresarehumantoo/tap/dfinstall`, or apt), delete `~/.local/bin/dfinstall` and `~/.local/bin/dfinstall-mcp` as well, since `~/.local/bin` comes first on `PATH` and the old build would keep running. If Claude Code's MCP server was registered by path (`claude mcp list`), point it at the new `dfinstall-mcp`.

Then `dfinstall install all` and `dfinstall doctor`. Doctor's `dfinstall on PATH`, `checkout files` and `settings` rows report an older binary that still runs first, a clone missing files this dfinstall links, and a pre-2.0 `.config.yaml` that can be deleted once nothing older uses the clone.

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

Anything the override file already holds is not written again, so a repeated move, or a setting you had already put there, does not duplicate. `~/.gitconfig.local` stays the place for identity and site settings; the tracked `gitconfig` sets no identity. `~/.gitconfig.user` is included last, so a name there wins over one in `~/.gitconfig.local`. You can write to these files directly too. `~/.zshenv`, extra files in `~/.zsh.d/`, and anything else dfinstall does not link are never touched.

### Drift warnings

A new zsh checks, in the background, whether something changed a managed file behind dfinstall's back: a link replaced by a file or pointing at another clone, managed settings edited (for example Claude Code's `/config` rewriting `tui` in `~/.claude/settings.json`), or a linked file edited through its link. That last one is a script running `git config --global`: the link stays intact and the change lands in the clone as an uncommitted edit. The result shows once at a later prompt:

```
⚠ dfinstall: managed files changed outside dfinstall
  ~/.claude/settings.json: managed keys differ: tui
  what to do about each: dfinstall diff
```

It adds nothing to startup, runs at most every ten minutes, prints each finding once rather than in every pane, and never fixes anything itself. Links that were never installed are not reported. `DF_NO_DRIFT_CHECK=1` turns it off.

## Backup & Restore

On the **first** `install` run — when no `config.yaml` exists yet — dfinstall automatically snapshots everything it's about to touch. After a successful run it writes `config.yaml` with `skip_backup: true`, so later runs don't snapshot by default.

> [!IMPORTANT]
> A **failed** first run stays a first run. If any module errors, the config isn't written, so the automatic backup is still armed next time. `--dry-run` never consumes the first run either.

Override the default behavior with any of:

- **`--backup`** — always snapshot, whatever the config says
- **`skip_backup: false`** in `config.yaml` — snapshot on every install
- **`backup_dir`** in `config.yaml` — custom location (default `~/.local/share/dfinstall/backups/`)

```bash
dfinstall install all --backup    # force a snapshot
dfinstall restore --list          # see what's available
dfinstall restore                 # revert to the latest
```

Each entry records the original state — missing, symlink, or regular file — so restore undoes precisely what dfinstall changed.

> [!NOTE]
> System paths under `/etc/` are deliberately excluded from snapshots, so changes a module makes there are not restorable.

## Configuration

Config lives at `~/.config/dfinstall/config.yaml` (under `$XDG_CONFIG_HOME` when set), generated on first run. Releases before 2.0 kept it in the clone as `.config.yaml`; that file is read until the new one exists and is then left alone, so delete it once nothing older uses the clone. [`.config.yaml.example`](.config.yaml.example) documents every key — it's a reference, not a template that gets copied.

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
| `opencode_enabled` | `false` | Opt-in for the `opencode` module |
| `doc_sets` | python, bash, git, go, docker, kubernetes, node | Offline doc sets the `docs` module fetches: DevDocs slugs, or `gitlab` |

## Modules

Modules run in this order — dependencies first:

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
| 10 | **git** | gitconfig — delta pager, histogram diff, aliases |
| 11 | **nvim** | Neovim config with Lazy.nvim + headless sync |
| 12 | **windev** | _Opt-in._ Windows cross-dev toolchains + `winbuild` — see [docs](docs/windev.md) |
| 13 | **tmux** | Tmux config (Alt+A prefix, vi mode, custom theme) |
| 14 | **konsole** | Konsole profile and color scheme |
| 15 | **ghostty** | Ghostty terminal config |
| 16 | **sway** | Sway + waybar + swaync desktop — see [docs](docs/sway.md). No-ops without `sway` |
| 17 | **aerospace** | macOS tiling window manager (AeroSpace). No-ops off macOS |
| 18 | **sketchybar** | macOS status bar (SketchyBar). No-ops off macOS |
| 19 | **mackeys** | macOS keyboard like a Windows laptop: modifier swap, Ctrl shortcuts in every app, key repeat, no Accessibility needed. Tested only on macOS 26.7.1 (25G241). No-ops off macOS |
| 20 | **alttab** | macOS Option-Tab app switcher that looks like Command-Tab, and Win+L to lock, no Accessibility needed. Tested only on macOS 26.7.1 (25G241). Only `install alttab` sets it up |
| 21 | **htop** | htop config |
| 22 | **newsboat** | Feed reader with a curated security feed set; images inline via chafa, links in w3m |
| 23 | **docs** | Offline usage docs from DevDocs and GitLab (`doc_sets`), read with the `docs` script |
| 24 | **claude** | Claude Code global `CLAUDE.md` + managed keys merged into `settings.json` |
| 25 | **opencode** | opencode for Anthropic API / Bedrock users, locked to those providers with Claude Code-style permissions — see [docs](docs/modules.md#opencode). Opt-in |
| 26 | **wsl** | wsl.conf, sysctl tuning, .wslconfig, Windows home symlink, git fsmonitor |
| 27 | **vmguest** | Hypervisor guest tools when running in a VM; no-op on bare metal or WSL |
| 28 | **defaultshell** | Sets zsh as the default login shell |

> [!NOTE]
> **Opt-in modules** are skipped by `install all` until you enable them explicitly with `dfinstall install <name>`. Currently that's **windev** and **opencode**. Once enabled the choice persists in `config.yaml`, and future `install all` runs keep it current.

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

`diff` and `doctor` report multi-clone symlink drift, and `install` with module `all` records the canonical clone — matching the CLI exactly, because both go through the same code. `install` takes `upgrade: true` to do what `update` does; it returns the pull's report, and when the pull changes dfinstall's source it rebuilds the binaries and asks for a reconnect, since the running server keeps its old code. Moving local edits into override files needs a terminal, so there it only reports them.

</details>

## Building from Source

**Requirements:** Go 1.26+ ([install](https://go.dev/doc/install)), Git, and optionally Make.

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make build          # -> bin/dfinstall
```

Without Make — note both `-X` flags:

```bash
go build -ldflags "\
  -X github.com/sresarehumantoo/dotfiles/v2/internal/core.DefaultDotfilesDir=$(pwd) \
  -X github.com/sresarehumantoo/dotfiles/v2/internal/core.Version=$(git describe --tags --always --dirty)" \
  -o bin/dfinstall ./cmd/dfinstall
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
assets/                  # Logo SVG (+ generator) and the demo GIF
bootstrap/               # WSL bootstrap wizard (PowerShell + bash), macOS bootstrap
config/                  # Config files symlinked into ~
  shell/                 #   zsh/bash dotfiles
  devtools/              #   utility scripts -> ~/.local/bin/
  git/ nvim/ tmux/ ...   #   one directory per tool
  sway/ waybar/ ...      #   the Sway desktop
  aerospace/ macos/ ...  #   macOS window manager, bar, keyboard
cmd/
  dfinstall/             # CLI entry point (Cobra)
  dfinstall-mcp/         # MCP server (stdio JSON-RPC)
internal/
  core/                  # Module interface + LinkSet, linking, backup/restore,
                         #   output, spinner, env detection, install session,
                         #   canonical dir + drift, subprocess timeouts,
                         #   toolkit registry, version stamp
  modules/               # One file per module, plus shared machinery
tests/                   # Black-box tests (package tests)
                         #   in-package tests live beside the code in internal/
docs/                    # In-depth documentation
scripts/                 # e2e scenarios, corporate-network lab, release helpers
.goreleaser.yaml         # Release archives, .debs, checksums
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
make test           # go test ./cmd/... ./internal/... ./tests/...
make test-race      # run the suite under the race detector
make lint           # go vet
make fmt            # gofmt -s -w (in place)
make fmt-check      # gofmt -s check (non-mutating, same as CI)
make check-decoupling  # guard against site-coupled content in this public repo
make ci             # everything CI runs (fmt-check + lint + check-decoupling + build-all + test + test-race)
make e2e            # end-to-end scenarios in Docker (SCENARIO=sudo runs one)
make corplab        # corporate-network lab: TLS inspection + Nexus (SCENARIO=nexus runs one)
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

- **Go** — `gofmt -s` check, `go vet`, a decoupling guard, builds both binaries, `go test`, then the suite again under `-race`. The Go version tracks `go.mod`.
- **Go on macOS** — builds both binaries and runs `go test` on `macos-latest`, and checks that `bootstrap/macos-setup.sh` parses under the stock `/bin/bash` 3.2.
- **ShellCheck** — lints `config/devtools/`, `bootstrap/`, and `scripts/` (three separate scandirs).

`main` is protected: all three checks (Go, Go on macOS, ShellCheck) must pass before a PR can merge.

[Dependabot](.github/dependabot.yml) opens weekly PRs against `develop` for Go modules and Actions bumps — minor/patch grouped into one PR per ecosystem, majors individually since they tend to need attention. The same checks gate them.

## Documentation

| Guide | Covers |
|-------|--------|
| [Architecture](docs/architecture.md) | Core systems, module interface, output pipeline, linking |
| [Module Reference](docs/modules.md) | Detailed breakdown of every module |
| [Building from Source](docs/building.md) | Requirements, dependencies, cross-compilation |
| [Devtools Scripts](docs/devtools.md) | Utility scripts and shared helpers |
| [Keybindings](docs/keybindings.md) | Custom bindings across tmux, nvim, zsh, and the menus |
| [Neovim Tutorial](docs/nvim-tutorial/README.md) | Hands-on chapters to work through in nvim: editing, search, LSP, debugging, notes |
| [Sway Desktop](docs/sway.md) | The `sway` module — waybar, swaync, outputs, and the Alt reservation |
| [Windows Cross-Development](docs/windev.md) | The opt-in `windev` module — toolchains, nvim, `winbuild` |
| [Contributing](docs/contributing.md) | Adding modules, conventions, testing |

## License

[MIT](LICENSE) © Owen Pierce
