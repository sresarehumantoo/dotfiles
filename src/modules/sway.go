package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
		// swayosd's overlay. Linked because its upstream default colours come
		// from the system GTK theme, which renders a light pill on this dark
		// desktop. ⚠ That file is GTK4 CSS, unlike every other stylesheet this
		// module links — see its header before editing.
		{Src: core.ConfigPath("swayosd", "style.css"), Dst: core.XDGTarget("swayosd", "style.css")},
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
		// Brightness with a floor and a 0-100 user scale, behind the
		// XF86MonBrightness keys. Not swayosd-client directly: 0.1.0 has no
		// minimum and will take the panel to black.
		{Src: core.ConfigPath("sway", "sway-brightness"), Dst: core.HomeTarget(".local", "bin", "sway-brightness")},
	}
}

// Scripts that must be executable at the source, since a symlink inherits the
// target's mode (same reason as devtools).
var swayScripts = []string{"sway-powermenu", "sway-quickpanel", "sway-brightness"}

// swayPackages is the desktop this repo's sway config actually describes.
//
// WHY THIS LIST EXISTS: the configs name these binaries directly — in keybinds,
// in waybar `exec`/`on-click`, in swaync's button grid — and a missing one does
// not announce itself. It produces a key that does nothing, a bar module that
// silently hides, or a panel button that no-ops. The failure is invisible, so
// the dependency has to be declared somewhere a fresh machine will read.
//
// Grouped by what breaks without it. Names are canonical (apt); resolvePkgs
// maps them for pacman, and installPkg drops anything with no candidate in the
// configured sources rather than failing the whole batch.
var swayPackages = []string{
	// Compositor and session.
	"sway",
	"swaybg",        // `output * bg` in config/sway/config
	"swayidle",      // the idle/lock timeout chain
	"swaylock",      // $lock, and the swaync panel's lock button
	"swaynag",       // the $mod+Shift+e exit confirmation
	"xwayland",      // X11 clients under sway
	"fuzzel",        // launcher
	"brightnessctl", // backlight fallback when swayosd is absent
	// ⚠ No terminal here. `set $term ghostty` in config/sway/config, and
	// ghostty is not an apt package on Debian — it has its own module. Adding
	// a "fallback" terminal such as foot would install software nobody asked
	// for on every sway box, which is the opposite of what this list is for.

	// Bar and notifications.
	"waybar",
	"sway-notification-center", // swaync — the panel and the bell
	"mako-notifier",            // documented one-line fallback for swaync

	// Status-area feature set. These are what make the restrained bar work:
	// the numbers were removed from it on the understanding that swayosd
	// shows them on screen and pavucontrol/the panel handle the details.
	"swayosd",     // on-screen volume/brightness overlay driven by the media keys
	"pavucontrol", // right-click on the volume module
	"playerctl",   // mpris module + XF86Audio play/next/prev
	"pipewire-pulse",
	"wireplumber",

	// Screenshots ($mod+Print, and the swaync grid's two capture buttons).
	"grim",
	"slurp",
	"wl-clipboard",

	// Applets the bar and panel hand off to.
	"network-manager-gnome", // nm-connection-editor
	"blueman",               // blueman-manager
}

func swayInstalled() bool {
	_, err := exec.LookPath("sway")
	return err == nil
}

// missingSwayPackages returns the subset of swayPackages whose marker binary is
// absent from PATH.
//
// Deliberately probes BINARIES rather than asking dpkg/pacman. A package query
// answers "is this package installed" per package manager, which differs across
// distros and says nothing on a box where sway was built from source or
// installed to /opt — which is exactly this machine's case (sway lives in
// /opt/sway-next/bin). What the configs actually require is the command being
// callable, so that is what gets checked. Same lesson as the fonts module,
// where gating on `fc-list` rather than the module's own artifact meant the
// install silently never ran.
func missingSwayPackages() []string {
	var missing []string
	for _, pkg := range swayPackages {
		if _, err := exec.LookPath(swayPkgBinary(pkg)); err != nil {
			missing = append(missing, pkg)
		}
	}
	return missing
}

// swayPkgBinary maps a package name to the command it provides, for the cases
// where they differ. Anything not listed provides a binary of the same name.
func swayPkgBinary(pkg string) string {
	switch pkg {
	case "sway-notification-center":
		return "swaync"
	case "mako-notifier":
		return "mako"
	case "network-manager-gnome":
		return "nm-connection-editor"
	case "blueman":
		return "blueman-manager"
	case "swayosd":
		return "swayosd-server"
	case "pipewire-pulse":
		return "pipewire-pulse"
	case "wireplumber":
		return "wireplumber"
	case "wl-clipboard":
		return "wl-copy"
	case "xwayland":
		return "Xwayland"
	}
	return pkg
}

func (m SwayModule) Install(ctx context.Context) error {
	// A compositor config is meaningless on WSL and on any box without sway;
	// follow ghostty's precedent and no-op rather than littering ~/.config.
	//
	// ⚠ The `InstallingAll` half is what keeps that promise while still making
	// `dfinstall install sway` able to build the desktop from nothing. Asking
	// for sway by name is a request to set it up; `install all` on a WSL box
	// or a server is not, and must never start apt-getting a compositor.
	if !swayInstalled() && core.InstallingAll {
		core.Debug("sway not installed — skipping config")
		return nil
	}

	if missing := missingSwayPackages(); len(missing) > 0 {
		if core.DryRun {
			core.Status("Would install %d missing sway package(s): %s",
				len(missing), strings.Join(missing, ", "))
		} else {
			core.Info("Installing %d missing sway package(s)...", len(missing))
			if err := installPkg(ctx, missing...); err != nil {
				// Not fatal. The configs are still worth linking — a box
				// missing pavucontrol should not also be left without a bar —
				// and Status() reports the gap either way.
				core.AlwaysWarn("some sway packages failed to install: %v", err)
			}
		}
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
	st := m.Links().Status("sway")
	// Surface absent dependencies here, because nothing else will: a missing
	// swayosd or pavucontrol leaves the links fully healthy while a media key
	// or a right-click quietly does nothing.
	if missing := missingSwayPackages(); len(missing) > 0 {
		st.Extra = fmt.Sprintf("%d package(s) missing: %s",
			len(missing), strings.Join(missing, ", "))
	}
	return st
}
