package modules

import (
	"fmt"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

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
	{"shell/zsh/ssh.zsh", ".zsh.d/ssh.zsh"},
}

func (ShellModule) Install() error {
	// Scan for custom shell files before linking overwrites zshrc
	discovered := ScanCustomShellFiles()
	newFiles := FilterNewFiles(discovered)

	if len(newFiles) > 0 {
		core.PauseSpinner()
		preserved, dismissed, err := RunPreserveMenu(newFiles)
		core.ResumeSpinner()

		if err != nil {
			core.Warn("custom file preservation: %v", err)
		} else {
			changed := false
			if len(preserved) > 0 {
				core.Cfg.PreservedFiles = MergeUnique(core.Cfg.PreservedFiles, preserved)
				changed = true
			}
			if len(dismissed) > 0 {
				core.Cfg.DismissedFiles = MergeUnique(core.Cfg.DismissedFiles, dismissed)
				changed = true
			}
			if changed {
				if err := core.SaveConfig(); err != nil {
					core.Warn("failed to save config: %v", err)
				}
			}
		}
	}

	// Write custom-sources.zsh if there are any preserved files
	if len(core.Cfg.PreservedFiles) > 0 {
		if err := WriteCustomSourcesFile(core.Cfg.PreservedFiles); err != nil {
			core.Warn("failed to write custom-sources.zsh: %v", err)
		}
	}

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
	if n := len(core.Cfg.PreservedFiles); n > 0 {
		s.Extra = fmt.Sprintf("+%d preserved", n)
	}
	return s
}
