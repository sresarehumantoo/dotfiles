package modules

import (
	"context"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

type TealdeerModule struct{}

func (TealdeerModule) Name() string { return "tealdeer" }

// Since 1.9 tealdeer reads the XDG config dir on macOS too, falling back to
// Application Support only while the XDG one does not exist.
func (TealdeerModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("tealdeer", "config.toml"), Dst: core.XDGTarget("tealdeer", "config.toml")},
	}
}

func (m TealdeerModule) Install(ctx context.Context) error {
	core.Info("Linking tealdeer config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	core.Ok("tealdeer config done")
	return nil
}

func (m TealdeerModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("tealdeer config uninstalled")
	return nil
}

func (m TealdeerModule) Status() core.ModuleStatus { return m.Links().Status("tealdeer") }
