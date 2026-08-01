package modules

import (
	"context"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type SwayModule struct{}

func (SwayModule) Name() string { return "sway" }

func (SwayModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("sway", "config"), Dst: core.XDGTarget("sway", "config")},
		{Src: core.ConfigPath("waybar", "config"), Dst: core.XDGTarget("waybar", "config")},
		{Src: core.ConfigPath("waybar", "style.css"), Dst: core.XDGTarget("waybar", "style.css")},
		// Portal routing is per-desktop and keyed on XDG_CURRENT_DESKTOP, so
		// this file only takes effect under sway and leaves GNOME's portals
		// alone. Managed here so `diff`/`status` see it like any other link.
		{Src: core.ConfigPath("sway", "sway-portals.conf"), Dst: core.XDGTarget("xdg-desktop-portal", "sway-portals.conf")},
	}
}

func swayInstalled() bool {
	_, err := exec.LookPath("sway")
	return err == nil
}

func (m SwayModule) Install(ctx context.Context) error {
	// A compositor config is meaningless on WSL and on any box without sway;
	// follow ghostty's precedent and no-op rather than littering ~/.config.
	if !swayInstalled() {
		core.Debug("sway not installed — skipping config")
		return nil
	}
	core.Info("Linking sway + waybar config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	core.Ok("sway config done")
	return nil
}

func (m SwayModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("sway config uninstalled")
	return nil
}

func (m SwayModule) Status() core.ModuleStatus {
	// Report nothing rather than "missing" when sway isn't installed —
	// Install skips it, so it isn't a gap.
	if !swayInstalled() {
		return core.ModuleStatus{Name: "sway"}
	}
	return m.Links().Status("sway")
}
