package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// rootFile is a config source installed as a root-owned copy under /root/.
type rootFile struct {
	src string // relative to config/
	dst string // absolute path under /root/
}

var rootFiles []rootFile

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
		rootFiles = append(rootFiles, rootFile{l.src, filepath.Join("/root", l.dst)})
	}

	// Git
	rootFiles = append(rootFiles, rootFile{"git/gitconfig", "/root/.gitconfig"})

	// Nvim
	for _, l := range nvimLinks {
		rootFiles = append(rootFiles, rootFile{l.Src, filepath.Join("/root/.config/nvim", l.Dst)})
	}

	// Tmux
	rootFiles = append(rootFiles, rootFile{"tmux/tmux.conf", "/root/.config/tmux/tmux.conf"})
	rootFiles = append(rootFiles, rootFile{"tmux/tmux.conf", "/root/.tmux.conf"})

	// Htop
	rootFiles = append(rootFiles, rootFile{"htop/htoprc", "/root/.config/htop/htoprc"})
}

// InstallRoot installs root-owned copies of a curated set of configs into
// /root/ via sudo.
//
// These are copies, not symlinks into the clone: root's shell and nvim would
// otherwise execute files the invoking user can edit, so anything running as
// that user could get root the next time root opened a shell. Re-run after
// editing the sources to bring /root up to date.
func InstallRoot(ctx context.Context) error {
	if core.IsMac() {
		return fmt.Errorf("dfinstall root is Linux only")
	}
	if core.DryRun {
		for _, f := range rootFiles {
			core.Info("would install %s -> %s", core.ConfigPath(f.src), f.dst)
		}
		return nil
	}
	core.Info("Installing configs into /root/ (via sudo)...")

	var failures int
	for _, f := range rootFiles {
		src := core.ConfigPath(f.src)
		if _, err := os.Stat(src); err != nil {
			core.Warn("source missing, skipping: %s", src)
			failures++
			continue
		}
		// install unlinks an existing destination before writing, so a
		// symlink left by an older dfinstall is replaced, not written through.
		if err := runCmd(ctx, "sudo", "install", "-D", "-o", "root", "-g", "root", "-m", "0644", src, f.dst); err != nil {
			core.Warn("failed to install %s: %v", f.dst, err)
			failures++
			continue
		}
		core.Changed("installed: %s", f.dst)
	}

	if failures > 0 {
		core.Warn("%d file(s) failed", failures)
	}
	core.Ok("Root configs done (%d/%d installed)", len(rootFiles)-failures, len(rootFiles))

	installRootClones(ctx)
	return nil
}

// rootClone is a git checkout the root .zshrc depends on. marker decides
// whether it is installed, so an omz dir holding only custom/ still counts as
// missing.
type rootClone struct {
	name, url, dir, marker string
	shallow                bool
}

// Oh My Zsh comes first: the plugin and theme clones land inside its custom/.
var rootClones = []rootClone{
	{"oh-my-zsh", omzRepo, "/root/.oh-my-zsh", "oh-my-zsh.sh", true},
	{"zsh-autosuggestions", "https://github.com/zsh-users/zsh-autosuggestions",
		"/root/.oh-my-zsh/custom/plugins/zsh-autosuggestions", "zsh-autosuggestions.zsh", false},
	{"powerlevel10k", "https://github.com/romkatv/powerlevel10k.git",
		"/root/.oh-my-zsh/custom/themes/powerlevel10k", "powerlevel10k.zsh-theme", true},
}

// installRootClones clones whatever root's shell is missing. Existing
// checkouts are left alone, never pulled: updating root's oh-my-zsh is its own
// updater's job.
func installRootClones(ctx context.Context) {
	if a := gitAccessLocal(ctx, omzRepo); !a.usable() {
		core.Warn("root shell plugins skipped, oh-my-zsh needs git: %s", a.Reason)
		return
	}
	for _, c := range rootClones {
		if core.SudoCmd(ctx, "test", "-e", filepath.Join(c.dir, c.marker)).Run() == nil {
			core.Ok("root %s already installed", c.name)
			continue
		}
		args := []string{"git", "clone", "-q"}
		if c.shallow {
			args = append(args, "--depth=1")
		}
		args = append(args, c.url, c.dir)
		if err := runNetCmd(ctx, "sudo", args...); err != nil {
			core.Warn("root %s clone failed: %v", c.name, err)
			continue
		}
		core.Changed("cloned %s into /root", c.name)
	}
}

// RootStatus reports how many /root copies match their source. /root is not
// readable by the invoking user, so the hashes are taken through sudo.
func RootStatus(ctx context.Context) (current, stale int) {
	args := []string{"sha256sum", "--"}
	for _, f := range rootFiles {
		args = append(args, f.dst)
	}
	// Missing files make sha256sum exit non-zero; the ones it could read are
	// still on stdout.
	out, _ := core.SudoCmd(ctx, args...).Output()
	installed := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		if sum, path, ok := strings.Cut(line, "  "); ok {
			installed[path] = sum
		}
	}

	for _, f := range rootFiles {
		want, err := core.FileHash(core.ConfigPath(f.src))
		if err == nil && installed[f.dst] == want {
			current++
		} else {
			stale++
		}
	}
	return
}
