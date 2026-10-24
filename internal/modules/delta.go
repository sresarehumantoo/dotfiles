package modules

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

type DeltaModule struct{}

func (DeltaModule) Name() string { return "delta" }

func (DeltaModule) Install(ctx context.Context) error {
	upgrade := runtime.GOOS == "linux" && upgradeable(deltaLocal())
	if localBinInstalled("delta") && !upgrade {
		core.Ok("delta already installed")
		return nil
	}

	if core.DryRun {
		if upgrade {
			core.Info("would update delta if a newer release exists")
		} else {
			core.Info("would install delta")
		}
		return nil
	}

	core.Info("Installing delta...")

	if core.IsMac() {
		return installPkg(ctx, "git-delta")
	}
	if runtime.GOOS == "linux" {
		return installDeltaRelease(ctx)
	}

	core.Warn("Install delta manually from https://github.com/dandavison/delta/releases")
	return nil
}

// deltaLocal is where installDeltaRelease puts the binary.
func deltaLocal() string { return core.HomeTarget(".local", "bin", "delta") }

// deltaReleaseAsset picks the release tarball for goarch: the static musl
// build where one is published (x86_64 only, today), so it runs on any libc,
// else the glibc build.
func deltaReleaseAsset(assets []ghAsset, goarch string) (ghAsset, bool) {
	for _, target := range []string{"unknown-linux-musl", "unknown-linux-gnu"} {
		if a, ok := pickAsset(assets, assetFilter{
			Contains:   target,
			ArchTokens: archTokensFor(goarch),
			Suffix:     ".tar.gz",
		}); ok {
			return a, true
		}
	}
	return ghAsset{}, false
}

// installDeltaRelease installs the release binary into ~/.local/bin. delta is
// a single binary, so it needs neither root nor dpkg (whose maintainer scripts
// would run as root). Falls back to the package manager only when the release
// cannot be used.
func installDeltaRelease(ctx context.Context) error {
	dst := deltaLocal()
	if err := core.CheckTarget(dst); err != nil {
		return err
	}
	tag, assets, err := latestRelease(ctx, "dandavison/delta")
	if err != nil {
		core.Warn("%v", err)
		return deltaPkgFallback(ctx)
	}
	if releaseCurrent(ctx, dst, tag) {
		core.Ok("delta %s is the latest release", tag)
		return nil
	}
	asset, ok := deltaReleaseAsset(assets, runtime.GOARCH)
	if !ok {
		core.Warn("no delta release binary published for %s", runtime.GOARCH)
		return deltaPkgFallback(ctx)
	}

	tmp, err := os.MkdirTemp("", "delta-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	tarball := filepath.Join(tmp, asset.Name)
	if err := downloadVerified(ctx, asset, tarball); err != nil {
		core.Warn("%v", err)
		return deltaPkgFallback(ctx)
	}
	if err := runCmd(ctx, "tar", "-xzf", tarball, "-C", tmp); err != nil {
		return err
	}
	bin := filepath.Join(tmp, strings.TrimSuffix(asset.Name, ".tar.gz"), "delta")
	if err := runCmd(ctx, "install", "-D", "-m", "0755", bin, dst); err != nil {
		return err
	}
	core.Changed("delta installed to %s", dst)
	return nil
}

// deltaPkgFallback installs delta from the distro package manager. Used when the
// GitHub release cannot be reached or carries no asset for this architecture.
// Returns nil either way — a missing delta degrades git diffs, it does not
// justify failing the whole install run.
func deltaPkgFallback(ctx context.Context) error {
	// An upgrade that cannot use the release keeps the delta it already has.
	if upgradeable(deltaLocal()) {
		return nil
	}
	if err := installPkg(ctx, "git-delta"); err != nil {
		core.Warn("Could not install delta automatically. Install from https://github.com/dandavison/delta/releases")
	}
	return nil
}

func (DeltaModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "delta"}
	if localBinInstalled("delta") {
		s.Linked = 1
		s.Extra = "installed"
	} else {
		s.Missing = 1
		s.Extra = "not found"
	}
	return s
}
