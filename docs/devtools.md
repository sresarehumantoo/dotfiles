# Devtools Scripts

Utility scripts installed to `~/.local/bin/` by the devtools module. All scripts use colored output with unicode symbols and include `-h`/`--help` support.

## Shared Helpers (`_lib.sh`)

Most scripts source `_lib.sh` for consistent output and common guards (the exception is `tlog-clean`, which is self-contained and reads only stdin/files):

```bash
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"
```

### Output Functions

| Function | Symbol | Color | Description |
|----------|--------|-------|-------------|
| `info` | `▸` | blue | Informational message |
| `ok` | `✓` | green | Success |
| `warn` | `⚠` | yellow | Warning (stderr) |
| `err` | `✗` | red | Error (stderr) |
| `die` | `✗` | red | Error + exit 1 |
| `header` | `──` | cyan/bold | Section header |
| `step` | `…` | dim | Progress step |

### Guard Functions

| Function | Checks |
|----------|--------|
| `require_wsl` | Running inside WSL (checks `/proc/sys/fs/binfmt_misc/WSLInterop`) |
| `require_cmd <name>` | Command exists in `$PATH` |
| `require_git_repo` | Inside a git working tree |

Each guard calls `die` with a clear message on failure.

### Confirmation

```bash
confirm "Delete these files?"   # returns 0 for yes, 1 for no
```

Styled prompt with `[y/N]` default. Use with `||` for abort:

```bash
confirm "Continue?" || { info "Aborted."; exit 0; }
```

---

## Scripts

### sysinfo

System resource overview. No arguments, no confirmation needed.

```
$ sysinfo

── System ──

  OS:          Debian GNU/Linux 12 (bookworm)
  Kernel:      6.6.87.2-microsoft-standard-WSL2
  Env:         WSL2

── CPU ──

  Model:       13th Gen Intel(R) Core(TM) i7-13700K
  Cores:       24

── Memory ──

  Used:        4.2G / 16G

── Disk ──

  /            100G  45G  55G  45%
  /mnt/c       1.0T 600G 400G  60%

── Docker ──

  (docker system df output)
```

Sections: System, CPU, Memory, Disk (root + mounted Windows drives), Docker (if running).

### docker-cleanup

Full Docker system purge. Requires confirmation.

- Stops each running container individually (one failure doesn't block the rest)
- Runs `docker system prune -af --volumes`
- Shows disk usage before and after

```
$ docker-cleanup

── Current Docker disk usage ──
TYPE          TOTAL     ACTIVE    SIZE      RECLAIMABLE
Images        5         2         1.2GB     800MB (66%)
...

  ? This will remove ALL Docker data. Continue? [y/N] y

── Stopping running containers ──
  … Stopping my-app...
  ✓ Stopped my-app

── Pruning everything ──
...

── Docker disk usage after cleanup ──
...

  ✓ Docker cleanup complete.
```

### git-prune-branches

Removes local branches whose remote tracking branch is gone. Never deletes the current branch or `main`/`master`.

- Runs `git fetch --prune` first
- Lists branches to delete and asks for confirmation
- Continues past individual deletion failures

```
$ git-prune-branches

── Fetching remote tracking info ──
  … Running git fetch --prune...
  ✓ Fetch complete

── Stale branches ──
  ▸ feature/old-thing
  ▸ fix/deprecated-api

  ? Delete these 2 branch(es)? [y/N] y

  ✓ Deleted feature/old-thing
  ✓ Deleted fix/deprecated-api

  ✓ All stale branches removed.
```

### git

Linked as `~/.local/bin/git`, ahead of the real git on PATH, so it applies to anything that runs `git`: shells, editors, and coding agents. It strips AI attribution from commit messages and prints nothing while doing it:

- `Co-authored-by:` lines naming an assistant (Claude, Copilot, Cursor, Codex, Gemini, ...). Human co-authors are kept.
- `Generated with/by ...` lines naming one, such as `🤖 Generated with [Claude Code](...)`.
- `Claude-Session:` lines and `claude.ai/code/session` links.

How:

- **commit, merge** (and aliases to them) run with `core.hooksPath` pointed at `~/.cache/git-scrub/hooks`, which the wrapper creates for itself. Its `commit-msg` cleans the message before the commit is created, so the hash git prints is the final one. Every other hook there hands off to the repository's own hooks (`.git/hooks`, or its `core.hooksPath`, husky included).
- **Any other git** (`/usr/bin/git` run directly, a tool that never reads your shell files) still gets the `commit-msg` scrub: `install devtools` links the same hooks into `~/.local/share/dfinstall/git-hooks` and writes `~/.config/dfinstall/git-scrub.gitconfig`, which the tracked gitconfig includes, making that directory the global `core.hooksPath`. The two directories are kept apart so the wrapper never writes into one dfinstall manages. Uninstalling devtools removes the file, so repositories fall back to their own hooks. A repository that sets its own `core.hooksPath` (husky does) overrides the global one, so there only the wrapper scrubs, and the push rewrite below is the backstop.
- **`--no-verify`** skips `commit-msg`, so after a commit or merge any unpublished commit still carrying attribution is re-created with only the message changed.
- **push** first rewrites, on every local branch it is about to send (named refspecs, `--all`/`--mirror`, `push.default=matching`, else the current branch), the commits that no remote-tracking ref contains, from the first one carrying attribution onward. It uses `commit-tree` + `update-ref`, so trees, authors and dates are kept and a dirty worktree is fine. A published commit is never rewritten. If the rewrite fails, the push is refused.

Every rewrite is logged to `~/.local/state/git-scrub.log`. `DF_GIT_NO_SCRUB=1` runs the real git untouched, and turns off the global hooks' scrub and the [claude-no-attribution](#claude-no-attribution) hook too; export it from `~/.profile.local` to opt out on a machine for good. Every other subcommand is a plain `exec` (about 2ms of overhead). What counts as attribution is defined once, in `_scrub.sh`, which the `gh` wrapper shares.

### gh

Linked as `~/.local/bin/gh`, the same way, for what reaches GitHub without passing through git. GitHub keeps the edit history of PR, issue and comment bodies where anyone who can see them can read it, so this has to happen before the call. It cleans:

| Command | Flags |
|---|---|
| `pr`, `issue` (create, edit, comment, review, merge) | `--body`/`-b`, `--body-file`/`-F`, including `-` for stdin |
| `release` | `--notes`/`-n`, `--notes-file`/`-F` |
| `api` | `-f`/`-F` field values, `key=@file` and `@-`, `--input` (JSON) |

`pr merge --body` matters because the squash or merge commit is created on GitHub, out of the git wrapper's reach. JSON and GraphQL are cleaned where a line break is the two characters `\n`, so `--input` bodies and `gh api graphql -f query=...` mutations with an inline body are covered, and the JSON stays valid. Body files are cleaned into a temporary copy; the original is never modified. Every other command is a plain `exec`. Not covered: `gh` aliases that expand to one of these, bodies typed into gh's interactive editor, and anything that reaches the API without `gh` (curl, the GitHub MCP server, the web UI).

### xfreerdp

Linked as `~/.local/bin/xfreerdp`, ahead of the real one, for RemoteApp (`.rdp` files with `remoteapplicationmode:i:1`) through Xwayland. Without it, xfreerdp 2.x under sway gives RemoteApp windows that ignore clicks or only repaint when focus changes. Two causes, each fixed before the real xfreerdp starts:

- **Monitor origin.** In a multimon session xfreerdp makes X monitor 0 the remote primary and computes the offset from local 0,0, but nothing in its X11 client applies that offset to RemoteApp windows or mouse input. XRandR lists the primary monitor first, and Xwayland has none (sway sets none, and Xwayland is restarted whenever its last X client exits), so the wrapper makes the output at `+0+0` primary when no primary is set.
- **Maximize forwarding.** Sway marks tiled X11 windows `_NET_WM_STATE_MAXIMIZED_*`, and xfreerdp forwards that to the server as a maximize across the whole desktop. `_xfreerdp-nomax.c` is an `LD_PRELOAD` shim that hides those two atoms from xfreerdp's own property reads, so a tile is forwarded as an ordinary move/resize. The wrapper builds it with `cc` on first use into `~/.cache/dfinstall/libxfreerdp-nomax.so` (needs `libx11-dev`), and rebuilds it when the source changes.

Outside a Wayland session the wrapper is a plain `exec`. If the shim cannot be built it says so and runs xfreerdp without it.

### wsl-resize-disk

Compacts the WSL2 virtual disk (ext4.vhdx). WSL-only.

- Auto-detects the VHDX path for the current distro
- Reports VHDX file size and actual filesystem usage
- Runs `fstrim` inside WSL
- Enables sparse mode for automatic future reclamation
- `--compact` generates a `.bat` script that compacts the disk via export/re-import (no admin required)
- Also prints elevated Optimize-VHD / diskpart commands as an alternative

Flags:

| Flag | Description |
|------|-------------|
| `--compact` | Generate a `.bat` script that compacts the disk via export/re-import (no admin required) |
| `--dangerous` | With `--compact`: skip the backup step and compact in-place (export, unregister, re-import). For when free disk space is too low for a full backup. Risks data loss if interrupted. |
| `--vhdx-path PATH` | Use a specific VHDX file instead of auto-detecting (accepts WSL or Windows paths) |
| `--export-dir DIR` | Write temporary export files to `DIR` instead of `%TEMP%` (accepts WSL or Windows paths). Useful when the local disk is low but OneDrive or a network share has space. |

```
$ wsl-resize-disk
  … Resolving VHDX path...
  ✓ Detected VHDX: C:\Users\owen\AppData\Local\Packages\...\ext4.vhdx

── Disk usage ──
  ▸ VHDX file size: 25600 MB (25 GB)
  ▸ Filesystem usage: 12000 MB used of 20000 MB total
  ▸ Potential savings: ~13600 MB

── Trimming unused blocks ──
/: 1.2 GiB (1234567890 bytes) trimmed
  ✓ Trim complete

── Compaction options ──
  ▸ Option 1: Export/re-import (no admin required)
      wsl-resize-disk --compact
  ...
```

Use `--compact` to generate a `.bat` file that performs the compaction:

```
$ wsl-resize-disk --compact
  ...
  ✓ Generated: /path/to/dotfiles/powershell/wsl-compact.bat

  ▸ Run from Windows (no admin required):
      explorer.exe "\\wsl$\..."
      # Then double-click wsl-compact.bat
```

The generated script shuts down WSL, exports the distro, re-imports it to create a fresh compacted image, then swaps the VHDX file. No PowerShell modules or admin privileges required.

### wsl-restart

Restarts WSL from within WSL. WSL-only. Requires confirmation.

| Flag | Description |
|------|-------------|
| `--script` | Generate a PowerShell script (`wsl-restart.ps1`) that shuts down and restarts the distro from Windows, instead of shutting down from within WSL. No elevation required. |

```
$ wsl-restart
  ⚠ This will shut down WSL and terminate all sessions.
  ? Continue? [y/N] y
  … Shutting down WSL...
```

### clipboard-vm

Diagnoses and attempts to fix SPICE clipboard sharing in a QEMU/KVM guest. Walks the full chain: virtio channel → system daemon → per-session agent. Targets QEMU/KVM but will prompt to continue on other detected hypervisors.

- Detects virtualization (`systemd-detect-virt`, falling back to DMI vendor)
- Checks the SPICE virtio channel (`/dev/virtio-ports/com.redhat.spice.0`); prints host-side fix instructions (virt-manager / libvirt XML) if missing
- Installs `spice-vdagent` if absent (apt/apt-get/dnf/pacman)
- Enables and starts the `spice-vdagentd` system daemon and verifies its socket
- Starts the per-session `spice-vdagent`, with extra warnings for Wayland sessions (limited clipboard support) and headless/SSH sessions
- Reminds you to reconnect the SPICE viewer (the clipboard handshake only happens at viewer-connect time)
- Prints a compact, camera-friendly summary block at the end
- Always writes a full report to `~/clipboard-vm-report.txt` (the whole point: clipboard is broken, so you can `cat` it, scp it off, or photograph it later)

| Flag | Description |
|------|-------------|
| `--reset` | Kill the per-session agent, restart `spice-vdagentd`, and start a fresh per-session agent. Use when checks pass but clipboard still doesn't work. |

```
$ clipboard-vm

── clipboard-vm — 2026-05-25 12:00:00 ──

── VM detection ──
  ▸ Virtualization: kvm
...
── SUMMARY ──
  virt:           kvm
  channel:        present
  ...
  ▸ Full report: /home/owen/clipboard-vm-report.txt
```

### jwt-decode

Decodes a JWT and pretty-prints its header and payload, with `iat`, `nbf` and `exp` shown as local times and flagged when the token is expired or not yet valid. A leading `Bearer ` is ignored. The header is always printed, since it is readable even when the rest is not: for an encrypted (five-part) JWE the payload is reported as encrypted with its `enc`, and a detached (`header..sig`) or non-JSON payload is described rather than failing the decode. Pure perl core modules plus `openssl`, so it needs no jq.

Pass the token as an argument, or on stdin to keep it out of shell history:

```
$ wl-paste | jwt-decode
$ jwt-decode "$TOKEN"
```

By default the signature is shown but **not** checked: the decode tells you what a token claims, the verify tells you whether to trust it. `--verify` checks it and sets the exit code (0 valid, 1 invalid, 2 cannot verify). HS256/384/512 take `--secret` (or `--secret-file`); RS/PS/ES take `--key` with a PEM public key. `alg: none` is reported as unsigned (exit 2), and an unsupported alg says so rather than claiming a verdict.

```
$ jwt-decode --verify --secret hunter2 "$TOKEN"
$ jwt-decode --verify --key issuer-pub.pem "$TOKEN"
```

### jwt-encode

Builds and signs a JWT from a JSON payload (inline `--payload`, `--payload-file`, or stdin). `--alg` defaults to HS256 and also takes `none`, `RS*`, `PS*` and `ES*`; HS* sign with `--secret`, the rest with `--key` (an unencrypted PEM private key). `--iat`, `--exp SECONDS` and `--nbf SECONDS` fill the time claims relative to now, and `--kid`/`--typ` set header fields. The token is printed to stdout.

```
$ echo '{"sub":"owen","role":"admin"}' | jwt-encode --alg HS256 --secret hunter2 --exp 3600
$ jwt-encode --alg none --payload '{"sub":"admin"}'
$ jwt-encode --alg ES256 --key ec.key --payload "$CLAIMS"
```

It shares the base64url and ECDSA raw↔DER conversion with `jwt-decode` through `_jwt.pl`. It signs exactly what you give it and makes no claim that a server should accept the result, which is the point when testing whether one does.

### tlog-clean

Strips ANSI escape codes, powerline/nerd-font glyphs, and terminal noise from tmux-logging captures, producing clean, grep-friendly text. Unlike the other devtools, it does not source `_lib.sh`; it is a self-contained filter.

- Simulates a virtual terminal line buffer in Perl to correctly resolve cursor movements (CSI moves, backspace, carriage return) and in-line edits
- Detects powerlevel10k-style prompts and rewrites them as a simple `directory $ command` line
- Drops `PROMPT_EOL_MARK` lines and collapses runs of whitespace
- Reads from one or more `FILE` arguments, or from stdin when given no arguments or `-`

```
# From files
$ tlog-clean ~/.local/share/tmux/logs/tmux-*.log

# From stdin (pipe)
$ cat session.log | tlog-clean
$ tlog-clean session.log | grep -i error
```

### tmux-restore

Toggles tmux session auto-restore (tmux-continuum + tmux-resurrect). Auto-restore is ON by default: a new tmux server restores the last save within its first seconds, which is the only time continuum restores, so nothing running is overwritten. `off` creates a marker file that `tmux.conf` checks; the change applies the next time the tmux server starts. Manual restore is always available inside tmux via `prefix + Ctrl-r`. With auto-restore off, a fresh server pauses autosave (SAVE PAUSED) so it cannot replace the save nobody restored; see the shell section of [modules.md](modules.md).

Marker file: `~/.config/tmux/restore-off` (present = OFF, absent = ON). Any run also removes the obsolete opt-in marker `~/.config/tmux/restore-on`.

| Command | Description |
|---------|-------------|
| `on` | Enable auto-restore at the next tmux server start (removes the marker); the default |
| `off` | Disable auto-restore (creates the marker) |
| `toggle` | Flip whichever way it's currently set |
| `status` | Show current state, the marker path, and (if a server is running) the live `@continuum-restore` value, warning on mismatch. This is the default when no command is given. |

```
$ tmux-restore status
  ▸ Auto-restore: on
  ▸ Marker: /home/owen/.config/tmux/restore-off

$ tmux-restore off
  ✓ Auto-restore: OFF  (marker: /home/owen/.config/tmux/restore-off)
  ▸ Effect lands at the next tmux server start.
```

After flipping, reload a running session's config with `tmux source-file ~/.tmux.conf`, though `@continuum-restore` is only fully consulted at server start.

### tmux-copy-clean

Strips the powerlevel10k prompt from text copied in tmux copy mode, so a selection spanning several commands yields the commands and their output without the prompt glyphs or the right-side status. `tmux.conf` pipes both `y` and a mouse drag through it with `--load`, which replaces the raw copy with the cleaned one and sends it to the clipboard over OSC 52. Without `--load` it filters stdin to stdout. It keys on the prompt's powerline end symbols set in `config/shell/p10k.zsh`, so changing those means updating the script.

### claude-tmux

Brings Claude Code conversations back when tmux-resurrect restores a server. Resurrect saves each pane's command line, which never holds a session id, so it cannot tell which conversation a pane had. This keeps that mapping in four steps, none of which you run by hand:

| Command | Run by | Does |
|---------|--------|------|
| `record` | Claude Code's SessionStart hook (claude module) | Sets the `@claude-session` option on `$TMUX_PANE` from the hook's JSON. Fires on startup, resume, `/clear` and compaction, so it follows the pane's current conversation. Prints nothing, since SessionStart stdout is added to the conversation. |
| `end` | Claude Code's SessionEnd hook (claude module) | Clears the pane's tag and marks its map entry `exited`. |
| `save` | resurrect's post-save-all hook (`tmux.conf`) | Writes `session:window.pane`, session id and directory to `~/.local/state/claude-tmux/panes` for every tagged pane, and for every untagged pane running `claude` with an empty id. |
| `resume` | resurrect, in a restored pane whose saved command started with `claude` | `claude --resume <id>` for that pane's position, tagging the pane straight away so a save before SessionStart fires keeps it. Without an id it uses `claude --continue` only when the map shows no other Claude pane in that directory, since `--continue` opens the directory's latest conversation and two panes would both get it. Otherwise, and when the pane has no entry or its directory does not match, it opens the picker (`claude --resume`). An entry marked `exited` restores a plain shell with a one-line note naming the conversation. |

The id is stored on the pane and turned into a position at save time because `renumber-windows` moves windows during a session, while a restore preserves the saved positions exactly. `record` and `end` also update the pane's map entry at once. The restore list is the last save, which continuum writes every 15 minutes and never at shutdown, so without that a conversation exited after the last save came back on the next restore.

### opencode-tmux

The same for opencode, which has no session hooks. Instead it wraps the process: the `opencode` shell function (opencode module) runs `opencode-tmux run mini ...` inside tmux, which picks a session id up front (opencode 2 creates a session under an id given with `-s`), tags the pane with `@opencode-session`, runs opencode, and marks the map entry `exited` when it returns. `save` and `resume` work as in claude-tmux, with the map in `~/.local/state/opencode-tmux/panes`; `resume` runs `opencode mini -s <id>`, which replays the conversation into the fresh pane, or `-c` for an untagged pane alone in its directory, or a new session. The pane's process is this script rather than opencode, so `tmux.conf` matches it as `~opencode-tmux run`.

### claude-no-attribution

Claude Code's PreToolUse hook for Bash (claude module): refuses, before it runs, any command that runs `git` or `gh` with AI attribution in it, the same assistant `Co-authored-by` trailers, `Generated with` lines and session links the [git](#git) wrapper strips. Claude gets the refusal on stderr and is told to retry without that text. A human co-author, or a command that only searches for attribution, passes.

The wrappers only see the git and gh that resolve to them; this sees the command itself, so it also covers `/usr/bin/git`, a git found ahead of `~/.local/bin`, and a repository whose own `core.hooksPath` displaces the global one. The hook does nothing when the script is not installed, so the claude module works without devtools. The AI names come from `_scrub.sh`.

### winbuild

Dispatches a build to a Windows compilation server over SSH: rsyncs the project up, runs a configurable build command remotely, and rsyncs artifacts back. Installed by the [windev](modules.md#windev) module (not by devtools), but lives here so it picks up `_lib.sh` and the standard shellcheck scope. For end-to-end setup including SSH config and BUILD_CMD recipes, see the [Windows Cross-Development guide](windev.md#remote-windows-build-server).

On first run with no config, writes a template to `~/.config/dfinstall/winbuild.conf` and exits. Edit it (set `HOST` at minimum) and re-run.

| Flag | Description |
|------|-------------|
| `--host HOST` | Override `HOST` (SSH alias from `~/.ssh/config`, or `user@host`) |
| `--remote-base PATH` | Override `REMOTE_BASE` (e.g. `C:/builds`) |
| `--artifact-dir DIR` | Override `ARTIFACT_DIR`: the subdir on remote whose contents come back to `./<dir>/` |
| `--cmd "CMD"` | Override `BUILD_CMD` |
| `--dry-run` | Show the steps, run nothing |

Config file (`~/.config/dfinstall/winbuild.conf`):

```bash
HOST="winbuild"                                  # SSH alias
REMOTE_BASE="C:/builds"                          # OpenSSH-for-Windows accepts forward slashes
BUILD_CMD="msbuild /m /p:Configuration=Release"  # or cmake / dotnet publish / etc.
ARTIFACT_DIR="build-win"
```

The current project name is appended to `REMOTE_BASE` (`REMOTE_DIR="$REMOTE_BASE/$(basename "$PWD")"`). rsync excludes `.git`, `node_modules`, `target`, `bin`, `obj`, and the artifact dir on upload.

```
$ winbuild --dry-run

── winbuild → winbuild ──

  ▸ project:    myapp
  ▸ remote dir: C:/builds/myapp
  ▸ build cmd:  msbuild /m /p:Configuration=Release
  ▸ artifacts:  C:/builds/myapp/build-win/  →  ./build-win/
  ▸ Uploading source...
  …  would: rsync -az --delete ...
  ▸ Building remotely...
  …  would: ssh winbuild cd "C:/builds/myapp" && msbuild ...
  ▸ Downloading artifacts...
  …  would: rsync -az winbuild:C:/builds/myapp/build-win/ ./build-win/
  ✓ Done. Artifacts in ./build-win/
```

---

### `demorec`

Records the screen to mp4 and re-encodes it small. Unlike `asciinema` (the
`record` alias), it captures **real pixels**, so it picks up anything that never
enters the escape stream: Ghostty `custom-shader` cursor trails above all.
Use `record` when the content is text; use `demorec` when the point is how it looks.

```bash
demorec outputs                         # list displays
demorec start --output eDP-1            # one display
demorec start --area 100,100,1200,800   # or an arbitrary region
demorec status
demorec stop --render                   # -> demo-small.mp4, raw discarded
demorec stop                            # keep the pristine capture instead
demorec render demo.mp4                 # -> demo-small.mp4
```

`--output` and `--area` are mutually exclusive. How the display is named and
selected differs per backend, which is why `outputs` exists:

| Backend | ID | Selection | Listed via |
|---|---|---|---|
| wlroots | connector (`DP-2`) | `wf-recorder -o` | `swaymsg`/`hyprctl`/`wlr-randr` |
| GNOME | connector (`eDP-1`) | resolved to a rect, then `ScreencastArea` | Mutter `DisplayConfig` |
| WSL | numeric index (`0`) | `ddagrab output_idx` | PowerShell `Screen::AllScreens` |

GNOME's Screencast has no display selector at all, so `--output` there looks the
monitor's rect up from Mutter (in logical pixels, the same space
`ScreencastArea` uses) and captures that region. On WSL the index is a DXGI
output index; Windows usually enumerates screens in the same order, but that is
**not guaranteed**. If the wrong screen is captured, try the other index rather
than trusting the listing's order.

### Which terminal you use does not matter

Capture is desktop-level, not application-level: `ddagrab` grabs the composited
Windows desktop, `wf-recorder` and Screencast grab compositor output. So Windows
Terminal, Ghostty under WSLg, or anything else are all just windows that appear
in the frame; nothing in demorec is Ghostty-specific.

What *does* differ is whether there is a cursor trail to capture in the first
place. The trail is a Ghostty `custom-shader` (GLSL, fed cursor state by
Ghostty). Windows Terminal has its own unrelated shader hook,
`experimental.pixelShaderPath`, which is HLSL and is **not** given cursor
position, so a cursor trail is not expressible in it. Recording Windows
Terminal works fine; there is simply no trail in the result.

The raw capture is large: GNOME's encoder is pinned at fixed QP 26 with
`complexity=low` and `deblocking=off`, which on a 1920x1200 screen is about
11.6 Mbit/s (~1.4 MB/s). Measured on a real 9.7s clip:

| | Size |
|---|---|
| raw capture | 13.5 MiB |
| after `render` (crf 22) | 139 KiB |

That is a 99% reduction at 0.9997 SSIM: terminal content is trivially
compressible, and even crf 30 measures 0.9988, so the crf dial barely matters.
Because the raw is disposable once encoded, `stop --render` does both steps and
deletes it; `--keep-raw` retains it, and plain `stop` skips encoding entirely
(which is what you want when judging the shader itself, since `render` is the
size pass, not the fidelity pass). A failed render never deletes the raw.

**Capture is platform-specific; rendering is not.**

| Session | Backend | Stop signal |
|---|---|---|
| wlroots (sway, Hyprland, Wayfire, river) | `wf-recorder` via `wlr-screencopy` | `SIGINT` |
| GNOME/Wayland | `org.gnome.Shell.Screencast` over D-Bus | `SIGTERM` |
| WSL | `ffmpeg.exe` via interop, `ddagrab` source filter | `SIGTERM` |

The wlroots path is the simplest of the three and produces the smallest raw
capture: wf-recorder's default damage tracking asks the compositor for a frame
only when the screen actually changes, so the file is already variable-rate
before `render` touches it. Two flags matter and are easy to get backwards:
`-r` forces a *constant* framerate by duplicating frames, which throws that
saving away, so demorec passes `-B` (the framerate hint that preserves VFR)
instead. wf-recorder also has **no cursor option at all**, so `--no-cursor` is
warned about rather than honored there, and it finalizes on **SIGINT** while
ignoring SIGTERM, hence the per-backend stop signal recorded in the session
file.

Detection order is WSL, then wlroots, then GNOME. wlroots is checked before
GNOME so a sway session cannot fall through to the D-Bus path, which would fail
confusingly. Mutter is not wlroots and does not implement `wlr-screencopy`, so
wf-recorder cannot work under GNOME: it exits immediately with `compositor
doesn't support wlr-screencopy-unstable-v1`.

ffmpeg cannot capture on Wayland itself: its `pipewiregrab` is an unmerged RFC,
absent from every stable release. And nothing *inside* WSL can capture at all:
WSLg is Weston with a RAIL shell, so there is no `wlr-screencopy`, no portal, and
no desktop output to grab. The window is composited by DWM on the Windows side,
which is the only place it can be seen. `render` is a plain file-to-file
transcode, so Linux ffmpeg handles it on both platforms.

Two behaviors that are easy to get wrong and are deliberate here:

- **GNOME kills a screencast the instant the calling D-Bus connection drops**
  (`Fatal error while recording: Sender has vanished`). A one-shot `gdbus call`
  returns success and then records nothing, leaving a 48-byte file holding only
  an `ftyp` box. `start` therefore leaves a small Python holder running for the
  duration, and `stop` signals it.
- **WSL capture takes seconds to actually begin.** Launching a Windows process
  through interop, initializing D3D11 and acquiring the Desktop Duplication
  interface all cost real time, and a freshly downloaded `ffmpeg.exe` may also
  be scanned by Defender on first run. `start` therefore waits until ffmpeg has
  genuinely written to the output before reporting `Recording`, and prints how
  long that took, so the message means capture is live rather than merely
  requested. If it is consistently slow, a Defender exclusion for the binary is
  the usual culprit; setting `DEMOREC_DIR` also skips the Windows profile
  lookup, which is otherwise cached after the first run.
- **The clip ends where the demo did, not where the recorder exited.** Stopping
  is not instant (on WSL it takes seconds), and all of it was being recorded,
  along with the stop command being typed. `stop` therefore notes where in the
  clip it was invoked *before* killing anything, and `render` cuts there.
  `--tail N` drops N seconds more, for the typing itself. The cut is clamped so
  a short recording cannot be trimmed away entirely.
- **`stop` avoids PowerShell.** Locating the Windows process by command line
  needs a CIM query, and PowerShell startup dominated the time `stop` took. The
  PID is now identified at start instead, by diffing the running `ffmpeg.exe`
  list either side of the launch; `tasklist` costs a fraction of a PowerShell
  start. If that diff is ambiguous (another ffmpeg started at the same moment)
  the PID is left unset and `stop` falls back to the CIM query.
- **Signals do not cross the WSL interop boundary.** Killing the Linux-side
  process leaves `ffmpeg.exe` running as an orphaned Windows process, and
  `kill -0` on the now-dead shim looks exactly like a clean exit, so `stop`
  would report success while the recording continued and the file stayed locked.
  On WSL, `stop` therefore asks Windows directly: it finds the `ffmpeg.exe`
  whose command line contains the output filename (so an unrelated ffmpeg is
  never touched) and runs `taskkill /PID … /T /F`, then re-queries to confirm.
  A stop that cannot confirm the process is gone **keeps the session file** so
  it can be retried, rather than stranding a live recording with no state.
- **The WSL muxer is fragmented** (`+frag_keyframe+empty_moov`) so a hard kill
  still yields a playable file instead of one with no `moov` atom. GNOME's own
  pipeline does the same (`fragment-mode=first-moov-then-finalize`), so both
  backends behave alike on an unclean stop. ffmpeg is therefore run with
  `-nostdin` and simply signalled, rather than being sent `q` down a FIFO: a
  FIFO would have to be held open by a shell that exits the moment `start`
  returns, so ffmpeg would hit EOF on stdin at an unpredictable point. The
  fragmented muxer already makes the graceful path unnecessary.

`render` drops duplicate frames (`mpdecimate`) at variable frame rate, keeping
the retained frames at their real timestamps while collapsing the long static
stretches a terminal spends most of its time in. It never rescales: resampling
text is what makes screencasts look mushy.

Trailing duplicates are dropped too, so a clip **ends at the last on-screen
change**, not when recording stopped. That discards dead air, but it also means
a demo ending on its result has that result flash past. `--hold N` keeps the
final frame up for N seconds. It is not simply `tpad`: that pads at end of
stream and would drag all the trailing dead air back in as a held frame, so
`render` first probes for the last change (a decode-only pass) and limits the
input to that point. Measured on an 11.4s capture whose last change was at 4.7s:

| | Duration |
|---|---|
| default | 4.5s: ends at the last change |
| `--hold 2` | 6.7s: content, then a 2s hold |
| `--no-decimate --hold 2` | 13.4s: nothing was truncated, so nothing is compensated |

**A blinking cursor is the single biggest cost.** It changes pixels twice a
second forever, so almost nothing dedupes. Measured on a 10s idle 1200x800 clip:

| | Frames kept (of 300) | Rendered size |
|---|---|---|
| No blink | 1 | 4.6 KB |
| 2 Hz blink | 40 | 10.9 KB |

Setting `cursor-style-blink = false` in the Ghostty config while recording is
worth more than any encoder flag. It does not affect the cursor trail, which
fires on movement rather than blink.

**WSL prerequisites.** Only two, and one is optional:

| Command | Needed for | Source |
|---|---|---|
| `ffmpeg.exe` | capture, and `render` if there is no Linux ffmpeg | supplied by you (see below) |
| `ffmpeg` (Linux) | optional; `render` prefers it when present | `apt install ffmpeg` |
| `powershell.exe`, `tasklist.exe`, `taskkill.exe` | `outputs` and `stop` | built into Windows |
| `wslpath` | path translation | built into WSL |

`ffmpeg.exe` is the only thing you have to provide. Rendering is a file-to-file
transcode, so it falls back to the Windows binary (translating both paths with
`wslpath`) when no Linux ffmpeg is installed; the native one is preferred when
present because it takes Linux paths directly and skips the interop hop.

Nothing needs installing on Windows: a standalone build works, and
`DEMOREC_FFMPEG` points at it wherever you keep it, with no Windows PATH change
required. If the location is already on the Windows PATH, interop resolves plain
`ffmpeg.exe` and the variable is unnecessary. The static BtbN builds are a single
self-contained `ffmpeg.exe`, and `ddagrab` is compiled in (verified against
`ffmpeg-n7.1-latest-win64-gpl`):

`wsl-ffmpeg` does this for you:

```bash
wsl-ffmpeg                       # asks where, defaulting to your Windows profile's bin/
wsl-ffmpeg --dir /mnt/c/tools    # or say outright; Windows paths accepted too
```

It resolves the Windows profile itself (`$env:USERPROFILE` through PowerShell,
then `wslpath`), so the account name never has to be typed, and it works in
Linux-style `/mnt/...` paths throughout. It picks the latest **static** BtbN
build, extracts just `ffmpeg.exe`, and then checks the binary runs and that
`ddagrab` is compiled in: a build without it cannot capture, and finding that
out now beats finding out mid-recording. It warns if the target is on the Linux
filesystem, is a no-op if already installed (`--force` to replace), and prints
the `DEMOREC_FFMPEG` line unless the location is already on the Windows PATH.

By hand instead:

```bash
curl -L -o ffmpeg.zip \
  https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-win64-gpl-7.1.zip
unzip -j ffmpeg.zip '*/bin/ffmpeg.exe'    # -j flattens; the zip has a version prefix
export DEMOREC_FFMPEG=/mnt/c/Users/<you>/bin/ffmpeg.exe   # wherever you put it
```

Keep it on the Windows filesystem rather than the Linux one: a Windows binary
run from `\\wsl.localhost\...` goes over the 9p bridge. The same applies more
sharply to the capture output, which is why `demorec` defaults `DEMOREC_DIR` on
WSL to `Videos/demos` under the **Windows** user profile (resolved via
`$env:USERPROFILE` and `wslpath`) rather than `$HOME`. An ~11 Mbit/s stream
written across that bridge is slow enough to drop frames or fail outright. If
the profile cannot be resolved, it warns and falls back to `$HOME`.

> The WSL backend is **untested**: written from the `ddagrab` docs and interop
> behavior, but never exercised on a real WSL box. Verify before relying on it.
>
> The wlroots backend has **not been run in a real wlroots session** either, but
> less of it is guesswork: the argument vector was executed against the real
> wf-recorder binary (0.5.0), which accepted every flag and the geometry string
> and failed only at `wlr-screencopy`, and the start-failure path, backend
> detection, missing-binary message and cursor warning were all driven on GNOME.
> What remains unverified is a successful recording and the SIGINT finalize.

---

## ghostty-shader

Enables or disables Ghostty's cursor-trail shader **for the current machine
only**, without touching the shared `config/ghostty/config`.

```
ghostty-shader status    # current setting + detected GL renderer
ghostty-shader on        # force the trail on
ghostty-shader off       # force the trail off
ghostty-shader auto      # decide from the detected renderer
ghostty-shader reset     # drop the override, inherit the shared config
```

It writes `~/.config/ghostty/ghostty.local`, which the main config pulls in as
its last directive with `config-file = ?ghostty.local`. Includes load *after*
the file that declares them, so the override wins; the `?` makes it optional, so
`reset` (deleting the file) cleanly reverts.

**Why this exists.** With no GPU behind it the trail is ruinously expensive.
Measured on Ghostty 1.2.0, idle window, nothing typed, 10s samples, comparing
hardware GL against `LIBGL_ALWAYS_SOFTWARE=1`:

| config | renderer | idle CPU |
|---|---|---|
| no shader | software | 0.0% |
| no shader | GPU | 0.5% |
| shader, `animation = always` | GPU | 15.6% |
| shader, `animation = false` | software | 66.9% |
| shader, `animation = always` | software | 762-790% |
| shader, `animation = true` (focused) | software | 806% |

Software rasterization is common under WSLg when Mesa's d3d12 backend does not
bind (microsoft/wslg#1129, #996, #1470).

Two traps in those numbers:

- **Turning only the animation off is not a fix.** It still costs ~67% of a
  core, because `cursor-style-blink = true` forces a redraw twice a second and
  every redraw is a full-screen shader pass. The only effective lever is
  removing the shader.
- **`always` vs `true` barely differ.** A focused window pays the same either
  way, so do not "optimize" this by switching animation modes.

Renderer detection prefers `glxinfo`/`eglinfo` and says so when it has them.
Without mesa-utils it falls back to the presence of `/dev/dri/renderD128`, whose
absence is the documented signature of WSLg failing to bring up d3d12. That is
strong evidence of software rendering, but its presence is not proof of the
reverse, and the output labels the guess as a guess.

`status` and every mutating action re-read `ghostty +show-config`, so what is
reported is what Ghostty actually resolved rather than what was written. A full
Ghostty restart is required: a config reload rebuilds the font grid but does
not recompute cell size or icon constraints, and does not restore a default.

---

## Script Conventions

All devtools scripts follow these rules (from CLAUDE.md):

- `set -euo pipefail` at the top
- `-h`/`--help` support
- Confirmation prompts before destructive operations
- Guard clauses (WSL check, command existence, git repo check)
- Portable across WSL distros -- no hardcoded distro-specific paths
- Use `$WSL_DISTRO_NAME`, `cmd.exe`, and `wslpath` for Windows interop
