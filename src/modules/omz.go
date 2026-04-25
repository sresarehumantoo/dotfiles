package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type OmzModule struct{}

func (OmzModule) Name() string { return "omz" }

func (OmzModule) Install() error {
	if core.DryRun {
		core.Info("would install oh-my-zsh, zsh-autosuggestions, powerlevel10k")
		return nil
	}

	core.Info("Setting up Oh My Zsh...")

	home, _ := os.UserHomeDir()
	omzDir := filepath.Join(home, ".oh-my-zsh")

	if err := installOmz(omzDir); err != nil {
		core.Warn("Oh My Zsh install failed: %v", err)
	}

	// zsh-autosuggestions plugin
	zshCustom := os.Getenv("ZSH_CUSTOM")
	if zshCustom == "" {
		zshCustom = filepath.Join(omzDir, "custom")
	}

	zasDir := filepath.Join(zshCustom, "plugins", "zsh-autosuggestions")
	if _, err := os.Stat(zasDir); os.IsNotExist(err) {
		core.Info("Installing zsh-autosuggestions...")
		cmd := exec.Command("git", "clone",
			"https://github.com/zsh-users/zsh-autosuggestions", zasDir)
		if core.Level >= core.LogVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			core.Warn("zsh-autosuggestions clone failed: %v", err)
		}
	} else {
		core.Ok("zsh-autosuggestions already installed")
	}

	// powerlevel10k theme
	p10kDir := filepath.Join(zshCustom, "themes", "powerlevel10k")
	if _, err := os.Stat(p10kDir); os.IsNotExist(err) {
		core.Info("Installing powerlevel10k...")
		cmd := exec.Command("git", "clone", "--depth=1",
			"https://github.com/romkatv/powerlevel10k.git", p10kDir)
		if core.Level >= core.LogVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			core.Warn("powerlevel10k clone failed: %v", err)
		}
	} else {
		core.Ok("powerlevel10k already installed")
	}

	// Write extended plugins file if selections exist or --extended was used
	if core.ExtendedMode || len(core.Cfg.ExtendedPlugins) > 0 {
		if err := WriteExtendedPluginsFile(core.Cfg.ExtendedPlugins); err != nil {
			core.Warn("extended plugins file: %v", err)
		}
	}

	core.Ok("Oh My Zsh setup done")
	return nil
}

// installOmz installs Oh My Zsh into omzDir using a direct git clone
// (rather than the upstream curl|install.sh wrapper). The wrapper refuses
// to run if $ZSH exists at all, which makes recovery impossible after a
// partial install — and "partial install" is easy to hit because our own
// plugin/theme git clones into custom/ create the directory tree even
// when oh-my-zsh.sh itself is missing.
//
// Existence is determined by oh-my-zsh.sh, not the directory, so a stale
// custom/ subtree from a botched run no longer masks the failure as success.
func installOmz(omzDir string) error {
	marker := filepath.Join(omzDir, "oh-my-zsh.sh")
	if _, err := os.Stat(marker); err == nil {
		core.Ok("oh-my-zsh already installed")
		return nil
	}

	// Partial-install handling: preserve any custom/ subtree before
	// reinstalling so we don't blow away user-cloned plugins/themes.
	customDir := filepath.Join(omzDir, "custom")
	var savedCustom string
	if _, err := os.Stat(omzDir); err == nil {
		core.Notice("oh-my-zsh dir exists but oh-my-zsh.sh is missing — reinstalling")
		if _, err := os.Stat(customDir); err == nil {
			tmp, err := os.MkdirTemp("", "omz-custom-*")
			if err != nil {
				return fmt.Errorf("create temp for custom/: %w", err)
			}
			savedCustom = filepath.Join(tmp, "custom")
			if err := os.Rename(customDir, savedCustom); err != nil {
				return fmt.Errorf("preserve custom/: %w", err)
			}
		}
		if err := os.RemoveAll(omzDir); err != nil {
			return fmt.Errorf("remove partial omz dir: %w", err)
		}
	}

	core.Info("Installing Oh My Zsh (git clone)...")
	cmd := exec.Command("git", "clone", "--depth=1",
		"https://github.com/ohmyzsh/ohmyzsh.git", omzDir)
	if core.Level >= core.LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Restore preserved custom/ — overwrite OMZ's default sample custom/
	// since the user's contents are more important.
	if savedCustom != "" {
		if err := os.RemoveAll(customDir); err != nil {
			return fmt.Errorf("remove fresh custom/: %w", err)
		}
		if err := os.Rename(savedCustom, customDir); err != nil {
			return fmt.Errorf("restore custom/: %w", err)
		}
		os.Remove(filepath.Dir(savedCustom)) // remove the temp parent
	}

	if _, err := os.Stat(marker); err != nil {
		return fmt.Errorf("install completed but marker missing at %s", marker)
	}
	core.Ok("oh-my-zsh installed")
	return nil
}

func (OmzModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "omz"}
	home, _ := os.UserHomeDir()
	omzDir := filepath.Join(home, ".oh-my-zsh")

	// Check oh-my-zsh.sh (the canonical marker), not the directory — a
	// partial install leaves the dir but no marker.
	checks := []string{
		filepath.Join(omzDir, "oh-my-zsh.sh"),
		filepath.Join(omzDir, "custom", "plugins", "zsh-autosuggestions"),
		filepath.Join(omzDir, "custom", "themes", "powerlevel10k"),
	}

	for _, p := range checks {
		if _, err := os.Stat(p); err == nil {
			s.Linked++
		} else {
			s.Missing++
		}
	}

	if n := len(core.Cfg.ExtendedPlugins); n > 0 {
		s.Extra = fmt.Sprintf("+%d extended", n)
	}
	return s
}
