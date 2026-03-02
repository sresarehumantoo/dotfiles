package modules

import (
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

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
	cmdArgs := append(args, pkgs...)
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
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		core.ResumeSpinner()
		return err
	}

	return cmd.Run()
}

func (PackagesModule) Install() error {
	core.Info("Installing core packages...")

	pkgs := []string{"git", "zsh", "curl", "wget", "htop", "rsync"}

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
	if _, err := exec.LookPath(syntaxHL); err != nil {
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
