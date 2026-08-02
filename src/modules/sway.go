package modules

import (
	"context"
	"os"
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
		// swaync is the active notification daemon. mako stays installed and
		// managed as a one-line fallback (see the autostart block in
		// config/sway/config) — mako cannot animate, which is why it lost.
		{Src: core.ConfigPath("swaync", "config.json"), Dst: core.XDGTarget("swaync", "config.json")},
		{Src: core.ConfigPath("swaync", "style.css"), Dst: core.XDGTarget("swaync", "style.css")},
		{Src: core.ConfigPath("mako", "config"), Dst: core.XDGTarget("mako", "config")},
		// Portal routing is per-desktop and keyed on XDG_CURRENT_DESKTOP, so
		// this file only takes effect under sway and leaves GNOME's portals
		// alone. Managed here so `diff`/`status` see it like any other link.
		{Src: core.ConfigPath("sway", "sway-portals.conf"), Dst: core.XDGTarget("xdg-desktop-portal", "sway-portals.conf")},
		// Session picker behind waybar's power button. Lives in ~/.local/bin
		// like the devtools scripts so waybar can name it without a path.
		{Src: core.ConfigPath("sway", "sway-powermenu"), Dst: core.HomeTarget(".local", "bin", "sway-powermenu")},
		// Volume + brightness panel behind the waybar volume/brightness
		// readouts. Same reason for living in ~/.local/bin.
		{Src: core.ConfigPath("sway", "sway-quickpanel"), Dst: core.HomeTarget(".local", "bin", "sway-quickpanel")},
	}
}

// Scripts that must be executable at the source, since a symlink inherits the
// target's mode (same reason as devtools).
var swayScripts = []string{"sway-powermenu", "sway-quickpanel"}

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
	if !core.DryRun {
		for _, name := range swayScripts {
			if err := os.Chmod(core.ConfigPath("sway", name), 0755); err != nil {
				core.Warn("chmod failed for %s: %v", name, err)
			}
		}
	}
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
