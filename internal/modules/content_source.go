package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// contentArchiveName is the content archive a release publishes, following
// the content archive's name_template in .goreleaser.yaml.
func contentArchiveName(tag string) string {
	return fmt.Sprintf("%s-content_%s.tar.gz", core.AppDirName, strings.TrimPrefix(tag, "v"))
}

// syncPackagedContent brings the content store to this binary's version from
// wherever its install channel ships content: beside the binary for a deb or
// Homebrew install, the matching release's archive for a release binary.
func syncPackagedContent(ctx context.Context, sess *core.InstallSession) error {
	switch ch := core.BuildChannel(); ch {
	case core.ChannelDeb, core.ChannelBrew:
		dir := core.PackagedContentDir()
		if dir == "" {
			return fmt.Errorf("this %s install of dfinstall has no packaged content beside it; reinstall the package", ch)
		}
		return sess.SyncContentDir(dir, core.Version)
	case core.ChannelRelease:
		return syncReleaseContent(ctx, sess, core.Version)
	default:
		return fmt.Errorf("a %s build of dfinstall has no packaged content; use a clone of the dotfiles repo", ch)
	}
}

// syncReleaseContent downloads tag's content archive, checks it against its
// published digest and installs it. Nothing is fetched when the store already
// holds that version intact, or under DryRun.
func syncReleaseContent(ctx context.Context, sess *core.InstallSession, tag string) error {
	if core.ContentUpToDate(tag) {
		return nil
	}
	name := contentArchiveName(tag)
	if core.DryRun {
		core.Changed("would download %s and install content %s", name, tag)
		return nil
	}
	assets, err := taggedRelease(ctx, releaseRepo, tag)
	if err != nil {
		return err
	}
	return installReleaseContent(ctx, sess, tag, assets)
}

// installReleaseContent downloads and installs tag's content archive from a
// release's assets.
func installReleaseContent(ctx context.Context, sess *core.InstallSession, tag string, assets []ghAsset) error {
	name := contentArchiveName(tag)
	i := slices.IndexFunc(assets, func(a ghAsset) bool { return a.Name == name })
	if i < 0 {
		return fmt.Errorf("release %s has no %s", tag, name)
	}
	tmp, err := os.MkdirTemp("", "dfinstall-content-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, name)
	if err := downloadVerified(ctx, assets[i], archive); err != nil {
		return err
	}
	return sess.SyncContentTarball(archive, tag)
}
