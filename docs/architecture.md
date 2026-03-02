# Architecture

This document covers the core systems that make up `dfinstall`.

## Overview

```
dfinstall install all [--backup]
        |
        v
  LoadConfig()             <- core/config.go (.config.yaml)
        |
        v
  RegisterAllModules()     <- modules/register.go (sets order)
        |
        v
  DetectEnvironment()      <- core/env.go (WSL? Git Bash?)
        |
        v
  shouldBackup()           <- first run / --backup / config
        |
        v
  [StartBackup()]          <- core/backup.go (if backup needed)
        |
        v
  for each module:
    module.Install()       <- modules/<name>.go
      -> core.LinkFile()   <- core/link.go (symlink with backup)
        -> BackupFile()    <- core/backup.go (records pre-install state)
      -> core.Info/Ok()    <- core/output.go (respects log level)
        |
        v
  [FinishBackup()]         <- core/backup.go (writes manifest)
        |
        v
  [SaveConfig()]           <- core/config.go (first run only)
        |
        v
  spinner / summary        <- core/spinner.go
```

## Module System

### Interface

Every module implements three methods (defined in `core/module.go`):

```go
type Module interface {
    Name() string           // identifier used in CLI ("shell", "nvim", etc.)
    Install() error         // perform installation
    Status() ModuleStatus   // report current state
}

type ModuleStatus struct {
    Name    string
    Linked  int      // items successfully in place
    Missing int      // items not yet linked/installed
    Extra   string   // freeform info
}
```

### Registry

Modules are registered in `modules/register.go` via `core.RegisterModule()`. **Order matters** -- earlier modules are installed first, so dependencies (packages, fonts, omz) come before things that need them (shell, nvim).

Lookup functions:

| Function | Returns |
|----------|---------|
| `core.AllModules()` | ordered slice of all modules |
| `core.GetModule(name)` | single module by name |
| `core.ModuleNames()` | string slice of names |

### Data-Driven Pattern

Most modules follow the same structure -- a slice of `{src, dst}` pairs looped in both `Install()` and `Status()`:

```go
var shellLinks = []struct{ src, dst string }{
    {"shell/zshrc", ".zshrc"},
    {"shell/aliases", ".aliases"},
    // ...
}

func (ShellModule) Install() error {
    for _, l := range shellLinks {
        core.LinkFile(core.ConfigPath(l.src), core.HomeTarget(l.dst))
    }
}
```

This keeps modules declarative and easy to extend.

## CLI

Built with [Cobra](https://github.com/spf13/cobra). Seven commands:

| Command | Description |
|---------|-------------|
| `install <module\|all>` | Install one or all modules |
| `update <module\|all>` | Alias for install — re-apply modules |
| `status` | Print table of link counts per module |
| `doctor` | Run 25+ health checks |
| `restore [timestamp]` | Restore files from a backup snapshot |
| `root` | Symlink configs into `/root/` via sudo |

### Install / Update Flags

| Flag | Behavior |
|------|----------|
| `--backup` | Force a backup snapshot regardless of config |
| `--extended` | Show interactive menu to select extended OMZ plugins |

### Restore Flags

| Flag | Behavior |
|------|----------|
| `--list` | List available backups (timestamp + entry count) |

### Global Flags

| Flag | Level | Behavior |
|------|-------|----------|
| *(none)* | `LogQuiet` | Animated spinner, suppressed detail |
| `-v` / `--verbose` | `LogVerbose` | Full `[info]` `[ok]` `[warn]` `[err]` output |
| `--debug` | `LogDebug` | Verbose + `[debug]` messages |

Flags are persistent (apply to all subcommands). The level is set in `PersistentPreRun` and stored in `core.Level`.

## Output System

Defined in `core/output.go`. Five log functions, each with a colored prefix:

| Function | Color | Quiet mode | Verbose+ |
|----------|-------|------------|----------|
| `Info()` | blue | suppressed | printed |
| `Ok()` | green | suppressed | printed |
| `Status()` | green | **always printed** | always printed |
| `Warn()` | yellow | buffered | printed immediately |
| `Err()` | red | **always printed** | always printed |
| `Debug()` | magenta | suppressed | suppressed (debug only) |

`Status()` is for direct user-facing feedback after interactive prompts (e.g. the extended plugin menu). It prints with a green checkmark regardless of log level.

In quiet mode, warnings are buffered and flushed after the spinner stops via `FlushWarnings()`. Errors always print and will clear the spinner line first (using an atomic `spinnerRunning` flag for thread safety).

### Spinner

`core/spinner.go` provides an animated braille-dot progress indicator:

```
  ⠹ Installing nvim (9/14)
```

- 10-frame animation at 80ms intervals
- Thread-safe text updates via mutex
- `Pause()` / `Resume()` — temporarily suspend the spinner for interactive prompts (e.g. sudo password)
- `PauseSpinner()` / `ResumeSpinner()` — package-level helpers that safely pause/resume the active spinner (no-op if none running)
- `PrintResult(total, failed)` renders the final line (`✓ Done` or `⚠ Done with errors`)
- `PrintHint(msg)` renders a dimmed follow-up message

The spinner runs in a background goroutine and is only used when `core.Level == LogQuiet`. Modules that invoke commands requiring terminal access (sudo, chsh) call `PauseSpinner()` before and `ResumeSpinner()` after to ensure prompts are visible.

## Linking

`core/link.go` handles all symlink operations:

### LinkFile(src, dst)

1. Record pre-install state via `BackupFile(dst)` (no-op if no backup session)
2. Create parent directories if missing
3. If `dst` is already a correct symlink -- no-op
4. If `dst` is a wrong symlink -- repoint it
5. If `dst` is a regular file -- back it up to `dst.bak`
6. Create the symlink

Every operation is idempotent. Running `dfinstall install all` twice produces the same result.

### Path Helpers

| Helper | Resolves to |
|--------|-------------|
| `ConfigPath("shell/zshrc")` | `<dotfiles>/config/shell/zshrc` |
| `HomeTarget(".zshrc")` | `$HOME/.zshrc` |
| `XDGTarget("nvim")` | `$XDG_CONFIG_HOME/nvim` (or `~/.config/nvim`) |

## Environment Detection

`core/env.go` runs once at startup via `DetectEnvironment()`:

| Check | Method |
|-------|--------|
| WSL | `/proc/version` contains "microsoft" |
| Git Bash | `$MSYSTEM` or `$MINGW_PREFIX` set |

Git Bash is **rejected** with an error pointing the user to WSL. WSL is detected and logged, enabling WSL-only modules (like wsl.conf setup).

### Dotfiles Directory Resolution

Checked in order:

1. `$DOTFILES` environment variable
2. Build-time path (baked via `-ldflags` in the Makefile)
3. Walk up from executable looking for `go.mod`
4. Current working directory (fallback)

## File Hashing

`core/hash.go` provides SHA-256 file hashing used by the `fonts` and `wsl` modules to detect whether installed files match the source. This avoids unnecessary writes and enables status checks without reading full file contents.

## Configuration

`core/config.go` manages a YAML config file at `<dotfiles>/.config.yaml`. Loaded in `PersistentPreRun` before any command runs.

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skip_backup` | bool | `false` | Skip automatic backups on install |
| `backup_dir` | string | *(empty)* | Custom backup directory (falls back to `~/.local/share/dfinstall/backups/`) |
| `extended_plugins` | []string | *(empty)* | Extended OMZ plugins selected via `--extended` |

### Auto-Backup Logic

Three states drive backup behavior on `install`:

| Condition | Backup? | Then |
|-----------|---------|------|
| `--backup` flag | Yes | nothing extra |
| No config file (first run) | Yes | save config with `skip_backup: true` |
| Config exists, `skip_backup: false` | Yes | respect user preference |
| Config exists, `skip_backup: true` | No | -- |

The key distinction is `CfgFileExists` — whether the config file was present at load time. This separates "first run, no config" from "user explicitly set `skip_backup: false`". On first run, after the auto-backup, the config is saved with `skip_backup: true` so subsequent runs skip by default. An existing config's `skip_backup` value is never overwritten.

### Functions

| Function | Purpose |
|----------|---------|
| `LoadConfig()` | Read and parse `.config.yaml`, set `CfgFileExists` |
| `SaveConfig()` | Write `Cfg` to `.config.yaml` with comment header |
| `ConfigFilePath()` | Return full path to the config file |

## Backup & Restore

`core/backup.go` provides a structured backup system that can snapshot target files before dfinstall modifies them.

### Storage Layout

```
~/.local/share/dfinstall/backups/<timestamp>/
  manifest.json
  files/
    home--owen--.zshrc          # flattened path (/ -> --)
    home--owen--.gitconfig
```

### Session Lifecycle

1. `StartBackup()` -- creates a timestamped directory and initializes the session
2. `BackupFile(dst)` -- called from `LinkFile` for each target path. Records the pre-install state:
   - **missing** -- path didn't exist (restore will delete whatever dfinstall places)
   - **symlink** -- records the original target (restore recreates it)
   - **file** -- copies to backup dir with SHA-256 hash (restore copies it back)
3. `FinishBackup()` -- writes `manifest.json`, cleans up if no entries were recorded

`BackupFile` is a no-op when no session is active, so the call in `LinkFile` has zero cost during normal installs. It also deduplicates paths and skips `/etc/` (system paths need sudo and are handled separately by the wsl module).

### Restore

`RestoreBackup(timestamp)` reads the manifest and reverses each entry:

- `missing` -> `os.Remove()` the dfinstall symlink
- `symlink` -> remove current, recreate original symlink
- `file` -> remove current, copy backup file back

Individual failures are warned but don't stop the restore. A summary error is returned if any entries failed.

### Functions

| Function | Purpose |
|----------|---------|
| `StartBackup()` | Begin a new session |
| `BackupFile(dst)` | Record state of one path |
| `FinishBackup()` | Write manifest, clean up empty |
| `BackupActive()` | Check if a session is running |
| `ListBackups()` | Return available backups, newest first |
| `RestoreBackup(ts)` | Restore from a specific backup |
| `BackupDir()` | Base directory (config `backup_dir` or `~/.local/share/dfinstall/backups/`) |

## Error Handling

- Modules return `error` from `Install()`. The install loop logs the error and continues to the next module.
- Individual link/chmod failures within a module (like devtools) are warned and counted, with a summary error returned at the end.
- In quiet mode, errors are always printed immediately (clearing the spinner line). Warnings are buffered and shown after the spinner stops.
- `doctor` never fails -- it prints warnings and a summary.
