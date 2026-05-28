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

## Script Conventions

All devtools scripts follow these rules (from CLAUDE.md):

- `set -euo pipefail` at the top
- `-h`/`--help` support
- Confirmation prompts before destructive operations
- Guard clauses (WSL check, command existence, git repo check)
- Portable across WSL distros -- no hardcoded distro-specific paths
- Use `$WSL_DISTRO_NAME`, `cmd.exe`, and `wslpath` for Windows interop
