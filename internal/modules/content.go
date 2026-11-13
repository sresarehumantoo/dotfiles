package modules

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
)

// DefaultContentRepo is the dotfiles repository `init` clones when given none.
const DefaultContentRepo = "https://github.com/sresarehumantoo/dotfiles.git"

// InitOptions are the inputs to InitContent. Every field is optional.
type InitOptions struct {
	Repo string // clone URL; DefaultContentRepo when empty
	Ref  string // branch or tag; the repository's default branch when empty
	Dir  string // checkout location; DefaultContentDir when empty
	// Clone makes a clone even where this dfinstall has packaged content.
	// Repo, Ref and Dir imply it.
	Clone bool
	// Packaged installs the content this dfinstall's package or release
	// ships, instead of cloning.
	Packaged bool
	// Force switches a clone install to packaged content even when the
	// clone holds edits or commits that would stop being linked.
	Force bool
}

// DefaultContentDir is where `init` puts the checkout when not told otherwise.
func DefaultContentDir() string {
	return core.AppDataPath("dotfiles")
}

// InitContent gives a binary that runs from no clone the content it links
// from: the content its package or release ships, or a clone it makes (or
// adopts) when asked for one or when the build ships none. With no options and
// content already recorded it does nothing, so it is safe to run again, and an
// existing clone install stays one.
//
// Before cloning it asks what git may do here, and fails naming the reason,
// pointing at packaged content where this build has some.
//
// The default branch is the default ref because `update` fast-forwards the
// checkout's branch, which a tag checkout does not have, and main only ever
// moves to a release.
func InitContent(ctx context.Context, o InitOptions) (string, error) {
	clone := o.Clone || o.Repo != "" || o.Ref != "" || o.Dir != ""
	if o.Packaged {
		if clone {
			return "", errors.New("--packaged takes no --clone, --repo, --ref or --dir")
		}
		return initPackaged(ctx, o.Force)
	}
	if o.Dir == "" {
		// --clone from packaged content goes on to clone and move the links.
		if canon := core.ReadCanonicalDir(); canon != "" && !(o.Clone && core.UsingPackagedContent()) {
			core.Status("dotfiles content already set up at %s", canon)
			return canon, nil
		}
		if !clone && packagedContentAvailable() {
			return initPackaged(ctx, false)
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

	switch a := gitReachable(ctx, gitAccessLocal(ctx, o.Repo)); {
	case a.usable():
	case a.State == gitUnreachable:
		if packagedContentAvailable() {
			return "", fmt.Errorf("cannot reach %s: %s; if git cannot reach it from this network, run: dfinstall init --packaged", o.Repo, a.Reason)
		}
		return "", fmt.Errorf("cannot reach %s: %s", o.Repo, a.Reason)
	case packagedContentAvailable():
		return "", fmt.Errorf("cannot clone %s: %s; to use the content packaged with this dfinstall instead, run: dfinstall init --packaged", o.Repo, a.Reason)
	default:
		return "", fmt.Errorf("cannot clone %s: %s, and a %s build of dfinstall has no packaged content to use instead", o.Repo, a.Reason, core.BuildChannel())
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

// adoptContent records dir as the dotfiles checkout. Leaving packaged content
// for it also moves the links back, so `init --dir <clone>` undoes
// `init --packaged` in one step.
func adoptContent(dir, format string, args ...any) error {
	prev := core.ReadCanonicalDir()
	if prev == dir {
		core.Status("dotfiles checkout already set up at %s", dir)
		return nil
	}
	leaving := prev != "" && prev == core.ContentCurrentDir()
	if core.DryRun {
		core.Status("would record %s as the dotfiles checkout", dir)
		if leaving {
			core.Status("would move the links from packaged content to %s", dir)
		}
		return nil
	}
	var sess *core.InstallSession
	if leaving {
		var err error
		if sess, err = core.BeginInstall(core.InstallOptions{ContentOnly: true, ForceBackup: true}); err != nil {
			return err
		}
		defer sess.Finish()
	}
	prev, changed := core.AdoptCanonical(dir)
	if core.ReadCanonicalDir() != dir {
		if sess != nil {
			sess.MarkFailed()
		}
		return fmt.Errorf("could not record %s as the dotfiles checkout", dir)
	}
	core.Status(format, args...)
	if leaving {
		n := repointLinks(prev) + relinkSeeds()
		core.Status("moved %d link(s) from packaged content to %s", n, dir)
		return nil
	}
	if changed && prev != "" {
		core.Notice("dotfiles checkout switched from %s", prev)
	}
	core.Status("next: dfinstall install all")
	return nil
}

// PrepareContent brings packaged content to this dfinstall's version before
// an install links from it, as after a package upgrade. A clone install is
// left alone; pulling it is --upgrade's job.
func PrepareContent(ctx context.Context, sess *core.InstallSession) error {
	if !core.UsingPackagedContent() || core.ContentUpToDate(core.Version) {
		return nil
	}
	if v := core.CurrentContentVersion(); v != "" && v != core.Version {
		core.Status("updating packaged content %s to %s", v, core.Version)
	}
	if err := packagedSync(ctx, sess); err != nil {
		return fmt.Errorf("packaged content not updated, linking from %s: %w", core.CurrentContentVersion(), err)
	}
	return nil
}

// packagedSync installs this binary's packaged content. Tests replace it,
// since where that content comes from depends on how the test binary was
// built.
var packagedSync = syncPackagedContent

// packagedContentAvailable reports whether this dfinstall has packaged
// content to install: beside it for a deb or Homebrew install, in its
// release for a release binary.
func packagedContentAvailable() bool {
	switch core.BuildChannel() {
	case core.ChannelDeb, core.ChannelBrew:
		return core.PackagedContentDir() != ""
	case core.ChannelRelease:
		return true
	}
	return false
}

// initPackaged installs this binary's packaged content and makes it the
// content links come from. From a clone install it is a migration: refused
// while the clone holds edits or commits that would stop applying, backed up,
// and the existing links are moved over. The clone itself is never touched.
func initPackaged(ctx context.Context, force bool) (string, error) {
	if os.Getenv("DOTFILES") != "" {
		return "", errors.New("$DOTFILES is set and overrides packaged content; unset it first")
	}
	store := core.ContentCurrentDir()
	if err := core.CheckTarget(store); err != nil {
		return "", err
	}
	prev := core.ReadCanonicalDir()
	from := ""
	if prev != store {
		from = prev
	}
	if from != "" {
		kept, err := cloneOnlyChanges(ctx, from)
		if err != nil {
			return "", err
		}
		if len(kept) > 0 && !force {
			return "", fmt.Errorf("%s has changes that would stop applying with packaged content:\n  %s\ncommit and push them, move them into override files with 'dfinstall update', or rerun with --force",
				from, strings.Join(kept, "\n  "))
		}
		for _, k := range kept {
			core.AlwaysWarn("no longer applied: %s", k)
		}
	}

	sess, err := core.BeginInstall(core.InstallOptions{ContentOnly: true, ForceBackup: from != ""})
	if err != nil {
		return "", err
	}
	defer sess.Finish()
	if err := packagedSync(ctx, sess); err != nil {
		sess.MarkFailed()
		return "", err
	}
	if core.DryRun {
		core.Status("would record packaged content as the dotfiles content")
		if from != "" {
			core.Status("would move the links from %s to packaged content", from)
		}
		return store, nil
	}
	core.AdoptCanonical(store)
	if core.ReadCanonicalDir() != store {
		sess.MarkFailed()
		return "", errors.New("could not record packaged content as the dotfiles content")
	}
	version := core.CurrentContentVersion()
	if from == "" {
		core.Status("using packaged content %s", version)
		if prev != store {
			core.Status("next: dfinstall install all")
		}
		return store, nil
	}
	if _, err := core.WriteFileIfChanged(core.AppDataPath("previous-clone"), []byte(from+"\n"), 0o644); err != nil {
		core.Warn("could not record the previous clone: %v", err)
	}
	n := repointLinks(from)
	core.Status("using packaged content %s; moved %d link(s) from %s", version, n, from)
	core.Status("%s is left as it is; to switch back: dfinstall init --dir %s", from, from)
	return store, nil
}

// cloneOnlyChanges lists what applies on this machine only because links point
// into the clone at dir: uncommitted edits under config/, and commits no
// remote has. A directory that is not a git checkout has nothing to list.
// Edits to seeded files are left out: the migration copies those files, edits
// and all.
func cloneOnlyChanges(ctx context.Context, dir string) ([]string, error) {
	if _, err := gitOut(ctx, dir, "rev-parse", "--git-dir"); err != nil {
		return nil, nil
	}
	var kept []string
	edited, err := gitLines(ctx, dir, "diff", "--name-only", "HEAD", "--", "config")
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	seeded := seededSources(dir)
	for _, rel := range edited {
		if !slices.Contains(seeded, rel) {
			kept = append(kept, "edited "+rel)
		}
	}
	unpushed, err := gitOut(ctx, dir, "rev-list", "--count", "HEAD", "--not", "--remotes")
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	if unpushed != "0" {
		kept = append(kept, unpushed+" commit(s) on "+dir+" that no remote has")
	}
	return kept, nil
}

// managedPairs is every file any module manages, whatever its policy.
func managedPairs() core.LinkSet {
	var all core.LinkSet
	for _, m := range core.AllModules() {
		if le, ok := m.(core.LinkExporter); ok {
			all = append(all, le.Links()...)
		}
	}
	return all
}

// seededSources lists the seeded files' paths relative to root, the content
// the links currently come from.
func seededSources(root string) []string {
	var rels []string
	for _, l := range managedPairs() {
		if l.Policy != core.PolicySeed {
			continue
		}
		if rel, err := filepath.Rel(root, l.Src); err == nil && !strings.HasPrefix(rel, "..") {
			rels = append(rels, filepath.ToSlash(rel))
		}
	}
	return rels
}

// relinkSeeds turns the seeded copies back into links to the clone now
// recorded, wherever that loses nothing. Returns how many it turned.
func relinkSeeds() int {
	n := 0
	for _, l := range managedPairs() {
		if l.Policy != core.PolicySeed {
			continue
		}
		ok, err := core.SeedToLink(l.Src, l.Dst)
		if err != nil {
			core.Warn("could not link %s again: %v", l.Dst, err)
		}
		if ok {
			n++
		}
	}
	return n
}

// repointLinks moves every managed link that points into the content root
// from to the same file under the current dotfiles content. Links anywhere
// else, and paths that are not links, are left alone. Returns how many moved.
func repointLinks(from string) int {
	prefix := strings.TrimSuffix(from, "/") + "/"
	root := core.DotfilesDir()
	moved := 0
	for _, l := range managedPairs() {
		target, err := os.Readlink(l.Dst)
		if err != nil || !strings.HasPrefix(target, prefix) || l.Src != core.SubPath(root, strings.TrimPrefix(target, prefix)) {
			continue
		}
		if err := l.Apply(); err != nil {
			core.Warn("could not move %s: %v", l.Dst, err)
			continue
		}
		if l.Check() == "ok" {
			moved++
		}
	}
	return moved
}
