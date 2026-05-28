package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// WindevModule installs a cross-development environment for building Windows
// software (C/C++, C#/.NET, Go, Rust) from WSL/Linux — both local
// cross-compilation toolchains and a remote Windows build-server helper — plus
// the matching Neovim LSP/format/debug configs.
//
// It is opt-in: registered like any module so `dfinstall install windev`,
// status, diff and uninstall all work, but excluded from `install all` until
// core.Cfg.WindevEnabled is set (see main.go). Uninstall removes the module's
// own files but deliberately leaves the heavy toolchains in place.
type WindevModule struct{}

func (WindevModule) Name() string { return "windev" }

// windevDir holds module-managed binaries that have no system package
// (OmniSharp, netcoredbg).
func windevDir() string { return core.HomeTarget(".local", "share", "windev") }

// windevLinks are the symlinks the module owns (uninstalled with it). _lib.sh
// is linked separately during install (shared with the devtools module) and is
// intentionally not listed here so uninstall doesn't break devtools scripts.
var windevLinks = []struct{ src, dst string }{
	{"devtools/winbuild", ".local/bin/winbuild"},
}

func windevNvimSrc() string { return core.ConfigPath("nvim", "windev", "windev.lua") }

func windevNvimDst() string {
	return core.XDGTarget("nvim", "lua", "custom", "plugins", "windev.lua")
}

func windevZshPath() string {
	return filepath.Join(core.XDGConfigHome(), "dfinstall", "windev.zsh")
}

func (WindevModule) Install() error {
	core.Info("Setting up Windows cross-development environment...")

	if core.DryRun {
		core.Info("would install MinGW-w64 + cmake/ninja/clangd/clang-format (C/C++ cross-compile)")
		core.Info("would install gopls + delve via 'go install' (Go)")
		core.Info("would add rustup target x86_64-pc-windows-gnu + rust-analyzer/rustfmt/clippy (Rust)")
		core.Info("would install .NET SDK (dotnet-install.sh), OmniSharp, netcoredbg, csharpier (C#)")
		core.Info("would link nvim windev.lua + winbuild helper and write %s", windevZshPath())
		return nil
	}

	// Each language set is best-effort: a failure warns but doesn't abort the
	// rest, so one missing toolchain never blocks the nvim/helper wiring.
	installWindevCpp()
	installWindevGo()
	installWindevRust()
	installWindevDotnet()

	if err := linkWindevFiles(); err != nil {
		return err
	}
	if err := writeWindevPathSnippet(); err != nil {
		core.Warn("failed to write windev PATH snippet: %v", err)
	}

	core.Ok("Windows cross-dev environment ready — open a new shell to pick up PATH changes")
	return nil
}

// --- toolchain installers (best-effort) ---

func installWindevCpp() {
	core.Info("Installing C/C++ Windows cross toolchain (MinGW-w64)...")
	// mingw-w64 provides x86_64-w64-mingw32-{gcc,g++}; clangd/clang-format power
	// the Neovim C/C++ experience; cmake/ninja are common build drivers.
	if err := installPkg("mingw-w64", "cmake", "ninja-build", "clangd", "clang-format"); err != nil {
		core.Warn("C/C++ toolchain install had issues: %v", err)
	}
}

func installWindevGo() {
	// Go cross-compiles to Windows natively (GOOS=windows). Ensure the
	// toolchain, then the LSP (gopls) and debugger (delve).
	if !ensureToolchain("go", "golang", 1, "Go dev tools") {
		return
	}
	goInstall("gopls", "golang.org/x/tools/gopls@latest")
	goInstall("dlv", "github.com/go-delve/delve/cmd/dlv@latest")
}

func goInstall(bin, pkg string) {
	if _, err := exec.LookPath(bin); err == nil {
		core.Ok("%s already installed", bin)
		return
	}
	core.Info("Installing %s via go install...", bin)
	if err := runCmd("go", "install", pkg); err != nil {
		core.Warn("go install %s failed: %v", pkg, err)
	}
}

func installWindevRust() {
	// Reuse toolkit.go's installRustup — it's idempotent, handles curl/PATH,
	// and returns true on success. Returns false (already-warned) if rustup
	// couldn't be installed; skip the rest in that case.
	if _, err := exec.LookPath("rustup"); err != nil {
		if !installRustup() {
			core.Warn("rustup unavailable — skipping Rust windows-gnu setup")
			return
		}
	}
	rustup := rustupExe()
	core.Info("Adding Rust windows-gnu target + components...")
	// windows-gnu links via the MinGW toolchain installed above (no MSVC needed).
	if err := runCmd(rustup, "target", "add", "x86_64-pc-windows-gnu"); err != nil {
		core.Warn("rustup target add failed: %v", err)
	}
	if err := runCmd(rustup, "component", "add", "rust-analyzer", "rustfmt", "clippy"); err != nil {
		core.Warn("rustup component add failed: %v", err)
	}
}

// rustupExe resolves the rustup binary path — PATH first, then ~/.cargo/bin
// (where installRustup deposits it before this process's PATH catches up).
func rustupExe() string {
	if p, err := exec.LookPath("rustup"); err == nil {
		return p
	}
	cand := core.HomeTarget(".cargo", "bin", "rustup")
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return "rustup"
}

func installWindevDotnet() {
	dotnet := dotnetBin()
	if dotnet == "" {
		core.Info("Installing .NET SDK (dotnet-install.sh → ~/.dotnet)...")
		if err := installDotnetSDK(); err != nil {
			core.Warn(".NET SDK install failed: %v — skipping C# setup", err)
			return
		}
		dotnet = dotnetBin()
	} else {
		core.Ok(".NET SDK already installed")
	}
	if dotnet == "" {
		core.Warn("dotnet not found after install — skipping C# tooling")
		return
	}

	// Formatter (global dotnet tool → ~/.dotnet/tools, on PATH via windev.zsh).
	if err := runCmd(dotnet, "tool", "install", "-g", "csharpier"); err != nil {
		core.Debug("csharpier install skipped: %v (likely already installed)", err)
	}
	// LSP + debugger have no apt packages — fetch release tarballs into windevDir.
	installOmnisharp()
	installNetcoredbg()
}

func dotnetBin() string {
	if p, err := exec.LookPath("dotnet"); err == nil {
		return p
	}
	cand := core.HomeTarget(".dotnet", "dotnet")
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return ""
}

func installDotnetSDK() error {
	tmp, err := os.MkdirTemp("", "dotnet-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	script := filepath.Join(tmp, "dotnet-install.sh")
	if err := runCmd("curl", "-fsSL", "-o", script, "https://dot.net/v1/dotnet-install.sh"); err != nil {
		return fmt.Errorf("download dotnet-install.sh: %w", err)
	}
	if err := os.Chmod(script, 0755); err != nil {
		return err
	}
	return runCmd("bash", script, "--channel", "LTS", "--install-dir", core.HomeTarget(".dotnet"))
}

func installOmnisharp() {
	asset := pick(runtime.GOARCH, "omnisharp-linux-x64.tar.gz", "omnisharp-linux-arm64.tar.gz")
	if asset == "" {
		core.Warn("no OmniSharp release for arch %s — skipping C# LSP", runtime.GOARCH)
		return
	}
	core.Info("Installing OmniSharp (C# LSP)...")
	url := "https://github.com/OmniSharp/omnisharp-roslyn/releases/latest/download/" + asset
	if err := fetchTarball("omnisharp", url, windevDir()); err != nil {
		core.Warn("OmniSharp install failed: %v", err)
	}
}

func installNetcoredbg() {
	asset := pick(runtime.GOARCH, "netcoredbg-linux-amd64.tar.gz", "netcoredbg-linux-arm64.tar.gz")
	if asset == "" {
		core.Warn("no netcoredbg release for arch %s — skipping C# debugger", runtime.GOARCH)
		return
	}
	core.Info("Installing netcoredbg (C# debugger)...")
	url := "https://github.com/Samsung/netcoredbg/releases/latest/download/" + asset
	if err := fetchTarball("netcoredbg", url, windevDir()); err != nil {
		core.Warn("netcoredbg install failed: %v", err)
	}
}

// pick returns the amd64 or arm64 value for the current arch, or "" otherwise.
func pick(arch, amd64, arm64 string) string {
	switch arch {
	case "amd64":
		return amd64
	case "arm64":
		return arm64
	}
	return ""
}

// fetchTarball downloads a .tar.gz and extracts it into destDir/<name>,
// wiping any prior extraction first.
func fetchTarball(name, url, destDir string) error {
	if err := core.EnsureDir(destDir); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", name+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	tgz := filepath.Join(tmp, name+".tar.gz")
	if err := runCmd("curl", "-fsSL", "-o", tgz, url); err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}

	target := filepath.Join(destDir, name)
	_ = os.RemoveAll(target)
	if err := core.EnsureDir(target); err != nil {
		return err
	}
	if err := runCmd("tar", "-xzf", tgz, "-C", target); err != nil {
		return fmt.Errorf("extract %s: %w", name, err)
	}
	return nil
}

// --- file wiring ---

func linkWindevFiles() error {
	if err := core.EnsureDir(core.HomeTarget(".local", "bin")); err != nil {
		return err
	}

	// _lib.sh is normally provided by the devtools module; link it here too so
	// winbuild works even when windev is installed standalone. Same src means
	// the link is idempotent with devtools and isn't touched on uninstall.
	libSrc := core.ConfigPath("devtools", "_lib.sh")
	if err := os.Chmod(libSrc, 0755); err != nil {
		core.Warn("chmod failed for _lib.sh: %v", err)
	}
	if err := core.LinkFile(libSrc, core.HomeTarget(".local", "bin", "_lib.sh")); err != nil {
		core.Warn("could not link _lib.sh: %v", err)
	}

	for _, l := range windevLinks {
		src := core.ConfigPath(l.src)
		if err := os.Chmod(src, 0755); err != nil {
			core.Warn("chmod failed for %s: %v", l.src, err)
		}
		if err := core.LinkFile(src, core.HomeTarget(l.dst)); err != nil {
			return err
		}
	}

	// nvim plugin file — auto-loaded by kickstart's { import = 'custom.plugins' }.
	if err := core.EnsureDir(core.XDGTarget("nvim", "lua", "custom", "plugins")); err != nil {
		return err
	}
	return core.LinkFile(windevNvimSrc(), windevNvimDst())
}

// writeWindevPathSnippet writes a zsh fragment (sourced by zshrc) that puts the
// language toolchains on PATH. Regenerated on every install; no user input, so
// no name validation is needed.
func writeWindevPathSnippet() error {
	dir := filepath.Dir(windevZshPath())
	if err := core.EnsureDir(dir); err != nil {
		return err
	}
	const content = `# Generated by dfinstall (windev module) — Windows cross-dev PATH.
# Regenerated on each 'dfinstall install windev'; edits will be overwritten.
typeset -U path
[[ -d "$HOME/.cargo/bin" ]] && path=("$HOME/.cargo/bin" $path)
if [[ -d "$HOME/.dotnet" ]]; then
  export DOTNET_ROOT="$HOME/.dotnet"
  path=("$HOME/.dotnet" $path)
fi
[[ -d "$HOME/.dotnet/tools" ]] && path=("$HOME/.dotnet/tools" $path)
if command -v go >/dev/null 2>&1; then
  _gobin="$(go env GOPATH 2>/dev/null)/bin"
  [[ -d "$_gobin" ]] && path=("$_gobin" $path)
  unset _gobin
fi
export PATH
`
	return os.WriteFile(windevZshPath(), []byte(content), 0644)
}

func (WindevModule) Uninstall() error {
	for _, l := range windevLinks {
		if err := core.UnlinkFile(core.ConfigPath(l.src), core.HomeTarget(l.dst)); err != nil {
			return err
		}
	}
	if err := core.UnlinkFile(windevNvimSrc(), windevNvimDst()); err != nil {
		return err
	}
	if !core.DryRun {
		if err := os.Remove(windevZshPath()); err != nil && !os.IsNotExist(err) {
			core.Warn("could not remove %s: %v", windevZshPath(), err)
		}
	}
	core.Ok("windev unlinked (language toolchains left installed)")
	return nil
}

func (WindevModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "windev"}
	if !core.Cfg.WindevEnabled {
		s.Extra = "disabled"
		return s
	}

	// Owned symlinks (winbuild + nvim plugin file).
	checkLink := func(src, dst string) {
		if core.CheckLink(src, dst) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	for _, l := range windevLinks {
		checkLink(core.ConfigPath(l.src), core.HomeTarget(l.dst))
	}
	checkLink(windevNvimSrc(), windevNvimDst())

	// Toolchain summary (rough — PATH is fully wired only in a new shell).
	hasBin := func(b string) bool { _, err := exec.LookPath(b); return err == nil }
	var have []string
	for _, c := range []struct {
		ok    bool
		label string
	}{
		{hasBin("x86_64-w64-mingw32-gcc"), "mingw"},
		{dotnetBin() != "", "dotnet"},
		{hasBin("rustup") || rustupInstalled(), "rust"},
		{hasBin("go"), "go"},
	} {
		if c.ok {
			have = append(have, c.label)
		}
	}
	if len(have) > 0 {
		s.Extra = strings.Join(have, "+")
	}
	return s
}

func rustupInstalled() bool {
	_, err := os.Stat(core.HomeTarget(".cargo", "bin", "rustup"))
	return err == nil
}
