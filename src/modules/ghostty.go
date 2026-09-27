package modules

import (
	"context"
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type GhosttyModule struct{}

func (GhosttyModule) Name() string { return "ghostty" }

// ghosttyShaders are the cursor-trail shaders referenced by `custom-shader` in
// the config. Ghostty resolves that relative path against the config file's
// *resolved* dir, so with config symlinked it already finds these in the repo
// (verified with `ghostty +show-config` on 1.2.0). Linking them anyway is the
// cheap insurance: it keeps the shader loading if the config is ever a real
// file rather than a symlink, or if that resolution changes upstream. Linked
// per-file rather than as a directory so uninstall removes only what we placed.
var ghosttyShaders = []string{
	"cursor_warp.glsl",
	"cursor_sweep.glsl",
	"cursor_tail.glsl",
}

func (GhosttyModule) Links() core.LinkSet {
	links := core.LinkSet{
		{Src: core.ConfigPath("ghostty", "config"), Dst: core.XDGTarget("ghostty", "config")},
	}
	for _, shader := range ghosttyShaders {
		links = append(links, core.LinkPair{
			Src: core.ConfigPath("ghostty", "shaders", shader),
			Dst: core.XDGTarget("ghostty", "shaders", shader),
		})
	}
	return links
}

func ghosttyInstalled() bool {
	if _, err := exec.LookPath("ghostty"); err == nil {
		return true
	}
	// On macOS the binary lives inside the app bundle, off PATH.
	if core.IsMac() {
		for _, app := range []string{"/Applications/Ghostty.app", core.HomeTarget("Applications", "Ghostty.app")} {
			if _, err := os.Stat(app); err == nil {
				return true
			}
		}
	}
	return false
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
