# Devtools Scripts

Utility scripts installed to `~/.local/bin/` by the devtools module. All scripts use colored output with unicode symbols and include `-h`/`--help` support.

## Shared Helpers (`_lib.sh`)

Every script sources `_lib.sh` for consistent output and common guards:

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
- Runs `fstrim` inside WSL
- Enables sparse mode for automatic future reclamation
- Suggests [wslcompact](https://github.com/okibcn/wslcompact) for non-elevated compaction
- Prints PowerShell commands for the Windows-side compaction step (elevated)

```
$ wsl-resize-disk
  … Resolving VHDX path...
  ✓ Detected VHDX: C:\Users\owen\AppData\Local\Packages\...\ext4.vhdx

── Trimming unused blocks ──
/: 1.2 GiB (1234567890 bytes) trimmed
  ✓ Trim complete

── Next: compact from elevated PowerShell ──
  ▸ 1. Shut down WSL:
      wsl --shutdown

  ▸ 2. Run one of the following:
   ...
```

### wsl-restart

Restarts WSL from within WSL. WSL-only. Requires confirmation.

```
$ wsl-restart
  ⚠ This will shut down WSL and terminate all sessions.
  ? Continue? [y/N] y
  … Shutting down WSL...
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
