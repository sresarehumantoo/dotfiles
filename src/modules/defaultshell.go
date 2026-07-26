package modules

import (
	"context"
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type DefaultShellModule struct{}

func (DefaultShellModule) Name() string { return "defaultshell" }

func (DefaultShellModule) Install(ctx context.Context) error {
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

	if core.DryRun {
		core.Info("would change default shell to %s", zshPath)
		return nil
	}

	// Bare chsh prompts for the user's password on stdin. Under the MCP server
	// stdin is the JSON-RPC stream, so reading it would consume protocol bytes
	// and wedge the session — there is no password to supply there anyway.
	if !core.HasSudoPass() && !core.Interactive {
		core.Warn("cannot change shell non-interactively — run: chsh -s %s", zshPath)
		return nil
	}

	core.Info("Changing default shell to zsh...")
	core.PauseSpinner()
	// Use sudo chsh when the password is known (bootstrap) to avoid an
	// interactive prompt.
	var cmd *exec.Cmd
	if core.HasSudoPass() {
		cmd = core.SudoCmd(ctx, "chsh", "-s", zshPath, os.Getenv("USER"))
	} else {
		cmd = exec.CommandContext(ctx, "chsh", "-s", zshPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
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
