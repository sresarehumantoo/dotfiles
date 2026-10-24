package modules

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

const sketchybarFormula = "FelixKratz/formulae/sketchybar"

var sketchybarPlugins = []string{
	"aerospace.sh", "battery.sh", "caffeinate.sh", "clock.sh",
	"mode.sh", "network.sh", "volume.sh", "vpn.sh",
}

type SketchybarModule struct{}

func (SketchybarModule) Name() string { return "sketchybar" }

func (SketchybarModule) Unsupported() string { return macOnly() }

func (SketchybarModule) Links() core.LinkSet {
	if !core.IsMac() {
		return nil
	}
	links := core.LinkSet{
		{Src: core.ConfigPath("sketchybar", "sketchybarrc"), Dst: core.XDGTarget("sketchybar", "sketchybarrc")},
		{Src: core.ConfigPath("sketchybar", "colors.sh"), Dst: core.XDGTarget("sketchybar", "colors.sh")},
	}
	for _, p := range sketchybarPlugins {
		links = append(links, core.LinkPair{
			Src: core.ConfigPath("sketchybar", "plugins", p),
			Dst: core.XDGTarget("sketchybar", "plugins", p),
		})
	}
	return links
}

func sketchybarInstalled() bool {
	_, err := exec.LookPath("sketchybar")
	return err == nil
}

func (m SketchybarModule) Install(ctx context.Context) error {
	if !core.IsMac() {
		core.Ok("SketchyBar is macOS only, skipping")
		return nil
	}

	// Same split as aerospace: `install all` only configures a bar that is
	// already there, asking for it by name installs it.
	if !sketchybarInstalled() {
		if core.InstallingAll {
			core.Debug("SketchyBar not installed, skipping config")
			return nil
		}
		if core.DryRun {
			core.Status("Would install SketchyBar (brew %s)", sketchybarFormula)
		} else {
			core.Info("Installing SketchyBar...")
			if err := runCmd(ctx, "brew", "install", sketchybarFormula); err != nil {
				return fmt.Errorf("installing SketchyBar: %w", err)
			}
		}
	}

	core.Info("Linking SketchyBar config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	if core.DryRun {
		core.Status("Would (re)start the sketchybar brew service")
		return nil
	}

	// The brew service is a login item (a LaunchAgent, which macOS announces
	// and an MDM may forbid), so only an explicit `install sketchybar` creates
	// it. `install all` just reloads a bar that is already running.
	if core.InstallingAll {
		if _, err := runProbe(ctx, "pgrep", "-x", "sketchybar"); err == nil {
			if err := runCmd(ctx, "sketchybar", "--reload"); err != nil {
				core.Warn("sketchybar --reload: %v", err)
			}
		}
	} else if err := runCmd(ctx, "brew", "services", "restart", "sketchybar"); err != nil {
		// restart also starts a stopped service, and is what rereads the config.
		core.Warn("Could not start the sketchybar service: %v", err)
	}

	// The bar draws in the menu bar's strip. Hiding the native one is a system
	// setting that needs a logout, so say so rather than flip it.
	if out, err := runProbe(ctx, "defaults", "read", "NSGlobalDomain", "_HIHideMenuBar"); err != nil || strings.TrimSpace(string(out)) != "1" {
		core.Notice("Set the menu bar to hide automatically (System Settings > Menu Bar, or Control Center before macOS 26) so SketchyBar is not drawn underneath it")
	}

	core.Ok("SketchyBar done")
	return nil
}

// Uninstall stops the service and removes the config; the formula stays,
// like AeroSpace's app.
func (m SketchybarModule) Uninstall(ctx context.Context) error {
	if core.IsMac() && sketchybarInstalled() && !core.DryRun {
		if err := runCmd(ctx, "brew", "services", "stop", "sketchybar"); err != nil {
			core.Warn("Could not stop the sketchybar service: %v", err)
		}
	}
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("SketchyBar config uninstalled")
	return nil
}

func (m SketchybarModule) Status() core.ModuleStatus {
	if !core.IsMac() || !sketchybarInstalled() {
		return core.ModuleStatus{Name: "sketchybar"}
	}
	return m.Links().Status("sketchybar")
}
