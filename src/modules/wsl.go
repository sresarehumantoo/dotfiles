package modules

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type WslModule struct{}

func (WslModule) Name() string { return "wsl" }

func (WslModule) Install(ctx context.Context) error {
	if !core.IsWSL() {
		core.Ok("Not running in WSL, skipping")
		return nil
	}

	if core.DryRun {
		core.Info("would configure WSL: wsl.conf, sysctl, Windows exe shims, wslconfig (host-sized), win-home symlink, git untracked cache, ghostty shader choice")
		return nil
	}

	core.Info("Configuring WSL environment...")

	wslConfChanged := installWslConf(ctx)
	installSysctl(ctx)

	// interop-dependent steps need to actually launch a Windows process, which
	// requires interop to be enabled and WSL restarted after a wsl.conf change.
	if hasInterop() {
		// Shims first: everything below, and every devtools script, addresses
		// these by name once appendWindowsPath=false takes effect.
		installWinShims()
		installWslconfig(ctx)
		linkWinHome()
	} else {
		core.Warn("Windows interop not available — skipping shims, .wslconfig and the home symlink.")
		core.Warn("From PowerShell run: wsl --shutdown")
		core.Warn("Then relaunch and run: dfinstall install wsl")
	}

	configureGitCache(ctx)
	configureGhosttyShader(ctx)

	if wslConfChanged {
		core.Notice("/etc/wsl.conf changed — run `wsl --shutdown` from PowerShell for it to take effect.")
		core.Notice("That is what switches off the Windows PATH; the shims above already cover it.")
	}

	return nil
}

// installWslConf installs /etc/wsl.conf and returns true if the file was changed.
func installWslConf(ctx context.Context) bool {
	wslConf := core.ConfigPath("wsl", "wsl.conf")
	if _, err := os.Stat(wslConf); err != nil {
		return false
	}

	// Rendered, not copied: the template carries @DEFAULT_USER@ rather than a
	// literal username. renderedWslConf is the single source of truth that
	// Status and the doctor check also use.
	srcData, err := renderedWslConf()
	if err != nil {
		return false
	}
	if user := currentUsername(); user != "" {
		core.Info("wsl.conf default user: %s", user)
	} else {
		core.Warn("Could not determine the installing user — /etc/wsl.conf will omit `default=`")
	}

	dstPath := "/etc/wsl.conf"
	if dstData, err := os.ReadFile(dstPath); err == nil {
		if bytes.Equal(srcData, dstData) {
			core.Ok("/etc/wsl.conf already up to date")
			return false
		}
		core.Notice("Updating /etc/wsl.conf (backing up to /etc/wsl.conf.bak)")
		// Don't overwrite the original if we couldn't back it up first.
		if err := sudoCopy(ctx, dstPath, dstPath+".bak"); err != nil {
			core.Warn("could not back up %s: %v — leaving it unchanged", dstPath, err)
			return false
		}
	}

	// The rendered content differs from the file on disk, so this cannot be a
	// plain `sudo cp` of the template. Stage it and move it into place.
	staged, err := os.CreateTemp("", "wsl.conf-*")
	if err != nil {
		core.Warn("could not stage %s: %v", dstPath, err)
		return false
	}
	defer os.Remove(staged.Name())
	if _, err := staged.Write(srcData); err != nil {
		staged.Close()
		core.Warn("could not stage %s: %v", dstPath, err)
		return false
	}
	if err := staged.Close(); err != nil {
		core.Warn("could not stage %s: %v", dstPath, err)
		return false
	}

	if err := sudoCopy(ctx, staged.Name(), dstPath); err != nil {
		core.Warn("failed to install %s: %v", dstPath, err)
		return false
	}
	// CreateTemp makes the file 0600 and owned by the installing user; /etc/wsl.conf
	// is read by WSL as root before login and must be world-readable.
	if err := sudoRun(ctx, "chmod", "0644", dstPath); err != nil {
		core.Warn("could not chmod %s: %v", dstPath, err)
	}
	core.Ok("/etc/wsl.conf installed")
	return true
}

func installSysctl(ctx context.Context) {
	sysctlSrc := core.ConfigPath("wsl", "99-wsl-sysctl.conf")
	if _, err := os.Stat(sysctlSrc); err != nil {
		return
	}

	srcData, err := os.ReadFile(sysctlSrc)
	if err != nil {
		return
	}

	sudoRun(ctx, "mkdir", "-p", "/etc/sysctl.d")

	dstPath := "/etc/sysctl.d/99-wsl.conf"
	if dstData, err := os.ReadFile(dstPath); err == nil {
		if bytes.Equal(srcData, dstData) {
			core.Ok("sysctl config already up to date")
			return
		}
	}

	if err := sudoCopy(ctx, sysctlSrc, dstPath); err != nil {
		core.Warn("failed to install %s: %v", dstPath, err)
		return
	}
	core.Info("Applying sysctl tweaks...")
	if err := sudoRun(ctx, "sysctl", "-p", dstPath); err != nil {
		core.Warn("Some sysctl values may not apply until restart")
	}
	core.Ok("/etc/sysctl.d/99-wsl.conf installed")
}

// resolveWinHome returns the WSL mount path for the Windows user home directory
// (e.g. /mnt/c/Users/<username>), or empty string on failure.
// Shared with doctor checks, which have no context — bounded by ProbeTimeout.
//
// The cmd.exe call is the one that talks to Windows, so it is the one that
// hangs when interop stalls; it has to go through runProbe like the wslpath
// call below, or Status() and doctor block forever.
//
// ⚠ cmd.exe is addressed by ABSOLUTE PATH, not by name. With
// appendWindowsPath=false a bare `cmd.exe` no longer resolves, and the bare
// form here used to make Status() and doctor report "could not resolve Windows
// home" on a completely healthy machine.
//
// Memoized: Install, Status and doctor each call this, and every miss is a
// Windows process launch.
var (
	winHomeOnce   sync.Once
	winHomeCached string
)

func resolveWinHome() string {
	winHomeOnce.Do(func() { winHomeCached = probeWinHome() })
	return winHomeCached
}

func probeWinHome() string {
	cmdExe := winBinary("cmd.exe")
	if cmdExe == "" {
		return ""
	}

	out, err := runProbe(context.Background(), cmdExe, "/C", "echo %USERPROFILE%")
	if err != nil {
		return ""
	}

	winUserDir := strings.TrimSpace(strings.ReplaceAll(string(out), "\r", ""))
	if winUserDir == "" {
		return ""
	}

	wslPath, err := runProbe(context.Background(), "wslpath", winUserDir)
	if err != nil {
		return ""
	}

	resolved := strings.TrimSpace(string(wslPath))
	if fi, err := os.Stat(resolved); err != nil || !fi.IsDir() {
		return ""
	}
	return resolved
}

func installWslconfig(ctx context.Context) {
	wslconfigSrc := core.ConfigPath("wsl", "wslconfig")
	if _, err := os.Stat(wslconfigSrc); err != nil {
		return
	}

	tmpl, err := os.ReadFile(wslconfigSrc)
	if err != nil {
		return
	}

	// The template carries no sizing of its own — it is filled in from the
	// host this is running on. See renderHostSizing for the policy and
	// config/wsl/wslconfig for why hardcoded numbers were a bug.
	specs := detectHostSpecs(ctx)
	if specs.memBytes > 0 || specs.logicalCPUs > 0 {
		core.Info("Detected host: %d logical CPUs, %.1f GB RAM",
			specs.logicalCPUs, float64(specs.memBytes)/float64(gib))
	} else {
		core.Warn("Could not detect host CPU/RAM — .wslconfig will omit sizing and WSL defaults apply")
	}
	srcData := []byte(renderWslconfig(string(tmpl), specs))

	wslWinHome := resolveWinHome()
	if wslWinHome == "" {
		core.Warn("Could not resolve Windows home — copy wsl/wslconfig to C:\\Users\\<you>\\.wslconfig manually")
		return
	}

	dst := wslWinHome + "/.wslconfig"

	if dstData, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(srcData, dstData) {
			core.Ok(".wslconfig already up to date")
			return
		}
		core.Notice("Updating %s (backing up to .wslconfig.bak)", dst)
		// Regular copy since this is in user's Windows home
		os.Rename(dst, dst+".bak")
	}

	if err := os.WriteFile(dst, srcData, 0644); err != nil {
		core.Warn("Could not write .wslconfig: %v", err)
		return
	}
	core.Ok(".wslconfig installed at %s", dst)
}

// linkWinHome creates a symlink at ~/username pointing to the Windows home
// directory (e.g. /home/user/user -> /mnt/c/Users/user).
func linkWinHome() {
	wslWinHome := resolveWinHome()
	if wslWinHome == "" {
		core.Warn("Could not resolve Windows home — skipping ~/username symlink")
		return
	}

	winUser := filepath.Base(wslWinHome)
	link := core.HomeTarget(winUser)

	if err := core.LinkFile(wslWinHome, link); err != nil {
		core.Warn("Could not create Windows home symlink: %v", err)
	}
}

// configureGitCache enables git's untracked cache.
//
// ⚠ It deliberately does NOT set core.fsmonitor. That was here for years and
// was dead the whole time: git's built-in FSMonitor daemon is implemented for
// Windows and macOS only, and the WSL side of WSL is Linux. Measured on
// git 2.47.3 —
//
//	$ git fsmonitor--daemon status
//	fatal: fsmonitor--daemon not supported on this platform
//
// git silently ignores the setting rather than erroring, which is exactly why
// it survived: it looked like a working optimization in every `git config -l`.
// core.untrackedcache is the one that is real on Linux and does help.
func configureGitCache(ctx context.Context) {
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	// Clear the dead setting if a previous install wrote it, so the config
	// stops advertising an optimization that never ran. Unset returns non-zero
	// when the key is absent, which is the normal case — not an error.
	_ = exec.CommandContext(ctx, "git", "config", "--global", "--unset", "core.fsmonitor").Run()

	if err := exec.CommandContext(ctx, "git", "config", "--global", "core.untrackedcache", "true").Run(); err != nil {
		core.Warn("failed to enable git untrackedcache: %v", err)
		return
	}
	core.Ok("git untracked cache enabled")
}

// sudoRun runs a command as root. core.SudoCmd already execs directly when we
// are root, and runCmd handles spinner pausing and failure output.
func sudoRun(ctx context.Context, args ...string) error {
	return runCmd(ctx, "sudo", args...)
}

func sudoCopy(ctx context.Context, src, dst string) error {
	return sudoRun(ctx, "cp", src, dst)
}

func (WslModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "wsl"}
	if !core.IsWSL() {
		s.Extra = "not WSL"
		return s
	}

	// Check /etc/wsl.conf. ⚠ Must compare against the RENDERED template, not
	// the template itself — the raw file contains @DEFAULT_USER@ and would
	// never match an installed copy, reporting permanent unfixable drift.
	if _, err := os.Stat(core.ConfigPath("wsl", "wsl.conf")); err == nil {
		if wslConfState() == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}

	// Check sysctl
	sysctlSrc := core.ConfigPath("wsl", "99-wsl-sysctl.conf")
	if _, err := os.Stat(sysctlSrc); err == nil {
		if core.FilesMatch(sysctlSrc, "/etc/sysctl.d/99-wsl.conf") {
			s.Linked++
		} else {
			s.Missing++
		}
	}

	// Check Windows home symlink
	if wslWinHome := resolveWinHome(); wslWinHome != "" {
		winUser := filepath.Base(wslWinHome)
		link := core.HomeTarget(winUser)
		if core.CheckLink(wslWinHome, link) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}

	// Windows exe shims. Only counted against what this host actually has, so
	// a missing pwsh.exe or winget.exe is not reported as drift.
	present, expected := countWinShims()
	s.Linked += present
	s.Missing += expected - present

	return s
}

// Uninstall removes only what this module owns. The sysctl drop-in and
// /etc/wsl.conf are left in place deliberately: removing them needs sudo and
// would silently change how the distro boots, which is not what someone
// uninstalling a dotfiles module is asking for. The .bak files written on
// install are the documented way back.
func (WslModule) Uninstall(ctx context.Context) error {
	if !core.IsWSL() {
		core.Ok("Not running in WSL, skipping")
		return nil
	}
	if core.DryRun {
		core.Info("would remove Windows exe shims and the ghostty shader override")
		return nil
	}

	if n := removeWinShims(); n > 0 {
		core.Ok("Removed %d Windows shim(s) from ~/.local/bin", n)
		core.Notice("If you also set appendWindowsPath=false in /etc/wsl.conf, set it back")
		core.Notice("to true and run `wsl --shutdown`, or Windows tools will be unreachable.")
	}

	if path := ghosttyLocalPath(); path != "" {
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), shaderManagedHeader) {
			if err := os.Remove(path); err == nil {
				core.Ok("Removed the Ghostty shader override")
			}
		}
	}

	return nil
}
