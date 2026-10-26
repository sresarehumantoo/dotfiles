# Building from Source

How to compile `dfinstall` from source and set up a development environment.

## Requirements

| Requirement | Minimum | Notes |
|-------------|---------|-------|
| **Go** | 1.26 | The version declared in `go.mod`. [Install Go](https://go.dev/doc/install) |
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
  -ldflags "-X github.com/sresarehumantoo/dotfiles/v2/internal/core.DefaultDotfilesDir=$(pwd)" \
  -o bin/dfinstall ./cmd/dfinstall
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

Releases are cut by promoting `develop` to `main` with a PR, then tagging the `develop` side of that merge:

```bash
gh pr create --base main --head develop   # CI green, then:
gh pr merge
git fetch origin
git diff --quiet origin/main origin/main^2 && git tag v1.11.0 origin/main^2
git push origin v1.11.0
```

`origin/main^2` has exactly the tree `main` has, and unlike the merge commit it is also on `develop`, so `git describe` (and Go's version stamp) on a `develop` build names the latest release. Tags before `v1.11.0` sit on the merge commits instead, which is why `develop` merged `main` back once. The workflow refuses a tag that is not reachable from both branches.

Pushing a `v*` tag runs `.github/workflows/release.yml`:

1. **GoReleaser** (`.goreleaser.yaml`) builds both binaries for linux and darwin on amd64 and arm64 and publishes the GitHub release: one `dfinstall_<version>_<os>_<arch>.tar.gz` per platform, a `.deb` per Linux arch, and `checksums.txt`. Each binary is built twice, with `core.Channel=release` for the archives and `core.Channel=deb` for the packages. ⚠ `modules.releaseArchiveName` looks for exactly that archive name to self-update, so the `name_template` must not change without it.
2. **Homebrew**: `scripts/release/brew-formula.sh` writes `Formula/dfinstall.rb` from `checksums.txt` and pushes it to `sresarehumantoo/homebrew-tap`. The formula installs the release archive, and a release binary running from a Cellar reports the `brew` channel. GoReleaser's own formula support is deprecated in favor of casks, which quarantine what they download.
3. **apt**: `scripts/release/apt-repo.sh` adds the `.deb`s to a signed repository (suite `stable`, component `main`, newest three versions kept) on the `gh-pages` branch under `apt/`, served by GitHub Pages.

Tags containing `-` (`v2.0.0-rc1`) are GitHub prereleases and skip steps 2 and 3. CI runs `goreleaser check` on every push, so a broken config fails before a tag does.

One-time setup the workflow needs, without which steps 2 and 3 skip with a notice:

| What | Where |
|---|---|
| `sresarehumantoo/homebrew-tap` repository | GitHub |
| `HOMEBREW_TAP_DEPLOY_KEY`: private half of an SSH key whose public half is a write deploy key on the tap only | repo Actions secret |
| `APT_SIGNING_KEY`: ASCII-armored private key of the passphrase-less ed25519 signing key `68923E9D2B5D025E4EB9FBCE48E2D3B9E6EE8E62` (no expiry: an expired key makes apt reject the repository) | repo Actions secret; the secret cannot be read back, so keep a backup of the key outside GitHub |
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
| `test` | `go test ./cmd/... ./internal/... ./tests/...` | Run all unit tests |
| `test-race` | `go test -race ./cmd/... ./internal/... ./tests/...` | Run the suite under the race detector |
| `lint` | `go vet ./cmd/... ./internal/... ./tests/...` | Static analysis |
| `fmt` | `gofmt -s -w cmd/ internal/ tests/` | Format source code in place |
| `fmt-check` | `gofmt -s -l cmd internal tests` | Check formatting without modifying (same check as CI) |
| `check-decoupling` | `scripts/check-decoupling.sh` | Fail if site-specific content leaked into tracked files |
| `ci` | `fmt-check` + `lint` + `check-decoupling` + `build-all` + `test` + `test-race` | Run every check CI runs, locally |
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
# Run every check CI's Go job runs (fmt-check + vet + decoupling + build both + test + race)
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
  -ldflags "-X github.com/sresarehumantoo/dotfiles/v2/internal/core.DefaultDotfilesDir=/home/user/dotfiles" \
  -o bin/dfinstall-arm64 ./cmd/dfinstall

# Linux AMD64
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X github.com/sresarehumantoo/dotfiles/v2/internal/core.DefaultDotfilesDir=/home/user/dotfiles" \
  -o bin/dfinstall-amd64 ./cmd/dfinstall
```

Note: the baked-in dotfiles path should match the target machine. If the path varies, use the `$DOTFILES` environment variable at runtime instead.

## Project Structure

```
cmd/
  dfinstall/         # CLI entry point (main.go)
  dfinstall-mcp/     # MCP server entry point (main.go) → bin/dfinstall-mcp
internal/
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
    virt.go          #   Virtualization detection
  modules/           # One file per module
    register.go        #   Module registration (order matters)
    omz_extended.go    #   Extended plugin menu and file writer
    shell_preserve.go  #   Custom shell file scan and custom-sources.zsh writer
    packages.go        #   Shared package manager helpers (runCmd, installPkg)
    github_release.go  #   GitHub release lookup and verified download, through a Nexus mirror when found
    mirrors.go         #   npm/pip/Go/cargo mirror detection (doctor's package mirrors row)
    json_settings.go   #   managedJSON: merge tracked keys into a settings file a program also writes
    ...                #   one file per module, 28 in all
tests/                 # Black-box tests (package tests)
                       # In-package tests also live in internal/core/*_test.go
                       # and internal/modules/*_test.go for unexported internals
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
| `self-update` | A release binary on the blocked network: the published v2.0.1 archive fetched through Nexus, `dfinstall update` finding the latest release and replacing both binaries through Nexus's GitHub proxies, then nothing newer on a second run, and with both proxies offline a warning, the old binary kept and a clean exit |

### macOS harness

`scripts/macos-harness/run.sh [fresh|upgrade|all]` tests the macOS bootstrap and dfinstall on a Mac that is already set up and in daily use, without changing it. No Mac in this project is a clean machine, and uninstalling a work Mac's tooling to test from scratch is not an option.

- **`fresh`** runs `bootstrap/macos-setup.sh --from-source` and `dfinstall install all` as a new account would: `HOME`, every `XDG_*` and `TMPDIR` under `/tmp/dfinstall-macos-harness-<time>/`, and a Homebrew that installs nothing. Each formula and cask it is asked for is looked up in the real Homebrew, read-only, so a misspelled name fails as it would for real, and the list is in the report.
- **`upgrade`** runs this checkout's dfinstall against the real setup with read-only commands only (`status`, `doctor`, `diff`, `install all --dry-run`), plus the bootstrap's preflight and TLS-inspection probe, sourced, and checks whether curl, git, Python, Node, npm, pip, Homebrew and Go reach the network as the account is configured (its own registries, proxy and CA variables; caches in the sandbox), and which of the hosts releases and installs use answer.
- **The summary** at the end is about ten lines of public hosts and generic labels (mirrors show as `mirror`, never their hostname), so it can be photographed or retyped off a machine files cannot leave. `report.md` beside it has everything else.
- **Shims** stand in for every command that acts on the whole machine or the login session whatever `HOME` says: `defaults` writes, `launchctl`, `hidutil`, `chsh`, `sudo`, `activateSettings`, `osascript`, `ditto` and more. They record each call, and reads go through to the real tool, except a sandboxed app's preferences: reading another app's container needs Full Disk Access and can raise a privacy prompt, so the shim refuses and records it. ⚠ A shim only works for a command run by name, and only ahead of the Homebrew `bin` that dfinstall puts first on `PATH`; anything new run by absolute path (as `activateSettings` was) needs a PATH lookup first.
- **The guarantee**: the real state dfinstall manages is snapshotted before and after (dotfile links, `~/.config/dfinstall`, the dotfiles clone, the `defaults` domains it writes, key remapping, launch agents, login shell, Homebrew's package list). Any difference fails the run.

Not covered, because it cannot be sandboxed: real package installs, apps in `/Applications`, and the effect of `defaults`/`hidutil`/`launchctl` calls (the report lists which would have been made). `--interactive` keeps the terminal so prompts can be answered; review `report.md` before sharing it.

### macOS CI

GitHub's macOS runners are throwaway Macs with passwordless sudo, so `.github/workflows/macos.yml` does for real what the harness can only record:

| Job | Runner | What it proves |
|---|---|---|
| Install harness | Apple Silicon, Intel | the harness itself leaves a real Mac unchanged, and (`--strict`) every formula and cask dfinstall asks for exists |
| Homebrew formula | Apple Silicon, Intel | the snapshot release installs with the real Homebrew from a tap, reports the `brew` channel, passes `brew test`/`audit --strict`/`style`, and `update` defers to `brew upgrade` |
| Fresh install as a standard account | Apple Silicon | `scripts/macos-ci/fresh-user.sh`: a new non-admin user (the managed work Mac's shape) bootstraps from nothing with a Homebrew of its own, casks land in `~/Applications`, a second install changes nothing, uninstall removes every link |
| macOS-only modules | Apple Silicon | `scripts/macos-ci/mac-modules.sh`: mackeys, alttab, sketchybar and aerospace installed and removed for real, checking the defaults keys, launch agents, Homebrew service and links each leaves behind |
| TLS inspection | Apple Silicon | `scripts/macos-ci/tls-inspection.sh`: mitmproxy's CA trusted in the System keychain as MDM would install a proxy root; the bootstrap detects the inspection and wires curl, git, Node, pip and Homebrew, and Go verifies from the keychain alone |

None of them can stand in for MDM, Santa, a corporate Nexus, or a particular Mac's existing setup: that is the harness's `upgrade` scenario, run on the Mac itself.

## Continuous Integration

CI runs on every push and pull request to `main` and `develop`, and on manual dispatch, via GitHub Actions (`.github/workflows/ci.yml`):

| Job | Checks |
|-----|--------|
| `go` | `gofmt -s` formatting (`make fmt-check`), `go vet` (`make lint`), the decoupling guard (`make check-decoupling`), build of both the `dfinstall` and MCP binaries (`make build` + `make build-mcp`), the full test suite (`make test`), and the suite again under the race detector (`make test-race`). The Go version tracks `go.mod`. |
| `go-macos` | Builds both binaries and runs `make test` on `macos-latest`, then checks that `bootstrap/macos-setup.sh` parses and runs `--help` under the stock `/bin/bash` 3.2. |
| `goreleaser` | `goreleaser check`, so a broken `.goreleaser.yaml` fails here rather than on the next tag. |
| `shellcheck` | Lints `config/devtools/`, `bootstrap/` and `scripts/`, one step each, via [`ludeeus/action-shellcheck`](https://github.com/ludeeus/action-shellcheck) with `-x --source-path=SCRIPTDIR` so the dynamic `_lib.sh` source resolves. Any finding fails the job, a `note` included, and each step scans its whole directory rather than the diff. `make ci` does not run it. |

The `main` branch is protected: `go`, `go-macos` and `shellcheck` must pass before a pull request can merge.
