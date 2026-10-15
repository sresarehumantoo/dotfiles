package modules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// updateClone fast-forwards a plugin or theme dfinstall cloned earlier. One
// with local commits, or edits in the files upstream changed, fails the
// fast-forward and is left as it is.
func updateClone(ctx context.Context, name, dir string) {
	if !core.UpgradeMode {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return
	}
	if err := runNetCmd(ctx, "git", "-C", dir, "pull", "--ff-only", "-q"); err != nil {
		core.Warn("%s not updated: %v", name, err)
		return
	}
	core.Ok("%s updated", name)
}
