package modules

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// DefaultContentRepo is the dotfiles repository `init` clones when given none.
const DefaultContentRepo = "https://github.com/sresarehumantoo/dotfiles.git"

// InitOptions are the inputs to InitContent. Every field is optional.
type InitOptions struct {
	Repo string // clone URL; DefaultContentRepo when empty
	Ref  string // branch or tag; the repository's default branch when empty
	Dir  string // checkout location; DefaultContentDir when empty
}

// DefaultContentDir is where `init` puts the checkout when not told otherwise.
func DefaultContentDir() string {
	return core.AppDataPath("dotfiles")
}

// InitContent gives a binary that runs from no clone the checkout it links
// from: it clones one (or adopts one already at the target) and records it as
// the canonical dotfiles dir. With no Dir and a valid canonical checkout
// already recorded it does nothing, so it is safe to run again.
//
// The default branch is the default ref because `update` fast-forwards the
// checkout's branch, which a tag checkout does not have, and main only ever
// moves to a release.
func InitContent(ctx context.Context, o InitOptions) (string, error) {
	if o.Dir == "" {
		if canon := core.ReadCanonicalDir(); canon != "" {
			core.Status("dotfiles checkout already set up at %s", canon)
			return canon, nil
		}
		o.Dir = DefaultContentDir()
	}
	if o.Dir == "" {
		return "", fmt.Errorf("checkout location cannot be resolved")
	}
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return "", err
	}
	if err := core.CheckTarget(dir); err != nil {
		return "", err
	}
	if o.Repo == "" {
		o.Repo = DefaultContentRepo
	}

	if core.LooksLikeDotfilesRepo(dir) {
		return dir, adoptContent(dir, "using the existing checkout at %s", dir)
	}
	switch entries, err := os.ReadDir(dir); {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", err
	case len(entries) > 0:
		return "", fmt.Errorf("%s exists and is not a dotfiles checkout; choose another --dir or empty it", dir)
	}

	if core.DryRun {
		core.Status("would clone %s into %s and record it as the dotfiles checkout", o.Repo, dir)
		return dir, nil
	}

	args := []string{"clone"}
	if o.Ref != "" {
		args = append(args, "--branch", o.Ref)
	}
	args = append(args, "--", o.Repo, dir)
	if err := runNetCmd(ctx, "git", args...); err != nil {
		return "", fmt.Errorf("clone %s: %w", o.Repo, err)
	}
	if !core.LooksLikeDotfilesRepo(dir) {
		if err := core.RemoveManagedDir(dir); err != nil {
			core.Warn("could not remove %s: %v", dir, err)
		}
		return "", fmt.Errorf("%s has no config/ directory, so it is not a dotfiles repository", o.Repo)
	}
	if _, err := gitOut(ctx, dir, "symbolic-ref", "-q", "HEAD"); err != nil {
		core.Notice("%s is checked out detached, so 'dfinstall update' cannot pull it; 'git -C %s switch <branch>' to track one", o.Ref, dir)
	}
	return dir, adoptContent(dir, "cloned %s into %s", o.Repo, dir)
}

func adoptContent(dir, format string, args ...any) error {
	if core.ReadCanonicalDir() == dir {
		core.Status("dotfiles checkout already set up at %s", dir)
		return nil
	}
	if core.DryRun {
		core.Status("would record %s as the dotfiles checkout", dir)
		return nil
	}
	prev, changed := core.AdoptCanonical(dir)
	if core.ReadCanonicalDir() != dir {
		return fmt.Errorf("could not record %s as the dotfiles checkout", dir)
	}
	core.Status(format, args...)
	if changed && prev != "" {
		core.Notice("dotfiles checkout switched from %s", prev)
	}
	core.Status("next: dfinstall install all")
	return nil
}
