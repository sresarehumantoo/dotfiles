# Architecture

This document covers the core systems that make up `dfinstall`.

## Overview

```
dfinstall install all
        |
        v
  RegisterAllModules()     <- modules/register.go (sets order)
        |
        v
  DetectEnvironment()      <- core/env.go (WSL? Git Bash?)
        |
        v
  for each module:
    module.Install()       <- modules/<name>.go
      -> core.LinkFile()   <- core/link.go (symlink with backup)
      -> core.Info/Ok()    <- core/output.go (respects log level)
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

Built with [Cobra](https://github.com/spf13/cobra). Three commands:

| Command | Description |
|---------|-------------|
| `install <module\|all>` | Install one or all modules |
| `status` | Print table of link counts per module |
| `doctor` | Run 25+ health checks |

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
| `Warn()` | yellow | buffered | printed immediately |
| `Err()` | red | **always printed** | always printed |
| `Debug()` | magenta | suppressed | suppressed (debug only) |

In quiet mode, warnings are buffered and flushed after the spinner stops via `FlushWarnings()`. Errors always print and will clear the spinner line first (using an atomic `spinnerRunning` flag for thread safety).

### Spinner

`core/spinner.go` provides an animated braille-dot progress indicator:

```
  ⠹ Installing nvim (9/14)
```

- 10-frame animation at 80ms intervals
- Thread-safe text updates via mutex
- `PrintResult(total, failed)` renders the final line (`✓ Done` or `⚠ Done with errors`)
- `PrintHint(msg)` renders a dimmed follow-up message

The spinner runs in a background goroutine and is only used when `core.Level == LogQuiet`.

## Linking

`core/link.go` handles all symlink operations:

### LinkFile(src, dst)

1. Create parent directories if missing
2. If `dst` is already a correct symlink -- no-op
3. If `dst` is a wrong symlink -- repoint it
4. If `dst` is a regular file -- back it up to `dst.bak`
5. Create the symlink

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

## Error Handling

- Modules return `error` from `Install()`. The install loop logs the error and continues to the next module.
- Individual link/chmod failures within a module (like devtools) are warned and counted, with a summary error returned at the end.
- In quiet mode, errors are always printed immediately (clearing the spinner line). Warnings are buffered and shown after the spinner stops.
- `doctor` never fails -- it prints warnings and a summary.
