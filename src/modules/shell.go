package modules

import "github.com/sresarehumantoo/dotfiles/src/core"

type ShellModule struct{}

func (ShellModule) Name() string { return "shell" }

var shellLinks = []struct{ src, dst string }{
	{"shell/zshrc", ".zshrc"},
	{"shell/aliases", ".aliases"},
	{"shell/p10k.zsh", ".p10k.zsh"},
	{"shell/bashrc", ".bashrc"},
	{"shell/profile", ".profile"},
	{"shell/zsh/options.zsh", ".zsh.d/options.zsh"},
	{"shell/zsh/keybinds.zsh", ".zsh.d/keybinds.zsh"},
	{"shell/zsh/path.zsh", ".zsh.d/path.zsh"},
	{"shell/zsh/exports.zsh", ".zsh.d/exports.zsh"},
}

func (ShellModule) Install() error {
	core.Info("Linking shell dotfiles...")
	for _, l := range shellLinks {
		if err := core.LinkFile(core.ConfigPath(l.src), core.HomeTarget(l.dst)); err != nil {
			return err
		}
	}
	core.Ok("Shell dotfiles done")
	return nil
}

func (ShellModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "shell"}
	for _, l := range shellLinks {
		if core.CheckLink(core.ConfigPath(l.src), core.HomeTarget(l.dst)) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}
