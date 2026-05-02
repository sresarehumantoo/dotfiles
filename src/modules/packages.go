package modules

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// aptUpdateAttempts and aptUpdateBackoff control retry behaviour for
// `<apt> update`. Mirror failures and DNS hiccups during fresh provisioning
// (especially WSL) are usually transient.
var (
	aptUpdateAttempts = 3
	aptUpdateBackoff  = 2 * time.Second
)

// aptUpdateWithRetry runs `<apt-bin> update` with exponential backoff on
// failure. Returns nil on first success; otherwise the last error after all
// attempts are exhausted.
func aptUpdateWithRetry() error {
	bin := core.AptBin()
	if bin == "" {
		return fmt.Errorf("no apt binary found on PATH")
	}

	var lastErr error
	for attempt := 1; attempt <= aptUpdateAttempts; attempt++ {
		err := runCmd("sudo", bin, "update")
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < aptUpdateAttempts {
			wait := time.Duration(attempt) * aptUpdateBackoff
			core.Warn("%s update failed (attempt %d/%d): %v — retrying in %s", bin, attempt, aptUpdateAttempts, err, wait)
			time.Sleep(wait)
		}
	}
	return lastErr
}

// pacmanNames maps canonical (apt) package names to pacman equivalents.
// Empty string means the package is not needed on Arch (bundled with another).
var pacmanNames = map[string]string{
	"fd-find":                 "fd",
	"build-essential":         "base-devel",
	"golang":                  "go",
	"python3-pip":             "python-pip",
	"locales":                 "", // glibc provides locales on Arch
	"python3-venv":            "", // part of python on Arch
	"pipx":                    "python-pipx",
	"bat":                     "bat",
	"tealdeer":                "tealdeer",
	"neovim":                  "neovim",
	"nodejs":                  "nodejs",
	"npm":                     "npm",
	"xclip":                   "xclip",
	"zsh-syntax-highlighting": "zsh-syntax-highlighting",
	"docker-ce":               "docker",
	"docker-ce-cli":           "",
	"containerd.io":           "",
	"docker-buildx-plugin":    "docker-buildx",
	"docker-compose-plugin":   "docker-compose",
}

// ResolvePkgs translates canonical package names for the given package manager.
// Exported for testing.
func ResolvePkgs(mgr string, pkgs []string) []string {
	return resolvePkgs(mgr, pkgs)
}

// resolvePkgs translates canonical package names for the given package manager.
func resolvePkgs(mgr string, pkgs []string) []string {
	if mgr != "pacman" {
		return pkgs
	}
	var out []string
	for _, p := range pkgs {
		if mapped, ok := pacmanNames[p]; ok {
			if mapped == "" {
				continue // skip — not needed on Arch
			}
			out = append(out, mapped)
		} else {
			out = append(out, p) // no mapping — use as-is
		}
	}
	return out
}

// resolvePkg translates a single canonical package name for the given package manager.
func resolvePkg(mgr string, pkg string) string {
	result := resolvePkgs(mgr, []string{pkg})
	if len(result) == 0 {
		return ""
	}
	return result[0]
}

type PackagesModule struct{}

func (PackagesModule) Name() string { return "packages" }

// detectPkgManager returns the install command prefix for the detected package manager.
func detectPkgManager() (name string, args []string) {
	if bin := core.AptBin(); bin != "" {
		return bin, []string{"sudo", bin, "install", "-y"}
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf", []string{"sudo", "dnf", "install", "-y"}
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return "pacman", []string{"sudo", "pacman", "-S", "--noconfirm"}
	}
	if _, err := exec.LookPath("brew"); err == nil {
		return "brew", []string{"brew", "install"}
	}
	return "", nil
}

var aptUpdated bool

// repairAptSources removes corrupt DEB822 .sources files left by a prior
// dfinstall bug that wrote literal \n instead of real newlines.
func repairAptSources() {
	matches, _ := filepath.Glob("/etc/apt/sources.list.d/*.sources")
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// A valid DEB822 file is multiline; a corrupt one has everything on one line with literal \n
		if strings.Contains(string(data), `\nURIs:`) || strings.Contains(string(data), `\nSuites:`) {
			core.Notice("Removing corrupt apt source: %s", path)
			runCmd("sudo", "rm", path)
		}
	}
}

func installPkg(pkgs ...string) error {
	name, args := detectPkgManager()
	if name == "" {
		core.Err("No supported package manager found. Install manually: %v", pkgs)
		return nil
	}

	// Ensure apt cache is fresh on first use (minimal systems ship with empty lists)
	if (name == "apt-get" || name == "apt") && !aptUpdated {
		repairAptSources()
		core.Info("Refreshing package lists...")
		if err := aptUpdateWithRetry(); err != nil {
			core.Warn("%s update failed after retries: %v", name, err)
		}
		aptUpdated = true
	}

	resolved := resolvePkgs(name, pkgs)
	if len(resolved) == 0 {
		return nil
	}

	// Preflight for apt: drop packages with no installation candidate so a
	// single missing one (e.g. radare2 on trixie) doesn't fail the whole
	// bulk install. Only runs for apt — pacman/dnf/brew handle this
	// differently and fail per-package already.
	if name == "apt-get" || name == "apt" {
		available, missing := filterAptAvailable(resolved)
		if len(missing) > 0 {
			core.AlwaysWarn("Skipping apt packages with no installation candidate: %s", strings.Join(missing, ", "))
		}
		resolved = available
		if len(resolved) == 0 {
			return nil
		}
	}

	core.SpinnerDetail("Installing: %s", strings.Join(resolved, ", "))
	cmdArgs := append(args, resolved...)
	return runCmd(cmdArgs[0], cmdArgs[1:]...)
}

// filterAptAvailable splits packages into those that have an installation
// candidate in the current apt sources vs. those that don't (renamed,
// obsoleted, or simply not in any configured source). Uses 'apt-cache
// madison' which exits 0 either way; non-empty stdout means available.
func filterAptAvailable(pkgs []string) (available, missing []string) {
	for _, p := range pkgs {
		out, err := exec.Command("apt-cache", "madison", p).Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			available = append(available, p)
		} else {
			missing = append(missing, p)
		}
	}
	return available, missing
}

// ContainsSudoInvocation scans cmd args for sudo invocations, including
// inside `bash -c` script strings (heuristic — looks for `sudo ` as a token).
// Used to make sure we pause the spinner around any potential password
// prompt, even when sudo is buried in a shell command string. Exported for
// testing.
func ContainsSudoInvocation(name string, args []string) bool {
	if name == "sudo" {
		return true
	}
	for _, a := range args {
		if a == "sudo" {
			return true
		}
		// `bash -c "... | sudo tee ..."` — match `sudo ` as a token so we
		// don't false-positive on words like "presudoku".
		if strings.Contains(a, "sudo ") {
			return true
		}
	}
	return false
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	directSudo := name == "sudo"
	needsTTY := ContainsSudoInvocation(name, args)

	if directSudo {
		cmd = core.SudoCmd(args...)
	}

	// Output routing:
	//   verbose: straight to terminal so the user sees everything live.
	//   default: capture both streams so we can replay them on failure
	//            (without -v, apt's actual error message used to vanish).
	//            For sudo, also tee stderr to the terminal so password
	//            prompts and sudo errors are visible in real time.
	var capture bytes.Buffer
	if core.Level >= core.LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &capture
		if needsTTY {
			cmd.Stderr = io.MultiWriter(os.Stderr, &capture)
		} else {
			cmd.Stderr = &capture
		}
	}

	// When password is piped via _DFINSTALL_SUDO_PASS, no TTY needed and
	// no spinner interference is possible.
	if directSudo && os.Getenv("_DFINSTALL_SUDO_PASS") != "" {
		return cmd.Run()
	}

	// Hold the spinner pause across the full run, not just the fork. The
	// previous Start/Resume/Wait pattern resumed the spinner while sudo
	// was still trying to prompt, overdrawing the prompt line.
	if needsTTY {
		core.PauseSpinner()
		defer core.ResumeSpinner()
	}

	err := cmd.Run()

	// On failure in default mode, surface what the command actually said.
	// Without this the user only saw stray stderr lines and had to rerun
	// with -v to find the real error.
	if err != nil && core.Level < core.LogVerbose {
		out := strings.TrimSpace(capture.String())
		if out != "" {
			if !needsTTY {
				core.PauseSpinner()
				defer core.ResumeSpinner()
			}
			core.Err("command failed: %s %s", name, strings.Join(args, " "))
			for _, line := range tailLines(out, 30) {
				fmt.Fprintf(os.Stderr, "    %s\n", line)
			}
		}
	}
	return err
}

// tailLines returns the last n lines of s, with an "... elided ..." marker
// if there were more.
func tailLines(s string, n int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return lines
	}
	out := []string{fmt.Sprintf("(... %d earlier lines elided ...)", len(lines)-n)}
	return append(out, lines[len(lines)-n:]...)
}

func (PackagesModule) Install() error {
	if core.DryRun {
		core.Info("would install packages: git, zsh, curl, wget, htop, rsync, ...")
		return nil
	}

	core.Info("Installing core packages...")

	// binary → package(s) mapping; only install what's missing
	wanted := []struct {
		bin  string
		pkgs []string
	}{
		{"git", []string{"git"}},
		{"zsh", []string{"zsh"}},
		{"curl", []string{"curl"}},
		{"wget", []string{"wget"}},
		{"htop", []string{"htop"}},
		{"rsync", []string{"rsync"}},
		// nvim is intentionally omitted — apt's neovim is too old (Debian
		// stable ships 0.7–0.10, telescope.nvim requires >= 0.11). The nvim
		// module installs the official prebuilt tarball instead.
		{"tmux", []string{"tmux"}},
		{"node", []string{"nodejs", "npm"}},
		{"python3", []string{"python3"}},
		{"go", []string{"golang"}},
	}

	var pkgs []string
	for _, w := range wanted {
		if _, err := exec.LookPath(w.bin); err != nil {
			pkgs = append(pkgs, w.pkgs...)
		}
	}
	// locales has no binary to check — ensure the package is present
	if !dpkgInstalled("locales") {
		pkgs = append(pkgs, "locales")
	}

	if len(pkgs) == 0 {
		core.Ok("All core packages already installed")
	} else {
		core.Info("Installing: %s", strings.Join(pkgs, ", "))
		if err := installPkg(pkgs...); err != nil {
			core.Warn("Some packages may have failed to install: %v", err)
		}
	}

	// zsh-syntax-highlighting
	syntaxHL := "/usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh"
	if _, err := os.Stat(syntaxHL); err != nil {
		if err := installPkg("zsh-syntax-highlighting"); err != nil {
			core.Warn("Install zsh-syntax-highlighting manually")
		}
	}

	core.Ok("Core packages done")
	return nil
}

func (PackagesModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "packages"}
	tools := []string{"git", "zsh", "curl", "wget", "htop", "rsync", "nvim", "tmux", "node", "python3", "go"}
	for _, t := range tools {
		if _, err := exec.LookPath(t); err == nil {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}
