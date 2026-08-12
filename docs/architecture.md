# Architecture

This document covers the core systems that make up `dfinstall`.

## Overview

```
dfinstall install all [--backup]
        |
        v
  signal.NotifyContext()   <- cmd/dfinstall/main.go (ctx bound to SIGINT/SIGTERM)
        |
        v
  LoadConfig() error       <- core/config.go (.config.yaml; warns and continues
        |                      on an unreadable file, never overwrites it)
        v
  RegisterAllModules()     <- modules/register.go (sets order)
        |
        v
  DetectEnvironment()      <- core/env.go (WSL? Git Bash? distro?)
        |
        v
  core.BeginInstall(...)   <- core/install_session.go
        |                     - adopts the canonical clone (install all)
        |                     - decides whether to back up (first run/--backup/config)
        |                     - StartBackup() if so
        v
  core.PromptSudo(ctx)     <- core/env.go (primes credentials before the spinner)
        |
        v
  for each module:
    if core.SkipInAll(name) -> skip   <- user skip_modules + un-enabled opt-ins
    module.Install(ctx)    <- modules/<name>.go
      -> m.Links().Apply() <- core/module.go -> core.LinkFile()
        -> BackupFile()    <- core/backup.go (records pre-install state)
      -> runCmd(ctx, ...)  <- modules/packages.go (deadline-bounded subprocess)
      -> core.Info/Ok()    <- core/output.go (respects log level)
        |
        v
  sess.Finish()            <- core/install_session.go
        |                     - FinishBackup() writes the manifest
        |                     - SaveConfig() on first run, or when an opt-in mode
        |                       (--extended/--toolkit/windev) changed config
        |                     - neither happens under --dry-run
        v
  spinner / summary        <- core/spinner.go
```

Both the CLI and the MCP server go through `BeginInstall`/`Finish` and
`SkipInAll`. They previously hand-rolled this and drifted: the MCP server took
no backup and installed opt-out modules.

## Module System

### Interface

Every module implements three methods (defined in `core/module.go`):

```go
type Module interface {
    Name() string                      // identifier used in CLI ("shell", "nvim", etc.)
    Install(ctx context.Context) error // perform installation
    Status() ModuleStatus              // report current state
}

type ModuleStatus struct {
    Name    string
    Linked  int      // items successfully in place
    Missing int      // items not yet linked/installed
    Extra   string   // freeform info
}
```

`Install` takes a context so a long install can be canceled — the CLI binds it
to SIGINT, the MCP server to the per-request context. **Modules must pass it
down to every subprocess they spawn.** `Status` deliberately takes none: it is a
fast synchronous read for display, and anything slow enough to need canceling
doesn't belong in it.

### Optional Interfaces

Link-based modules can optionally implement these interfaces for uninstall and diff support:

```go
type Uninstaller interface {
    Uninstall(ctx context.Context) error
}

type LinkExporter interface {
    Links() LinkSet
}

type LinkPair struct {
    Src string
    Dst string
}

// LinkSet is a module's complete set of managed symlinks.
type LinkSet []LinkPair

func (ls LinkSet) Apply() error                      // create every link
func (ls LinkSet) Remove() error                     // unlink every link
func (ls LinkSet) Status(name string) ModuleStatus   // count what's in place
```

Implementing `LinkExporter` is what makes a module's links visible to `diff` and
to drift detection, so **every module with links should**.

Non-link modules (locale, packages, extras, delta, fonts, omz, wsl, vmguest, defaultshell) don't implement either interface because their side effects can't be cleanly reversed via symlink removal. (The toolkit module is an exception — it has no links but implements `Uninstaller` to remove the tools it installed.)

### Registry

Modules are registered in `modules/register.go` via `core.RegisterModule()`. **Order matters** -- earlier modules are installed first, so dependencies (packages, fonts, omz) come before things that need them (shell, nvim).

Lookup functions:

| Function | Returns |
|----------|---------|
| `core.AllModules()` | ordered slice of all modules |
| `core.GetModule(name)` | single module by name |
| `core.ModuleNames()` | string slice of names |

### One LinkSet per module

`Links()` is the single source of truth; `Install`, `Uninstall` and `Status` all
derive from it:

```go
func (KonsoleModule) Links() core.LinkSet {
    return core.LinkSet{
        {Src: core.ConfigPath("konsole", "konsolerc"), Dst: core.XDGTarget("konsolerc")},
        {Src: core.ConfigPath("konsole", "Dotfiles.profile"),
         Dst: core.HomeTarget(".local", "share", "konsole", "Dotfiles.profile")},
    }
}

func (m KonsoleModule) Install(ctx context.Context) error   { return m.Links().Apply() }
func (m KonsoleModule) Uninstall(ctx context.Context) error { return m.Links().Remove() }
func (m KonsoleModule) Status() core.ModuleStatus           { return m.Links().Status("konsole") }
```

These four used to each spell out the same paths independently. Changing a path
in three of them left the fourth silently disagreeing — `Status` reporting on
links `Install` no longer creates, or `Uninstall` missing one. `konsole` was the
worst case: its first entry targets a different root, so all four methods had to
remember to slice `konsoleLinks[1:]`.

`tests/linkset_test.go` asserts that every module's `Status` counts exactly what
its `Links()` exports, so a hand-rolled `Status` fails the build.

## CLI

Built with [Cobra](https://github.com/spf13/cobra). Nine commands:

| Command | Description |
|---------|-------------|
| `install <module\|all>` | Install one or all modules |
| `update <module\|all>` | Alias for install — re-apply modules |
| `uninstall <module\|all>` | Remove symlinks created by dfinstall |
| `diff` | Show drift between config and filesystem |
| `status` | Print table of link counts per module |
| `doctor` | Run health checks — 22 on a normal machine, plus conditional ones (extended plugins, SteamOS, WSL, alias collisions) |
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
| `--dry-run` | forces `LogVerbose` | Preview changes without modifying the filesystem (sets `core.DryRun`) |
| `--version` | — | Print the binary's version (`core.Version`, stamped from `git describe` via the Makefile's `-ldflags`; `"dev"` for a plain `go build`) |

`-v`/`--verbose`, `--debug` and `--dry-run` are persistent (apply to all subcommands). The level is set in `PersistentPreRun` and stored in `core.Level`. `--version` is Cobra's built-in flag, enabled by setting `Version` on the root command.

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

In quiet mode, both notices and warnings are buffered and flushed after the spinner stops via `FlushWarnings()` (notices first, then warnings). Errors and `AlwaysWarn()` always print and will clear the spinner line first (all state lives on the `Spinner` behind one mutex; `Pause`/`Resume` nest via a depth counter and nothing draws when stdout is not a TTY).

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

All are variadic — `HomeTarget(".local", "bin", name)`.

| Helper | Resolves to |
|--------|-------------|
| `ConfigPath("shell", "zshrc")` | `<dotfiles>/config/shell/zshrc` |
| `HomeTarget(".zshrc")` | `$HOME/.zshrc` |
| `XDGTarget("nvim")` | `$XDG_CONFIG_HOME/nvim` (or `~/.config/nvim`) |
| `HomeDir() (string, error)` | `$HOME`, or an error if it can't be resolved |
| `SubPath(base, parts...)` | extend a `HomeTarget`/`XDGTarget` result, propagating an empty base |
| `CheckTarget(path)` | the refusal `LinkFile` applies, for callers that act on a path directly |
| `RemoveManagedDir(path)` | `os.RemoveAll` guarded by `checkTarget` |

**Never call `os.UserHomeDir()` outside `core`.** With `$HOME` unset it returns
`("", err)`, and `filepath.Join("", ".tmux")` is the *relative* `.tmux` — so a
discarded error silently retargets every managed path at the current working
directory. That is not hypothetical: `uninstall tmux` with `$HOME` unset used to
delete `.tmux/plugins` from whatever directory you were standing in.

`HomeTarget`/`XDGTarget` return `""` when the home directory can't be resolved,
and `LinkFile`, `UnlinkFile`, `EnsureDir` and `RemoveManagedDir` all run
`checkTarget`, which rejects empty and non-absolute paths.

**That empty return only helps if you propagate it.** `filepath.Join("",
"custom")` is `"custom"` — the relative path all over again — and it never
reaches `checkTarget`, because `git clone`, `os.WriteFile` and `os.MkdirAll`
don't go through `LinkFile`. So:

- Extend a home- or XDG-derived path with **`core.SubPath(base, parts...)`**,
  never `filepath.Join`. It returns `""` for an empty base.
- Prefer `XDGTarget("dfinstall", "plugins.zsh")` over
  `filepath.Join(XDGConfigHome(), ...)` for the same reason.
- Before acting on a path directly — a clone destination, a file you write —
  call **`core.CheckTarget(path)`** and make the same refusal.

With `$HOME` and `$XDG_CONFIG_HOME` unset, `install all` used to create
`./dfinstall/`, `./custom/plugins/zsh-autosuggestions/` and
`./custom/themes/powerlevel10k/` in whatever directory you were standing in.

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
| `AdoptCanonical(dir)` | Record `dir` as canonical if it isn't already and reset the cached `DotfilesDir()`; returns `(prev string, changed bool)` |

For an `install all`, `core.BeginInstall` (`core/install_session.go`) calls `AdoptCanonical()` before the module loop — and only when not under `--dry-run`, since the pointer is machine-global and a preview must change nothing (it records `CanonicalPrev`/`CanonicalNow` on the session instead). It records the invoking clone as canonical, then the module loop repoints any stray symlinks — so `install all` both switches the canonical clone and consolidates a machine onto it. A partial `install <module>` links into the canonical clone (not necessarily the one you're sitting in) and `AlwaysWarn()`s if those differ.

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
| `FetchRegistry(ctx, url)` | Fetch over HTTP(S) with `net/http` under a `NetworkTimeout` deadline, `file://` path, or plain file path; validates and writes the cache |
| `InspectRegistry(ctx, url)` | Same fetch and validation, **without** writing the cache — this is what `registry validate` uses |
| `LoadCachedRegistry()` | Read the registry from the local cache file |
| `LoadOrFetchRegistry(ctx, forceRefresh)` | Load from cache, or fetch remotely if missing / `forceRefresh` |
| `ValidateRegistry(r)` | Check version, unique valid names, required category/binary, method-specific fields, distro filters |
| `RegistryCachePath()` | `~/.local/share/dfinstall/toolkit-registry.json` |
| `CleanRegistryCache()` | Remove the cached registry file |
| `ToolMatchesDistro(t)` | Whether a tool applies to the current distro (no filter = all) |

Tool names are validated against `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`. The cache is auto-cleaned after install (EDR-safe — no tool list lingers on disk).

## Virtualization Detection

`core/virt.go` detects whether dfinstall is running inside a VM or container, driving the `vmguest` module.

| Function | Purpose |
|----------|---------|
| `DetectVirt(ctx)` | Returns a `VirtType` — prefers `systemd-detect-virt` (under a `ProbeTimeout` derived from `ctx`), falls back to DMI inspection (`/sys/class/dmi/id/`) for minimal images |
| `IsVM(ctx)` | True only for hardware-virtualized guests (excludes containers and WSL) |
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
| `windev_enabled` | bool | `false` | Whether the opt-in windev module is enabled (drives `core.SkipInAll`) |
| `toolkit_registry_url` | string | *(empty)* | Custom toolkit registry URL override |

### Auto-Backup Logic

The backup decision lives in `shouldBackup` inside `core/install_session.go`
and runs as part of `BeginInstall`. Four states drive it:

| Condition | Backup? | Then |
|-----------|---------|------|
| `--dry-run` | No | and no config write either — a preview changes nothing |
| `--backup` flag | Yes | nothing extra |
| No config file (first run) | Yes | save config with `skip_backup: true` |
| Config exists, `skip_backup: false` | Yes | respect user preference |
| Config exists, `skip_backup: true` | No | -- |

The key distinction is `CfgFileExists` — whether the config file was present at load time. This separates "first run, no config" from "user explicitly set `skip_backup: false`". On first run, after the auto-backup, the config is saved with `skip_backup: true` so subsequent runs skip by default. An existing config's `skip_backup` value is never overwritten.

### Functions

| Function | Purpose |
|----------|---------|
| `LoadConfig() error` | Read and parse `.config.yaml`. **Only `fs.ErrNotExist` counts as a first run** — any other failure (permissions, EISDIR, malformed YAML) marks the config unreadable and is returned to the caller, which warns and continues on defaults |
| `SaveConfig() error` | Write `Cfg` via temp file + `Sync` + rename. **Refuses** to write when `LoadConfig` couldn't read the existing file — otherwise a first-run save replaces the user's settings with defaults |
| `ConfigFilePath()` | Return full path to the config file |
| `IsModuleSkipped(name)` | Is the module in `skip_modules`? |
| `SkipInAll(name)` | Should `install all` skip it? `IsModuleSkipped` **plus** un-enabled opt-ins. Use this, not `IsModuleSkipped`, in any `AllModules()` loop |
| `SetWindevOptIn()` / `ClearWindevOptIn()` | Record/drop the windev opt-in |

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

## Subprocess Execution

`core/exec.go` defines the deadlines; `modules/packages.go` provides the wrappers.

The rule is that **every** subprocess is cancelable: `src/` contains no
`exec.Command` calls at all, only `exec.CommandContext`, so a Ctrl-C (or an MCP
request cancellation) tears down whatever is running. The three wrappers below
are how a command *additionally* gets a deadline.

| Wrapper | Deadline | For |
|---------|----------|-----|
| `runProbe(ctx, ...)` | `ProbeTimeout` — 30s | quick queries: `dpkg -s`, `cargo --version`, `pipx list` |
| `runNetProbe(ctx, ...)` | `NetworkTimeout` — 10m | network reads, e.g. the GitHub releases API |
| `runCmd(ctx, ...)` | `InstallTimeout` — 45m | everything that installs or builds |

These are hang detectors, not performance budgets — each sits well past the
slowest healthy run of its class. Before them, a `curl` to a blackholed host or
an `apt` blocked on a dpkg lock hung the whole run behind a spinner with no way
out but Ctrl-C, and under the MCP server there is no terminal to Ctrl-C from.

`runCmd` also owns output routing (straight to the terminal under `-v`,
otherwise captured and replayed on failure), spinner pausing, and sudo TTY
teeing. Building an `exec.Cmd` by hand loses all of that.

### Direct `exec.CommandContext` call sites

A handful of short-lived local commands still call `exec.CommandContext`
directly. They inherit the caller's context (so they remain cancelable) but get
**no** timeout of their own:

| Site | Command |
|------|---------|
| `modules/extras.go` | `tldr --update` |
| `modules/wsl.go` | `git config --global` (×2) |
| `modules/tmux.go` | `tmux start-server` |
| `modules/defaultshell.go` | `chsh` |

`chsh` is deliberate: it prompts for the user's password, so it needs
`os.Stdin`/`os.Stdout`/`os.Stderr` wired straight to the terminal, which is
exactly what `runCmd`'s capture-and-replay routing takes away. The rest are
candidates for a wrapper. `fonts.go` no longer appears here: its `curl`,
`fc-list` and `fc-cache` calls all go through `runCmd`/`runProbe`, and `unzip`
is gone entirely (the archive is decompressed in-process via `ulikunitz/xz` +
`archive/tar`).

Not every direct call site is unbounded, though: `core/virt.go`'s
`systemd-detect-virt` probe and the sudo probes in `core/env.go` derive their own
`ProbeTimeout` context before spawning.

## Install Session

`core/install_session.go` wraps the setup and teardown every install path
shares, so the CLI and the MCP server cannot drift apart on it:

```go
sess, err := core.BeginInstall(core.InstallOptions{All: true, ForceBackup: flagBackup})
if err != nil { return err }
defer sess.Finish()
```

`BeginInstall` adopts the canonical clone (when `All`), decides whether to back
up, and starts the backup. `Finish` writes the manifest and persists config.
Rendering stays with the caller — the session reports `CanonicalPrev`,
`CanonicalNow` and `DidBackup()` rather than printing.

Before this existed the MCP server took no backup at all and ignored the windev
opt-in, while being annotated idempotent and non-destructive.

Three rules the session enforces, each of which was once a data-loss bug:

- **`BeginInstall` takes a process-wide lock that `Finish` releases.** The
  backup session, the canonical pointer and `Cfg` are all global. Two overlapping
  installs meant the second `StartBackup` replaced the first's manager, and the
  first `FinishBackup` then wrote its manifest into the second's directory —
  leaving the first's copied files with no manifest, invisible to `restore`. The
  CLI only ever runs one install; the MCP server dispatches tool calls from a
  worker pool, so its handlers really can overlap. They queue instead.
- **Call `sess.MarkFailed()` when the install fails.** Otherwise the deferred
  `Finish` still persists the first-run config, and `skip_backup: true` disarms
  the automatic backup before the run that actually replaces the user's dotfiles.
- **Nothing here mutates persisted state under `--dry-run`** — not the config,
  and not the canonical pointer. A preview that adopts the clone you previewed
  from is exactly the multi-clone drift the pointer exists to prevent. The
  session still *reports* the adoption it would have made.

## Status & Diff Reporting

`modules/report.go` collects and renders both reports once:

| Function | Purpose |
|----------|---------|
| `StatusRows()` | one `ModuleStatus` per module, with the "skipped" annotation applied |
| `WriteStatus(w)` | render the status table to any writer |
| `CollectDiff()` | walk every module, returning a `DiffReport` |
| `DiffReport.Write(w, fixHint, consolidateCmd)` | render it |

The writer is why one implementation serves both surfaces: the CLI passes
`os.Stdout`, the MCP server a `strings.Builder`. The two hint strings are the
only thing that differs between them — each names its own command.

## Error Handling

- Modules return `error` from `Install(ctx)`. The install loop logs the error and continues to the next module.
- Individual link/chmod failures within a module (like devtools) are warned and counted, with a summary error returned at the end.
- In quiet mode, errors are always printed immediately (clearing the spinner line). Warnings are buffered and shown after the spinner stops.
- `doctor` never fails -- it prints warnings and a summary.
