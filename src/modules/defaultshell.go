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
	zshPath, err := loginZsh()
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
	// With passwordless sudo (a NOPASSWD rule, a container) sudo chsh needs no
	// password at all, where bare chsh asks for the user's own and fails
	// without a terminal.
	useSudo := core.HasSudoPass()
	if !useSudo {
		_, err := runProbe(ctx, "sudo", "-n", "true")
		useSudo = err == nil
	}

	if !useSudo && !core.Interactive {
		core.Warn("cannot change shell non-interactively — run: chsh -s %s", zshPath)
		return nil
	}

	core.Info("Changing default shell to zsh...")
	core.PauseSpinner()
	var cmd *exec.Cmd
	if useSudo {
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
	zshPath, err := loginZsh()
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

// loginZsh is the zsh to make the login shell. On macOS that is the system
// /bin/zsh: brew's zsh wins a PATH lookup but is not in /etc/shells, so chsh
// refuses it.
func loginZsh() (string, error) {
	if core.IsMac() {
		return "/bin/zsh", nil
	}
	return exec.LookPath("zsh")
}
