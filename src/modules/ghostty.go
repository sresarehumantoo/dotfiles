package modules

import (
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type GhosttyModule struct{}

func (GhosttyModule) Name() string { return "ghostty" }

func (GhosttyModule) Install() error {
	if _, err := exec.LookPath("ghostty"); err != nil {
		core.Debug("ghostty not installed — skipping config")
		return nil
	}
	core.Info("Linking Ghostty config...")
	if err := core.EnsureDir(core.XDGTarget("ghostty")); err != nil {
		return err
	}
	if err := core.LinkFile(core.ConfigPath("ghostty", "config"), core.XDGTarget("ghostty", "config")); err != nil {
		return err
	}
	core.Ok("Ghostty config done")
	return nil
}

func (GhosttyModule) Uninstall() error {
	if err := core.UnlinkFile(core.ConfigPath("ghostty", "config"), core.XDGTarget("ghostty", "config")); err != nil {
		return err
	}
	core.Ok("Ghostty config uninstalled")
	return nil
}

func (GhosttyModule) Links() []core.LinkPair {
	return []core.LinkPair{
		{Src: core.ConfigPath("ghostty", "config"), Dst: core.XDGTarget("ghostty", "config")},
	}
}

func (GhosttyModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "ghostty"}
	if _, err := exec.LookPath("ghostty"); err != nil {
		return s
	}
	if core.CheckLink(core.ConfigPath("ghostty", "config"), core.XDGTarget("ghostty", "config")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	return s
}
