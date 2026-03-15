package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// distroIcons maps /etc/os-release ID values to Nerd Font v3 icons.
// Same mapping as powerlevel10k (internal/icons.zsh + internal/p10k.zsh).
var distroIcons = map[string]string{
	"arch":        "\uF303",
	"debian":      "\uF306",
	"ubuntu":      "\uF31b",
	"fedora":      "\uF30a",
	"centos":      "\uF304",
	"rhel":        "\U000F111B",
	"rocky":       "\U000F032B",
	"almalinux":   "\U000F031D",
	"amzn":        "\uF270",
	"kali":        "\uF327",
	"alpine":      "\uF300",
	"nixos":       "\uF313",
	"manjaro":     "\uF312",
	"opensuse":    "\uF314",
	"tumbleweed":  "\uF314",
	"gentoo":      "\uF30d",
	"void":        "\U000F032E",
	"artix":       "\U000F031F",
	"linuxmint":   "\uF30e",
	"elementary":  "\uF309",
	"raspbian":    "\uF315",
	"slackware":   "\uF319",
	"devuan":      "\uF307",
	"coreos":      "\uF305",
	"mageia":      "\uF310",
	"sabayon":     "\uF317",
	"aosc":        "\uF301",
	"steamos":     "\uF1B6",
	"endeavouros": "\U000F0322",
	"guix":        "\U000F0325",
	"neon":        "\uF17C",
}

// detectDistroIcon reads /etc/os-release and returns the Nerd Font icon
// for the current Linux distribution. Falls back to the generic Linux icon.
func detectDistroIcon() string {
	genericLinux := "\uF17C" //

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return genericLinux
	}

	var id string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"'")
			break
		}
	}

	if id == "" {
		return genericLinux
	}

	// Check exact match first
	if icon, ok := distroIcons[id]; ok {
		return icon
	}

	// Check substring match (e.g. ID=opensuse-tumbleweed matches opensuse)
	for distro, icon := range distroIcons {
		if strings.Contains(id, distro) {
			return icon
		}
	}

	return genericLinux
}

// writeDistroIcon detects the distro and writes the icon to
// ~/.config/dfinstall/distro-icon for tmux status bar consumption.
func writeDistroIcon() {
	icon := detectDistroIcon()
	dir := filepath.Join(core.XDGConfigHome(), "dfinstall")
	if err := os.MkdirAll(dir, 0755); err != nil {
		core.Debug("create dfinstall config dir: %v", err)
		return
	}
	path := filepath.Join(dir, "distro-icon")
	if err := os.WriteFile(path, []byte(icon), 0644); err != nil {
		core.Debug("write distro icon: %v", err)
	}
}

type TmuxModule struct{}

func (TmuxModule) Name() string { return "tmux" }

func (TmuxModule) Install() error {
	if core.DryRun {
		core.Info("would link tmux.conf, write distro icon, clone TPM, install plugins")
		return nil
	}

	core.Info("Setting up tmux...")

	// Detect distro and write icon for tmux status bar
	writeDistroIcon()

	tmuxDir := core.XDGTarget("tmux")
	if err := core.EnsureDir(tmuxDir); err != nil {
		return err
	}

	tmuxConf := core.XDGTarget("tmux", "tmux.conf")

	// Remove old oh-my-tmux artifacts if present
	if data, err := os.ReadFile(tmuxConf); err == nil {
		if strings.Contains(string(data), "gpakosz") {
			core.Warn("Removing old oh-my-tmux base config")
			os.Remove(tmuxConf)
		}
	}
	os.Remove(core.XDGTarget("tmux", "tmux.conf.local"))
	os.Remove(core.HomeTarget(".tmux.conf.local"))

	if err := core.LinkFile(core.ConfigPath("tmux", "tmux.conf"), tmuxConf); err != nil {
		return err
	}

	// Legacy symlink for tmux < 3.1
	if err := core.LinkFile(tmuxConf, core.HomeTarget(".tmux.conf")); err != nil {
		return err
	}

	// Install TPM (Tmux Plugin Manager)
	home, _ := os.UserHomeDir()
	tpmDir := filepath.Join(home, ".tmux", "plugins", "tpm")

	if _, err := os.Stat(tpmDir); os.IsNotExist(err) {
		core.Info("Installing TPM...")
		cmd := exec.Command("git", "clone", "--depth=1",
			"https://github.com/tmux-plugins/tpm", tpmDir)
		if core.Level >= core.LogVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			core.Warn("TPM clone failed: %v", err)
		}
	} else {
		core.Ok("TPM already installed")
	}

	// Install plugins via TPM
	installScript := filepath.Join(tpmDir, "bin", "install_plugins")
	if _, err := os.Stat(installScript); err == nil {
		core.Info("Installing tmux plugins...")
		// TPM resolves its plugin path from tmux's global environment.
		// Set it so install_plugins works outside a running tmux session.
		pluginsDir := filepath.Join(home, ".tmux", "plugins") + "/"
		setEnv := exec.Command("tmux", "start-server", ";",
			"set-environment", "-g", "TMUX_PLUGIN_MANAGER_PATH", pluginsDir)
		if err := setEnv.Run(); err != nil {
			core.Debug("tmux set-environment: %v", err)
		}

		cmd := exec.Command(installScript)
		if core.Level >= core.LogVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			core.Warn("TPM plugin install failed: %v", err)
		}
	}

	core.Ok("tmux setup done")
	return nil
}

func (TmuxModule) Uninstall() error {
	tmuxConf := core.XDGTarget("tmux", "tmux.conf")
	if err := core.UnlinkFile(core.ConfigPath("tmux", "tmux.conf"), tmuxConf); err != nil {
		return err
	}
	if err := core.UnlinkFile(tmuxConf, core.HomeTarget(".tmux.conf")); err != nil {
		return err
	}

	// Remove TPM and plugins
	if !core.DryRun {
		home, _ := os.UserHomeDir()
		pluginsDir := filepath.Join(home, ".tmux", "plugins")
		if _, err := os.Stat(pluginsDir); err == nil {
			core.Info("Removing TPM and plugins...")
			os.RemoveAll(pluginsDir)
		}
	} else {
		core.Info("would remove ~/.tmux/plugins/")
	}

	core.Ok("tmux config uninstalled")
	return nil
}

func (TmuxModule) Links() []core.LinkPair {
	tmuxConf := core.XDGTarget("tmux", "tmux.conf")
	return []core.LinkPair{
		{Src: core.ConfigPath("tmux", "tmux.conf"), Dst: tmuxConf},
		{Src: tmuxConf, Dst: core.HomeTarget(".tmux.conf")},
	}
}

func (TmuxModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "tmux"}
	if core.CheckLink(core.ConfigPath("tmux", "tmux.conf"), core.XDGTarget("tmux", "tmux.conf")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	if core.CheckLink(core.XDGTarget("tmux", "tmux.conf"), core.HomeTarget(".tmux.conf")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}

	// Check TPM
	home, _ := os.UserHomeDir()
	tpmDir := filepath.Join(home, ".tmux", "plugins", "tpm")
	if _, err := os.Stat(tpmDir); err == nil {
		pluginsDir := filepath.Join(home, ".tmux", "plugins")
		entries, _ := os.ReadDir(pluginsDir)
		count := 0
		for _, e := range entries {
			if e.IsDir() && e.Name() != "tpm" {
				count++
			}
		}
		if count > 0 {
			s.Extra = fmt.Sprintf("tpm +%d plugins", count)
		} else {
			s.Extra = "tpm"
		}
	}

	return s
}
