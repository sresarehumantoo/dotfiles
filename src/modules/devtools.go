package modules

import (
	"fmt"
	"os"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type DevtoolsModule struct{}

func (DevtoolsModule) Name() string { return "devtools" }

var devtoolsScripts = []struct{ src, dst string }{
	{"devtools/_lib.sh", ".local/bin/_lib.sh"},
	{"devtools/wsl-resize-disk", ".local/bin/wsl-resize-disk"},
	{"devtools/wsl-restart", ".local/bin/wsl-restart"},
	{"devtools/docker-cleanup", ".local/bin/docker-cleanup"},
	{"devtools/git-prune-branches", ".local/bin/git-prune-branches"},
	{"devtools/sysinfo", ".local/bin/sysinfo"},
}

func (DevtoolsModule) Install() error {
	core.Info("Installing devtools scripts...")

	if err := core.EnsureDir(core.HomeTarget(".local", "bin")); err != nil {
		return err
	}

	var failed int
	for _, s := range devtoolsScripts {
		src := core.ConfigPath(s.src)
		if err := os.Chmod(src, 0755); err != nil {
			core.Warn("chmod failed for %s: %v", s.src, err)
			failed++
			continue
		}
		if err := core.LinkFile(src, core.HomeTarget(s.dst)); err != nil {
			core.Warn("link failed for %s: %v", s.dst, err)
			failed++
			continue
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d devtools script(s) failed to install", failed)
	}
	core.Ok("Devtools scripts done")
	return nil
}

func (DevtoolsModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "devtools"}
	for _, l := range devtoolsScripts {
		if core.CheckLink(core.ConfigPath(l.src), core.HomeTarget(l.dst)) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}
