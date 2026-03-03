package modules

import "github.com/sresarehumantoo/dotfiles/src/core"

type HtopModule struct{}

func (HtopModule) Name() string { return "htop" }

func (HtopModule) Install() error {
	core.Info("Linking htop config...")
	if err := core.EnsureDir(core.XDGTarget("htop")); err != nil {
		return err
	}
	if err := core.LinkFile(core.ConfigPath("htop", "htoprc"), core.XDGTarget("htop", "htoprc")); err != nil {
		return err
	}
	core.Ok("htop config done")
	return nil
}

func (HtopModule) Uninstall() error {
	if err := core.UnlinkFile(core.ConfigPath("htop", "htoprc"), core.XDGTarget("htop", "htoprc")); err != nil {
		return err
	}
	core.Ok("htop config uninstalled")
	return nil
}

func (HtopModule) Links() []core.LinkPair {
	return []core.LinkPair{
		{Src: core.ConfigPath("htop", "htoprc"), Dst: core.XDGTarget("htop", "htoprc")},
	}
}

func (HtopModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "htop"}
	if core.CheckLink(core.ConfigPath("htop", "htoprc"), core.XDGTarget("htop", "htoprc")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	return s
}
