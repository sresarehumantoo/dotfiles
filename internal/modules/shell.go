package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

type ShellModule struct{}

func (ShellModule) Name() string { return "shell" }

var shellLinks = []struct{ src, dst string }{
	{"shell/zshrc", ".zshrc"},
	{"shell/aliases", ".aliases"},
	{"shell/p10k.zsh", ".p10k.zsh"},
	{"shell/bashrc", ".bashrc"},
	{"shell/profile", ".profile"},
	// zsh never reads ~/.profile; this is the only thing that bridges it. See
	// the header of config/shell/zprofile for what silently did not apply.
	{"shell/zprofile", ".zprofile"},
	{"shell/zsh/options.zsh", ".zsh.d/options.zsh"},
	{"shell/zsh/keybinds.zsh", ".zsh.d/keybinds.zsh"},
	{"shell/zsh/path.zsh", ".zsh.d/path.zsh"},
	{"shell/zsh/exports.zsh", ".zsh.d/exports.zsh"},
	{"shell/zsh/ssh.zsh", ".zsh.d/ssh.zsh"},
	{"shell/zsh/locale.zsh", ".zsh.d/locale.zsh"},
	{"shell/zsh/drift.zsh", ".zsh.d/drift.zsh"},
	{"shell/zsh/git.zsh", ".zsh.d/git.zsh"},
}

func (ShellModule) Install(ctx context.Context) error {
	if newFiles := FilterNewFiles(ScanCustomShellFiles()); len(newFiles) > 0 {
		// Kept by default: dropping a file the old shell sourced is the
		// surprising outcome, and keeping one is a line in dismissed_files away
		// from undoing.
		var rels, names []string
		for _, f := range newFiles {
			rels = append(rels, f.RelPath)
			names = append(names, "~/"+f.RelPath)
		}
		if core.DryRun {
			core.Changed("would keep sourcing %s", strings.Join(names, ", "))
		} else {
			core.Cfg.PreservedFiles = MergeUnique(core.Cfg.PreservedFiles, rels)
			core.Changed("kept sourcing %s; to stop, list it under dismissed_files in %s", strings.Join(names, ", "), core.ConfigFilePath())
			if err := core.SaveConfig(); err != nil {
				core.Warn("failed to save config: %v", err)
			}
		}
	}

	if !core.DryRun {
		// Drop preserved entries that no longer exist or aren't sourceable
		// (e.g. .dmrc captured by an older dfinstall before the content
		// sniffer existed — would error at every shell start).
		if cleaned, dropped := SanitizePreservedFiles(core.Cfg.PreservedFiles); len(dropped) > 0 {
			core.Notice("Removing %d unsourceable preserved file(s): %v", len(dropped), dropped)
			core.Cfg.PreservedFiles = cleaned
			if err := core.SaveConfig(); err != nil {
				core.Warn("failed to save config after sanitize: %v", err)
			}
		}

		if sourced := SourcedFiles(); len(sourced) > 0 {
			if err := WriteCustomSourcesFile(sourced); err != nil {
				core.Warn("failed to write custom-sources.zsh: %v", err)
			}
		} else {
			// Nothing to source — remove any stale generated file
			if err := os.Remove(CustomSourcesFilePath()); err == nil {
				core.Changed("removed %s: no files left to source", CustomSourcesFilePath())
			} else if !os.IsNotExist(err) {
				core.Debug("could not remove stale custom-sources.zsh: %v", err)
			}
		}

		// Check for alias/function collisions between managed and preserved files
		ReportAliasCollisions()
	}

	core.Info("Linking shell dotfiles...")
	if err := (ShellModule{}).Links().Apply(); err != nil {
		return err
	}

	installCompletions(ctx)

	core.Ok("Shell dotfiles done")
	return nil
}

// completionDst returns the path where zsh completions are installed.
func completionDst() string {
	return core.HomeTarget(".zsh.d", "_dfinstall.zsh")
}

// installCompletions generates and installs zsh completions for dfinstall.
func installCompletions(ctx context.Context) {
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

	// Under `go run` the binary is a temporary build, so take completions
	// from the clone's. Only then: an installed dfinstall must not describe
	// itself with whatever older build is still in the clone.
	if strings.Contains(exe, "go-build") {
		builtExe := filepath.Join(core.DotfilesDir(), "bin", "dfinstall")
		if _, err := os.Stat(builtExe); err == nil {
			exe = builtExe
		}
	}

	raw, err := runProbe(ctx, exe, "completion", "zsh")
	if err != nil {
		core.Debug("completions: generation failed: %v", err)
		return
	}

	// Strip any banner output before the actual completion script
	out := raw
	if idx := strings.Index(string(raw), "#compdef"); idx >= 0 {
		out = raw[idx:]
	}

	// Self-bootstrap compinit when sourced before any framework has done
	// so (e.g. if oh-my-zsh failed to install). The user sees their
	// completions either way; without this they'd see
	// `command not found: compdef` at every shell start.
	guard := []byte("(( $+functions[compdef] )) || { autoload -Uz compinit && compinit -u 2>/dev/null }\n" +
		"(( $+functions[compdef] )) || return 0\n")
	out = append(guard, out...)

	dst := completionDst()
	if err := core.EnsureDir(filepath.Dir(dst)); err != nil {
		core.Warn("completions: %v", err)
		return
	}

	if _, err := core.WriteFileIfChanged(dst, out, 0644); err != nil {
		core.Warn("completions: %v", err)
	}
}

func (m ShellModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
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

// shellLinks stays the source table because shell_preserve.go needs the
// destinations in their $HOME-relative form to exclude them from its scan.
func (ShellModule) Links() core.LinkSet {
	ls := make(core.LinkSet, len(shellLinks))
	for i, l := range shellLinks {
		ls[i] = core.LinkPair{Src: core.ConfigPath(l.src), Dst: core.HomeTarget(l.dst)}
	}
	return ls
}

func (m ShellModule) Status() core.ModuleStatus {
	s := m.Links().Status("shell")
	if n := len(SourcedFiles()); n > 0 {
		s.Extra = fmt.Sprintf("+%d preserved", n)
	}
	return s
}
