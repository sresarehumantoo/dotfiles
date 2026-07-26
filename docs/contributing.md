# Contributing

How to extend the dotfiles project with new modules, scripts, and configs.

## Adding a New Module

### 1. Create the module file

Create `src/modules/<name>.go`:

```go
package modules

import (
    "context"

    "github.com/sresarehumantoo/dotfiles/src/core"
)

type FooModule struct{}

func (FooModule) Name() string { return "foo" }

// Links is the single source of truth for what this module manages.
// Install, Uninstall and Status all derive from it — never restate a path.
func (FooModule) Links() core.LinkSet {
    return core.LinkSet{
        {Src: core.ConfigPath("foo", "config.toml"), Dst: core.XDGTarget("foo", "config.toml")},
    }
}

func (m FooModule) Install(ctx context.Context) error {
    core.Info("Setting up foo...")
    if err := m.Links().Apply(); err != nil {
        return err
    }
    core.Ok("Foo done")
    return nil
}

func (m FooModule) Uninstall(ctx context.Context) error {
    if err := m.Links().Remove(); err != nil {
        return err
    }
    core.Ok("Foo uninstalled")
    return nil
}

func (m FooModule) Status() core.ModuleStatus { return m.Links().Status("foo") }
```

That is the whole module. No `EnsureDir` — `LinkFile` creates parent directories.
No separate `Status` loop — `LinkSet.Status` counts what `Links()` exports.

`htop.go` is the minimal real example, `konsole.go` shows two destination roots,
and `devtools.go` shows an `Install` that needs extra work (chmod, best-effort
per-file errors) while still driving off `Links()`.

> **Why one `Links()`.** `Install`, `Uninstall`, `Status` and `Links` used to
> each spell out the same paths. Change a path in three of them and the fourth
> silently disagrees — `Status` reports on links `Install` no longer creates, or
> `Uninstall` misses one. `tests/linkset_test.go` asserts every module's
> `Status` counts exactly what its `Links()` exports.

### 2. Add config files

Put your config files under `config/foo/`. These are the source files that get symlinked into the home directory.

### 3. Register the module

Add it to `src/modules/register.go`. **Placement determines install order** -- put it after its dependencies:

```go
func RegisterAllModules() {
    // ... existing modules ...
    core.RegisterModule(&FooModule{})
}
```

### 4. Non-link modules

A module that installs packages rather than symlinks skips `Links()` and
implements `Install(ctx)` directly. It must start with a dry-run guard:

```go
func (FooModule) Install(ctx context.Context) error {
    if core.DryRun {
        core.Info("would install foo")
        return nil
    }
    return installPkg(ctx, "foo")
}
```

If the module is **opt-in** (installed only on explicit request, like windev),
it also has to be handled in `core.SkipInAll` — everything that loops over
`core.AllModules()` for an `install all`, including the `diff` report, gates on
that rather than `core.IsModuleSkipped`.

### 5. Update the test

Add `"foo"` to the expected order in `tests/module_test.go`:

```go
expected := []string{
    "locale", "packages", "extras", "toolkit", "delta", "fonts", "omz",
    "shell", "devtools", "git", "nvim", "windev", "tmux",
    "konsole", "ghostty", "htop", "wsl", "vmguest", "defaultshell",
    "foo",  // <-- add here matching register.go order
}
```

### 6. Build and test

```bash
make build && make test && make lint
```

## Adding a Devtools Script

### 1. Create the script

Create `config/devtools/<script-name>`:

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_lib.sh"

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    header "my-script"
    echo "What this script does."
    exit 0
fi

# Guard clauses
require_cmd docker

# Confirmation for destructive ops
confirm "Do the thing?" || { info "Aborted."; exit 0; }

# Do work
header "Doing the thing"
step "Working..."
ok "Done."
```

### 2. Add to devtools module

In `src/modules/devtools.go`, add to the `devtoolsScripts` slice:

```go
var devtoolsScripts = []string{
    // ... existing scripts ...
    "my-script",
}
```

Just the filename — `Links()` derives `config/devtools/<name>` →
`~/.local/bin/<name>` from it.

### 3. Test

```bash
make build && make test
bash -n config/devtools/my-script  # syntax check
```

## Conventions

### Go Code

- **Module structs:** `FooModule{}`, always value receivers
- **Path helpers:** `ConfigPath()` for sources, `HomeTarget()` for `$HOME`, `XDGTarget()` for `$XDG_CONFIG_HOME`
- **Logging:** `core.Info`, `core.Ok`, `core.Warn`, `core.Err`, `core.Debug`
- **Error handling:** Return errors, don't call `os.Exit()`. Warn and continue for non-fatal issues.
- **One `Links()`:** declare `Links() core.LinkSet` once and call `.Apply()` / `.Remove()` / `.Status(name)` from the other three methods. Never list a path in more than one place.
- **Never call `os.UserHomeDir()`** outside `core` — use `core.HomeDir()` / `core.HomeTarget()`. With `$HOME` unset it returns `("", err)`, and joining that gives a *relative* path, which once caused `uninstall tmux` to delete `.tmux/plugins` from the current working directory. For recursive deletes use `core.RemoveManagedDir`, not `os.RemoveAll`.
- **Extend a home-derived path with `core.SubPath`, not `filepath.Join`.** `HomeTarget`/`XDGTarget` return `""` when home is unresolvable, but `filepath.Join("", "custom")` turns that straight back into a relative path — and `git clone`, `os.WriteFile` and `os.MkdirAll` never reach `checkTarget`. Before acting on such a path directly, call `core.CheckTarget(path)`.
- **`core.SkipInAll(name)`, not `IsModuleSkipped`,** in any loop over `AllModules()` for an install-all or a fixable-drift report. The MCP server once used the latter and installed an opt-out module.

### Shell Scripts

- `set -euo pipefail` at the top
- Source `_lib.sh` for output helpers and guards
- `-h`/`--help` support (use `header` for the name)
- Confirmation prompts before destructive operations
- Guard clauses via `require_wsl`, `require_cmd`, `require_git_repo`
- Portable across WSL distros -- use `$WSL_DISTRO_NAME`, `cmd.exe`, `wslpath` for Windows paths
- No hardcoded distro-specific paths

### Output

The output system has three levels. Code should use the right function:

| Use | Function | When shown |
|-----|----------|-----------|
| Starting a task | `Info()` | verbose+ |
| Task completed | `Ok()` | verbose+ |
| User-facing feedback | `Status()` | always |
| Non-fatal issue | `Warn()` | always (buffered in quiet mode) |
| Fatal issue | `Err()` | always |
| Internal detail | `Debug()` | debug only |

In default (quiet) mode, the CLI shows a spinner. `Info`/`Ok` calls are suppressed. `Warn` calls are buffered and printed after the spinner stops. `Err` and `Status` calls always print immediately. Use `Status()` for direct feedback after interactive prompts (like the extended plugin menu).

### External Commands

Every subprocess goes through `runCmd(ctx, ...)` (`src/modules/packages.go`).
There are **no** raw `exec.Command` calls in `src/` — `go vet` won't catch a new
one, so this is on you.

```go
if err := runCmd(ctx, "git", "clone", "--depth=1", url, dest); err != nil {
    core.Warn("clone failed: %v", err)
}
```

- **Thread the context.** `Install(ctx)` receives it; pass it to every command,
  and to every helper that runs one. The CLI binds it to SIGINT so Ctrl-C tears
  down a running `apt`; the MCP server binds it to the request.
- **Don't wire `Stdout`/`Stderr` yourself.** `runCmd` sends output to the
  terminal under `-v` and otherwise captures both streams, replaying the tail on
  failure. Hand-rolled routing is how failures used to surface as a bare
  `exit status 128` with no reason.
- **Use `runCmd(ctx, "sudo", ...)`** rather than `core.SudoCmd` plus a manual
  `PauseSpinner`. `runCmd` detects sudo (even inside a `bash -c` string) and
  pauses the spinner for the password prompt. `core.SudoCmd` execs directly when
  already root.
- **Quick queries use `runProbe(ctx, ...)`**, network fetches `runNetProbe`.
  They carry `ProbeTimeout` (30s) and `NetworkTimeout` (10m) from
  `src/core/exec.go`; `runCmd` carries `InstallTimeout` (45m). Every command
  must have a deadline — a hung `curl` or an apt blocked on a dpkg lock used to
  hang the whole run with no way out.
- **`PauseSpinner`/`ResumeSpinner` are for interactive prompts only** (huh
  forms, `bufio` reads, `chsh`) — not for command output.
- **Validate before writing to shell files:** any value written to a file that
  is later `source`d must be validated (alphanumeric + hyphens/underscores).

## Testing

Tests live in two places, deliberately:

- **`tests/`** (23 files, `package tests`) — black-box. Anything reachable
  through the exported API goes here.
- **In-package** (`src/core/*_test.go`, `src/modules/*_test.go`) — for
  invariants about unexported state: the spinner's state machine, `checkTarget`,
  `pickAsset`, `artifactFor`. Asserting these from `tests/` would mean exporting
  internals purely for the test.

Notable coverage: module registration order, symlink create/unlink/dry-run,
backup and restore round-trips, config load/save integrity (including refusing
to overwrite an unreadable config), registry validation as a security boundary,
`LinkSet` behaviour and the Status/Links agreement, status and diff rendering,
spinner concurrency, and GitHub release asset selection from a fixture.

```bash
make test       # go test ./src/... ./tests/...
make test-race  # go test -race ./src/... ./tests/...
make lint       # go vet
make ci         # fmt-check + lint + build both binaries + test + test-race
```

Tests use `t.TempDir()` and `t.Setenv` and don't touch real system files. Any
test that writes config must point `$DOTFILES` at a temp dir — otherwise
`SaveConfig` writes to the real repo's `.config.yaml`.

### Verifying a change by hand

`make ci` is necessary, not sufficient. This tool symlinks, deletes and shells
out under sudo across a real `$HOME`, and most install paths have no test.

- Drive the real binary in a sandbox: `HOME=<tmp> DOTFILES=<tmp-clone> ./bin/dfinstall <cmd>`.
  Both variables matter — set only `HOME` and it still reads the real repo's
  config; set only `DOTFILES` and it writes into your actual home.
- Before believing a before/after comparison is "identical", confirm **both**
  sides produced output. A failed build or an unconfigured fixture yields two
  empty files and a false pass.
- Prefer `git worktree` over `git stash` for building a "before" binary — stash
  drops untracked files and can fail to reapply a deletion.
- For a bug fix, run the new test against the old code and watch it fail.
- `--dry-run` is the safe way to exercise destructive paths, and is worth
  testing in its own right: it has been the *source* of data loss here before.

## CI

Every push and pull request runs:

- **`go`** — `gofmt -s` formatting, `go vet`, build of the `dfinstall` and MCP
  binaries, the test suite, and the suite again under the race detector.
- **`shellcheck`** — all shell scripts under `config/`.
