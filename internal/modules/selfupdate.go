package modules

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// releaseRepo is where dfinstall's own releases are published.
const releaseRepo = "sresarehumantoo/dotfiles"

// releaseBinaries are the executables a release archive carries.
var releaseBinaries = []string{"dfinstall", "dfinstall-mcp"}

// maxBinarySize bounds what extraction will write for one binary, so a
// malformed archive cannot fill the disk.
const maxBinarySize = 256 << 20

// releaseArchiveName is the archive a release publishes for one platform. It
// follows GoReleaser's default name template, which the release config keeps.
func releaseArchiveName(tag, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", core.AppDirName, strings.TrimPrefix(tag, "v"), goos, goarch)
}

// updateBinary is the binary half of `update` for every channel but source:
// a release archive replaces itself, and a binary some package manager owns is
// left to it. Returns the binary to re-exec, or "" to carry on with this one.
func updateBinary(ctx context.Context, channel string) string {
	switch channel {
	case core.ChannelRelease:
		exe, err := selfUpdate(ctx)
		if err != nil {
			core.AlwaysWarn("dfinstall not updated, continuing with %s: %v", core.Version, err)
		}
		return exe
	case core.ChannelBrew:
		core.Notice("dfinstall is installed by Homebrew; update it with 'brew upgrade dfinstall'")
	case core.ChannelDeb:
		core.Notice("dfinstall is installed from a .deb; update it with 'sudo apt-get update && sudo apt-get install --only-upgrade dfinstall'")
	case core.ChannelGo:
		core.Notice("dfinstall is installed with go install; update it with 'go install %s@latest'", core.MainPackagePath())
	default:
		core.AlwaysWarn("dfinstall was built for an unknown install channel %q; update it the way it was installed", channel)
	}
	return ""
}

// selfUpdate replaces the running binary, and its sibling, with the latest
// release when that is newer. Returns the path to re-exec, "" when nothing
// was replaced.
func selfUpdate(ctx context.Context) (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "", fmt.Errorf("locate the running binary: %w", err)
	}
	tag, assets, err := latestRelease(ctx, releaseRepo)
	if err != nil {
		return "", err
	}
	if !newerRelease(tag, core.Version) {
		core.Status("dfinstall %s is the latest release", core.Version)
		return "", nil
	}
	if core.DryRun {
		core.Notice("would update dfinstall %s to %s", core.Version, tag)
		return "", nil
	}
	if err := installRelease(ctx, tag, assets, exe); err != nil {
		return "", err
	}
	core.Status("updated dfinstall %s to %s", core.Version, tag)
	return exe, nil
}

// installRelease downloads tag's archive for this platform, checks it against
// its published digest, and swaps its binaries in beside exe. The running
// binary is always replaced, its sibling only when it is installed there too.
// Each swap is a rename within the directory, so a failure leaves either the
// old binary or the new one, never a partial file.
func installRelease(ctx context.Context, tag string, assets []ghAsset, exe string) error {
	name := releaseArchiveName(tag, runtime.GOOS, runtime.GOARCH)
	i := slices.IndexFunc(assets, func(a ghAsset) bool { return a.Name == name })
	if i < 0 {
		return fmt.Errorf("release %s has no %s", tag, name)
	}
	binDir := filepath.Dir(exe)
	tmp, err := os.MkdirTemp(binDir, ".dfinstall-update-")
	if err != nil {
		return fmt.Errorf("cannot write to %s, where dfinstall is installed: %w", binDir, err)
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, name)
	if err := downloadVerified(ctx, assets[i], archive); err != nil {
		return err
	}
	extracted, err := extractBinaries(archive, tmp)
	if err != nil {
		return fmt.Errorf("unpack %s: %w", name, err)
	}
	running := filepath.Base(exe)
	if _, ok := extracted[running]; !ok {
		return fmt.Errorf("%s does not contain %s", name, running)
	}
	for _, bin := range releaseBinaries {
		src, ok := extracted[bin]
		if !ok {
			continue
		}
		dst := filepath.Join(binDir, bin)
		if _, err := os.Lstat(dst); bin != running && err != nil {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("replace %s: %w", dst, err)
		}
	}
	return nil
}

// extractBinaries writes the releaseBinaries found in a .tar.gz into dir, by
// base name only, and returns where each landed. Every other entry is ignored,
// so no path in the archive is ever written.
func extractBinaries(archive, dir string) (map[string]string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	out := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		base := path.Base(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || !slices.Contains(releaseBinaries, base) || out[base] != "" {
			continue
		}
		if hdr.Size > maxBinarySize {
			return nil, fmt.Errorf("%s is %d bytes, over the %d limit", hdr.Name, hdr.Size, maxBinarySize)
		}
		dst := filepath.Join(dir, base)
		w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(w, io.LimitReader(tr, maxBinarySize))
		if err == nil {
			err = w.Sync()
		}
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", base, err)
		}
		out[base] = dst
	}
	return out, nil
}

// newerRelease reports whether tag is a later release than current. Both are
// compared by their x.y.z; a current version with none, such as "dev", is
// always older.
func newerRelease(tag, current string) bool {
	want, ok := parseSemver(tag)
	if !ok {
		return false
	}
	have, ok := parseSemver(current)
	if !ok {
		return true
	}
	return slices.Compare(want, have) > 0
}

func parseSemver(s string) ([]int, bool) {
	m := semverRe.FindString(s)
	if m == "" {
		return nil, false
	}
	var v []int
	for _, p := range strings.Split(m, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		v = append(v, n)
	}
	return v, true
}
