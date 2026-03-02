# Building from Source

How to compile `dfinstall` from source and set up a development environment.

## Requirements

| Requirement | Minimum | Notes |
|-------------|---------|-------|
| **Go** | 1.24+ | [Install Go](https://go.dev/doc/install) |
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

The `-ldflags` flag bakes the dotfiles directory path into the binary at compile time. This allows `dfinstall` to locate its `config/` directory regardless of where it's invoked from. If omitted, the binary falls back to environment variables, walking up from the executable, and the current directory.

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

| Target | Command | Description |
|--------|---------|-------------|
| `build` | `go build ...` | Compile to `bin/dfinstall` |
| `test` | `go test ./src/... ./tests/...` | Run all unit tests |
| `lint` | `go vet ./src/... ./tests/...` | Static analysis |
| `fmt` | `gofmt -s -w src/ tests/` | Format source code |
| `install` | `make build && bin/dfinstall install all` | Build and install everything |
| `clean` | `rm -rf bin/` | Remove build artifacts |

## Development Workflow

```bash
# Build and test
make build && make test && make lint

# Test a single module
./bin/dfinstall install shell -v

# Test extended plugin menu
./bin/dfinstall install omz --extended -v

# Run all modules with verbose output
./bin/dfinstall install all -v

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
    register.go      #   Module registration (order matters)
    omz_extended.go  #   Extended plugin menu and file writer
    packages.go      #   Shared package manager helpers (runCmd, installPkg)
    ...              #   14 modules total
tests/               # Unit tests (6 files)
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
