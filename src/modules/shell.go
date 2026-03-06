package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	if !core.DryRun {
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

		// Check for alias/function collisions between managed and preserved files
		ReportAliasCollisions()
	}

	core.Info("Linking shell dotfiles...")
	for _, l := range shellLinks {
		if err := core.LinkFile(core.ConfigPath(l.src), core.HomeTarget(l.dst)); err != nil {
			return err
		}
	}

	installCompletions()

	core.Ok("Shell dotfiles done")
	return nil
}

// completionDst returns the path where zsh completions are installed.
func completionDst() string {
	return core.HomeTarget(".zsh.d", "_dfinstall.zsh")
}

// installCompletions generates and installs zsh completions for dfinstall.
func installCompletions() {
	if core.DryRun {
		core.Info("would install completions to %s", completionDst())
		return
	}

	// Find the dfinstall executable
	exe, err := os.Executable()
	if err != nil {
		core.Debug("completions: could not find executable: %v", err)
		return
	}

	// Prefer the built binary if running via go run
	builtExe := filepath.Join(core.DotfilesDir(), "bin", "dfinstall")
	if _, err := os.Stat(builtExe); err == nil {
		exe = builtExe
	}

	raw, err := exec.Command(exe, "completion", "zsh").Output()
	if err != nil {
		core.Debug("completions: generation failed: %v", err)
		return
	}

	// Strip any banner output before the actual completion script
	out := raw
	if idx := strings.Index(string(raw), "#compdef"); idx >= 0 {
		out = raw[idx:]
	}

	dst := completionDst()
	if err := core.EnsureDir(filepath.Dir(dst)); err != nil {
		core.Warn("completions: %v", err)
		return
	}

	if err := os.WriteFile(dst, out, 0644); err != nil {
		core.Warn("completions: %v", err)
		return
	}
	core.Ok("installed completions: %s", dst)
}

func (ShellModule) Uninstall() error {
	for _, l := range shellLinks {
		if err := core.UnlinkFile(core.ConfigPath(l.src), core.HomeTarget(l.dst)); err != nil {
			return err
		}
	}
	// Remove generated files
	for _, f := range []string{CustomSourcesFilePath(), completionDst()} {
		if !core.DryRun {
			os.Remove(f)
		} else {
			core.Info("would remove: %s", f)
		}
	}
	core.Ok("Shell dotfiles uninstalled")
	return nil
}

func (ShellModule) Links() []core.LinkPair {
	pairs := make([]core.LinkPair, len(shellLinks))
	for i, l := range shellLinks {
		pairs[i] = core.LinkPair{Src: core.ConfigPath(l.src), Dst: core.HomeTarget(l.dst)}
	}
	return pairs
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
