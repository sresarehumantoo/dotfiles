package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// RunDoctor performs health checks on the environment.
func RunDoctor() {
	fmt.Println("Running health checks...")
	fmt.Println()

	allOk := true

	checks := []struct {
		name  string
		check func() string // "" = ok, non-empty = problem
	}{
		{"go", checkCommand("go")},
		{"nvim", checkCommand("nvim")},
		{"zsh", checkCommand("zsh")},
		{"tmux", checkCommand("tmux")},
		{"git", checkCommand("git")},
		{"delta", checkCommand("delta")},
		{"curl", checkCommand("curl")},
		{"fzf", checkCommand("fzf")},
		{"ripgrep", checkCommand("rg")},
		{"docker", checkCommand("docker")},
		{"terraform", checkCommand("terraform")},
		{"pip3", checkCommand("pip3")},
		{"oh-my-zsh", checkDir(homeDir(".oh-my-zsh"))},
		{"zsh-autosuggestions", checkDir(homeDir(".oh-my-zsh", "custom", "plugins", "zsh-autosuggestions"))},
		{"powerlevel10k", checkDir(homeDir(".oh-my-zsh", "custom", "themes", "powerlevel10k"))},
		{"fonts", checkFontMatch("HackNerdFont-Regular.ttf")},
		{"nvim config", checkLink(
			core.ConfigPath("nvim", "init.lua"),
			filepath.Join(core.XDGConfigHome(), "nvim", "init.lua"),
		)},
		{"shell config", checkLink(
			core.ConfigPath("shell", "zshrc"),
			homeDir(".zshrc"),
		)},
		{"git config", checkLink(
			core.ConfigPath("git", "gitconfig"),
			homeDir(".gitconfig"),
		)},
		{"tmux config", checkLink(
			core.ConfigPath("tmux", "tmux.conf"),
			filepath.Join(core.XDGConfigHome(), "tmux", "tmux.conf"),
		)},
	}

	if len(core.Cfg.ExtendedPlugins) > 0 {
		checks = append(checks, struct {
			name  string
			check func() string
		}{"extended plugins", checkFile(ExtendedPluginsFilePath())})
	}

	if core.IsSteamOS() {
		checks = append(checks,
			struct {
				name  string
				check func() string
			}{"steamos-readonly", checkCommand("steamos-readonly")},
			struct {
				name  string
				check func() string
			}{"pacman", checkCommand("pacman")},
		)
	}

	if core.IsWSL() {
		checks = append(checks,
			struct {
				name  string
				check func() string
			}{"wsl.conf", checkFileMatch(
				core.ConfigPath("wsl", "wsl.conf"),
				"/etc/wsl.conf",
			)},
			struct {
				name  string
				check func() string
			}{"sysctl config", checkFileMatch(
				core.ConfigPath("wsl", "99-wsl-sysctl.conf"),
				"/etc/sysctl.d/99-wsl.conf",
			)},
			struct {
				name  string
				check func() string
			}{"windows home symlink", checkWinHomeLink()},
		)
	}

	for _, c := range checks {
		if msg := c.check(); msg == "" {
			core.Ok("%s", c.name)
		} else {
			core.Warn("%s — %s", c.name, msg)
			allOk = false
		}
	}

	// Check alias collisions between managed and preserved shell files
	if len(core.Cfg.PreservedFiles) > 0 {
		if ReportAliasCollisions() {
			allOk = false
		} else {
			core.Ok("alias collisions: none")
		}
	}

	fmt.Println()
	if allOk {
		core.Ok("All checks passed!")
	} else {
		core.Warn("Some checks failed. Run 'dfinstall install all' to fix.")
	}
}

func homeDir(parts ...string) string {
	home, _ := os.UserHomeDir()
	args := append([]string{home}, parts...)
	return filepath.Join(args...)
}

func checkCommand(name string) func() string {
	return func() string {
		if _, err := exec.LookPath(name); err != nil {
			return "not found"
		}
		return ""
	}
}

func checkDir(path string) func() string {
	return func() string {
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			return "not found"
		}
		return ""
	}
}

func checkFile(path string) func() string {
	return func() string {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "not found"
		}
		return ""
	}
}

// checkLink verifies a symlink at dst points to src.
func checkLink(src, dst string) func() string {
	return func() string {
		switch core.CheckLink(src, dst) {
		case "ok":
			return ""
		case "wrong":
			return "wrong target"
		case "file":
			return "regular file (not symlinked)"
		default:
			return "not found"
		}
	}
}

// checkFileMatch verifies dst exists and has identical content to src (by hash).
func checkFileMatch(src, dst string) func() string {
	return func() string {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return "not found"
		}
		if !core.FilesMatch(src, dst) {
			return "outdated"
		}
		return ""
	}
}

// checkFontMatch checks a font is installed and matches the bundled source if available.
func checkFontMatch(name string) func() string {
	return func() string {
		fontDir := core.HomeTarget(".local", "share", "fonts")
		dst := filepath.Join(fontDir, name)
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return "not found"
		}
		// If bundled source exists, verify content matches
		src := core.ConfigPath("fonts", name)
		if _, err := os.Stat(src); err == nil {
			if !core.FilesMatch(src, dst) {
				return "outdated"
			}
		}
		return ""
	}
}

func checkWinHomeLink() func() string {
	return func() string {
		wslWinHome := resolveWinHome()
		if wslWinHome == "" {
			return "could not resolve Windows home"
		}
		winUser := filepath.Base(wslWinHome)
		switch core.CheckLink(wslWinHome, core.HomeTarget(winUser)) {
		case "ok":
			return ""
		case "wrong":
			return "wrong target"
		case "file":
			return "regular file (not symlinked)"
		default:
			return "not found"
		}
	}
}
