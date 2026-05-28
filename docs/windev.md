# Windows Cross-Development

The `windev` module installs everything needed to build Windows software
(C/C++, C#/.NET, Go, Rust) from WSL/Linux — local cross-compilation
toolchains, a remote Windows build-server helper, and matching Neovim LSP /
formatter / debugger configs.

It is **opt-in** and **not part of `install all`** — `install all` would
otherwise drag in MinGW-w64, the .NET SDK, OmniSharp, netcoredbg, and the Rust
windows-gnu target for every machine. Once enabled, it persists in
`.config.yaml` (`windev_enabled: true`) and is re-applied by future `install
all` / `update all` runs so the toolchains stay current.

## Quick start

```bash
# One command: install the toolchains, drop the nvim language config, link the
# winbuild helper, write the PATH snippet, and flip windev_enabled: true.
dfinstall install windev

# Then open a new shell (PATH changes need to be picked up).
exec zsh
```

Verify:

```bash
dfinstall status                       # windev → mingw+dotnet+rust+go
x86_64-w64-mingw32-g++ --version       # MinGW cross-compiler
dotnet --version                       # .NET SDK
rustup target list | grep windows-gnu  # rust target
which gopls dlv                        # Go LSP + debugger
```

Preview without touching anything:

```bash
dfinstall install windev --dry-run -v
```

## What gets installed

### Local cross-compile toolchains

| Language | Installed by windev | Builds Windows binaries via |
|---|---|---|
| **C / C++** | `mingw-w64`, `cmake`, `ninja-build`, `clangd`, `clang-format` (apt) | `x86_64-w64-mingw32-{gcc,g++}` |
| **Go** | `gopls`, `dlv` via `go install` (Go itself comes from the `packages` module) | `GOOS=windows GOARCH=amd64 go build` |
| **Rust** | `rustup` (official installer if absent), target `x86_64-pc-windows-gnu`, components `rust-analyzer` + `rustfmt` + `clippy` | `cargo build --target x86_64-pc-windows-gnu` (links via MinGW) |
| **C# / .NET** | .NET SDK (`dotnet-install.sh` → `~/.dotnet`), OmniSharp + netcoredbg tarballs into `~/.local/share/windev/`, `csharpier` (global dotnet tool) | `dotnet publish -c Release -r win-x64` |

Each language is best-effort: a failure on one (e.g. a network blip during
the .NET download) warns but doesn't abort the rest, so the nvim/helper wiring
still goes in.

### Neovim language support

A single auto-loaded plugin file:

```
config/nvim/windev/windev.lua  →  ~/.config/nvim/lua/custom/plugins/windev.lua
```

Kickstart's `{ import = 'custom.plugins' }` picks it up automatically — no
edits to `init.lua`. `clangd`, `gopls`, and `rust_analyzer` are already in
init.lua's `servers` table and attach as soon as the binaries are on PATH.
The windev file adds:

- **C# LSP** via OmniSharp (binary from `~/.local/share/windev/omnisharp/`).
- **Formatters** (conform): `clang-format` (c/cpp), `csharpier` (cs),
  `goimports`+`gofmt` (go), `rustfmt` (rust).
- **Linters** (nvim-lint): `cpplint` (c/cpp), `golangci-lint` (go),
  `clippy` (rust).
- **DAP adapters**: `codelldb` for C/C++/Rust (if on PATH) and `netcoredbg`
  for C# (from `~/.local/share/windev/`).

The file uses dependency-ordered loading + live runtime-table mutation so it
extends conform / nvim-lint / nvim-dap without clobbering init.lua's existing
setup. `ft = { cs, c, cpp, go, rust }` defers loading until a relevant
filetype is opened.

### Helper script

`~/.local/bin/winbuild` — see [Remote build server](#remote-windows-build-server) below.

### Shell PATH

A generated zsh snippet at `~/.config/dfinstall/windev.zsh`, sourced from
zshrc, prepends `~/.cargo/bin`, `~/.dotnet`, `~/.dotnet/tools`, and
`$(go env GOPATH)/bin` to PATH and exports `DOTNET_ROOT`.

## Building Windows binaries locally

After `install windev` and a fresh shell:

### C / C++

```bash
x86_64-w64-mingw32-g++ -O2 -static -o hello.exe hello.cpp
file hello.exe   # → PE32+ executable (console) x86-64
```

With CMake:

```bash
cmake -S . -B build-win \
  -DCMAKE_SYSTEM_NAME=Windows \
  -DCMAKE_C_COMPILER=x86_64-w64-mingw32-gcc \
  -DCMAKE_CXX_COMPILER=x86_64-w64-mingw32-g++ \
  -G Ninja
cmake --build build-win
```

### Go

Go is a native cross-compiler — no separate toolchain needed:

```bash
GOOS=windows GOARCH=amd64 go build -o app.exe ./cmd/app
```

### Rust

```bash
cargo build --release --target x86_64-pc-windows-gnu
# binary lands at target/x86_64-pc-windows-gnu/release/<name>.exe
```

### C# / .NET

```bash
dotnet publish -c Release -r win-x64 --self-contained false
# bin/Release/<tfm>/win-x64/publish/<app>.exe
```

For a single-file, self-contained binary:

```bash
dotnet publish -c Release -r win-x64 --self-contained true \
  -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true
```

## Neovim — what to expect

Open a `.cs`, `.cpp`, `.go`, or `.rs` file:

- **LSP** attaches automatically (`:LspInfo` shows the active server).
- **Format on save** is wired via conform; trigger manually with `:Format` or
  `:ConformInfo` to see the configured formatters per filetype.
- **Linting** runs on `BufWritePost`/`BufReadPost`; `:lua require'lint'.try_lint()`
  forces a run.
- **Debug**: `:DapContinue` opens the launch picker for codelldb/netcoredbg
  configs (you'll be prompted for the binary/dll path).

Plugins are installed by `lazy.nvim` on first launch after `install windev`;
no manual `:Lazy sync` needed.

## Remote Windows build server

When local cross-compile isn't enough — e.g. you need genuine MSVC, signed
binaries, or per-PR builds on shared infrastructure — `winbuild` dispatches
the build to a Windows host over SSH:

1. `rsync` the project tree up to the host
2. `ssh`-run a configurable `BUILD_CMD` remotely
3. `rsync` artifacts back to `./<artifact-dir>/`

### First-run setup

```bash
winbuild           # no config yet — writes a template and exits
```

Template at `~/.config/dfinstall/winbuild.conf` (sourced as bash):

```bash
HOST="winbuild"                                  # SSH alias from ~/.ssh/config
REMOTE_BASE="C:/builds"                          # OpenSSH-for-Windows accepts forward slashes
BUILD_CMD="msbuild /m /p:Configuration=Release"
ARTIFACT_DIR="build-win"
```

Configure the SSH side in `~/.ssh/config`:

```ssh-config
Host winbuild
    HostName build01.corp.example.com
    User builder
    IdentityFile ~/.ssh/winbuild_ed25519
    ForwardAgent no
```

The Windows host needs OpenSSH Server running (built-in on Windows 10+), the
key authorized, and your toolchain on its `PATH` (Visual Studio + Build Tools
for MSVC, or whichever tools your `BUILD_CMD` invokes).

### Running a build

From the project root:

```bash
winbuild                                                # use BUILD_CMD from config
winbuild --cmd "cmake --build build --config Release"   # one-off command
winbuild --host winbuild2 --artifact-dir bin/Release    # override host + artifact dir
winbuild --dry-run                                      # show the steps, run nothing
```

The remote project directory is computed as `REMOTE_BASE/$(basename $PWD)` —
e.g. `C:/builds/myapp/`. On upload, rsync excludes `.git`, `node_modules`,
`target`, `bin`, `obj`, and the artifact dir.

### Recipes for `BUILD_CMD`

```bash
# MSVC via msbuild
BUILD_CMD="msbuild /m /p:Configuration=Release"

# CMake (multi-config generator)
BUILD_CMD="cmake --build build --config Release"

# .NET publish targeting win-x64
BUILD_CMD='powershell -NoProfile -Command "dotnet publish -c Release -r win-x64"'

# Run a specific PowerShell build script
BUILD_CMD='powershell -ExecutionPolicy Bypass -File build.ps1 -Configuration Release'
```

See [`winbuild` in the Devtools Scripts reference](devtools.md#winbuild) for
the full flag list.

## Updating

```bash
dfinstall install windev      # idempotent — re-runs installs, re-links files
dfinstall update windev       # alias for install
dfinstall install all -v      # also re-applies windev once enabled
```

Re-running picks up newer compiler packages from apt, refreshes `gopls`/`dlv`
to the latest tag, and re-runs `dotnet-install.sh` (which updates the .NET
SDK in place).

## Uninstalling

```bash
dfinstall uninstall windev
```

Removes the nvim plugin file, the `winbuild` symlink, the `windev.zsh` PATH
snippet, and clears `windev_enabled` in `.config.yaml`. Future `install all`
runs skip windev again.

**Deliberately not removed** (other workflows may rely on them): MinGW-w64,
the .NET SDK at `~/.dotnet`, `rustup`/`~/.cargo`, `gopls`/`dlv` in `$GOBIN`,
OmniSharp + netcoredbg in `~/.local/share/windev/`. Remove them manually if
you really want a clean slate:

```bash
sudo apt remove --purge mingw-w64 clangd clang-format cmake ninja-build
rm -rf ~/.dotnet ~/.local/share/windev
rustup target remove x86_64-pc-windows-gnu
```

## Troubleshooting

- **`x86_64-w64-mingw32-g++: command not found` after install.** Open a new
  shell (or `exec zsh`) — apt adds it to PATH, but the running shell hasn't
  picked it up. If it's still missing, `dpkg -l mingw-w64` to confirm the
  package landed and `dfinstall install windev -v` to see the apt log.

- **`dotnet: command not found`.** `dotnet-install.sh` drops the SDK in
  `~/.dotnet`, which is added to PATH by `~/.config/dfinstall/windev.zsh`.
  Make sure your zshrc is sourcing it (`grep windev.zsh ~/.zshrc`) and you've
  opened a new shell. The full binary path is `~/.dotnet/dotnet`.

- **Rust `linker error: x86_64-w64-mingw32-gcc not found`.** windows-gnu
  links via MinGW; ensure the C/C++ portion installed (`apt list --installed
  2>/dev/null | grep mingw-w64`). Re-run `dfinstall install windev`.

- **OmniSharp doesn't attach in nvim.** Check `:LspInfo`. The binary lives at
  `~/.local/share/windev/omnisharp/OmniSharp`; if that file is missing the
  release download failed (network) — re-run `dfinstall install windev`.

- **`winbuild` exits 1 with "No config at ...".** First run writes a template
  and exits. Edit it (set `HOST` at minimum) and re-run.

- **`winbuild` ssh fails with `Host key verification failed`.** Connect once
  manually (`ssh winbuild`) to accept the host key, then re-run.

- **DAP doesn't launch for C/C++/Rust.** `codelldb` isn't installed by the
  windev module (no clean apt package); install it via your distro (`apt
  install lldb` + `codelldb` from VSCode Marketplace) or `:MasonInstall
  codelldb` from inside nvim. The windev lua silently skips the codelldb
  adapter when the binary isn't on PATH.

## Caveats

- **Debugging cross-compiled Windows `.exe` from WSL is limited.** DAP
  targets are wired for native debuggers (delve for Go, codelldb/netcoredbg
  for Linux-native builds). For real Windows debugging, run the binary on
  Windows and attach from there.
- **First install needs network.** `dotnet-install.sh`, OmniSharp, and
  netcoredbg are downloaded from upstream; failures warn rather than abort.
- **All four languages always install.** The module is a bundle, not a
  menu — if you only want one language, you can pass `--dry-run -v` to see
  the full plan, then install just the bits you need manually and leave
  windev disabled.
