package modules

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		core.Info("would configure WSL: wsl.conf, sysctl, wslconfig, win-home symlink, git fsmonitor")
		return nil
	}

	core.Info("Configuring WSL environment...")

	installWslConf(ctx)
	installSysctl(ctx)

	// interop-dependent steps require cmd.exe, which is only available
	// after wsl.conf enables interop and WSL is restarted.
	if hasInterop() {
		installWslconfig()
		linkWinHome()
	} else {
		core.Warn("Windows interop not yet available — restart WSL to apply wsl.conf changes.")
		core.Warn("From PowerShell run: wsl --shutdown")
		core.Warn("Then relaunch and run: dfinstall install wsl")
	}

	configureGitFsmonitor(ctx)

	return nil
}

// hasInterop returns true if Windows interop (cmd.exe) is available.
func hasInterop() bool {
	_, err := exec.LookPath("cmd.exe")
	return err == nil
}

// installWslConf installs /etc/wsl.conf and returns true if the file was changed.
func installWslConf(ctx context.Context) bool {
	wslConf := core.ConfigPath("wsl", "wsl.conf")
	if _, err := os.Stat(wslConf); err != nil {
		return false
	}

	srcData, err := os.ReadFile(wslConf)
	if err != nil {
		return false
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

	if err := sudoCopy(ctx, wslConf, dstPath); err != nil {
		core.Warn("failed to install %s: %v", dstPath, err)
		return false
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
func resolveWinHome() string {
	cmd := exec.CommandContext(context.Background(), "cmd.exe", "/C", "echo %USERPROFILE%")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	winUserDir := strings.TrimSpace(strings.ReplaceAll(string(out), "\r", ""))

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

func installWslconfig() {
	wslconfigSrc := core.ConfigPath("wsl", "wslconfig")
	if _, err := os.Stat(wslconfigSrc); err != nil {
		return
	}

	srcData, err := os.ReadFile(wslconfigSrc)
	if err != nil {
		return
	}

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

func configureGitFsmonitor(ctx context.Context) {
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	if err := exec.CommandContext(ctx, "git", "config", "--global", "core.fsmonitor", "true").Run(); err != nil {
		core.Warn("failed to enable git fsmonitor: %v", err)
	}
	if err := exec.CommandContext(ctx, "git", "config", "--global", "core.untrackedcache", "true").Run(); err != nil {
		core.Warn("failed to enable git untrackedcache: %v", err)
	}
	core.Ok("git fsmonitor + untrackedcache enabled")
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

	// Check /etc/wsl.conf
	wslConf := core.ConfigPath("wsl", "wsl.conf")
	if _, err := os.Stat(wslConf); err == nil {
		if core.FilesMatch(wslConf, "/etc/wsl.conf") {
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

	return s
}
