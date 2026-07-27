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
- Always writes a full report to `~/clipboard-vm-report.txt` (the whole point — clipboard is broken, so you can `cat` it, scp it off, or photograph it later)

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

### tlog-clean

Strips ANSI escape codes, powerline/nerd-font glyphs, and terminal noise from tmux-logging captures, producing clean, grep-friendly text. Unlike the other devtools, it does not source `_lib.sh` — it is a self-contained filter.

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

Toggles tmux session auto-restore (tmux-continuum + tmux-resurrect). Auto-restore is OFF by default in this config — when ON, every tmux server start replays previously-captured pane contents over the current panes, clobbering in-progress output. The script flips a marker file that `tmux.conf` checks; the change applies the next time the tmux server starts. Manual restore is always available inside tmux via `prefix + Ctrl-r`.

Marker file: `~/.config/tmux/restore-on` (present = ON, absent = OFF).

| Command | Description |
|---------|-------------|
| `on` | Enable auto-restore at the next tmux server start (creates the marker) |
| `off` | Disable auto-restore (removes the marker) — the default |
| `toggle` | Flip whichever way it's currently set |
| `status` | Show current state, the marker path, and (if a server is running) the live `@continuum-restore` value, warning on mismatch. This is the default when no command is given. |

```
$ tmux-restore status
  ▸ Auto-restore: off
  ▸ Marker: /home/owen/.config/tmux/restore-on

$ tmux-restore on
  ✓ Auto-restore: ON  (marker: /home/owen/.config/tmux/restore-on)
  ▸ Effect lands at the next tmux server start.
```

After flipping, reload a running session's config with `tmux source-file ~/.tmux.conf`, though `@continuum-restore` is only fully consulted at server start.

### winbuild

Dispatches a build to a Windows compilation server over SSH: rsyncs the project up, runs a configurable build command remotely, and rsyncs artifacts back. Installed by the [windev](modules.md#windev) module (not by devtools), but lives here so it picks up `_lib.sh` and the standard shellcheck scope. For end-to-end setup including SSH config and BUILD_CMD recipes, see the [Windows Cross-Development guide](windev.md#remote-windows-build-server).

On first run with no config, writes a template to `~/.config/dfinstall/winbuild.conf` and exits. Edit it (set `HOST` at minimum) and re-run.

| Flag | Description |
|------|-------------|
| `--host HOST` | Override `HOST` (SSH alias from `~/.ssh/config`, or `user@host`) |
| `--remote-base PATH` | Override `REMOTE_BASE` (e.g. `C:/builds`) |
| `--artifact-dir DIR` | Override `ARTIFACT_DIR` — the subdir on remote whose contents come back to `./<dir>/` |
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
enters the escape stream — Ghostty `custom-shader` cursor trails above all.
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
**not guaranteed** — if the wrong screen is captured, try the other index rather
than trusting the listing's order.

### Which terminal you use does not matter

Capture is desktop-level, not application-level: `ddagrab` grabs the composited
Windows desktop, `wf-recorder` and Screencast grab compositor output. So Windows
Terminal, Ghostty under WSLg, or anything else are all just windows that appear
in the frame — nothing in demorec is Ghostty-specific.

What *does* differ is whether there is a cursor trail to capture in the first
place. The trail is a Ghostty `custom-shader` (GLSL, fed cursor state by
Ghostty). Windows Terminal has its own unrelated shader hook,
`experimental.pixelShaderPath`, which is HLSL and is **not** given cursor
position — so a cursor trail is not expressible in it. Recording Windows
Terminal works fine; there is simply no trail in the result.

The raw capture is large — GNOME's encoder is pinned at fixed QP 26 with
`complexity=low` and `deblocking=off`, which on a 1920x1200 screen is about
11.6 Mbit/s (~1.4 MB/s). Measured on a real 9.7s clip:

| | Size |
|---|---|
| raw capture | 13.5 MiB |
| after `render` (crf 22) | 139 KiB |

That is a 99% reduction at 0.9997 SSIM — terminal content is trivially
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
before `render` touches it. Two flags matter and are easy to get backwards —
`-r` forces a *constant* framerate by duplicating frames, which throws that
saving away, so demorec passes `-B` (the framerate hint that preserves VFR)
instead. wf-recorder also has **no cursor option at all**, so `--no-cursor` is
warned about rather than honoured there, and it finalises on **SIGINT** while
ignoring SIGTERM — hence the per-backend stop signal recorded in the session
file.

Detection order is WSL, then wlroots, then GNOME. wlroots is checked before
GNOME so a sway session cannot fall through to the D-Bus path, which would fail
confusingly. Mutter is not wlroots and does not implement `wlr-screencopy`, so
wf-recorder cannot work under GNOME — it exits immediately with `compositor
doesn't support wlr-screencopy-unstable-v1`.

ffmpeg cannot capture on Wayland itself — its `pipewiregrab` is an unmerged RFC,
absent from every stable release. And nothing *inside* WSL can capture at all:
WSLg is Weston with a RAIL shell, so there is no `wlr-screencopy`, no portal, and
no desktop output to grab. The window is composited by DWM on the Windows side,
which is the only place it can be seen. `render` is a plain file-to-file
transcode, so Linux ffmpeg handles it on both platforms.

Two behaviours that are easy to get wrong and are deliberate here:

- **GNOME kills a screencast the instant the calling D-Bus connection drops**
  (`Fatal error while recording: Sender has vanished`). A one-shot `gdbus call`
  returns success and then records nothing, leaving a 48-byte file holding only
  an `ftyp` box. `start` therefore leaves a small Python holder running for the
  duration, and `stop` signals it.
- **WSL capture takes seconds to actually begin.** Launching a Windows process
  through interop, initialising D3D11 and acquiring the Desktop Duplication
  interface all cost real time, and a freshly downloaded `ffmpeg.exe` may also
  be scanned by Defender on first run. `start` therefore waits until ffmpeg has
  genuinely written to the output before reporting `Recording`, and prints how
  long that took — so the message means capture is live rather than merely
  requested. If it is consistently slow, a Defender exclusion for the binary is
  the usual culprit; setting `DEMOREC_DIR` also skips the Windows profile
  lookup, which is otherwise cached after the first run.
- **Signals do not cross the WSL interop boundary.** Killing the Linux-side
  process leaves `ffmpeg.exe` running as an orphaned Windows process, and
  `kill -0` on the now-dead shim looks exactly like a clean exit — so `stop`
  would report success while the recording continued and the file stayed locked.
  On WSL, `stop` therefore asks Windows directly: it finds the `ffmpeg.exe`
  whose command line contains the output filename (so an unrelated ffmpeg is
  never touched) and runs `taskkill /PID … /T /F`, then re-queries to confirm.
  A stop that cannot confirm the process is gone **keeps the session file** so
  it can be retried, rather than stranding a live recording with no state.
- **The WSL muxer is fragmented** (`+frag_keyframe+empty_moov`) so a hard kill
  still yields a playable file instead of one with no `moov` atom. GNOME's own
  pipeline does the same (`fragment-mode=first-moov-then-finalise`), so both
  backends behave alike on an unclean stop. ffmpeg is therefore run with
  `-nostdin` and simply signalled, rather than being sent `q` down a FIFO: a
  FIFO would have to be held open by a shell that exits the moment `start`
  returns, so ffmpeg would hit EOF on stdin at an unpredictable point. The
  fragmented muxer already makes the graceful path unnecessary.

`render` drops duplicate frames (`mpdecimate`) at variable frame rate, which
keeps real timing while collapsing the long static stretches a terminal spends
most of its time in. It never rescales — resampling text is what makes
screencasts look mushy.

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
`DEMOREC_FFMPEG` points at it wherever you keep it — no Windows PATH change
required. If the location is already on the Windows PATH, interop resolves plain
`ffmpeg.exe` and the variable is unnecessary. The static BtbN builds are a single
self-contained `ffmpeg.exe`, and `ddagrab` is compiled in (verified against
`ffmpeg-n7.1-latest-win64-gpl`):

```bash
curl -L -o ffmpeg.zip \
  https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-win64-gpl-7.1.zip
unzip -j ffmpeg.zip '*/bin/ffmpeg.exe'    # -j flattens; the zip has a version prefix
export DEMOREC_FFMPEG=/mnt/c/Users/<you>/bin/ffmpeg.exe   # wherever you put it
```

Keep it on the Windows filesystem rather than the Linux one — a Windows binary
run from `\\wsl.localhost\...` goes over the 9p bridge. The same applies more
sharply to the capture output, which is why `demorec` defaults `DEMOREC_DIR` on
WSL to `Videos/demos` under the **Windows** user profile (resolved via
`$env:USERPROFILE` and `wslpath`) rather than `$HOME`. An ~11 Mbit/s stream
written across that bridge is slow enough to drop frames or fail outright. If
the profile cannot be resolved, it warns and falls back to `$HOME`.

> The WSL backend is **untested** — written from the `ddagrab` docs and interop
> behaviour, but never exercised on a real WSL box. Verify before relying on it.
>
> The wlroots backend has **not been run in a real wlroots session** either, but
> less of it is guesswork: the argument vector was executed against the real
> wf-recorder binary (0.5.0), which accepted every flag and the geometry string
> and failed only at `wlr-screencopy`, and the start-failure path, backend
> detection, missing-binary message and cursor warning were all driven on GNOME.
> What remains unverified is a successful recording and the SIGINT finalise.

---

## Script Conventions

All devtools scripts follow these rules (from CLAUDE.md):

- `set -euo pipefail` at the top
- `-h`/`--help` support
- Confirmation prompts before destructive operations
- Guard clauses (WSL check, command existence, git repo check)
- Portable across WSL distros -- no hardcoded distro-specific paths
- Use `$WSL_DISTRO_NAME`, `cmd.exe`, and `wslpath` for Windows interop
