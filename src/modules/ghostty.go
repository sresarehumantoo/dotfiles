package modules

import "github.com/sresarehumantoo/dotfiles/src/core"

type GhosttyModule struct{}

func (GhosttyModule) Name() string { return "ghostty" }

func (GhosttyModule) Install() error {
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

func (GhosttyModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "ghostty"}
	if core.CheckLink(core.ConfigPath("ghostty", "config"), core.XDGTarget("ghostty", "config")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	return s
}
