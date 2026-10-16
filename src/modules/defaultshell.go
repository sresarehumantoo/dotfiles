package modules

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

type DefaultShellModule struct{}

func (DefaultShellModule) Name() string { return "defaultshell" }

func (DefaultShellModule) Install(ctx context.Context) error {
	zshPath, err := loginZsh()
	if err != nil {
		core.Warn("zsh not found — install it first")
		return nil
	}

	if accountShell(ctx) == zshPath {
		core.Ok("Default shell is already zsh")
		return nil
	}

	if core.DryRun {
		core.Info("would change default shell to %s", zshPath)
		return nil
	}

	core.Info("Changing default shell to zsh...")

	// Plain chsh needs no root: it asks for the user's own password. That
	// needs a terminal, and under the MCP server stdin is the JSON-RPC stream,
	// where reading a password would consume protocol bytes.
	if core.Interactive && term.IsTerminal(int(os.Stdin.Fd())) {
		cmd := exec.CommandContext(ctx, "chsh", "-s", zshPath)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		core.PauseSpinner()
		err := cmd.Run()
		core.ResumeSpinner()
		if err == nil {
			core.Changed("default shell set to %s", zshPath)
			return nil
		}
	}

	// No terminal, or chsh refused (an account with no password yet, as during
	// bootstrap): sudo, but only where it needs no prompt.
	if _, err := runProbe(ctx, "sudo", "-n", "true"); err == nil {
		if err := runCmd(ctx, "sudo", "chsh", "-s", zshPath, os.Getenv("USER")); err == nil {
			core.Changed("default shell set to %s", zshPath)
			return nil
		}
	}
	core.Warn("Could not change shell — run: chsh -s %s", zshPath)
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
	// Status has no context to inherit; ProbeTimeout bounds the lookup.
	if shell := accountShell(context.Background()); shell == zshPath {
		s.Linked = 1
		s.Extra = "zsh"
	} else {
		s.Missing = 1
		s.Extra = shell
	}
	return s
}

// accountShell is the login shell recorded for the current user. Not $SHELL,
// which only changes at the next login, so after a successful chsh every run
// in the same session would change it (and ask for the password) again.
func accountShell(ctx context.Context) string {
	user := os.Getenv("USER")
	if user != "" {
		if core.IsMac() {
			// "UserShell: /bin/zsh"
			if out, err := runProbe(ctx, "dscl", ".", "-read", "/Users/"+user, "UserShell"); err == nil {
				if _, shell, ok := strings.Cut(strings.TrimSpace(string(out)), ": "); ok {
					return shell
				}
			}
		} else if out, err := runProbe(ctx, "getent", "passwd", user); err == nil {
			if fields := strings.Split(strings.TrimSpace(string(out)), ":"); len(fields) == 7 {
				return fields[6]
			}
		}
	}
	return os.Getenv("SHELL")
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
