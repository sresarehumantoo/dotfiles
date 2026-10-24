package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/internal/core"
)

const aerospaceCask = "nikitabobko/tap/aerospace"

type AerospaceModule struct{}

func (AerospaceModule) Name() string { return "aerospace" }

func (AerospaceModule) Unsupported() string { return macOnly() }

func (AerospaceModule) Links() core.LinkSet {
	if !core.IsMac() {
		return nil
	}
	return core.LinkSet{
		{Src: core.ConfigPath("aerospace", "aerospace.toml"), Dst: core.XDGTarget("aerospace", "aerospace.toml")},
	}
}

func aerospaceInstalled() bool {
	if _, err := exec.LookPath("aerospace"); err == nil {
		return true
	}
	for _, app := range []string{"/Applications/AeroSpace.app", core.HomeTarget("Applications", "AeroSpace.app")} {
		if _, err := os.Stat(app); err == nil {
			return true
		}
	}
	return false
}

func (m AerospaceModule) Install(ctx context.Context) error {
	if !core.IsMac() {
		core.Ok("AeroSpace is macOS only, skipping")
		return nil
	}

	// A tiling window manager takes over every window, so `install all` only
	// configures one that is already there. Asking for it by name installs it,
	// the same split sway makes.
	if !aerospaceInstalled() {
		if core.InstallingAll {
			core.Debug("AeroSpace not installed, skipping config")
			return nil
		}
		if core.DryRun {
			core.Status("Would install AeroSpace (brew cask %s)", aerospaceCask)
		} else {
			core.Info("Installing AeroSpace...")
			if err := runCmd(ctx, "brew", caskArgs(aerospaceCask)...); err != nil {
				return fmt.Errorf("installing AeroSpace: %w", err)
			}
		}
	}

	// AeroSpace reads ~/.aerospace.toml as well, and when both exist it reports
	// the ambiguity instead of loading either. That file is the user's, so say
	// so rather than moving it.
	if legacy := core.HomeTarget(".aerospace.toml"); legacy != "" {
		if _, err := os.Lstat(legacy); err == nil {
			core.AlwaysWarn("%s also exists; AeroSpace will not load either config until it is moved aside", legacy)
		}
	}

	core.Info("Linking AeroSpace config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	core.Ok("AeroSpace config done")
	return nil
}

// Uninstall removes the config only; the app stays, like Ghostty's.
func (m AerospaceModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("AeroSpace config uninstalled")
	return nil
}

func (m AerospaceModule) Status() core.ModuleStatus {
	// Report nothing rather than "missing" when AeroSpace isn't installed —
	// `install all` skips it, so it isn't a gap.
	if !core.IsMac() || !aerospaceInstalled() {
		return core.ModuleStatus{Name: "aerospace"}
	}
	return m.Links().Status("aerospace")
}
