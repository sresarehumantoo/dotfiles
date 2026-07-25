package modules

import (
	"context"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type GhosttyModule struct{}

func (GhosttyModule) Name() string { return "ghostty" }

func (GhosttyModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("ghostty", "config"), Dst: core.XDGTarget("ghostty", "config")},
	}
}

func ghosttyInstalled() bool {
	_, err := exec.LookPath("ghostty")
	return err == nil
}

func (m GhosttyModule) Install(ctx context.Context) error {
	if !ghosttyInstalled() {
		core.Debug("ghostty not installed — skipping config")
		return nil
	}
	core.Info("Linking Ghostty config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	core.Ok("Ghostty config done")
	return nil
}

func (m GhosttyModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("Ghostty config uninstalled")
	return nil
}

func (m GhosttyModule) Status() core.ModuleStatus {
	// Report nothing rather than "missing" when ghostty isn't installed —
	// Install skips it, so it isn't a gap.
	if !ghosttyInstalled() {
		return core.ModuleStatus{Name: "ghostty"}
	}
	return m.Links().Status("ghostty")
}
