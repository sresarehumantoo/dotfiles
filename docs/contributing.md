# Contributing

How to extend the dotfiles project with new modules, scripts, and configs.

## Adding a New Module

### 1. Create the module file

Create `src/modules/<name>.go`:

```go
package modules

import "github.com/owenpierce/dotfiles/src/core"

type FooModule struct{}

func (FooModule) Name() string { return "foo" }

var fooLinks = []struct{ src, dst string }{
    {"foo/config.toml", ".config/foo/config.toml"},
}

func (FooModule) Install() error {
    core.Info("Setting up foo...")
    for _, l := range fooLinks {
        if err := core.LinkFile(core.ConfigPath(l.src), core.HomeTarget(l.dst)); err != nil {
            return err
        }
    }
    core.Ok("Foo done")
    return nil
}

func (FooModule) Status() core.ModuleStatus {
    s := core.ModuleStatus{Name: "foo"}
    for _, l := range fooLinks {
        if core.CheckLink(core.ConfigPath(l.src), core.HomeTarget(l.dst)) == "ok" {
            s.Linked++
        } else {
            s.Missing++
        }
    }
    return s
}
```

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

### 4. Update the test

Add `"foo"` to the expected order in `tests/module_test.go`:

```go
expected := []string{
    "packages", "extras", "delta", "fonts", "omz",
    "shell", "devtools", "git", "nvim", "tmux",
    "ghostty", "htop", "wsl", "defaultshell",
    "foo",  // <-- add here matching register.go order
}
```

### 5. Build and test

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
var devtoolsScripts = []struct{ src, dst string }{
    // ... existing scripts ...
    {"devtools/my-script", ".local/bin/my-script"},
}
```

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
- **Data-driven:** Define a `var links = []struct{ src, dst string }` slice and loop it in both `Install()` and `Status()`

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
| Non-fatal issue | `Warn()` | always (buffered in quiet mode) |
| Fatal issue | `Err()` | always |
| Internal detail | `Debug()` | debug only |

In default (quiet) mode, the CLI shows a spinner. `Info`/`Ok` calls are suppressed. `Warn` calls are buffered and printed after the spinner stops. `Err` calls always print immediately.

## Testing

Tests live in `tests/` and cover:

| File | What it tests |
|------|--------------|
| `module_test.go` | Module registration order, lookup |
| `link_test.go` | Symlink creation, idempotency, backup, nested dirs |
| `env_test.go` | WSL detection from /proc/version |
| `status_test.go` | Status line formatting |
| `backup_test.go` | Backup/restore: path flattening, system path detection, entry types, dedup, empty cleanup, round-trip |

Run with:

```bash
make test       # go test ./src/... ./tests/...
make lint       # go vet
```

Tests use temp directories and don't touch real system files.

## Doctor Checks

When adding a module that installs a new tool or creates a new symlink, consider adding a check to `modules/doctor.go`. The doctor command validates the overall health of the environment and helps users diagnose issues.
