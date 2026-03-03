package modules

import "github.com/sresarehumantoo/dotfiles/src/core"

type GitModule struct{}

func (GitModule) Name() string { return "git" }

func (GitModule) Install() error {
	core.Info("Linking git config...")
	if err := core.LinkFile(core.ConfigPath("git", "gitconfig"), core.HomeTarget(".gitconfig")); err != nil {
		return err
	}
	core.Ok("Git config done")
	return nil
}

func (GitModule) Uninstall() error {
	if err := core.UnlinkFile(core.ConfigPath("git", "gitconfig"), core.HomeTarget(".gitconfig")); err != nil {
		return err
	}
	core.Ok("Git config uninstalled")
	return nil
}

func (GitModule) Links() []core.LinkPair {
	return []core.LinkPair{
		{Src: core.ConfigPath("git", "gitconfig"), Dst: core.HomeTarget(".gitconfig")},
	}
}

func (GitModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "git"}
	if core.CheckLink(core.ConfigPath("git", "gitconfig"), core.HomeTarget(".gitconfig")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	return s
}
