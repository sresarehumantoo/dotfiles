# Module Reference

Most modules are a single Go file in `internal/modules/`. They run in the order listed here.

---

## locale

**File:** `modules/locale.go`

Reports which UTF-8 locale shells will get, and changes nothing on the system. `config/shell/zsh/locale.zsh` sets `LANG` (when it is unset, `C` or `POSIX`) to `en_US.UTF-8` if that locale has been generated, else to `C.UTF-8`, which glibc 2.35+ ships built in. So a fresh machine gets UTF-8 with no root; what it does not get without generating `en_US.UTF-8` is en_US date formats and sorting. The module prints the Debian command for that rather than running it. The WSL bootstrap, which runs as root anyway, still generates `en_US.UTF-8`.

**Status:** One check: a UTF-8 locale is available, naming which (`en_US.UTF-8`, or `C.UTF-8` when en_US is not generated).

---

## packages

**File:** `modules/packages.go`

Installs core system packages using the detected package manager (apt-get, dnf, pacman, or brew).

**Packages:** git, zsh, curl, wget, htop, rsync, tmux, nodejs, npm, python3, golang, gh, locales, zsh-syntax-highlighting

`gh` is here because `config/git/gitconfig` uses `gh auth git-credential` as the github.com credential helper.

On macOS it also installs the GNU userland from Homebrew (`macGNUFormulae`): coreutils, findutils, gnu-sed, grep, gawk, gnu-tar, make, diffutils, bash and rsync, checked by formula since Apple ships its own `sed`, `bash` and `rsync`. `~/.profile` and `~/.zsh.d/path.zsh` put each keg's `libexec/gnubin` ahead of `/usr/bin`, so the unprefixed names are the GNU ones, as on Linux. The gnubin list is named rather than globbed: libtool has a gnubin too, and GNU libtool in front of Apple's breaks native builds. `path.zsh` also re-prepends Homebrew itself, because `/etc/zprofile`'s `path_helper` moves `/usr/bin` back to the front in every login shell, which includes each tmux pane.

Neovim is deliberately **not** installed here: apt's neovim is too old (Debian stable ships 0.7–0.10, telescope.nvim needs >= 0.11), so the nvim module installs the official prebuilt tarball instead.

Skips packages that are already installed. External command output is suppressed in default mode (shown with `-v`). Spinner pauses automatically for sudo password prompts.

**Status:** Checks 12 tool binaries via `$PATH`.

---

## extras

**File:** `modules/extras.go`

Three groups of additional tooling:

**CLI Utilities:** xclip, tree, fzf, ripgrep, fd-find, bat, jq, unzip, make, build-essential, tealdeer

**Python Tooling:** python3, pip3, python3-venv, pipx

**Docker:**
- Adds Docker apt repository (signed with GPG key), pinned at priority 100 in `/etc/apt/preferences.d/docker` so it can add packages but never replace one Debian or Ubuntu also ships. The NodeSource repo from `packages` gets the same pin next to its `nodejs` 600 pin.
- Installs docker-ce, docker-ce-cli, containerd.io, docker-buildx-plugin, docker-compose-plugin
- Runs Docker rootless for the user (docker-ce-rootless-extras, uidmap and slirp4netns, then `dockerd-rootless-setuptool.sh install`: a `docker.service` user unit and a `rootless` CLI context) instead of adding them to the `docker` group, whose members can mount `/` into a container as root. The system daemon stays installed for what rootless cannot do, such as `--privileged` work on the host kernel (registering binfmt_misc handlers), and is reached with `sudo docker`. Images and containers are separate per daemon.
  - Someone already in the `docker` group keeps that setup, and none of the rootless packages are installed (uidmap adds setuid helpers): switching the context would hide their existing images and containers. To move over, leave the group (`sudo gpasswd -d $USER docker`, then log in again) and re-run `install extras`.
  - Needs a systemd user session. Without one (a container, `su` without PAM) it prints the setup command instead. Where rootlesskit fails, such as Ubuntu 24.04+ restricting unprivileged user namespaces through AppArmor, it warns and points at <https://rootlesscontaine.rs/getting-started/common/>.
  - The daemon runs while you are logged in; `sudo loginctl enable-linger $USER` keeps it (and `--restart` containers) running from boot. Ports below 1024 need `net.ipv4.ip_unprivileged_port_start` lowered.
  - Arch: the `docker` package has no rootless mode (the AUR's docker-rootless-extras does), so it is installed without any group change.

**Rust:**
- rustup with the stable toolchain and `rust-src`, which rust-analyzer needs
- Linux: the official installer with `--no-modify-path`, because the rc files it would append to are symlinks into this repo; `~/.profile` and `path.zsh` put `~/.cargo/bin` on PATH
- macOS: Homebrew's keg-only `rustup` instead of `curl | sh`, which managed Macs often forbid; its `bin` (with the cargo/rustc proxies) goes on PATH the same way

**Terraform:**
- Linux: the latest release binary into `~/.local/bin`, no root. `SHA256SUMS` is verified against its detached signature, which must come from HashiCorp's release key (fingerprint pinned in `extras.go`); without `gpg` it falls back to the checksum alone and says so. Earlier versions added HashiCorp's apt repo instead; on those machines the repo and package are left as they are.
- macOS: Homebrew's `hashicorp/tap/terraform`

Reads `/etc/os-release` for `VERSION_CODENAME` to construct the Docker apt repo URL.

**Signing key refresh:** the Docker repo is re-checked on every converge, not just when first added. Upstreams rotate signing keys on their own schedule, and a stale key makes `apt update` reject the index (exit 100) while the repo file, the key file, and the installed binary all still look fine. Each converge fetches the repo's `InRelease` and verifies it against the keyring on disk; if that fails, the upstream key is downloaded and **appended** to the keyring, keeping the outgoing certificate for the duration of the rotation window. A key that fails to verify even after refresh is left alone and reported as a warning rather than overwritten. Offline or unreachable upstreams skip the check entirely, so no network blip can trigger a keyring rewrite.

After installing tealdeer, updates the tldr page cache (best-effort; skipped silently on network failure).

**Status:** Checks 19 binaries/packages (11 CLI utilities, 3 Python binaries, the `python3-venv` package, docker binary + rootless mode or an existing `docker` group membership, terraform, and the Docker apt keyring). The keyring check is offline: it catches an absent, unparseable, or expired key, but not a rotation; that is what the converge-time probe is for. Arch hosts install these tools from binaries and are not checked.

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
- `toolkit_registry_url` in `config.yaml` sets a persistent override
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

GitHub release downloads try a Nexus proxy of GitHub first when one is found through npm's registry setting; see [Package Mirrors](architecture.md#package-mirrors).
- **rustup:** Installs the rustup toolchain to `~/.cargo/bin/rustup`

The three GitHub-release methods (deb, AppImage, release_binary) share one
engine in `modules/github_release.go`: it fetches the latest release's assets
and picks the one matching this machine's architecture, skipping checksums,
signatures and other platforms' builds.

Selections are saved to `config.yaml` under `toolkit_tools`. Subsequent installs (without `--toolkit`) use the saved selections. To change, re-run with `--toolkit`.

**Status:** Shows `N/M tools` when tools are configured. Shows "run --toolkit to configure" when no tools are selected. Shows "registry not fetched" when no cache exists.

**Uninstall:** Removes what dfinstall placed: AppImage and release-binary files
from `~/.local/bin/`, git clones from `~/.local/share/toolkit/` (via
`core.RemoveManagedDir`), `.deb` packages via `sudo dpkg -r`, and rustup via
`rustup self uninstall`. apt/go/cargo/pipx tools must be removed manually, since
their package manager owns the location.

Where each method's artifact lives is decided in one place,
`modules/toolkit_artifact.go`, which the installers, `Status`, `Uninstall` and
the selection menu all consult. A second copy of that mapping would mean
uninstall deleting the wrong path.

---

## delta

**File:** `modules/delta.go`

Installs [delta](https://github.com/dandavison/delta), a syntax-highlighting pager for git diffs.

Linux: the release binary into `~/.local/bin`, no root and no dpkg. The static musl build is preferred (it runs on any libc), with the glibc build where no musl one is published (aarch64). The download is checked against GitHub's sha256 digest. Falls back to the system package manager only if the release cannot be used. macOS: Homebrew.

**Status:** Reports "installed" or "not found".

---

## fonts

**File:** `modules/fonts.go`

Installs the terminal fonts into `$XDG_DATA_HOME/fonts` (not a hardcoded
`~/.local/share/fonts`: fontconfig's default config resolves its font directory
against `XDG_DATA_HOME`, so a machine that sets it elsewhere would otherwise get
fonts in a directory nothing scans).

| Font | Source | Where |
|------|--------|-------|
| IosevkaTerm Nerd Font (Regular/Bold/Italic/BoldItalic) | Downloaded from the pinned Nerd Fonts release | `$XDG_DATA_HOME/fonts/IosevkaTerm/` |
| MesloLGS NF Regular.ttf | Bundled in `config/fonts/`, symlinked | `$XDG_DATA_HOME/fonts/` |

IosevkaTerm is downloaded rather than vendored (~28 MB per version is too much
to carry in `.git`), sha256-verified against the release's `SHA-256.txt` before
a byte is extracted, and only the four canonical faces are written, never the
Mono build (clamps icons to one cell) or Propo (not monospace). MesloLGS is the
vendored offline floor, so a box that never reaches the network still renders a
working prompt. Runs `fc-cache -f` when the font set changed.

**Pinned version:** `nerdFontsTag` in `modules/fonts.go`. The installed tag is
recorded in a `.nerd-fonts-tag` stamp beside the faces, so bumping the constant
actually causes a re-download. Detection is a filesystem check of the directory
the module owns, deliberately *not* an `fc-list` family query, which is
version-blind and also true of a copy hand-installed anywhere else.

**Migration:** removes the faces the pre-IosevkaTerm module installed
(`HackNerdFont*.ttf`), and the `MesloLGS NF Regular.ttf.bak` that linking
displaces: a `.bak` in a font directory is still a live font, since fontconfig
identifies files by content rather than extension.

**Unmanaged copies:** a copy of a managed family living outside
`$XDG_DATA_HOME/fonts/<dir>/` is a duplicate face, and `fontNotes()` reports it
so both `status` and `doctor` see it.

⚠ **Install warns about this exactly once, and that was not enough.** The
install-time warning sits in a branch reached only while the managed copy is
absent, so once it installs one, nothing looks again. Measured on a real machine:
`status` and `doctor` both showed a clean fonts row while fontconfig was
resolving **eight** copies of the family: four managed, four hand-installed,
byte-identical. A report that goes green while the condition persists is the same
failure the fonts doctor check was rewritten to fix.

⚠ **It cannot be `fc-list -q <family>`**, which is what install uses. That asks
"does this family exist anywhere", and once a managed copy exists the answer is
always yes: it can never see a duplicate. The question is about *paths*, so the
check lists them (`fc-list <family> file`) and filters out the managed directory.
Missing `fc-list` returns nothing rather than a guess.

⚠ Two measured details hold it together: the family match is **exact**, so
`IosevkaTerm Nerd Font` does not pull in `IosevkaTerm Nerd Font Mono` (a box
keeping the Mono build beside this one is not nagged about a duplicate that is
not one); and the managed-prefix test includes the separator, or a sibling
directory whose name merely starts with the managed one (`IosevkaTermPropo`
beside `IosevkaTerm`) reads as managed and its faces are never reported.

**Status:** counts the vendored links; reports a missing or stale download, any
unmanaged copies, and any legacy artifacts still to clean, in the INFO column.

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

Selections are saved to `config.yaml` under `extended_plugins` and written to `~/.config/dfinstall/plugins.zsh`, which the zshrc sources before the `plugins=()` declaration. Plugin names are validated against `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` before writing to prevent shell injection.

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
| `shell/zsh/drift.zsh` | `~/.zsh.d/drift.zsh` |
| `shell/zsh/git.zsh` | `~/.zsh.d/git.zsh` |
| `shell/zprofile` | `~/.zprofile` |

The zshrc sources p10k instant prompt, loads oh-my-zsh, then sources all `~/.zsh.d/*.zsh` files for modular configuration.

### `git diff` and `git show` open in Neovim

When nvim and its diffview.nvim plugin are installed, `git.zsh` wraps `git` so that `git diff` and `git show`, run from a terminal, open a side-by-side diff with a file panel. `git show` lists the commit (subject, author, date) beside the diff. `q` closes the view and nvim with it. Revisions, `--cached`/`--staged` and paths carry over (`git diff HEAD~2 HEAD -- src` works). Anything diffview would show differently goes to git and delta as before: other flags such as `--stat` or `--word-diff`, a blob (`git show HEAD:file`), a root commit, a range given to `show`, an empty diff, piped output, or a shell inside nvim. `command git diff` skips it once and `DF_GIT_NVIM=0` turns it off.

### zsh does not read `~/.profile`, so `~/.zprofile` bridges it

`~/.zprofile` is read for login shells, after `~/.zshenv` and before `~/.zshrc`, and its only job is to source `~/.profile` under `emulate sh`. Before it existed the shell module linked `.zshrc`, `.aliases`, `.p10k.zsh`, `.bashrc`, `.profile` and the `~/.zsh.d/*` files, and no `.zprofile`, `.zlogin` or `.zshenv` at all, and **zsh never reads `~/.profile` under any circumstances**, unlike bash.

On a desktop this is usually invisible, because the display manager sources `~/.profile` into the session and terminals inherit it. That is a convention, not a guarantee, and it does not hold here. Measured on this box 2026-09-12, the session `PATH` was:

```
/home/owen/.cargo/bin:/opt/swayfx/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin
```

with no `~/.local/bin`, which `~/.profile` prepends, so GDM never sourced it. Same root cause as the "GDM starts sway without a login shell" note in `config/waybar/config`, seen from the other side.

Under WSL it is never invisible: `wsl.exe` launches the login shell directly, so there is no session to inherit from. Everything in `~/.profile` silently failed to apply: `XDG_*`, `$HOME/bin` and `$HOME/go/bin` on `PATH`, the WSLg `XCURSOR_SIZE`/`GDK_SCALE` fixes (which are themselves WSL-only, so that block was dead exactly where it was needed), and the `~/.profile.local` hook where machine-local settings such as CA bundles live.

> [!CAUTION]
> **`~/.profile` exports `DF_PROFILE_SOURCED=1`, and `~/.zprofile` skips itself when it sees it.** That guard is what makes this safe to add on a machine where something else already sourced the file: `~/.profile` prepends to `PATH` unconditionally, so a second pass duplicates every entry it adds (measured: `$HOME/bin` twice). The sentinel is exported by `~/.profile` itself rather than set by `~/.zprofile`, because only that version can see a display manager's pass.

### Interactive shells start in tmux

The first thing `zshrc` does, above everything else, is put an interactive shell into tmux: `tmux new-session -A -s main`. `-A` attaches to `main` if it exists and creates it otherwise, so every terminal on the machine shares one session.

**To get a shell without tmux, set `DF_NO_TMUX`:**

```zsh
DF_NO_TMUX=1 ghostty      # one terminal, no tmux
export DF_NO_TMUX=1       # ...and anything it launches
```

Empty or `0` means "start tmux"; anything else means "don't". The name is negative, so requiring `1` to mean *no* tmux is the only reading that is not a coin flip. It follows the `DF_SMEAR_CURSOR` convention in `config/nvim`.

There was no opt-out before 2026-09-10, and that is what made this hard to live with. The only reliable route to a tmux-free shell was running `bash`. `TERM=dumb zsh` worked too, but by accident of the guard list, and it drags the prompt down to `robbyrussell`, drops the cursor-shape keybinds and un-colors tldr as side effects.

The block is skipped when: `DF_NO_TMUX` is set, tmux is not installed, `$TMUX` is already set, the shell is not interactive, `$TERM` is `linux` or `dumb`, the shell belongs to a VS Code (`TERM_PROGRAM=vscode`, which also covers Cursor and Windsurf) or JetBrains (`TERMINAL_EMULATOR=JetBrains-JediTerm`) integrated terminal, or the shell is an inbound ssh login.

### An inbound ssh login does not start tmux

Set `DF_TMUX_SSH=1` to opt back in, per host or per login. `DF_NO_TMUX` wins over it, so an explicit "no tmux" is always honored.

Nothing consulted `SSH_CONNECTION` before 2026-09-10, and it had two consequences:

- **Every ssh login joined that host's shared `main`.** Two terminals into one box mirrored each other's screen and fought over `window-size latest` (the tmux default; nothing in the 343-line `tmux.conf` sets it). The console session was in the same fight.
- **ssh *out of* a pane stacked two servers in one terminal.** `$TMUX` is not forwarded, so the remote shell sees an empty one and starts its own. Both then use `M-a` as prefix and both carry the root-table (`-n`) binds from `tmux.conf`: `M-n`, `M-1..9`, `M-Enter`, `M--`, `M-q`, `M-m`, `M-s`, `M-c`, `C-q`. The **outer** tmux eats every one of them, so the inner session cannot be driven by its own keys at all.

The second has no good fix from inside tmux: a shared prefix is a property of *both* configs, and the remote host is not always yours to configure. Not starting the inner server is what makes the keys work. `SSH_TTY` is checked beside `SSH_CONNECTION` because a login shell can carry one without the other depending on sshd's settings.

> [!NOTE]
> This trades away tmux's disconnect resilience on remote hosts, which is a real loss and the reason many people want tmux on ssh in the first place. `DF_TMUX_SSH=1` is how you take it back; put it in the remote host's own `~/.profile.local` if you want it permanently for one box. The default is the other way round because a session you did not ask for, shared with every other login and unable to receive its own keybinds, is worse than one you start deliberately with `tmux new -A -s <name>`.

> [!NOTE]
> **It is deliberately not `exec`.** It was until 2026-09-10, and `exec` replaces the shell, leaving nothing underneath it. That produced three complaints that never looked related to each other: detaching with `prefix d` **closed the terminal** instead of dropping to a prompt, so detach was not an escape hatch either; a tmux that failed to start (stale socket after a WSL VM restart, a `tmux.conf` you just broke, `/tmp` not writable) took the window down with it before the error could be read, making the failure invisible; and there was no way to `exit` back to a plain shell. Calling tmux normally costs one idle parent zsh for the length of the session and buys back all three. The explicit `exit` on success preserves the old behavior where detaching ends the terminal.

### Choosing a session, and knowing when one is new

Every terminal joins one shared session called `main`. `DF_TMUX_SESSION` overrides the name, which is how a terminal gets a session of its own:

```zsh
DF_TMUX_SESSION=review ghostty
```

**Sharing means mirroring, and that is the default.** Each terminal attaches another *client* to the same session, so both show the same window and both move when either one does, and `window-size latest` (the tmux default; nothing in the 343-line `tmux.conf` sets it) resizes the window to whichever client was used last, reflowing panes in the other. That is a preference rather than a defect, so the default is unchanged and `DF_TMUX_SESSION` is the way out. `aggressive-resize on` is the other available lever, and it only helps for clients viewing *different* windows.

**`tm` moves a session to this terminal without nesting** (defined in `aliases`). From a shell outside tmux (`DF_NO_TMUX`, or the plain shell left after tmux failed to start) it attaches; inside tmux it switches this client instead of starting a tmux in the pane. `tm work` names another session. `tm -d` also detaches the session's other clients, which **closes those terminals**, for the reason in the caution below. It is not `ta` because oh-my-zsh's tmux plugin defines that as an alias, and zsh refuses a function whose name is an alias.

> [!CAUTION]
> **Do not "fix" the mirroring with `-D`.** It is the obvious answer (a new client detaches the old one), and it is actively wrong now. A displaced client exits **0** (measured, tmux 3.5a), and the shell `exit`s on 0, so every new terminal you opened would **close the previous one**. `-D` was only ever safe while this block was an `exec` into a shell nobody could return to.

> [!CAUTION]
> **A resume and a fresh start are otherwise indistinguishable.** `new-session -A` attaches if the session exists and silently creates it if not, so when the server has gone (a WSL VM restart, a reboot, `tmux kill-server`) you get an empty session with nothing to say the old one is not coming back. `has-session` is now checked first, and a newly created session announces itself on the status line for 4s: either that the last save is being restored (continuum's auto-restore, on by default) or that nothing was.
>
> **`=` makes the match exact, and it is required.** tmux target names are *prefix* matches, so a plain `has-session -t main` is satisfied by a session called `mainwork` (measured) and would report a resume that is not happening. `DF_TMUX_SESSION` also has `.` and `:` rewritten to `_` first, as tmux does to the name it creates; otherwise `=foo.bar` is parsed as a pane target and never matches.
>
> **With auto-restore off (`tmux-restore off`), a fresh server pauses autosave.** Left alone, continuum's first autosave (15 minutes after the server starts) points resurrect's `last` at the empty session and overwrites `pane_contents.tar.gz`, which is one file rather than one per snapshot, so `prefix C-r` would restore nothing and the old scrollback would be gone. When a save exists, the block sets `@continuum-save-interval 0`, the banner names the window count and save time, and the status bar shows **SAVE PAUSED**. Restoring (`prefix C-r`) or saving deliberately (`prefix C-s`) resumes it, via resurrect's post-restore/post-save hooks in `tmux.conf`.
>
> **The server is started in `tmux-server.scope`** (`systemd-run --user --scope`) rather than inheriting the terminal's cgroup, which was one Ghostty window's scope. Without a user manager it falls back to starting in place.

> [!CAUTION]
> **The tmux SERVER gets the environment too, not just the panes.** The server is forked from the shell that starts it, which has not reached the `~/.zsh.d/*.zsh` sourcing yet, so `path.zsh`, `exports.zsh`, `locale.zsh` and `ssh.zsh` are now sourced explicitly just before `tmux new-session`. Panes always recovered on their own; anything spawned from the *server's* environment did not, and that is `run-shell`, `display-popup`, `tmux new-window <cmd>` and TPM. Measured before the fix: the server's `PATH` had no `~/.local/bin` at all (no `dfinstall`, no `sway-*` helper, nothing user-installed), and its `SSH_AUTH_SOCK` was `/run/user/1000/gcr/ssh` (gnome-keyring) while every pane used `~/.ssh/agent.sock`, so anything git-ish run from the server authenticated against a different agent holding different keys. It is a list rather than a glob of `~/.zsh.d` because `options.zsh` and `keybinds.zsh` configure this shell's own line editing and would be wasted work in a shell about to hand over; sourcing the four twice is harmless by construction, since `path.zsh` ends in `typeset -U path PATH` and the others only assign.

> [!CAUTION]
> **Two of the three IDE guards used to guard nothing.** `VSCODE_PID` is set for the extension-host process, not for the integrated terminal, and has been removed; `TERM_PROGRAM` is the check that works. `INTELLIJ_ENVIRONMENT_READER` is set only during JetBrains' one-shot env-probe shell and never in the terminal you type into, so **JetBrains terminals were being exec'd into tmux** until `TERMINAL_EMULATOR` was added beside it.

### Custom Shell File Preservation

Before linking, the shell module scans `$HOME` for custom shell files that aren't managed by dfinstall (e.g. `.companyrc`, `.work_env`, `.localrc`). New ones are kept automatically, with no prompt: the install notes which files it kept and where to opt out.

Preserved files are written to `~/.config/dfinstall/custom-sources.zsh`, which the zshrc sources after aliases. Each entry uses a guard: `[[ -f ~/.companyrc ]] && source ~/.companyrc`.

Kept files are recorded in `config.yaml` under `preserved_files`. To stop sourcing one, list it under `dismissed_files`: dismissed wins over preserved, and a file in either list is never reported again. Paths are validated against `^\.[a-zA-Z0-9][a-zA-Z0-9._-]*$` before writing to the shell-sourced file.

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
| `git` | Shadows `git` on PATH and strips AI attribution from commit messages on commit, merge and push |
| `gh` | Shadows `gh` on PATH and strips AI attribution from PR, issue, comment, release and `gh api` bodies |
| `_scrub.sh` | What counts as AI attribution, shared by the `git` and `gh` wrappers |
| `sysinfo` | System resource overview |
| `tlog-clean` | Strip ANSI escapes and powerline glyphs from tmux log captures |
| `clipboard-vm` | Diagnose and fix SPICE clipboard sharing in a QEMU/KVM guest (`--reset`) |
| `tmux-restore` | Toggle tmux session auto-restore (continuum + resurrect) on/off |
| `claude-tmux` | Resume each pane's Claude Code conversation when resurrect restores a server (`record`/`save`/`resume`) |
| `demorec` | Record the screen to mp4 and re-encode it small (`start`/`stop`/`status`/`render`) |
| `wsl-ffmpeg` | Install a standalone Windows ffmpeg.exe for demorec's WSL capture (`--dir`, `--yes`, `--force`) |
| `ghostty-shader` | Enable/disable Ghostty's cursor-trail shader per machine (`status`/`on`/`off`/`auto`/`reset`) |
| `docs` | read dfinstall's own docs and the offline doc sets the `docs` module fetched: `docs SET TOPIC` opens a page in nvim (less without it), `docs -s SET PATTERN` lists the pages that match |
| `newsboat-view` | newsboat's browser and image pager: images inline via chafa, links in w3m (`open`/`gui`/`yank`/`images`) |

`tlog-clean` uses a virtual terminal line buffer to correctly resolve cursor movements and zsh line-editor edits. Detects powerlevel10k-style prompts and replaces them with a clean `directory $ command` format, dropping git info and decorations. Supports file arguments and stdin piping.

All scripts are `chmod 755` before linking. Individual failures are warned and counted rather than stopping the whole module.

See [Devtools Scripts](devtools.md) for detailed script documentation.

**Status:** Checks 9 symlinks.

---

## git

**File:** `modules/gitmod.go`

Symlinks `config/git/gitconfig` to `~/.gitconfig`.

Asks for a git identity only when no global config sets one. The tracked `gitconfig` sets none, so a new machine is asked for `user.name` and `user.email` once. The answer is added to `~/.gitconfig.local` with `git config --file`, never written over it, since that file often carries site settings or is a symlink into another repo. Change it later with `git config --file ~/.gitconfig.local user.email <value>`.

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

- **23 symlinks** covering init.lua, lazy-lock.json, .stylua.toml, `notes.markdownlint.yaml` (the vault's relaxed lint rules), lua files for custom plugins and kickstart plugins, and an `after/ftplugin/markdown.lua` editor-options file
- **Neovim itself:** if nvim is missing or older than nvim-treesitter needs, installs the official prebuilt into `~/.local/opt` and links `~/.local/bin/nvim`, no root. An older distro or `/opt` copy is left installed (it is shadowed on PATH) and the command to remove it is printed. The tree-sitter CLI goes to `~/.local/bin` the same way.
- **Plugins:** Runs `nvim --headless "+Lazy! install" "+qa"` after linking, which only adds missing plugins; `--upgrade` runs `Lazy! sync` instead to update them
- **Backup:** If an existing nvim config is a git repo (not symlinks), backs it up to `~/.config/nvim.bak`

Plugin output is suppressed in default mode and shown in verbose/debug mode.

**Custom plugins:** colorizer, comment, flash, harpoon, markdown (render-markdown, bullets.vim), notes (obsidian.nvim), oil, smear-cursor, undotree, diffview

notes turns the directory in `$NOTES_DIR` (default `~/notes`) into an Obsidian-compatible vault, alongside the tutorial's sample vault at `docs/nvim-tutorial/vault` (found through the symlinked `init.lua`); vaults that don't exist are skipped. Markdown inside a vault gets relaxed linting and no format-on-save.

smear-cursor draws a cursor trail inside nvim. It disables itself when running under
Ghostty, whose `custom-shader` already draws one across the whole terminal; otherwise the
two stack into a double trail. Set `DF_SMEAR_CURSOR=1` to force it on anyway, or `0` to
force it off on a terminal without its own trail.

**Kickstart plugins:** autopairs, debug, gitsigns, indent_line, lint, neo-tree

**Status:** Checks 23 symlinks.

---

## windev

**File:** `modules/windev.go` &nbsp;·&nbsp; **See also:** [Windows Cross-Development guide](windev.md) for install + per-language usage walkthroughs.

**Opt-in.** Installs a Windows cross-development environment for building Windows software (C/C++, C#/.NET, Go, Rust) from WSL/Linux, plus the matching Neovim LSP/format/debug configs and a remote build-server helper. Excluded from `install all` until explicitly enabled with `dfinstall install windev`, which sets `windev_enabled: true` in `config.yaml`; subsequent `install all` keeps it current. `dfinstall uninstall windev` clears the flag.

### Local cross-compilation toolchains

| Language | Installs | Builds Windows binaries via |
|----------|----------|----------------------------|
| C / C++  | `mingw-w64`, `cmake`, `ninja-build`, `clangd`, `clang-format` (apt) | `x86_64-w64-mingw32-{gcc,g++}` |
| Go       | `gopls`, `dlv` (via `go install`); Go itself via the packages module | `GOOS=windows GOARCH=amd64 go build` |
| Rust     | `rustup` (official installer if absent), target `x86_64-pc-windows-gnu`, components `rust-analyzer`, `rustfmt`, `clippy` | `cargo build --target x86_64-pc-windows-gnu` (links via MinGW) |
| C# / .NET | .NET SDK (`dotnet-install.sh` → `~/.dotnet`), OmniSharp + netcoredbg into `~/.local/share/windev/`, `csharpier` (global dotnet tool) | `dotnet publish -c Release -r win-x64` |

Each language is best-effort: a failure warns but doesn't abort the rest, so a single missing toolchain never blocks the nvim/helper wiring.

### Neovim language support

Symlinks `config/nvim/windev/windev.lua` to `~/.config/nvim/lua/custom/plugins/windev.lua`; kickstart's `{ import = 'custom.plugins' }` picks it up automatically. The file:

- Sets up the **OmniSharp** LSP for C# (init.lua's `servers` table already covers `clangd`, `gopls`, and `rust_analyzer`; they attach as soon as the binaries are on PATH).
- Adds **conform formatters**: `clang-format` (c/cpp), `csharpier` (cs), `rustfmt` (rust). Go formatting (`goimports`+`gofumpt`) and linting are in the base config.
- Adds **nvim-lint linters**: `cpplint` (c/cpp), `clippy` (rust).
- Wires **DAP** adapters: `codelldb` for C/C++/Rust (if on PATH) and `netcoredbg` for C# (from `~/.local/share/windev/`).

The file uses dependencies + live runtime-table mutation so it extends conform/nvim-lint/nvim-dap without clobbering init.lua's existing config functions. Defers loading via `ft = { cs, c, cpp, go, rust }` to keep startup fast.

### Remote Windows build server

Links `config/devtools/winbuild` → `~/.local/bin/winbuild`, a small helper that dispatches builds to a Windows machine over SSH (rsync source up, run remote build, pull artifacts back). See [Devtools Scripts → winbuild](devtools.md#winbuild) for usage.

### Shell PATH

Writes `~/.config/dfinstall/windev.zsh` (regenerated on each install) prepending `~/.cargo/bin`, `~/.dotnet`, `~/.dotnet/tools`, and `$(go env GOPATH)/bin`, and exporting `DOTNET_ROOT`. The zshrc sources this snippet automatically.

### Caveats

- Cross-compiled Windows `.exe` debugging from WSL is limited; DAP targets are wired for the binaries' native debuggers (delve for Go, codelldb/netcoredbg for Linux-native builds). Remote-Windows debugging is out of scope.
- Downloads (dotnet-install.sh, OmniSharp, netcoredbg) need network access at install time. Failures warn rather than abort.

**Status:** Reports `disabled` when not enabled. When enabled, shows linked file counts and a `mingw+dotnet+rust+go` summary of detected toolchains.

**Uninstall:** Removes the nvim plugin file, `winbuild`, and `windev.zsh`, and clears `windev_enabled`. Language toolchains/SDKs are deliberately left installed; they may be shared with other workflows.

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

**Status bar:** 2-line layout: line 0 is a transparent spacer (`bg=terminal,fill=terminal`) creating a gap between the pane content and the status bar; line 1 is the real status bar. Two rounded pills (the left holds a badge and the window list, the right holds every readout) on a transparent strip, with one accent color used only where something is active. The left badge is a literal glyph in `tmux.conf` (U+F120, a terminal prompt); it was previously a distro icon read from `~/.config/dfinstall/distro-icon`, and `writeDistroIcon()` still writes that file but nothing reads it.

Key config: Alt+A prefix, vi mode, mouse enabled, 50k history, vim-style pane navigation, custom 8-color powerline theme.

**Save on shutdown** (where `systemctl` exists): links `tmux-save.service` and a `tmux-.scope.d/save-first.conf` drop-in into `~/.config/systemd/user/`, then enables the service. Its `ExecStop` runs resurrect's save when the user manager stops, and the drop-in orders every `tmux-*.scope` (tmux's per-pane `tmux-spawn-*` scopes and the `tmux-server` scope) after it, so nothing in tmux is signalled until the save is done. It skips the save while autosave is paused (see the shell section).

**Claude Code panes come back running Claude.** Resurrect only restarts commands on `@resurrect-processes`, and `claude` is not on its default list, so before this every Claude pane was restored as a bare shell. The saved command line carries no session id either (`claude --resume` reopens the picker, `claude` starts empty), so a pane that was running `claude` is restored with `claude-tmux resume`, which looks the conversation up by pane position in a map written by resurrect's post-save hook from the `@claude-session` pane option that the claude module's SessionStart hook sets. Without an entry, or when the pane's directory disagrees with the map, it falls back to `claude --continue`. The pattern has no `~`, so `doas claude` is not restored. See [claude-tmux](devtools.md#claude-tmux).

**Compositor restarts:** `update-environment` adds `SWAYSOCK` and `WAYLAND_DISPLAY`, and `exports.zsh` refreshes a pane's `SWAYSOCK` from tmux when the socket it names is gone, because the server outlives sway. A forward clock jump that leaves continuum's last-save stamp in the future is reset on client attach or focus.

**Uninstall:** Disables `tmux-save.service`, removes symlinks and deletes `~/.tmux/plugins/` (TPM) and `~/.config/tmux/plugins/` (where TPM installs plugins, since the config lives in `~/.config/tmux`).

**Status:** Checks the symlinks (4 with systemd, 2 without). Shows `tpm +N plugins` counted in `~/.config/tmux/plugins/`.

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

The whole module no-ops when `ghostty` isn't on `PATH`, including `Status()`, which
reports nothing rather than "missing".

Because Ghostty resolves a relative `custom-shader` against the config file's *resolved*
path, a symlinked config loads the shader straight out of the repo, so editing a `.glsl` is
live on the next Ghostty restart with no `dfinstall` run. The `shaders/` symlinks are
insurance for the case where that config is ever a real file. Config changes need a
restart, not just a reload.

On macOS it also links `config/ghostty/macos.conf`, which the config includes with
`config-file = ?macos.conf`: Ghostty's Linux default bindings (Ctrl+Shift for the
clipboard, tabs, windows and splits, Shift+PageUp/PageDown scrollback, prompt jumps,
font size, the command palette), added on top of the macOS ones so plain Ctrl stays with
the shell. Three Linux defaults are left out because something else owns them on the
Mac: Alt+1..9 (tmux's window binds) and Ctrl+Alt+arrows and Ctrl+Alt+Shift+J
(AeroSpace's mod is ctrl-alt); Cmd+Alt+arrows moves between splits instead. The include
is optional, so the missing file is silent on Linux.

**Status:** Checks 4 symlinks (5 on macOS).

---

## aerospace

**File:** `modules/aerospace.go`

macOS only. Links `config/aerospace/aerospace.toml` to `$XDG_CONFIG_HOME/aerospace/aerospace.toml`: the [AeroSpace](https://github.com/nikitabobko/AeroSpace) counterpart of the sway config, with the same keys on `ctrl-alt` instead of Super (cmd would take Paste, Save and Select All from every app; alt would swallow tmux's `M-` binds, since Ghostty sends Option as Alt). sway's `$mod+Ctrl` chords are `ctrl-alt-cmd`.

`install all` only configures an AeroSpace that is already installed; `dfinstall install aerospace` installs the cask (`nikitabobko/tap/aerospace`) first, into `~/Applications` when `/Applications` is not writable. Warns if `~/.aerospace.toml` also exists, because AeroSpace then loads neither. Uninstall removes the link and leaves the app.

**Status:** Checks 1 symlink, and reports nothing when AeroSpace isn't installed.

---

## sketchybar

**File:** `modules/sketchybar.go`

macOS only. The [SketchyBar](https://github.com/FelixKratz/SketchyBar) counterpart of waybar: links `config/sketchybar/sketchybarrc`, `colors.sh` and `plugins/*.sh` into `$XDG_CONFIG_HOME/sketchybar/`. Same gating as aerospace: `install all` only configures it when installed, `dfinstall install sketchybar` installs `FelixKratz/formulae/sketchybar` first. Only the by-name install runs `brew services restart sketchybar`: the service is a login item, which macOS announces and an MDM may forbid, so `install all` never creates one and only reloads a bar that is already running. Uninstall stops the service and removes the links.

The bar keeps waybar's glass strip, pills and Catppuccin colors. Workspaces 1 to 9 come from AeroSpace, whose config triggers `aerospace_workspace_change`, `aerospace_focus_change` and `aerospace_mode_change`, and reserves the bar's height in `gaps.outer.top`. Right side: VPN (only while a tunnel has an IPv4 address), a `caffeinate` toggle standing in for idle_inhibitor, volume, network, battery and the clock.

Not carried over, for want of a macOS equivalent: one workspace row per output (every display shows the same row), the notification count and do-not-disturb, the privacy indicator, media and temperature. The clock is at the right edge because a centered item would sit under the MacBook notch. The tray stays in the native menu bar, which should be set to hide automatically so it appears on hover above this bar; the module prints a notice until it is.

The plugins run under launchd with Homebrew's bin and the system dirs on PATH, so they are POSIX `sh` against Apple's BSD userland. Every plugin must be listed in `sketchybarPlugins`; `TestSketchybarPluginsListed` fails otherwise.

**Status:** Checks 10 symlinks, and reports nothing when SketchyBar isn't installed.

---

## mackeys

**File:** `modules/mackeys.go`

macOS only, and part of `install all`. Makes the built-in keyboard behave like a Windows laptop's without Accessibility, Input Monitoring or admin rights: every piece is a user preference or a key mapping any process may set, and each one persists across logins and reboots by itself, with no agent running.

**Tested on macOS 26.7.1 (build 25G241), with a MacBook's built-in keyboard, and nothing else.** Most of this module rests on undocumented behavior that Apple changes between releases without notice: the HID usage pages Globe is reported on, which `hidutil` properties the keyboard filter honors, the private `activateSettings`, and menu-title shortcuts. There is no compatibility promise for any other version, older or newer. On a different macOS, run `dfinstall doctor` and read `~/Library/Logs/mackeys-layout.log` before trusting it.

- **Modifier swap.** Globe and Control trade places, and so do Option and Command, so the bottom row reads Ctrl, Fn, Cmd, Opt like a laptop's Ctrl, Fn, Win, Alt. Option, which Ghostty sends as Alt, is next to space where Alt was. It writes the same per-keyboard preference as System Settings > Keyboard > Keyboard Shortcuts > Modifier Keys (`com.apple.keyboard.modifiermapping.<vendor>-<product>-0` in the ByHost global domain), which macOS reapplies at every login. It also sends the same mapping to the live HID service with `hidutil`, meant to apply it without a logout, but on macOS 26.7.1 that write has no effect: the swap comes only from the preference. A first install may therefore need a logout before the swap is live; that case has not been measured, since the tested Mac already had the swap set. The vendor and product IDs are read from `hidutil list`; when it does not report exactly one built-in keyboard the module warns and leaves the swap to System Settings. Globe is mapped on both HID pages a recent built-in keyboard reports it on (`0xFF00000003` and the vendor page `0xFF0100000003`), as System Settings does; mapping only the first leaves Globe unswapped on those models. External keyboards are not touched.
- **Ctrl shortcuts in every app, web apps included.** Global `NSUserKeyEquivalents` (System Settings > App Shortcuts > All Applications) rebinds menu items by title: Copy, Paste, Paste and Match Style (Ctrl+Shift+V), Cut, Select All, Undo, Redo (Ctrl+Y), Find, Save, Open, Print, New/Close Tab and Window, Reopen Closed Tab, private windows, Reload, Open Location, zoom, Bold/Italic/Underline. A browser passes a key the page did not handle to its menu, which is how Ctrl+C reaches a web app. Terminals (Ghostty, Terminal, iTerm2) get their own table, which macOS consults before the global one: clipboard, tab and window items move to Ctrl+Shift and everything else goes back to Command, so Ctrl+C, Ctrl+Z and Ctrl+W stay with the shell. Entries a user set for other titles are left alone, and uninstall removes only entries that still hold this module's value.
- **Key repeat.** `ApplePressAndHoldEnabled` off, so a held `j` repeats instead of offering accents, plus `KeyRepeat` 2 and `InitialKeyRepeat` 15. Tapping Globe does nothing (`AppleFnUsageType` 0), since it now sits where Fn does.
- **Scroll direction.** Natural scrolling off (`com.apple.swipescrolldirection` false), so content moves the way a Windows wheel or touchpad moves it.
- **Text navigation.** Links `config/macos/DefaultKeyBinding.dict` to `~/Library/KeyBindings/`: Home/End, Ctrl+Home/End, Ctrl+Left/Right by word and Ctrl+Backspace/Delete by word, with Shift variants, in Cocoa text fields, and the same moves on Cmd for the apps on the command layout below, where the Ctrl key sends Cmd. It also binds Ctrl+C/V/X/A/Z/Y and Ctrl+Shift+V to the clipboard, select-all and undo commands: Chromium (Chrome, Edge, Electron apps) hands a key to the page and these bindings before the menu, so without them Apple's emacs defaults win there (Ctrl+V pages down, Ctrl+A goes to the line start). Apps read the file at launch, so relaunch them after installing.
- **System shortcuts that took those keys.** Disables Mission Control (Ctrl+Up/Down), moving between Spaces (Ctrl+Left/Right) and switching input sources (Ctrl+Space), and applies the change with `activateSettings -u`. AeroSpace does not use native Spaces.

- **Passthrough apps.** Remote desktops and VMs (Windows App/Remote Desktop, Citrix Workspace, Horizon, VMware Fusion, Parallels, UTM) get the macOS defaults back, so Ctrl+C reaches the guest instead of copying on the Mac. Add a bundle ID to `passthroughApps` for another one; `osascript -e 'id of app "Name"'` prints it.
- **Login helper.** `dfinstall install mackeys` also writes `~/.local/bin/mackeys-apply` and a LaunchAgent (`io.github.sresarehumantoo.mackeys`) that runs it at login, on wake and every 10 minutes. It turns natural scrolling off again whenever it finds it back on, since that setting is easily reset, logging to `~/Library/Logs/mackeys.log`. It does not touch the modifier swap: macOS reapplies that preference itself, and the live mapping cannot be checked, since the keyboard filter never reports it (`hidutil --get` reads null on every Mac). Because it is a login item, `install all` never creates it, only keeps an existing one current.
- **Command layout for browsers, Electron and Office apps.** Web pages bind their shortcuts to Cmd on a Mac and never see Ctrl+Z as one, and Mac Office puts the Windows letters on Cmd (Cmd+Y redoes, Cmd+B bolds): Jira's editor keeps its own undo history, so the browser's Undo (all a Ctrl shortcut can reach) does nothing. `install mackeys` also builds `config/mackeys/layout.swift` into `~/.local/bin/mackeys-layout` and runs it from a LaunchAgent (`io.github.sresarehumantoo.mackeys-layout`, log `~/Library/Logs/mackeys-layout.log`). While an app in `commandLayoutApps` (Chrome, Edge, Brave, Arc, Vivaldi, Firefox, Slack, Teams, VS Code, Discord, Notion, Obsidian, Word, Excel, PowerPoint, Outlook, OneNote) is in front, the Ctrl key sends Command and the Win key sends Control. It does this with a `UserKeyMapping` that swaps Control and Command, which macOS applies after the modifier mapping, so it sits on top of the standard layout. A live `HIDKeyboardModifierMappingPairs` write is accepted by `hidutil` but has no effect on macOS 26, and that property reads back as null on every Mac. Leaving such an app clears the `UserKeyMapping`, including one set by hand. Pages then get the Cmd shortcuts they expect, and Win+Tab switches tabs, since Ctrl+Tab is now Cmd+Tab, the macOS app switcher. Those apps' menus get the macOS shortcuts back, plus Paste and Match Style on Cmd+Shift+V. The agent waits until no modifier is held before switching, since a layout changed under a held key leaves it stuck, and sets the layout again on wake and every 5 minutes. Both layouts and the app list are written to `~/Library/Application Support/dfinstall/mackeys-layout.json`, which alttab reads too. A Chrome PWA has its own bundle ID and keeps the standard layout. Safari is not on the list: it is sandboxed, and its preferences cannot be written without Full Disk Access, so its menus could not be moved back to Cmd. Any other app whose preferences refuse the write gets a warning and keeps the standard layout and the global shortcuts, rather than failing the module. It needs `swiftc` (`xcode-select --install`); without it every app keeps the standard layout. In VS Code's terminal, Ctrl+C copies; Win+C sends the interrupt.

Running apps keep the shortcuts and key repeat they started with, so the module prints a notice to reopen them when it changed something. Not covered, because each needs Accessibility: a scroll direction for the mouse that differs from the trackpad's (macOS has one setting for both), and window-level switching. Alt+Tab is the separate `alttab` module.

**When an app ignores a Ctrl shortcut.** Open its Edit menu and read the shortcut shown next to the item. If it shows ⌘ rather than ⌃, the menu title differs from the table (an app-specific "Copy Link", a different ellipsis): add the title, or a per-app entry. If it shows ⌃ and Ctrl still does nothing, the app consumes the key before its menu sees it. Without Accessibility nothing can rewrite the key at that point; Cmd still works there (the Win-key position), and Karabiner-Elements, deployed by IT, is the only general fix. If the app is a remote session that should receive Ctrl itself, add it to `passthroughApps`.

**Status:** Checks the `DefaultKeyBinding.dict` symlink. `dfinstall doctor` adds a `mac keyboard` row once installed, failing when the built-in keyboard's saved Modifier Keys preference does not hold the swap, press-and-hold or natural scrolling is back on, an installed login helper is not loaded, or an installed layout agent is not running.

---

## alttab

**File:** `modules/alttab.go`

macOS only. An Option-Tab app switcher for Windows muscle memory, plus Win+L to lock the screen (Command-L, since mackeys puts Command where the Windows key is, and Control-L in the apps on mackeys' command layout, where the Win key sends Control; it calls `SACLockScreenImmediate`, the same private function as the Lock Screen menu item, and falls back to sleeping the display). The hotkey is global, so Win+L locks from every app, a remote desktop included, and no app's own shortcut on that chord reaches it; browsers keep the address bar on Ctrl+L. Log Out stays on the stock Shift-Command-Q. The switcher: hold Option, tap Tab to step through apps in most-recently-used order (Shift-Tab steps back, Escape cancels), release Option to switch. A quick tap switches to the previous app without showing the panel, and the panel itself copies Command-Tab: app icons on a blurred rounded strip, the selected app's name under its icon.

**Tested on macOS 26.7.1 (build 25G241) only.** It calls the private `SACLockScreenImmediate` and depends on hotkey rules Apple has already changed once (15.0 versus 15.2), so there is no compatibility promise for any other version. On a different macOS, check `~/Library/Logs/alttab.log` first.

It needs no Accessibility, Input Monitoring or admin rights, which is the point on a managed Mac where AltTab or Karabiner would need them granted. Everything it does is open to an ordinary process: a Carbon `RegisterEventHotKey`, the running application list, a poll of the current modifier state to notice Option being released, and app activation. macOS 15.0 refused hotkeys whose only modifier is Option (Apple says 15.2 lifted that); if registration fails it falls back to Control-Option-Tab and logs which one it got. Activation is cooperative since macOS 14, so when a request is refused it opens the app through Launch Services the way a Dock click does. It switches apps, not windows, like Command-Tab: window titles need Screen Recording and raising one window needs Accessibility.

`dfinstall install alttab` compiles `config/alttab/main.swift` with `swiftc` from the Command Line Tools into `~/.local/bin/alttab`, writes `~/Library/LaunchAgents/io.github.sresarehumantoo.alttab.plist` and loads it, so it starts at every login and restarts after a crash. The agent is a login item, which macOS announces and an MDM may forbid, so `install all` never creates it and only rebuilds and restarts one that is already installed, whenever `main.swift` is newer than the binary. Logs go to `~/Library/Logs/alttab.log`. Uninstall unloads the agent and removes the plist and the binary.

**Status:** Notes a missing or out-of-date binary; reports nothing when not installed. `dfinstall doctor` adds an `alttab` row once installed, failing when the agent is not loaded or not running.

---

## htop

**File:** `modules/htop.go`

Symlinks `config/htop/htoprc` to `$XDG_CONFIG_HOME/htop/htoprc`.

**Status:** Checks 1 symlink.

---

## newsboat

**File:** `modules/newsboat.go`

Installs `newsboat`, `chafa` and `w3m`, links `config/newsboat/config` and `config/newsboat/urls` into `$XDG_CONFIG_HOME/newsboat/`, and creates an empty `config.local` beside them if there is none. A failed package install warns and still links the config.

- **Feeds:** `urls` holds a curated security set (advisories, threat intel, DFIR and detection, offensive research, malware/RE, cloud and appsec, news), tagged by category, plus query feeds: *Exploited in the wild* (0-day, KEV, actively exploited), *CVEs*, *Unread* and *Starred* (flag `s`). Titles mentioning CVE IDs, 0-days or KEV are highlighted everywhere.
- **Sync instead of local feeds:** set `urls-source` in `config.local` (e.g. `freshrss` with `freshrss-url`/`freshrss-login`/`freshrss-passwordeval`). The query feeds keep working because FreshRSS categories become tags with the same names. The config never fetches the page behind an item (`download-full-page no`), so reading an article contacts only the feed.
- **Images and links:** `browser` is `newsboat-view open`, which renders image URLs in the terminal and opens anything else in w3m (cookies off). `,i` pages through an article's images, `,o` hands a link to the desktop browser, `,y` copies it over OSC 52. Under Ghostty, chafa uses the kitty graphics protocol, through tmux too (`allow-passthrough on` in `tmux.conf`); elsewhere it picks sixel or text-cell art itself. Only `http(s)` URLs are opened, and downloads are capped at 25 MB.
- `config.local` is the user's file: `install` never overwrites it and `uninstall` leaves it. newsboat refuses to start when an `include` is missing, which is why the empty one is created.
- If `~/.newsboat` exists, newsboat uses it and ignores the XDG dir entirely; `install` warns about that.

**Status:** Checks 2 symlinks.

---

## docs

**File:** `modules/docs.go`

Fetches offline usage documentation, the pages a project publishes on its site rather than API dumps such as pydoc, into `~/.local/share/dfinstall/docs/<set>/`, and installs `w3m` to render them. Read them with the `docs` devtool: `docs python argparse`, `docs python argparse.Action` (opens at that entry), `docs gitlab ci/yaml`, `docs -s python add_mutually_exclusive_group`.

- **`dfinstall`** is always available and needs no fetching: the viewer reads this checkout's `README.md` and `docs/` live, with every heading as a topic (`docs dfinstall docs` opens this section, `docs dfinstall Configuration` the config table), so it follows `git pull` and works with the module skipped.
- **Sets** come from `doc_sets` in the config. Unset, that is python, bash, git, go, docker, kubernetes and node: about 50 MB on disk from an 8 MB download, six seconds on a fast link. Sets dropped from the list are deleted on the next install.
- **DevDocs** (`devdocs.io`) supplies every set except `gitlab`: one catalog and one format for hundreds of projects. A bare name takes the newest version the catalog lists (`python` is currently 3.15); `python~3.13` pins one. Pages are stored as DevDocs' HTML and rendered by w3m when opened.
- **`gitlab`** is not on DevDocs. It is the markdown under `doc/` in GitLab's own repo (a 42 MB tarball, 3,000 pages), opened in nvim as markdown. Section pages are Hugo `_index.md` files, indexed under the section name (`ci/yaml`).
- **Updates:** `install` leaves a set that is present alone. `update docs` refetches a DevDocs set only when its catalog entry changed, but refetches `gitlab` every time, since its archive carries nothing to compare.
- Neither source publishes checksums, so downloads are not verified beyond TLS. Every page path is checked to stay inside the set's directory.
- `uninstall docs` removes the fetched sets and keeps w3m.

## claude

**File:** `modules/claude.go`

Symlinks `config/claude/CLAUDE.md` to `~/.claude/CLAUDE.md`, the global Claude Code instructions loaded in every session. It ends with `@~/.claude/CLAUDE.local.md`, so each machine can layer untracked, host-specific instructions on top; Claude Code skips the import when the file is absent.

`~/.claude/settings.json` is **merged, not linked**, because Claude Code rewrites it itself (`/config`, `/model`, plugin toggles). Each top-level key in `config/claude/settings.json` (commit/PR attribution off, `includeCoAuthoredBy: false`, the update channel, a SessionStart hook running `claude-tmux record`) replaces the live value; every other key is left untouched. That includes `hooks` as a whole, so a hook added by hand to the live file is replaced on the next install; add it to the tracked file instead. Keep anything machine-specific (permissions, `autoMode.environment`, `env`) out of the tracked file: this repo is public. An unparsable live file is never rewritten. Uninstall removes the link and leaves the merged keys in place.

**Status:** Checks 1 symlink; stale managed settings keys are listed alongside.

---

## opencode

**File:** `modules/opencode.go`

**Opt-in.** Installs [opencode](https://opencode.ai) for **Anthropic API key and Amazon Bedrock** users. A Claude Pro/Max subscription does not work: Anthropic does not allow it outside Claude Code. Excluded from `install all` until `dfinstall install opencode` sets `opencode_enabled: true`; `dfinstall uninstall opencode` clears it.

**Binary.** Installed to `~/.local/bin/opencode` from the GitHub release (sha256 digest checked) or from npm. The order follows the machine: when npm points at a mirror (Nexus, Artifactory; see `doctor`'s `package mirrors` row) npm goes first, since a corporate proxy may block GitHub; otherwise the GitHub release, then npm. Either falls back to the other. From npm it installs the platform package (`opencode-linux-x64` and so on) with `--ignore-scripts` into `~/.local/share/dfinstall/opencode`, rather than `opencode-ai`, whose postinstall script only copies the binary out of that same package. Both channels ship byte-identical binaries. An opencode already on `PATH` from elsewhere is used as is and never upgraded or removed. `dfinstall update opencode` upgrades the one dfinstall installed; opencode's own updater is off.

**Settings.** `config/opencode/opencode.jsonc` (comments allowed) is merged by top-level key into `~/.config/opencode/opencode.json`, like Claude's settings, since opencode edits that file itself. What it sets:

- `enabled_providers: ["anthropic", "amazon-bedrock"]`: no other provider loads, even with its key in the environment. The file shows how to add others (`xai`, `openai`, `google`, `openrouter`); a provider added there applies to every machine, while one added to the live file on a single machine is reported as drift and reverted by the next install.
- `disabled_providers: ["opencode"]`: Zen. With no config at all opencode offers 10 free models without any key, some of which say prompts may be used for training.
- `share: "disabled"`, `autoupdate: false`, `plugin: []`.
- `instructions: ["~/.claude/CLAUDE.local.md"]`, because opencode ignores the `@` import at the end of the shared `CLAUDE.md`.
- `permission`, modeled on Claude Code's default mode. opencode's defaults allow nearly everything, including any shell command. Here edits and shell commands ask (read-only `git`, `ls`, `rg`, `wc` excepted), `/tmp` is writable without asking, web fetch and search ask, reads are allowed except SSH, AWS, GPG, Vault token and opencode credential paths, and editing opencode's, dfinstall's or Claude's own config is denied. Comments in the file show the one-line change back to opencode's defaults. `cat` and friends are not auto-allowed because a shell command ignores the read denies.

Machine-specific settings stay out of the tracked file: `model`, `small_model`, Bedrock `region`/`profile`. Bedrock reads `AWS_REGION`/`AWS_PROFILE` from the environment, and a config value would override them on every machine. Bedrock lists about 160 models, 68 of them Anthropic; a commented `whitelist` example shows how to trim it.

`~/.config/opencode/AGENTS.md` links to `config/claude/CLAUDE.md`, so opencode follows the same instructions without depending on the claude module.

**Untrusted repositories.** A project's `opencode.json` and `.opencode/` merge over user config. Measured: a cloned repo can switch providers, turn sharing back on, loosen permissions, and its `.opencode/plugins/*.js` **runs on any opencode command**, `opencode models` included. `~/.config/dfinstall/opencode.zsh`, sourced from `zshrc`, closes most of that without root:

- `OPENCODE_CONFIG_CONTENT` re-applies the managed provider, sharing, plugin, autoupdate and permission keys. opencode loads it after project config.
- `OPENCODE_PURE=1` loads no external plugins, project ones included.
- `OPENCODE_DISABLE_FREE_MODELS`, `OPENCODE_DISABLE_AUTOUPDATE`, `OPENCODE_DISABLE_LSP_DOWNLOAD`.
- `opencode-untrusted` runs opencode with `OPENCODE_DISABLE_PROJECT_CONFIG=1`, ignoring the checkout's config and `AGENTS.md` entirely.

What remains: a project can still **add** a permission pattern (say `"bash": {"curl *": "allow"}`), because permission rules merge one by one and only the ones the pin names are restored. Use `opencode-untrusted` for a checkout you have not read. The env file only reaches shells that source `zshrc`.

opencode loads `config.json`, `opencode.json` and `opencode.jsonc` in that order, later winning. Install refuses while a `config.json` or `opencode.jsonc` with real settings sits beside `opencode.json`; the `$schema`-only `opencode.jsonc` opencode writes on a first run is ignored.

**Uninstall:** Removes the link, `opencode.zsh`, the binary and the npm prefix, and clears `opencode_enabled`. The merged settings stay (removing them would turn free models and sharing back on), as do credentials (`~/.local/share/opencode/auth.json`) and sessions.

**Status:** Checks 1 symlink; notes a missing binary, an overriding config file, and stale managed keys.

---

## wsl

**File:** `modules/wsl.go`, `modules/wsl_interop.go`, `modules/wsl_shader.go`

WSL-specific setup. Skips entirely on non-WSL systems.

**Tasks:**

| Task | Target | Method |
|------|--------|--------|
| wsl.conf | `/etc/wsl.conf` | sudo copy (hash-checked) |
| sysctl | `/etc/sysctl.d/99-wsl.conf` | sudo copy (hash-checked) |
| Windows exe shims | `~/.local/bin/<name>.exe` | generated `exec` wrappers |
| .wslconfig | `C:\Users\<user>\.wslconfig` | rendered from template + host specs |
| Windows home link | `~/owen` -> `/mnt/c/Users/owen` | symlink |
| Ghostty shader choice | `~/.config/ghostty/ghostty.local` | prompt (interactive) |

**Status:** wsl.conf, sysctl conf, Windows home symlink, and the shims (counted only
against binaries this host actually has, so a missing `pwsh.exe` is not drift).
Reports "not WSL" on non-WSL systems.

**Uninstall:** removes the shims and the shader override. It deliberately leaves
`/etc/wsl.conf` and the sysctl drop-in alone: removing those needs sudo and would
change how the distro boots, which is not what uninstalling a dotfiles module implies.
The `.bak` files written on install are the way back.

### The Windows PATH is off, and that is load-bearing

`config/wsl/wsl.conf` sets `appendWindowsPath=false`. The default merges 15-40
`/mnt/c` directories into `$PATH`, every one of them drvfs/9p, i.e. a host round
trip per `stat()`. With `zsh-syntax-highlighting` doing a command lookup per word
as you type and `zsh-autosuggestions` searching per keystroke, that cost lands
directly on typing latency.

Three consequences to know before touching any of this:

- **Nothing is added back to `$PATH`.** Instead, `install wsl` writes `exec`
  wrappers into `~/.local/bin`, which is already on PATH and on ext4, so command
  lookup never touches 9p at all. That is faster than re-adding even two or three
  System32 directories.
- **`hasInterop()` must not use `exec.LookPath("cmd.exe")`.** With the Windows
  PATH gone that lookup fails on a healthy machine, and the old code read the
  failure as "interop is off", silently skipping `.wslconfig` and the home symlink
  forever. It now checks the `binfmt_misc` handler (both `WSLInterop` and
  `WSLInterop-late`; newer builds use the latter) plus `cmd.exe` on disk.
- **Everything Windows-facing resolves through `wslpath`**, which is a native
  Linux ELF inside the distro and needs no interop of its own. That is what breaks
  the chicken-and-egg where the module turning the PATH off needed the PATH to run.
  It also handles a non-`C:` system drive and a non-default automount root.

Shims are wrapper scripts rather than symlinks so they do not depend on how the
kernel resolves a link before matching MZ magic, and the resolved absolute path
stays readable in the file. They carry a managed-header marker; anything in
`~/.local/bin` without it is left alone by both install and uninstall.

### Both WSL config files are rendered, not copied

`config/wsl/wsl.conf` carries a `@DEFAULT_USER@` placeholder rather than a literal
`[user] default=owen`. That key decides which account WSL logs into, so shipping
one machine's username to another host names a user that may not exist there,
the same bug class as the sizing below, with worse consequences. It is filled from
the account running the installer, and **omitted entirely** if that cannot be
determined (WSL then falls back to the distro's initial user, which is correct).

⚠ **`renderedWslConf()` is the single source of truth**, and `Status`, `doctor`
and `install` all go through it. Comparing the raw template against the installed
file (which `core.FilesMatch`/`checkFileMatch` do) can never match once the
template holds a placeholder, so each would report permanent, unfixable drift on
a healthy machine. `checkFileMatch` is only valid for sources installed verbatim.

⚠ **Never name a placeholder token in a template's own prose.** Substitution is a
plain `ReplaceAll`, so a second mention is expanded too. In `wslconfig` that
shipped a real bug: the replacement is multi-line and not comment-prefixed, so it
injected bare `memory=`/`swap=`/`processors=` keys above the `[wsl2]` header where
they sit in no section at all. `TestRenderedWslconfigIsWellFormed` renders the real
file and rejects any key before a section header.

### What the installed `.wslconfig` is compared against

⚠ **This file used to be checked by nothing.** `installWslconfig` wrote it and
forgot it: `Status()` and `doctor` both covered `/etc/wsl.conf`, the sysctl
drop-in, the Windows-home link and the shims, but never this, so a Windows-side
tool resetting a key was silent and permanent until the next `install wsl`. It is
also the file most exposed to being rewritten from outside, since it lives in the
Windows home rather than the distro.

⚠ **The comparison is by KEY, never byte-for-byte.** Two things rule the obvious
implementations out: the template holds `@HOST_SIZING@`, so comparing against it
raw reports permanent unfixable drift (the `checkFileMatch` trap above), and
comparing against a *fresh render* would have to run the PowerShell/`cmd.exe`
host probe, which `Status()` must not do, being a fast synchronous read with no
context to cancel.

So `wslconfigDrift(template, installed)` reports on exactly the keys the template
declares, matching section and key case-insensitively:

- **sizing is never compared.** `memory`/`swap`/`processors` reach the file only
  through the placeholder, so they are absent from the template's key set. That
  is deliberate: they are host-derived, and a hand-tuned `memory=` is the owner's
  call, not drift.
- **keys the file has and the template does not are ignored.** The host sizing
  arrives that way, and beyond that the file is the user's to extend. This
  reports only on what the repo claims to own.

`TestWslconfigDrift_OurOwnRenderIsClean` is the guard that matters: a file this
repo rendered itself must report no drift, and the test also asserts that a byte
comparison *would* have differed, so the reason the code is shaped this way is
checked, not just asserted in a comment.

`doctor` names each drifted key and follows with the two-step remediation:
`dfinstall install wsl` (which keeps the current file as `.wslconfig.bak`), then
`wsl --shutdown` from PowerShell. Forgetting the second step reads as "the fix
did not work".

### A .wslconfig dfinstall did not write

Every render carries the header line ``…by `dfinstall install wsl`.``, and
that line is how dfinstall tells its own file from one something else wrote. A
`.wslconfig` without it (one a provisioning tool manages for the VM's memory
and swap, say) is **left as it is**: `install wsl` prints a notice instead of
replacing it, `status` counts it neither way, and the `doctor` row passes while
listing any keys that differ from this repo. To hand the file over, move it
aside and run `dfinstall install wsl`. A file written by v1.0.0, the last
release before the header, is treated the same way.

### .wslconfig sizing

`config/wsl/wslconfig` is a **template**. The `@HOST_SIZING@` line is replaced at
install time with `memory`/`swap`/`processors` derived from the Windows host:
CPU count from `cmd.exe /C echo %NUMBER_OF_PROCESSORS%` (an env var, so it is
cheap) and RAM from a PowerShell `Get-CimInstance` call. Reading `nproc` or
`/proc/meminfo` inside WSL would be circular: those already reflect the
`.wslconfig` being written.

Policy is half the host's RAM clamped to [2, 24] GB, swap at a quarter of that,
and all logical CPUs. **A failed probe omits the key rather than guessing**, so
the worst case is "WSL's own defaults apply". The file previously hardcoded
`memory=10GB` / `processors=8`, one developer's desktop, copied verbatim onto
whatever machine ran the installer. `wsl_interop_test.go` guards against that
returning.

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

After installing, enables/starts the relevant systemd units (`qemu-guest-agent`, `spice-vdagentd`, `open-vm-tools`) when `systemctl` is available; static units are started rather than enabled. A unit that is already running and enabled (or static) is left alone, and guest packages already installed are skipped, so a re-run needs no sudo. Prints hypervisor-specific hints for clipboard/drag-drop support (e.g. running `clipboard-vm` on KVM/QEMU, or installing `open-vm-tools-desktop` / `virtualbox-guest-x11`).

**Status:** Reports the detected virtualization type and checks the guest packages for that hypervisor. Returns empty on WSL and non-VM systems.

---

## defaultshell

**File:** `modules/defaultshell.go`

Sets zsh as the default login shell. Skips when the account's recorded login shell (`getent passwd`, or `dscl` on macOS) is already the zsh path resolved from `$PATH`; `$SHELL` is not used, because it only changes at the next login. A shell of `/usr/local/bin/zsh` against a resolved `/usr/bin/zsh` does *not* count as already set.

With a terminal it runs plain `chsh -s <zsh>`, which asks for the user's own password and needs no root. Without one (under the MCP server, where stdin is the JSON-RPC stream, or a piped bootstrap), or when plain `chsh` fails, it uses `sudo chsh` only if sudo needs no password (a NOPASSWD rule, cached credentials, the bootstrap drop-in). Otherwise it warns with the `chsh -s <path>` command to run.

**Status:** Reports "zsh" if the account's login shell is zsh, or that shell's path.
