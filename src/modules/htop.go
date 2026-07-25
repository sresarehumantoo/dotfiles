package modules

import (
	"context"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type HtopModule struct{}

func (HtopModule) Name() string { return "htop" }

func (HtopModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("htop", "htoprc"), Dst: core.XDGTarget("htop", "htoprc")},
	}
}

func (m HtopModule) Install(ctx context.Context) error {
	core.Info("Linking htop config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	core.Ok("htop config done")
	return nil
}

func (m HtopModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("htop config uninstalled")
	return nil
}

func (m HtopModule) Status() core.ModuleStatus { return m.Links().Status("htop") }
