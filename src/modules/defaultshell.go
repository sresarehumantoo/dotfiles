package modules

import (
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type DefaultShellModule struct{}

func (DefaultShellModule) Name() string { return "defaultshell" }

func (DefaultShellModule) Install() error {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		core.Warn("zsh not found — install it first")
		return nil
	}

	currentShell := os.Getenv("SHELL")
	if currentShell == zshPath {
		core.Ok("Default shell is already zsh")
		return nil
	}

	core.Info("Changing default shell to zsh...")
	core.PauseSpinner()
	cmd := exec.Command("chsh", "-s", zshPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		core.Warn("Could not change shell — run: chsh -s $(which zsh)")
	}
	core.ResumeSpinner()
	return nil
}

func (DefaultShellModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "defaultshell"}
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		s.Missing = 1
		s.Extra = "zsh not found"
		return s
	}
	if os.Getenv("SHELL") == zshPath {
		s.Linked = 1
		s.Extra = "zsh"
	} else {
		s.Missing = 1
		s.Extra = os.Getenv("SHELL")
	}
	return s
}
