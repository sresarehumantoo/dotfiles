package modules

import (
	"context"
	"fmt"
	"os"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

type DevtoolsModule struct{}

func (DevtoolsModule) Name() string { return "devtools" }

// Each script links from config/devtools/<name> to ~/.local/bin/<name>.
var devtoolsScripts = []string{
	"_lib.sh",
	"_jwt.pl",
	"_scrub.sh",
	"wsl-resize-disk",
	"wsl-restart",
	"docker-cleanup",
	"git-prune-branches",
	"git",
	"gh",
	"sysinfo",
	"tlog-clean",
	"clipboard-vm",
	"tmux-restore",
	"claude-tmux",
	"demorec",
	"wsl-ffmpeg",
	"ghostty-shader",
	"newsboat-view",
	"docs",
	"tmux-copy-clean",
	"jwt-decode",
	"jwt-encode",
}

// linuxOnlyDevtools drive WSL or read /proc, /sys or /dev/dri, so they are not
// linked on macOS.
var linuxOnlyDevtools = map[string]bool{
	"wsl-resize-disk": true,
	"wsl-restart":     true,
	"sysinfo":         true,
	"clipboard-vm":    true,
	"demorec":         true,
	"wsl-ffmpeg":      true,
	"ghostty-shader":  true,
}

// gitHooks are the githooks(5) names, the same list config/devtools/git keeps.
// Each links to the git wrapper, which scrubs attribution at commit-msg and
// hands every hook on to the repository's own.
var gitHooks = []string{
	"applypatch-msg", "pre-applypatch", "post-applypatch", "pre-commit", "pre-merge-commit",
	"prepare-commit-msg", "commit-msg", "post-commit", "pre-rebase", "post-checkout", "post-merge",
	"pre-push", "pre-receive", "update", "proc-receive", "post-receive", "post-update",
	"reference-transaction", "push-to-checkout", "pre-auto-gc", "post-rewrite",
	"sendemail-validate", "post-index-change",
}

func gitHooksDir() string { return core.AppDataPath("git-hooks") }

// gitScrubInclude is the file the tracked gitconfig includes to make
// gitHooksDir the global core.hooksPath. It exists only while devtools is
// installed, so a machine without the wrapper keeps its repositories' hooks.
func gitScrubInclude() string { return core.AppConfigPath("git-scrub.gitconfig") }

func (DevtoolsModule) Links() core.LinkSet {
	var ls core.LinkSet
	for _, name := range gitHooks {
		ls = append(ls, core.LinkPair{
			Src: core.ConfigPath("devtools", "git"),
			Dst: core.SubPath(gitHooksDir(), name),
		})
	}
	for _, name := range devtoolsScripts {
		if core.IsMac() && linuxOnlyDevtools[name] {
			continue
		}
		ls = append(ls, core.LinkPair{
			Src: core.ConfigPath("devtools", name),
			Dst: core.HomeTarget(".local", "bin", name),
		})
	}
	return ls
}

// Install can't use LinkSet.Apply directly: each script needs the executable
// bit on the source (the symlink inherits it), and a single bad script
// shouldn't abort the rest.
func (m DevtoolsModule) Install(ctx context.Context) error {
	core.Info("Installing devtools scripts...")

	var failed int
	for _, l := range m.Links() {
		core.EnsureExecutable(l.Src)
		if err := core.LinkFile(l.Src, l.Dst); err != nil {
			core.Warn("link failed for %s: %v", l.Dst, err)
			failed++
		}
	}

	include := fmt.Sprintf("# Written by dfinstall install devtools: a git that bypasses the\n# ~/.local/bin/git wrapper still runs its commit-msg scrub.\n[core]\n\thooksPath = %s\n", gitHooksDir())
	if _, err := core.WriteFileIfChanged(gitScrubInclude(), []byte(include), 0o644); err != nil {
		core.Warn("could not write %s: %v", gitScrubInclude(), err)
		failed++
	}

	if failed > 0 {
		return fmt.Errorf("%d devtools script(s) failed to install", failed)
	}
	core.Ok("Devtools scripts done")
	return nil
}

func (m DevtoolsModule) Uninstall(ctx context.Context) error {
	// The include goes first: left behind, it would point every repository's
	// hooks at a directory that no longer exists.
	if !core.DryRun {
		if err := os.Remove(gitScrubInclude()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", gitScrubInclude(), err)
		}
	}
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("Devtools scripts uninstalled")
	return nil
}

func (m DevtoolsModule) Status() core.ModuleStatus { return m.Links().Status("devtools") }
