package modules

import (
	"os"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type TmuxModule struct{}

func (TmuxModule) Name() string { return "tmux" }

func (TmuxModule) Install() error {
	core.Info("Setting up tmux...")

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
	return s
}
