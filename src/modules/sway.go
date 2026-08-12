package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		// swayosd's overlay. Linked because its upstream default colors come
		// from the system GTK theme, which renders a light pill on this dark
		// desktop. ⚠ That file is GTK4 CSS, unlike every other stylesheet this
		// module links — see its header before editing.
		{Src: core.ConfigPath("swayosd", "style.css"), Dst: core.XDGTarget("swayosd", "style.css")},
		// The launcher behind $mod+d. Unstyled until this file existed, which
		// left the one surface you summon and look straight at running on
		// upstream's Solarized defaults and the icon-less `hicolor` theme.
		{Src: core.ConfigPath("fuzzel", "fuzzel.ini"), Dst: core.XDGTarget("fuzzel", "fuzzel.ini")},
		// ⚠ GTK appearance, and it takes TWO files plus a `gsettings` line in
		// config/sway/config — none of the three covers the others. GTK3 apps
		// (pavucontrol, nm-connection-editor, blueman-manager: everything the
		// bar hands off to) read settings.ini; plain GTK4 apps read the GTK4
		// one; libadwaita apps ignore both and ask the portal, which is what
		// the gsettings line feeds. Miss one and that class of app opens white
		// against a dark desktop. Full reasoning in the files' own headers.
		//
		// Note the destination BASENAMES differ from the sources: GTK requires
		// `settings.ini` under a version-named directory, so the repo names
		// them by version to keep both readable side by side in config/gtk/.
		{Src: core.ConfigPath("gtk", "gtk3-settings.ini"), Dst: core.XDGTarget("gtk-3.0", "settings.ini")},
		{Src: core.ConfigPath("gtk", "gtk4-settings.ini"), Dst: core.XDGTarget("gtk-4.0", "settings.ini")},
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
		// Month calendar behind the clock (waybar's center module). A real
		// window, not a tooltip: waybar's clock renders {calendar} only in its
		// tooltip, which is hover-only and cannot be browsed.
		{Src: core.ConfigPath("sway", "sway-calendar"), Dst: core.HomeTarget(".local", "bin", "sway-calendar")},
		// Brightness with a floor and a 0-100 user scale, behind the
		// XF86MonBrightness keys. Not swayosd-client directly: 0.1.0 has no
		// minimum and will take the panel to black.
		{Src: core.ConfigPath("sway", "sway-brightness"), Dst: core.HomeTarget(".local", "bin", "sway-brightness")},
		// SwayFX effects, applied over IPC from an exec_always. NOT config
		// directives: the sway config is shared by all three GDM sessions and
		// plain sway fails the whole config load on an unknown command, so
		// putting `blur enable` in it would nag on every fallback login.
		{Src: core.ConfigPath("sway", "sway-fx"), Dst: core.HomeTarget(".local", "bin", "sway-fx")},
		// The animated 1-9 workspace row. A resident server feeding nine
		// `tail -F` clients, one per waybar module — waybar's own
		// sway/workspaces cannot render a traveling highlight, because it
		// derives every button's class from sway state and an intermediate
		// workspace is never "anything".
		{Src: core.ConfigPath("sway", "sway-workspaces"), Dst: core.HomeTarget(".local", "bin", "sway-workspaces")},

		// dwm-style monocle mode ($mod+m). A resident server plus the client
		// half the runtime keybindings call — sway has no monocle layout, and
		// `fullscreen` alone cannot be one because every focus command is a
		// no-op while a window is fullscreen. Measured, and written up in the
		// script.
		{Src: core.ConfigPath("sway", "sway-monocle"), Dst: core.HomeTarget(".local", "bin", "sway-monocle")},
	}
}

// Scripts that must be executable at the source, since a symlink inherits the
// target's mode (same reason as devtools).
var swayScripts = []string{"sway-powermenu", "sway-quickpanel", "sway-brightness", "sway-calendar", "sway-fx", "sway-workspaces", "sway-monocle"}

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

	// GObject-introspection data for gtk-layer-shell. config/sway/sway-calendar
	// does `gi.require_version("GtkLayerShell", "0.1")` — it is a layer surface,
	// not a floating window, which is what stops it appearing mid-screen and
	// sliding into place. Without this package the calendar dies on import, and
	// because waybar launches it from a click there is nowhere for the traceback
	// to go: the clock would simply do nothing. See swayPkgGlobs — this one ships
	// no binary, so it cannot be probed on PATH like everything else here.
	"gir1.2-gtklayershell-0.1",
}

// swayPkgGlobs covers packages that ship NO binary, where the PATH probe below
// cannot see them. The value is a set of globs; the package counts as present if
// any of them matches. GObject-introspection data forced this: a typelib is a
// hard runtime dependency of sway-calendar with nothing on PATH to look for.
//
// Two globs because Debian installs typelibs under a multiarch directory
// (/usr/lib/x86_64-linux-gnu/girepository-1.0) while other distros use the plain
// one — matching on the arch triplet directly would make this host-specific.
var swayPkgGlobs = map[string][]string{
	"gir1.2-gtklayershell-0.1": {
		"/usr/lib/*/girepository-1.0/GtkLayerShell-0.1.typelib",
		"/usr/lib/girepository-1.0/GtkLayerShell-0.1.typelib",
	},
}

// pkgFilePresent reports whether any of a package's marker globs matches.
func pkgFilePresent(globs []string) bool {
	for _, pattern := range globs {
		if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
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
		// Packages with no binary of their own are checked by file — see
		// swayPkgGlobs. LookPath would report every one of them missing forever.
		if globs, ok := swayPkgGlobs[pkg]; ok {
			if !pkgFilePresent(globs) {
				missing = append(missing, pkg)
			}
			continue
		}
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
