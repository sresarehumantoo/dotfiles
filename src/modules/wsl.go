package modules

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type WslModule struct{}

func (WslModule) Name() string { return "wsl" }

func (WslModule) Install() error {
	if !core.IsWSL() {
		core.Ok("Not running in WSL, skipping")
		return nil
	}

	core.Info("Configuring WSL environment...")

	installWslConf()
	installSysctl()
	installWslconfig()
	linkWinHome()
	configureGitFsmonitor()

	fmt.Println()
	core.Info("WSL config changes applied.")
	core.Info("To fully apply .wslconfig and wsl.conf, restart WSL from PowerShell:")
	core.Info("    wsl --shutdown")
	core.Info("Then relaunch your terminal.")
	fmt.Println()

	return nil
}

func installWslConf() {
	wslConf := core.ConfigPath("wsl", "wsl.conf")
	if _, err := os.Stat(wslConf); err != nil {
		return
	}

	srcData, err := os.ReadFile(wslConf)
	if err != nil {
		return
	}

	dstPath := "/etc/wsl.conf"
	if dstData, err := os.ReadFile(dstPath); err == nil {
		if bytes.Equal(srcData, dstData) {
			core.Ok("/etc/wsl.conf already up to date")
			return
		}
		core.Warn("Updating /etc/wsl.conf (backing up to /etc/wsl.conf.bak)")
		sudoCopy(dstPath, dstPath+".bak")
	}

	sudoCopyFrom(wslConf, dstPath)
	core.Ok("/etc/wsl.conf installed")
}

func installSysctl() {
	sysctlSrc := core.ConfigPath("wsl", "99-wsl-sysctl.conf")
	if _, err := os.Stat(sysctlSrc); err != nil {
		return
	}

	srcData, err := os.ReadFile(sysctlSrc)
	if err != nil {
		return
	}

	sudoRun("mkdir", "-p", "/etc/sysctl.d")

	dstPath := "/etc/sysctl.d/99-wsl.conf"
	if dstData, err := os.ReadFile(dstPath); err == nil {
		if bytes.Equal(srcData, dstData) {
			core.Ok("sysctl config already up to date")
			return
		}
	}

	sudoCopyFrom(sysctlSrc, dstPath)
	core.Info("Applying sysctl tweaks...")
	if err := sudoRun("sysctl", "-p", dstPath); err != nil {
		core.Warn("Some sysctl values may not apply until restart")
	}
	core.Ok("/etc/sysctl.d/99-wsl.conf installed")
}

// resolveWinHome returns the WSL mount path for the Windows user home directory
// (e.g. /mnt/c/Users/owen), or empty string on failure.
func resolveWinHome() string {
	cmd := exec.Command("cmd.exe", "/C", "echo %USERPROFILE%")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	winUserDir := strings.TrimSpace(strings.ReplaceAll(string(out), "\r", ""))

	wslPath, err := exec.Command("wslpath", winUserDir).Output()
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
		core.Warn("cmd.exe interop not available. Copy wsl/wslconfig to C:\\Users\\<you>\\.wslconfig manually")
		return
	}

	dst := wslWinHome + "/.wslconfig"

	if dstData, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(srcData, dstData) {
			core.Ok(".wslconfig already up to date")
			return
		}
		core.Warn("Updating %s (backing up to .wslconfig.bak)", dst)
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
// directory (e.g. /home/owen/owen -> /mnt/c/Users/owen).
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

func configureGitFsmonitor() {
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	exec.Command("git", "config", "--global", "core.fsmonitor", "true").Run()
	exec.Command("git", "config", "--global", "core.untrackedcache", "true").Run()
	core.Ok("git fsmonitor + untrackedcache enabled")
}

func sudoRun(args ...string) error {
	core.PauseSpinner()
	defer core.ResumeSpinner()

	if os.Geteuid() == 0 {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = os.Stdin
		if core.Level >= core.LogVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		return cmd.Run()
	}
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	if core.Level >= core.LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func sudoCopy(src, dst string) {
	sudoRun("cp", src, dst)
}

func sudoCopyFrom(src, dst string) {
	sudoRun("cp", src, dst)
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
