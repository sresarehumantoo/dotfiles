package modules

import (
	"context"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type KonsoleModule struct{}

func (KonsoleModule) Name() string { return "konsole" }

// Links spans two roots: konsolerc lives in ~/.config, while the profile and
// colorscheme go to ~/.local/share/konsole. Spelling both out here removes the
// konsoleLinks[1:] slicing that Install/Uninstall/Links/Status each had to
// remember — adding an entry at the front used to break all four.
func (KonsoleModule) Links() core.LinkSet {
	shareDir := func(name string) string {
		return core.HomeTarget(".local", "share", "konsole", name)
	}
	return core.LinkSet{
		{Src: core.ConfigPath("konsole", "konsolerc"), Dst: core.XDGTarget("konsolerc")},
		{Src: core.ConfigPath("konsole", "Dotfiles.profile"), Dst: shareDir("Dotfiles.profile")},
		{Src: core.ConfigPath("konsole", "Dotfiles.colorscheme"), Dst: shareDir("Dotfiles.colorscheme")},
	}
}

func (m KonsoleModule) Install(ctx context.Context) error {
	core.Info("Linking Konsole config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	core.Ok("Konsole config done")
	return nil
}

func (m KonsoleModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("Konsole config uninstalled")
	return nil
}

func (m KonsoleModule) Status() core.ModuleStatus { return m.Links().Status("konsole") }
