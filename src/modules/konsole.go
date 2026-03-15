package modules

import "github.com/sresarehumantoo/dotfiles/src/core"

type KonsoleModule struct{}

func (KonsoleModule) Name() string { return "konsole" }

var konsoleLinks = []struct{ src, dst string }{
	{src: "konsolerc", dst: "konsolerc"},
	{src: "Dotfiles.profile", dst: "Dotfiles.profile"},
	{src: "Dotfiles.colorscheme", dst: "Dotfiles.colorscheme"},
}

func (KonsoleModule) Install() error {
	core.Info("Linking Konsole config...")
	if err := core.EnsureDir(core.HomeTarget(".local", "share", "konsole")); err != nil {
		return err
	}
	// konsolerc → ~/.config/konsolerc
	if err := core.LinkFile(
		core.ConfigPath("konsole", "konsolerc"),
		core.XDGTarget("konsolerc"),
	); err != nil {
		return err
	}
	// profile and colorscheme → ~/.local/share/konsole/
	for _, l := range konsoleLinks[1:] {
		if err := core.LinkFile(
			core.ConfigPath("konsole", l.src),
			core.HomeTarget(".local", "share", "konsole", l.dst),
		); err != nil {
			return err
		}
	}
	core.Ok("Konsole config done")
	return nil
}

func (KonsoleModule) Uninstall() error {
	if err := core.UnlinkFile(
		core.ConfigPath("konsole", "konsolerc"),
		core.XDGTarget("konsolerc"),
	); err != nil {
		return err
	}
	for _, l := range konsoleLinks[1:] {
		if err := core.UnlinkFile(
			core.ConfigPath("konsole", l.src),
			core.HomeTarget(".local", "share", "konsole", l.dst),
		); err != nil {
			return err
		}
	}
	core.Ok("Konsole config uninstalled")
	return nil
}

func (KonsoleModule) Links() []core.LinkPair {
	pairs := []core.LinkPair{
		{Src: core.ConfigPath("konsole", "konsolerc"), Dst: core.XDGTarget("konsolerc")},
	}
	for _, l := range konsoleLinks[1:] {
		pairs = append(pairs, core.LinkPair{
			Src: core.ConfigPath("konsole", l.src),
			Dst: core.HomeTarget(".local", "share", "konsole", l.dst),
		})
	}
	return pairs
}

func (KonsoleModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "konsole"}
	// konsolerc
	if core.CheckLink(core.ConfigPath("konsole", "konsolerc"), core.XDGTarget("konsolerc")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	// profile and colorscheme
	for _, l := range konsoleLinks[1:] {
		if core.CheckLink(
			core.ConfigPath("konsole", l.src),
			core.HomeTarget(".local", "share", "konsole", l.dst),
		) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}
