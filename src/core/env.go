package core

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultDotfilesDir is set at build time via -ldflags.
var DefaultDotfilesDir string

var (
	dotfilesDir string
	isWSL       bool
	isGitBash   bool
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
