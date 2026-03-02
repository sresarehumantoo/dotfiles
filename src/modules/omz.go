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
	core.Info("Setting up Oh My Zsh...")

	home, _ := os.UserHomeDir()
	omzDir := filepath.Join(home, ".oh-my-zsh")

	// Install Oh My Zsh
	if _, err := os.Stat(omzDir); os.IsNotExist(err) {
		core.Info("Installing Oh My Zsh...")
		cmd := exec.Command("sh", "-c",
			`curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh | RUNZSH=no CHSH=no sh`)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "RUNZSH=no", "CHSH=no")
		if err := cmd.Run(); err != nil {
			core.Warn("Oh My Zsh install failed: %v", err)
		}
	} else {
		core.Ok("oh-my-zsh already installed")
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
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else {
		core.Ok("zsh-autosuggestions already installed")
	}

	// powerlevel10k theme
	p10kDir := filepath.Join(zshCustom, "themes", "powerlevel10k")
	if _, err := os.Stat(p10kDir); os.IsNotExist(err) {
		core.Info("Installing powerlevel10k...")
		cmd := exec.Command("git", "clone", "--depth=1",
			"https://github.com/romkatv/powerlevel10k.git", p10kDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
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

func (OmzModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "omz"}
	home, _ := os.UserHomeDir()
	omzDir := filepath.Join(home, ".oh-my-zsh")

	checks := []string{
		omzDir,
		filepath.Join(omzDir, "custom", "plugins", "zsh-autosuggestions"),
		filepath.Join(omzDir, "custom", "themes", "powerlevel10k"),
	}

	for _, dir := range checks {
		if _, err := os.Stat(dir); err == nil {
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
