package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type DeltaModule struct{}

func (DeltaModule) Name() string { return "delta" }

func (DeltaModule) Install(ctx context.Context) error {
	if _, err := exec.LookPath("delta"); err == nil {
		core.Ok("delta already installed")
		return nil
	}

	if core.DryRun {
		core.Info("would install delta")
		return nil
	}

	core.Info("Installing delta...")

	if core.AptBin() != "" {
		return installDeltaDeb(ctx)
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return installPkg(ctx, "git-delta")
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return installPkg(ctx, "git-delta")
	}
	if _, err := exec.LookPath("brew"); err == nil {
		return installPkg(ctx, "git-delta")
	}

	core.Warn("Install delta manually from https://github.com/dandavison/delta/releases")
	return nil
}

func installDeltaDeb(ctx context.Context) error {
	tmp, err := os.MkdirTemp("", "delta-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Resolve the asset from the release listing rather than constructing a URL.
	//
	// The old code built
	//   /releases/latest/download/git-delta_<arch>.deb
	// but delta embeds the version in the filename (git-delta_0.19.2_amd64.deb),
	// so that URL 404'd on EVERY run since the module was written. curl -f made
	// it exit 22, the fallback below swallowed it, and delta silently came from
	// the distro package instead — with `dfinstall status` still reporting
	// "installed", which is why it went unnoticed.
	//
	// Contains "git-delta_" is load-bearing: the same release also ships
	// git-delta-musl_<ver>_<arch>.deb, which satisfies both the .deb suffix and
	// the arch token and is listed FIRST, and pickAsset returns the first match.
	// "git-delta-musl_" does not contain "git-delta_", so this excludes it
	// without needing a new exclusion mechanism.
	assets, err := latestAssets(ctx, "dandavison/delta")
	if err != nil {
		return deltaPkgFallback(ctx)
	}
	asset, ok := pickAsset(assets, assetFilter{
		Contains:     "git-delta_",
		ArchTokens:   currentArchTokens(),
		Suffix:       ".deb",
		SkipSidecars: true,
		LinuxOnly:    true,
	})
	if !ok {
		core.Warn("no delta .deb published for %s", runtime.GOARCH)
		return deltaPkgFallback(ctx)
	}

	debPath := filepath.Join(tmp, "git-delta.deb")
	if err := runCmd(ctx, "curl", "-fsSL", asset.URL, "-o", debPath); err != nil {
		return deltaPkgFallback(ctx)
	}

	if err := runCmd(ctx, "sudo", "dpkg", "-i", debPath); err != nil {
		bin := core.AptBin()
		if bin == "" {
			core.Warn("dpkg failed and no apt binary available to fix dependencies")
		} else if fixErr := runCmd(ctx, "sudo", bin, "install", "-f", "-y"); fixErr != nil {
			core.Warn("%s install -f failed: %v", bin, fixErr)
		}
	}

	if _, err := exec.LookPath("delta"); err == nil {
		core.Ok("delta installed")
	}
	return nil
}

// deltaPkgFallback installs delta from the distro package manager. Used when the
// GitHub release cannot be reached or carries no asset for this architecture.
// Returns nil either way — a missing delta degrades git diffs, it does not
// justify failing the whole install run.
func deltaPkgFallback(ctx context.Context) error {
	if err := installPkg(ctx, "git-delta"); err != nil {
		core.Warn("Could not install delta automatically. Install from https://github.com/dandavison/delta/releases")
	}
	return nil
}

func (DeltaModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "delta"}
	if _, err := exec.LookPath("delta"); err == nil {
		s.Linked = 1
		s.Extra = "installed"
	} else {
		s.Missing = 1
		s.Extra = "not found"
	}
	return s
}
