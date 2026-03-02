package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/owenpierce/dotfiles/src/core"
)

// rootLink defines a symlink from a config source to a destination under /root/.
type rootLink struct {
	src string // relative to config/
	dst string // absolute path under /root/
}

var rootLinks []rootLink

func init() {
	// Shell
	for _, l := range []struct{ src, dst string }{
		{"shell/zshrc", ".zshrc"},
		{"shell/aliases", ".aliases"},
		{"shell/p10k.zsh", ".p10k.zsh"},
		{"shell/bashrc", ".bashrc"},
		{"shell/profile", ".profile"},
		{"shell/zsh/options.zsh", ".zsh.d/options.zsh"},
		{"shell/zsh/keybinds.zsh", ".zsh.d/keybinds.zsh"},
		{"shell/zsh/path.zsh", ".zsh.d/path.zsh"},
		{"shell/zsh/exports.zsh", ".zsh.d/exports.zsh"},
	} {
		rootLinks = append(rootLinks, rootLink{l.src, filepath.Join("/root", l.dst)})
	}

	// Git
	rootLinks = append(rootLinks, rootLink{"git/gitconfig", "/root/.gitconfig"})

	// Nvim
	for _, l := range nvimLinks {
		rootLinks = append(rootLinks, rootLink{l.Src, filepath.Join("/root/.config/nvim", l.Dst)})
	}

	// Tmux
	rootLinks = append(rootLinks, rootLink{"tmux/tmux.conf", "/root/.config/tmux/tmux.conf"})
	rootLinks = append(rootLinks, rootLink{"tmux/tmux.conf", "/root/.tmux.conf"})

	// Htop
	rootLinks = append(rootLinks, rootLink{"htop/htoprc", "/root/.config/htop/htoprc"})
}

// InstallRoot symlinks a curated set of configs into /root/ via sudo.
func InstallRoot() error {
	core.Info("Linking configs into /root/ (via sudo)...")

	// Collect unique parent directories to create
	dirs := make(map[string]bool)
	for _, l := range rootLinks {
		dirs[filepath.Dir(l.dst)] = true
	}
	for d := range dirs {
		if err := sudoMkdir(d); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	var failures int
	for _, l := range rootLinks {
		src := core.ConfigPath(l.src)
		if _, err := os.Stat(src); err != nil {
			core.Warn("source missing, skipping: %s", src)
			failures++
			continue
		}
		if err := sudoLink(src, l.dst); err != nil {
			core.Warn("failed to link %s -> %s: %v", l.dst, src, err)
			failures++
			continue
		}
		core.Ok("linked: %s -> %s", l.dst, src)
	}

	if failures > 0 {
		core.Warn("%d link(s) failed", failures)
	}
	core.Ok("Root configs done (%d/%d linked)", len(rootLinks)-failures, len(rootLinks))
	return nil
}

// RootStatus checks which root links are in place.
func RootStatus() (linked, missing int) {
	for _, l := range rootLinks {
		src := core.ConfigPath(l.src)
		if core.CheckLink(src, l.dst) == "ok" {
			linked++
		} else {
			missing++
		}
	}
	return
}

func sudoMkdir(dir string) error {
	cmd := exec.Command("sudo", "mkdir", "-p", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sudoLink(src, dst string) error {
	cmd := exec.Command("sudo", "ln", "-sfn", src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
