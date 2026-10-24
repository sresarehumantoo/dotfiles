# Building from Source

How to compile `dfinstall` from source and set up a development environment.

## Requirements

| Requirement | Minimum | Notes |
|-------------|---------|-------|
| **Go** | 1.25.5 | The version declared in `go.mod`. [Install Go](https://go.dev/doc/install) |
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

## Version stamping

The Makefile passes a second `-X` for `core.Version`, derived from the tag:

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
```

Both binaries read it — `dfinstall --version`, and the `serverInfo.version` the
MCP server reports in its `initialize` response. A plain `go build` without the
Makefile leaves it at `dev` rather than claiming a release.

**Anything that reports a version must read `core.Version`.** The MCP server
previously announced a hardcoded `"1.0.0"` literal that no release process
bumped, so it would have kept reporting `1.0.0` indefinitely.

```console
$ make build && ./bin/dfinstall --version
dfinstall version v1.0.0
```

## Releases

Pushing a `v*` tag runs `.github/workflows/release.yml`:

1. **GoReleaser** (`.goreleaser.yaml`) builds both binaries for linux and darwin on amd64 and arm64 and publishes the GitHub release: one `dfinstall_<version>_<os>_<arch>.tar.gz` per platform, a `.deb` per Linux arch, and `checksums.txt`. Each binary is built twice, with `core.Channel=release` for the archives and `core.Channel=deb` for the packages. ⚠ `modules.releaseArchiveName` looks for exactly that archive name to self-update, so the `name_template` must not change without it.
2. **Homebrew**: `scripts/release/brew-formula.sh` writes `Formula/dfinstall.rb` from `checksums.txt` and pushes it to `sresarehumantoo/homebrew-tap`. The formula installs the release archive, and a release binary running from a Cellar reports the `brew` channel. GoReleaser's own formula support is deprecated in favor of casks, which quarantine what they download.
3. **apt**: `scripts/release/apt-repo.sh` adds the `.deb`s to a signed repository (suite `stable`, component `main`, newest three versions kept) on the `gh-pages` branch under `apt/`, served by GitHub Pages.

Tags containing `-` (`v2.0.0-rc1`) are GitHub prereleases and skip steps 2 and 3. CI runs `goreleaser check` on every push, so a broken config fails before a tag does.

One-time setup the workflow needs, without which steps 2 and 3 skip with a notice:

| What | Where |
|---|---|
| `sresarehumantoo/homebrew-tap` repository | GitHub |
| `HOMEBREW_TAP_GITHUB_TOKEN`: a fine-grained token with contents write on the tap only | repo Actions secret |
| `APT_SIGNING_KEY`: ASCII-armored private key of a dedicated, passphrase-less signing subkey | repo Actions secret |
| GitHub Pages from the `gh-pages` branch | repo settings |

To try a release locally: `goreleaser release --snapshot --clean` writes everything to `dist/` without publishing. The snapshot version comes from the newest tag reachable from `HEAD`.

## Go Dependencies

Dependencies are managed via Go modules (`go.mod` / `go.sum`) and fetched automatically on first build.

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework (commands, flags, completions) |
| `github.com/fatih/color` | Colored terminal output |
| `github.com/charmbracelet/huh` | Interactive terminal forms (extended plugin menu) |
| `github.com/charmbracelet/bubbles` | Terminal UI components used by the menus |
| `github.com/mark3labs/mcp-go` | MCP protocol/stdio server (`bin/dfinstall-mcp`) |
| `golang.org/x/term` | Terminal detection (TTY guard for menus) |
| `gopkg.in/yaml.v3` | YAML config parsing (`config.yaml`) |

`go.mod` is the authority on versions; the seven above are the direct requirements.

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
| `test-race` | `go test -race ./src/... ./tests/...` | Run the suite under the race detector |
| `lint` | `go vet ./src/... ./tests/...` | Static analysis |
| `fmt` | `gofmt -s -w src/ tests/` | Format source code in place |
| `fmt-check` | `gofmt -s -l src tests` | Check formatting without modifying (same check as CI) |
| `ci` | `fmt-check` + `lint` + `build-all` + `test` + `test-race` | Run every check CI runs, locally |
| `e2e` | `scripts/e2e/run.sh <scenario>` | End-to-end scenarios in Docker; `SCENARIO=sudo` runs one |
| `corplab` | `scripts/corplab/run.sh [scenario]` | Corporate-network scenarios (TLS inspection, Nexus) in Docker Compose; `SCENARIO=nexus` runs one |
| `install` | `make build && bin/dfinstall install all` | Build and install everything |
| `uninstall` | `make build && bin/dfinstall uninstall all` | Remove all managed symlinks |
| `install-bin` | `install ... ~/.local/bin` | Install both binaries onto `PATH` (`~/.local/bin`) |
| `install-mcp` | `claude mcp add -s user ...` | Register the MCP server with Claude Code (user scope, works anywhere) |
| `uninstall-mcp` | `claude mcp remove dfinstall` | Unregister the MCP server from Claude Code |
| `clean` | `rm -rf bin/` | Remove build artifacts |

## Development Workflow

```bash
# Run every check CI runs (fmt-check + vet + build both + test + race)
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
  cmd/mcp/           # MCP server entry point (main.go) → bin/dfinstall-mcp
  core/              # Shared libraries
    apt.go           #   apt/dpkg helpers
    backup.go        #   Backup/restore system
    banner.go        #   ASCII art startup banner
    canonical.go     #   Canonical dotfiles-dir pointer
    config.go        #   YAML config management
    drift.go         #   Symlink drift detection across clones
    env.go           #   Environment detection (WSL, paths, sudo)
    exec.go          #   Subprocess timeouts (Probe/Network/Install)
    hash.go          #   SHA-256 file hashing
    install_session.go #  BeginInstall/Finish: adoption, backup, config
    link.go          #   Symlink creation and checking
    module.go        #   Module interface and registry
    output.go        #   Logging (Info/Ok/Warn/Err/Status/Debug)
    registry.go      #   Toolkit registry fetch/cache/validate
    spinner.go       #   Animated progress with pause/resume
    version.go       #   core.Version, stamped at build time
    virt.go          #   Virtualisation detection
  modules/           # One file per module
    register.go        #   Module registration (order matters)
    omz_extended.go    #   Extended plugin menu and file writer
    shell_preserve.go  #   Custom shell file scan and custom-sources.zsh writer
    packages.go        #   Shared package manager helpers (runCmd, installPkg)
    github_release.go  #   GitHub release lookup and verified download, through a Nexus mirror when found
    mirrors.go         #   npm/pip/Go/cargo mirror detection (doctor's package mirrors row)
    json_settings.go   #   managedJSON: merge tracked keys into a settings file a program also writes
    ...                #   19 modules total
tests/                 # Black-box tests (package tests)
                       # In-package tests also live in src/core/*_test.go
                       # and src/modules/*_test.go for unexported internals
```

## Testing

Tests use temporary directories and don't touch real system files. Run the full suite with:

```bash
make test
```

A selection of the black-box suite in `tests/` — not the full list, see the
directory for the rest:

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
| `install_session_test.go` | SkipInAll (windev and opencode opt-ins, skip_modules), SetOptIn/ClearOptIn, BeginInstall backup policy |
| `opencode_test.go` | opencode settings merge, env pin, overriding config files, dry run, uninstall, the shipped config's security keys |
| `install_session_state_test.go` | Canonical pointer writes under/without dry-run, a failed first run not persisting `skip_backup`, idempotent Finish |
| `canonical_test.go` | LinkRoot, canonical pointer read/write + self-heal, DotfilesDir precedence, adoption, drift split |
| `homepath_propagation_test.go` | Unresolvable `$HOME`: SubPath propagation, CheckTarget refusals, empty path helpers |
| `registry_inspect_test.go` | InspectRegistry doesn't write the cache, still validates; FetchRegistry does |

### End-to-end scenarios

The unit tests cannot see what needs a real system: sudo prompts, root-owned
files, apt. `make e2e` runs each script in `scripts/e2e/scenarios/` against the
working tree in a throwaway `debian:trixie` container (`scripts/e2e/Dockerfile`),
with the repo mounted read-only. They install real packages, so expect several
minutes each; CI does not run them.

| Scenario | Checks |
|----------|--------|
| `sudo` | How many sudo password prompts each command causes (none for anything installed into `~`, one per run otherwise), that dfinstall drops credentials it obtained and keeps ones the user already had, and where Neovim, tree-sitter, delta and Terraform land |
| `bootstrap` | `bootstrap/wsl-setup.sh` in the order `wsl-bootstrap.ps1` runs it: the temporary NOPASSWD drop-in, `install all` without a password, and its removal plus the expired fallback password at the end |
| `rootless` | `install extras` with a systemd user session. A user already in the `docker` group is left as they are, with no rootless packages; for anyone else Docker runs rootless with its own context, a re-run asks nothing, `sudo docker` still reaches the system daemon, and the user never joins the `docker` group |

A scenario whose script contains the line `# e2e: systemd` runs in a privileged
container with systemd as PID 1 instead, so `su -` opens a real login session
with a user manager; `rootless` needs that.

`scripts/e2e/session.exp` is the reusable part. It runs `label|command` steps in
one shell on one pty, so sudo's per-terminal credential cache behaves as it does
for a person, answers dfinstall's own questions with their defaults, and
reports each step's prompt count and exit code for the scenario to assert on.
A new scenario is a script that writes a steps file and checks the results with
`expect_step` and `check` from `scripts/e2e/lib.sh`.

### Corporate network lab

`make corplab` (`scripts/corplab/run.sh [scenario...]`) runs dfinstall and the
bootstraps on a simulated corporate network, built with Docker Compose:

- **dev**, the workstation, on an internal network with no route out except
  through **proxy**.
- **proxy**, an inline firewall. It decrypts HTTPS by re-signing every
  connection with the lab's own root CA (generated into
  `scripts/corplab/.state/ca` as PEM `.crt` and DER `.cer`, never committed),
  filters hosts with a block page or a reset, and answers DNS. A scenario sets
  the filter with `policy RULE...`; the rules are described in
  `proxy/filter.py`.
- **nexus**, Sonatype Nexus CE on the internal network, proxying npm, PyPI, Go,
  `github.com` and `api.github.com`. `setup-nexus.sh` configures it; the UI is
  on `http://127.0.0.1:8081`.

The first start takes a few minutes while Nexus initializes; the lab is left
running for the next run (`--down` stops it, `docker compose -f
scripts/corplab/compose.yaml down -v` removes it with Nexus's cache).

| Scenario | Checks |
|----------|--------|
| `network` | The lab itself: the stock trust store rejects intercepted connections, the lab CA verifies them, block pages, resets and exemptions |
| `tls-inspection` | `wsl-setup.sh check-tls` before and after `install-cert` of the DER certificate; curl, git, node, npm, pip, `dfinstall install delta`/`opencode` and opencode's own API calls, first with the CA in the system store, then without root through the per-user variables `macos-setup.sh` writes |
| `nexus` | The internet blocked outright: the `package mirrors` doctor row, opencode from npm through Nexus, delta from the GitHub release through Nexus's GitHub proxy, and a clear failure once that proxy is offline too |
| `ghostty-macos` | The macOS bootstrap's Ghostty step: Homebrew while Ghostty's site answers, the verified GitHub nightly once it is blocked, and a clear failure when GitHub is blocked as well. `brew`, `ditto` and `shasum` are stand-ins; the bootstrap code is not |

## Continuous Integration

CI runs on every push and pull request to `main` and `develop` via GitHub Actions (`.github/workflows/ci.yml`). Two jobs must pass:

| Job | Checks |
|-----|--------|
| `go` | `gofmt -s` formatting (`make fmt-check`), `go vet` (`make lint`), build of both the `dfinstall` and MCP binaries (`make build` + `make build-mcp`), the full test suite (`make test`), and the suite again under the race detector (`make test-race`). The Go version tracks `go.mod`. |
| `shellcheck` | Lints the standalone bash scripts in `config/devtools/` and `bootstrap/` via [`ludeeus/action-shellcheck`](https://github.com/ludeeus/action-shellcheck) with `-x --source-path=SCRIPTDIR` so the dynamic `_lib.sh` source resolves. |

The `main` branch is protected: both checks must pass before a pull request can merge.
