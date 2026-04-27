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
	case "debian", "ubuntu", "raspbian", "linuxmint", "kali", "devuan", "elementary", "parrot", "parrotsec":
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

// knownUpstreamCodenames are Debian and Ubuntu release codenames safe to
// pass as the suite name in third-party apt repos (Docker, Hashicorp, etc.).
// Derivative codenames (parrot's "lory", kali's "kali-rolling", mint's
// "wilma") are deliberately absent — they cause 404s against upstream repos.
var knownUpstreamCodenames = map[string]bool{
	// Debian
	"trixie": true, "bookworm": true, "bullseye": true, "buster": true, "sid": true,
	// Ubuntu LTS + recent interim
	"noble": true, "jammy": true, "focal": true, "bionic": true,
}

// debianVersionToCodename maps the major version found in /etc/debian_version
// to the corresponding upstream Debian codename. Used as a fallback when the
// derivative doesn't ship DEBIAN_CODENAME and its VERSION_CODENAME isn't a
// real Debian/Ubuntu release.
var debianVersionToCodename = map[string]string{
	"10": "buster",
	"11": "bullseye",
	"12": "bookworm",
	"13": "trixie",
}

// UpstreamDebianCodename returns the Debian/Ubuntu codename to use when
// configuring third-party apt repos. Reads /etc/os-release and
// /etc/debian_version. Defaults to "bookworm" if nothing usable is found.
func UpstreamDebianCodename() string {
	osRelease, _ := os.ReadFile("/etc/os-release")
	debianVersion, _ := os.ReadFile("/etc/debian_version")
	return ParseUpstreamDebianCodename(string(osRelease), string(debianVersion))
}

// ParseUpstreamDebianCodename is the testable core of UpstreamDebianCodename.
// Resolution order:
//  1. DEBIAN_CODENAME from os-release (Parrot 6+, some other derivatives)
//  2. VERSION_CODENAME from os-release, only if it's a known upstream codename
//  3. /etc/debian_version major version mapped to a codename
//  4. "bookworm" as a last resort
func ParseUpstreamDebianCodename(osRelease, debianVersion string) string {
	var versionCodename, debianCodename string
	for _, line := range strings.Split(osRelease, "\n") {
		switch {
		case strings.HasPrefix(line, "DEBIAN_CODENAME="):
			debianCodename = strings.Trim(strings.TrimPrefix(line, "DEBIAN_CODENAME="), "\"'")
		case strings.HasPrefix(line, "VERSION_CODENAME="):
			versionCodename = strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"'")
		}
	}

	if debianCodename != "" && knownUpstreamCodenames[debianCodename] {
		return debianCodename
	}
	if knownUpstreamCodenames[versionCodename] {
		return versionCodename
	}

	// /etc/debian_version: "12", "12.4", "trixie/sid", "kali-rolling", etc.
	v := strings.TrimSpace(debianVersion)
	if v != "" {
		major := v
		if i := strings.IndexAny(v, "./"); i > 0 {
			major = v[:i]
		}
		if codename, ok := debianVersionToCodename[major]; ok {
			return codename
		}
	}

	return "bookworm"
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
	cmd := SudoCmd("steamos-readonly", "disable")
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
	cmd := SudoCmd("steamos-readonly", "enable")
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
var sudoStopOnce sync.Once

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
		// Clear from environment so child processes don't inherit it.
		os.Unsetenv("DFINSTALL_SUDO_PASS")
		os.Setenv("_DFINSTALL_SUDO_PASS", pass)
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
		pass := os.Getenv("_DFINSTALL_SUDO_PASS")
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
	sudoStopOnce.Do(func() {
		if sudoKeepAliveStop != nil {
			close(sudoKeepAliveStop)
		}
	})
}

// SudoCmd returns an *exec.Cmd for running a command via sudo. When the
// sudo password was captured from DFINSTALL_SUDO_PASS at startup, it
// injects -S and pipes the password so no TTY prompt is needed. Otherwise
// stdin is connected to the terminal.
func SudoCmd(args ...string) *exec.Cmd {
	if pass := os.Getenv("_DFINSTALL_SUDO_PASS"); pass != "" {
		cmdArgs := append([]string{"-S"}, args...)
		cmd := exec.Command("sudo", cmdArgs...)
		cmd.Stdin = strings.NewReader(pass + "\n")
		return cmd
	}
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	return cmd
}
