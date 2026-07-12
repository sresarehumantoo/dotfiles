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

### Optional Interfaces

Link-based modules can optionally implement these interfaces for uninstall and diff support:

```go
type Uninstaller interface {
    Uninstall() error
}

type LinkExporter interface {
    Links() []LinkPair
}

type LinkPair struct {
    Src string
    Dst string
}
```

Non-link modules (locale, packages, extras, delta, fonts, omz, wsl, vmguest, defaultshell) don't implement either interface because their side effects can't be cleanly reversed via symlink removal. (The toolkit module is an exception — it has no links but implements `Uninstaller` to remove the tools it installed.)

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

Built with [Cobra](https://github.com/spf13/cobra). Nine commands:

| Command | Description |
|---------|-------------|
| `install <module\|all>` | Install one or all modules |
| `update <module\|all>` | Alias for install — re-apply modules |
| `uninstall <module\|all>` | Remove symlinks created by dfinstall |
| `diff` | Show drift between config and filesystem |
| `status` | Print table of link counts per module |
| `doctor` | Run 25+ health checks |
| `restore [timestamp]` | Restore files from a backup snapshot |
| `root` | Symlink configs into `/root/` via sudo |
| `registry validate <path\|url>` | Validate a toolkit registry file (for CI) |

### Install / Update Flags

| Flag | Behavior |
|------|----------|
| `--backup` | Force a backup snapshot regardless of config |
| `--extended` | Show interactive menu to select extended OMZ plugins |
| `--toolkit` | Show interactive menu to select toolkit tools |
| `--registry <path\|url>` | Override toolkit registry URL for a single run |
| `--dry-run` | Preview changes without modifying the filesystem (forces verbose output) |

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

Defined in `core/output.go`. Nine functions, each with a colored prefix:

| Function | Color | Quiet mode | Verbose+ |
|----------|-------|------------|----------|
| `Info()` | blue | suppressed | printed |
| `Ok()` | green | suppressed | printed |
| `Notice()` | blue | buffered (flushed after spinner) | printed immediately |
| `Status()` | green | **always printed** | always printed |
| `Warn()` | yellow | buffered (flushed after spinner) | printed immediately |
| `AlwaysWarn()` | yellow | **always printed** (clears spinner line) | printed immediately |
| `Err()` | red | **always printed** | always printed |
| `Debug()` | magenta | suppressed | suppressed (debug only) |
| `FlushWarnings()` | — | prints buffered notices then warnings | — |

`Status()` is for direct user-facing feedback after interactive prompts (e.g. the extended plugin menu). It prints with a green checkmark regardless of log level. `Notice()` is for expected operational messages (e.g. backups) that aren't warnings — always visible, but buffered in quiet mode. `AlwaysWarn()` surfaces a warning immediately (clearing the spinner line) when buffered output would arrive too late to act on.

In quiet mode, both notices and warnings are buffered and flushed after the spinner stops via `FlushWarnings()` (notices first, then warnings). Errors and `AlwaysWarn()` always print and will clear the spinner line first (using an atomic `spinnerRunning` flag for thread safety).

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
| Distro | `/etc/os-release` `ID`/`ID_LIKE` → `Debian`, `Fedora`, `Arch`, or `SteamOS` |

Git Bash is **rejected** (via `AssertEnvironment()`) with an error pointing the user to WSL. WSL is detected and logged, enabling WSL-only modules (like wsl.conf setup). The detected distro (`GetDistro()`, plus helpers `IsDebianBased()`, `IsArchBased()`, `IsSteamOS()`) drives package selection and the SteamOS readonly-root handling.

### Dotfiles Directory Resolution

`DotfilesDir()` answers "which clone should symlinks point into?" and is cached (reset via `ResetDotfilesDir()`). Resolution order:

1. `$DOTFILES` environment variable — explicit override, always wins
2. The machine-global **canonical pointer**, if it still points at a valid checkout
3. The clone this binary is physically running from — `InvokingCloneDir()`

`InvokingCloneDir()` (in `core/env.go`) answers the different question "where am I running from?" and has its own order: `$DOTFILES` → build-time baked path (`-ldflags` from the Makefile) → walk up from the executable looking for `go.mod` → current working directory.

### Canonical Clone Pointer

`core/canonical.go` records which clone of the repo is authoritative on this host, so that every clone's binary links into a single source (preventing symlink drift across multiple clones).

| Function | Purpose |
|----------|---------|
| `CanonicalPointerPath()` | `$XDG_CONFIG_HOME/dfinstall/dotfiles-dir` (lives outside any clone) |
| `ReadCanonicalDir()` | Return the recorded canonical dir, or `""` if unset/unreadable — **self-healing**: a stale path (clone deleted/renamed, no `config/` dir) is ignored, so `DotfilesDir()` falls through and the next `install all` rewrites it |
| `WriteCanonicalDir(dir)` | Record `dir` atomically (temp file + rename) |

`install all` calls `adoptCanonical()` first: it records the invoking clone as canonical, then the module loop repoints any stray symlinks — so `install all` both switches the canonical clone and consolidates a machine onto it. A partial `install <module>` links into the canonical clone (not necessarily the one you're sitting in) and `AlwaysWarn()`s if those differ.

### Link Drift Detection

`core/drift.go` reports when managed symlinks are spread across more than one clone.

| Symbol | Purpose |
|--------|---------|
| `LinkRoot(target)` | Extract the clone root from a managed symlink target by trimming the trailing `/config/...` |
| `DetectLinkDrift()` | Walk every `LinkExporter` module's links, group existing symlinks by clone root (missing/not-yet-linked targets ignored) |
| `LinkDrift.Split()` | True when symlinks span multiple roots, or all point at a non-canonical clone |
| `LinkDrift.SortedRoots()` | Clone roots in stable order for display |

`doctor` and `diff` call `DetectLinkDrift()` and warn (listing each root, marking the canonical one) when `Split()` is true, pointing the user at `dfinstall install all` to consolidate.

## File Hashing

`core/hash.go` provides SHA-256 file hashing used by the `fonts` and `wsl` modules to detect whether installed files match the source. This avoids unnecessary writes and enables status checks without reading full file contents.

## Toolkit Registry

`core/registry.go` manages the external toolkit registry — a JSON catalog of installable tools fetched at runtime so no tool names are compiled into the binary. The registry is fetched from `DefaultRegistryURL` (raw GitHub, `sresarehumantoo/dotfiles-toolkit`), overridable via the `toolkit_registry_url` config field or the `--registry` flag.

### Structures

```go
type Registry struct {
    Version int            // must be 1
    Tools   []RegistryTool
}

type RegistryTool struct {
    Name, Description, Category string
    Method  string   // apt, go, pipx, cargo, git_clone, appimage, deb, release_binary, rustup
    Binary  string
    // method-specific fields: Package, AppRepo, GitRepo, DebRepo, ReleaseRepo, AssetPattern
    Distros []string // optional distro filter: debian, arch, fedora
}
```

### Functions

| Function | Purpose |
|----------|---------|
| `FetchRegistry(url)` | Fetch from HTTP(S) URL (via `curl`), `file://` path, or plain file path; validates and writes the cache |
| `LoadCachedRegistry()` | Read the registry from the local cache file |
| `LoadOrFetchRegistry(forceRefresh)` | Load from cache, or fetch remotely if missing / `forceRefresh` |
| `ValidateRegistry(r)` | Check version, unique valid names, required category/binary, method-specific fields, distro filters |
| `RegistryCachePath()` | `~/.local/share/dfinstall/toolkit-registry.json` |
| `CleanRegistryCache()` | Remove the cached registry file |
| `ToolMatchesDistro(t)` | Whether a tool applies to the current distro (no filter = all) |

Tool names are validated against `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. The cache is auto-cleaned after install (EDR-safe — no tool list lingers on disk).

## Virtualization Detection

`core/virt.go` detects whether dfinstall is running inside a VM or container, driving the `vmguest` module.

| Function | Purpose |
|----------|---------|
| `DetectVirt()` | Returns a `VirtType` — prefers `systemd-detect-virt`, falls back to DMI inspection (`/sys/class/dmi/id/`) for minimal images |
| `IsVM()` | True only for hardware-virtualized guests (excludes containers and WSL) |
| `IsHardwareVirt(v)` | Shared predicate for "is this a true VM that benefits from guest tools" |
| `ParseSystemdVirt(s)` / `ParseDMIVendor(v, p)` | Exported parsers for testing |

`VirtType` values mirror `systemd-detect-virt` output (`kvm`, `qemu`, `vmware`, `oracle`, `microsoft`, `xen`, `wsl`, `lxc`, `docker`, `podman`, plus `container`/`unknown`/`none`).

The `vmguest` module (`modules/vmguest.go`) uses this: on a hardware VM it installs the matching guest packages (e.g. `qemu-guest-agent`/`spice-vdagent` for KVM, `open-vm-tools` for VMware) and enables the corresponding systemd units. It skips WSL, containers, and bare metal.

## Configuration

`core/config.go` manages a YAML config file at `<dotfiles>/.config.yaml`. Loaded in `PersistentPreRun` before any command runs.

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skip_backup` | bool | `false` | Skip automatic backups on install |
| `backup_dir` | string | *(empty)* | Custom backup directory (falls back to `~/.local/share/dfinstall/backups/`) |
| `extended_plugins` | []string | *(empty)* | Extended OMZ plugins selected via `--extended` |
| `preserved_files` | []string | *(empty)* | Custom shell files the user chose to keep sourcing after dfinstall replaces zshrc |
| `dismissed_files` | []string | *(empty)* | Custom shell files the user chose not to preserve (prevents re-prompting) |
| `skip_modules` | []string | *(empty)* | Modules to skip during `install all` (machine profiles) |
| `toolkit_tools` | []string | *(empty)* | Toolkit tools selected via `--toolkit` |
| `toolkit_registry_url` | string | *(empty)* | Custom toolkit registry URL override |

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
