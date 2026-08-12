package modules

import (
	"context"
	"fmt"
	"os"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type DevtoolsModule struct{}

func (DevtoolsModule) Name() string { return "devtools" }

// Each script links from config/devtools/<name> to ~/.local/bin/<name>.
var devtoolsScripts = []string{
	"_lib.sh",
	"wsl-resize-disk",
	"wsl-restart",
	"docker-cleanup",
	"git-prune-branches",
	"sysinfo",
	"tlog-clean",
	"clipboard-vm",
	"tmux-restore",
	"demorec",
	"wsl-ffmpeg",
	"ghostty-shader",
}

func (DevtoolsModule) Links() core.LinkSet {
	ls := make(core.LinkSet, len(devtoolsScripts))
	for i, name := range devtoolsScripts {
		ls[i] = core.LinkPair{
			Src: core.ConfigPath("devtools", name),
			Dst: core.HomeTarget(".local", "bin", name),
		}
	}
	return ls
}

// Install can't use LinkSet.Apply directly: each script needs the executable
// bit set on the source (the symlink inherits it), and a single bad script
// shouldn't abort the rest.
func (m DevtoolsModule) Install(ctx context.Context) error {
	core.Info("Installing devtools scripts...")

	var failed int
	for _, l := range m.Links() {
		if !core.DryRun {
			if err := os.Chmod(l.Src, 0755); err != nil {
				core.Warn("chmod failed for %s: %v", l.Src, err)
				failed++
				continue
			}
		}
		if err := core.LinkFile(l.Src, l.Dst); err != nil {
			core.Warn("link failed for %s: %v", l.Dst, err)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d devtools script(s) failed to install", failed)
	}
	core.Ok("Devtools scripts done")
	return nil
}

func (m DevtoolsModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("Devtools scripts uninstalled")
	return nil
}

func (m DevtoolsModule) Status() core.ModuleStatus { return m.Links().Status("devtools") }
