# Building from Source

How to compile `dfinstall` from source and set up a development environment.

## Requirements

| Requirement | Minimum | Notes |
|-------------|---------|-------|
| **Go** | 1.25+ | [Install Go](https://go.dev/doc/install) |
| **Git** | any | For cloning and module downloads |
| **Make** | any | Optional — convenience targets only |

No other system dependencies are needed to build. Runtime dependencies (zsh, curl, git, etc.) are installed by dfinstall itself.

## Quick Build

```bash
git clone https://github.com/sresarehumantoo/dotfiles ~/dotfiles
cd ~/dotfiles
make build
```

The compiled binary is written to `bin/dfinstall`.

## Without Make

```bash
go build \
  -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(pwd)" \
  -o bin/dfinstall ./src/cmd/dfinstall
```

The `-ldflags` flag bakes the dotfiles directory path into the binary at compile time. This is only one input to how `dfinstall` locates its `config/` directory. At runtime `DotfilesDir()` resolves in order:

1. `$DOTFILES` environment variable — explicit override, always wins.
2. The machine-global canonical pointer file (`~/.config/dfinstall/dotfiles-dir`) — records which clone is authoritative on this host, so every clone links from one source. `install all` writes it (and self-heals a stale pointer).
3. The clone the binary is physically running from — via `$DOTFILES`, then the baked-in path, then walking up from the executable to the nearest `go.mod`, then the current directory.

So the baked path is a fallback used only when neither `$DOTFILES` nor the canonical pointer applies. Recording the canonical clone on `install all` is what prevents symlink drift across multiple clones; verify with `dfinstall doctor` or `dfinstall diff`.

## Go Dependencies

Dependencies are managed via Go modules (`go.mod` / `go.sum`) and fetched automatically on first build.

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework (commands, flags, completions) |
| `github.com/fatih/color` | Colored terminal output |
| `github.com/charmbracelet/huh` | Interactive terminal forms (extended plugin menu) |
| `golang.org/x/term` | Terminal detection (TTY guard for menus) |
| `gopkg.in/yaml.v3` | YAML config parsing (`.config.yaml`) |

To update dependencies:

```bash
go get -u ./...
go mod tidy
```

## Make Targets

Run `make` (or `make help`) to list every target.

| Target | Command | Description |
|--------|---------|-------------|
| `help` | — | List all targets (the default goal) |
| `build` | `go build ...` | Compile to `bin/dfinstall` |
| `build-mcp` | `go build ...` | Compile the MCP server to `bin/dfinstall-mcp` |
| `build-all` | `build` + `build-mcp` | Compile both binaries |
| `test` | `go test ./src/... ./tests/...` | Run all unit tests |
| `lint` | `go vet ./src/... ./tests/...` | Static analysis |
| `fmt` | `gofmt -s -w src/ tests/` | Format source code in place |
| `fmt-check` | `gofmt -s -l src tests` | Check formatting without modifying (same check as CI) |
| `ci` | `fmt-check` + `lint` + `build-all` + `test` | Run every check CI runs, locally |
| `install` | `make build && bin/dfinstall install all` | Build and install everything |
| `uninstall` | `make build && bin/dfinstall uninstall all` | Remove all managed symlinks |
| `install-bin` | `install ... ~/.local/bin` | Install both binaries onto `PATH` (`~/.local/bin`) |
| `install-mcp` | `claude mcp add -s user ...` | Register the MCP server with Claude Code (user scope, works anywhere) |
| `uninstall-mcp` | `claude mcp remove dfinstall` | Unregister the MCP server from Claude Code |
| `clean` | `rm -rf bin/` | Remove build artifacts |

## Development Workflow

```bash
# Run every check CI runs (fmt-check + vet + build both + test)
make ci

# Test a single module
./bin/dfinstall install shell -v

# Test extended plugin menu
./bin/dfinstall install omz --extended -v

# Run all modules with verbose output
./bin/dfinstall install all -v

# Dry-run mode (preview without changes)
./bin/dfinstall install all --dry-run

# Debug mode (verbose + internal details)
./bin/dfinstall install all --debug
```

## Cross-Compilation

Go supports cross-compilation natively. To build for a different platform:

```bash
# Linux ARM64 (e.g. Raspberry Pi)
GOOS=linux GOARCH=arm64 go build \
  -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=/home/user/dotfiles" \
  -o bin/dfinstall-arm64 ./src/cmd/dfinstall

# Linux AMD64
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=/home/user/dotfiles" \
  -o bin/dfinstall-amd64 ./src/cmd/dfinstall
```

Note: the baked-in dotfiles path should match the target machine. If the path varies, use the `$DOTFILES` environment variable at runtime instead.

## Project Structure

```
src/
  cmd/dfinstall/     # CLI entry point (main.go)
  core/              # Shared libraries
    backup.go        #   Backup/restore system
    banner.go        #   ASCII art startup banner
    config.go        #   YAML config management
    env.go           #   Environment detection (WSL, paths)
    hash.go          #   SHA-256 file hashing
    link.go          #   Symlink creation and checking
    module.go        #   Module interface and registry
    output.go        #   Logging (Info/Ok/Warn/Err/Status/Debug)
    spinner.go       #   Animated progress with pause/resume
  modules/           # One file per module
    register.go        #   Module registration (order matters)
    omz_extended.go    #   Extended plugin menu and file writer
    shell_preserve.go  #   Custom shell file preservation menu and writer
    packages.go        #   Shared package manager helpers (runCmd, installPkg)
    ...                #   19 modules total
tests/                 # Unit tests (17 files)
```

## Testing

Tests use temporary directories and don't touch real system files. Run the full suite with:

```bash
make test
```

| Test file | Coverage |
|-----------|----------|
| `module_test.go` | Module registration order, lookup by name |
| `link_test.go` | Symlink creation, idempotency, repointing, backup, nested dirs |
| `backup_test.go` | Path flattening, system path detection, entry types, dedup, round-trip restore |
| `config_test.go` | Config load/save, missing file defaults, BackupDir override |
| `env_test.go` | WSL detection from /proc/version |
| `status_test.go` | Status line formatting |
| `shell_preserve_test.go` | Custom file scan/filter, managed/non-shell/symlink exclusion, path validation, injection rejection |
| `unlink_test.go` | UnlinkFile: correct/wrong/missing/regular-file cases |
| `dryrun_test.go` | DryRun mode: LinkFile, EnsureDir, UnlinkFile skip filesystem changes |
| `config_skip_test.go` | IsModuleSkipped helper with skip_modules config |

## Continuous Integration

CI runs on every push and pull request to `main` and `develop` via GitHub Actions (`.github/workflows/ci.yml`). Two jobs must pass:

| Job | Checks |
|-----|--------|
| `go` | `gofmt -s` formatting (`make fmt-check`), `go vet` (`make lint`), build of both the `dfinstall` and MCP binaries (`make build` + `make build-mcp`), and the full test suite (`make test`). The Go version tracks `go.mod`. |
| `shellcheck` | Lints the standalone bash scripts in `config/devtools/` and `bootstrap/` via [`ludeeus/action-shellcheck`](https://github.com/ludeeus/action-shellcheck) with `-x --source-path=SCRIPTDIR` so the dynamic `_lib.sh` source resolves. |

The `main` branch is protected: both checks must pass before a pull request can merge.
