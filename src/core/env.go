package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultDotfilesDir is set at build time via -ldflags.
var DefaultDotfilesDir string

// Distro represents a Linux distribution family.
type Distro int

const (
	DistroUnknown Distro = iota
	DistroDebian         // debian, ubuntu, raspbian, etc.
	DistroFedora         // fedora, rhel, centos, rocky, etc.
	DistroArch           // arch, manjaro, endeavouros, etc.
	DistroSteamOS        // steamos (Arch-based, readonly root)
)

var (
	dotfilesDir string
	isWSL       bool
	isGitBash   bool
	distro      Distro
	envDetected bool
)

// DetectEnvironment probes the runtime to determine WSL/GitBash status.
func DetectEnvironment() {
	if envDetected {
		return
	}
	envDetected = true

	// Git Bash / MSYS / MinGW
	if os.Getenv("MSYSTEM") != "" || os.Getenv("MINGW_PREFIX") != "" {
		isGitBash = true
		return
	}

	// WSL detection via /proc/version
	isWSL = checkWSL()

	// Distro detection via /etc/os-release
	distro = detectDistro()
}

func checkWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// ParseProcVersion checks whether content from /proc/version indicates WSL.
// Exported for testing.
func ParseProcVersion(content string) bool {
	return strings.Contains(strings.ToLower(content), "microsoft")
}

// AssertEnvironment exits if the environment is unsupported.
func AssertEnvironment() {
	if isGitBash {
		Err("===========================================================")
		Err("  Git Bash / MSYS / MinGW detected.")
		Err("")
		Err("  This dotfiles setup requires a full Linux environment.")
		Err("  Please install WSL2 and run this from inside WSL:")
		Err("")
		Err("    1.  wsl --install -d Debian")
		Err("    2.  Open the new Debian terminal")
		Err("    3.  git clone <this-repo> ~/dotfiles")
		Err("    4.  cd ~/dotfiles && ./install.sh")
		Err("")
		Err("  See: https://learn.microsoft.com/en-us/windows/wsl/install")
		Err("===========================================================")
		os.Exit(1)
	}

	if isWSL {
		data, _ := os.ReadFile("/proc/version")
		version := "WSL2"
		if idx := strings.Index(string(data), "WSL"); idx >= 0 {
			end := idx
			for end < len(data) && data[end] != ' ' && data[end] != '\n' {
				end++
			}
			version = string(data[idx:end])
		}
		Info("WSL detected (%s)", version)
	}

	if distro == DistroSteamOS {
		Info("SteamOS detected (Arch-based, readonly root)")
	}
}

// detectDistro reads /etc/os-release to determine the Linux distribution.
func detectDistro() Distro {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return DistroUnknown
	}
	return ParseOsRelease(string(data))
}

// ParseOsRelease parses /etc/os-release content and returns the detected distro.
// Exported for testing.
func ParseOsRelease(content string) Distro {
	var id, idLike string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"'")
		}
		if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"'")
		}
	}

	switch id {
	case "steamos":
		return DistroSteamOS
	case "arch", "manjaro", "endeavouros", "artix":
		return DistroArch
	case "debian", "ubuntu", "raspbian", "linuxmint", "kali", "devuan", "elementary":
		return DistroDebian
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return DistroFedora
	}

	// Fallback to ID_LIKE
	for _, like := range strings.Fields(idLike) {
		switch like {
		case "arch":
			return DistroArch
		case "debian", "ubuntu":
			return DistroDebian
		case "fedora", "rhel":
			return DistroFedora
		}
	}

	return DistroUnknown
}

// GetDistro returns the detected distribution.
func GetDistro() Distro {
	return distro
}

// IsSteamOS returns true if running on SteamOS.
func IsSteamOS() bool {
	return distro == DistroSteamOS
}

// IsArchBased returns true for Arch Linux and SteamOS.
func IsArchBased() bool {
	return distro == DistroArch || distro == DistroSteamOS
}

// IsDebianBased returns true for Debian, Ubuntu, and derivatives.
func IsDebianBased() bool {
	return distro == DistroDebian
}

// DisableReadonly disables the SteamOS readonly filesystem.
func DisableReadonly() error {
	PauseSpinner()
	defer ResumeSpinner()
	cmd := exec.Command("sudo", "steamos-readonly", "disable")
	cmd.Stdin = os.Stdin
	if Level >= LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("steamos-readonly disable: %w", err)
	}
	return nil
}

// EnableReadonly re-enables the SteamOS readonly filesystem.
func EnableReadonly() error {
	PauseSpinner()
	defer ResumeSpinner()
	cmd := exec.Command("sudo", "steamos-readonly", "enable")
	cmd.Stdin = os.Stdin
	if Level >= LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("steamos-readonly enable: %w", err)
	}
	return nil
}

// IsWSL returns true if running under Windows Subsystem for Linux.
func IsWSL() bool {
	return isWSL
}

// IsGitBash returns true if running under Git Bash/MSYS/MinGW.
func IsGitBash() bool {
	return isGitBash
}

// ResetDotfilesDir clears the cached dotfiles directory so it is re-resolved
// on the next call to DotfilesDir. Exported for testing.
func ResetDotfilesDir() {
	dotfilesDir = ""
}

// DotfilesDir returns the root of the dotfiles repository.
func DotfilesDir() string {
	if dotfilesDir != "" {
		return dotfilesDir
	}

	// 1. $DOTFILES env var
	if env := os.Getenv("DOTFILES"); env != "" {
		dotfilesDir = env
		Debug("dotfiles dir from $DOTFILES: %s", dotfilesDir)
		return dotfilesDir
	}

	// 2. Build-time baked path
	if DefaultDotfilesDir != "" {
		dotfilesDir = DefaultDotfilesDir
		Debug("dotfiles dir from build-time: %s", dotfilesDir)
		return dotfilesDir
	}

	// 3. Walk up from executable looking for go.mod
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				dotfilesDir = dir
				return dotfilesDir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Fallback: current working directory
	dotfilesDir, _ = os.Getwd()
	return dotfilesDir
}

// ConfigDir returns the path to config/ under the dotfiles root.
func ConfigDir() string {
	return filepath.Join(DotfilesDir(), "config")
}

// XDGConfigHome returns XDG_CONFIG_HOME or ~/.config.
func XDGConfigHome() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// ── Sudo credential management ─────────────────────────────────

var sudoKeepAliveStop chan struct{}
var sudoOnce sync.Once

// PromptSudo validates sudo credentials before the spinner starts and launches
// a background goroutine that refreshes them every 60 seconds. If
// DFINSTALL_SUDO_PASS is set (e.g. during bootstrap where the password is
// known), it is piped to sudo -S so no interactive prompt is needed.
func PromptSudo() {
	if DryRun {
		return
	}
	// Check if sudo even needs a password (e.g. NOPASSWD configured)
	if exec.Command("sudo", "-n", "true").Run() == nil {
		Debug("sudo: passwordless access available")
		startSudoKeepAlive()
		return
	}

	// If the password is known (e.g. fresh bootstrap sets it to "root"),
	// feed it non-interactively so the spinner is never interrupted.
	if pass := os.Getenv("DFINSTALL_SUDO_PASS"); pass != "" {
		Debug("sudo: using DFINSTALL_SUDO_PASS")
		cmd := exec.Command("sudo", "-S", "-v")
		cmd.Stdin = strings.NewReader(pass + "\n")
		if err := cmd.Run(); err != nil {
			Warn("sudo authentication via DFINSTALL_SUDO_PASS failed — will prompt")
		} else {
			startSudoKeepAlive()
			return
		}
	}

	Status("Some steps require sudo access")
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		Warn("sudo authentication failed — some steps may prompt again")
		return
	}

	startSudoKeepAlive()
}

// startSudoKeepAlive refreshes sudo credentials in the background.
func startSudoKeepAlive() {
	sudoOnce.Do(func() {
		sudoKeepAliveStop = make(chan struct{})
		pass := os.Getenv("DFINSTALL_SUDO_PASS")
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-sudoKeepAliveStop:
					return
				case <-ticker.C:
					if pass != "" {
						cmd := exec.Command("sudo", "-S", "-v")
						cmd.Stdin = strings.NewReader(pass + "\n")
						cmd.Run()
					} else {
						exec.Command("sudo", "-n", "-v").Run()
					}
				}
			}
		}()
	})
}

// StopSudoKeepAlive stops the background credential refresh.
func StopSudoKeepAlive() {
	if sudoKeepAliveStop != nil {
		close(sudoKeepAliveStop)
	}
}
