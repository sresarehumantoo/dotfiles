package modules

import (
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

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
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "apt-get", []string{"sudo", "apt-get", "install", "-y"}
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

func installPkg(pkgs ...string) error {
	name, args := detectPkgManager()
	if name == "" {
		core.Err("No supported package manager found. Install manually: %v", pkgs)
		return nil
	}
	resolved := resolvePkgs(name, pkgs)
	if len(resolved) == 0 {
		return nil
	}
	cmdArgs := append(args, resolved...)
	return runCmd(cmdArgs[0], cmdArgs[1:]...)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	// Detect if this command needs sudo (password prompt requires terminal)
	needsTTY := name == "sudo"
	if !needsTTY {
		for _, a := range args {
			if a == "sudo" {
				needsTTY = true
				break
			}
		}
	}

	if needsTTY {
		core.PauseSpinner()
		cmd.Stdin = os.Stdin
		if core.Level >= core.LogVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Start(); err != nil {
			core.ResumeSpinner()
			return err
		}
		// Resume spinner while the command runs — sudo has already
		// read any password prompt by the time Start returns control.
		core.ResumeSpinner()
		return cmd.Wait()
	}

	if core.Level >= core.LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func (PackagesModule) Install() error {
	if core.DryRun {
		core.Info("would install packages: git, zsh, curl, wget, htop, rsync, ...")
		return nil
	}

	core.Info("Installing core packages...")

	pkgs := []string{"git", "zsh", "curl", "wget", "htop", "rsync", "locales"}

	if _, err := exec.LookPath("nvim"); err != nil {
		pkgs = append(pkgs, "neovim")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		pkgs = append(pkgs, "tmux")
	}
	if _, err := exec.LookPath("node"); err != nil {
		pkgs = append(pkgs, "nodejs", "npm")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		pkgs = append(pkgs, "python3")
	}
	if _, err := exec.LookPath("go"); err != nil {
		pkgs = append(pkgs, "golang")
	}

	if err := installPkg(pkgs...); err != nil {
		core.Warn("Some packages may have failed to install: %v", err)
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
