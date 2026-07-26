package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

	// Prefer dpkg for accurate arch detection, fall back to GOARCH
	arch := runtime.GOARCH
	if out, err := runProbe(ctx, "dpkg", "--print-architecture"); err == nil {
		if a := strings.TrimSpace(string(out)); a != "" {
			arch = a
		}
	}

	url := fmt.Sprintf("https://github.com/dandavison/delta/releases/latest/download/git-delta_%s.deb", arch)
	debPath := filepath.Join(tmp, "git-delta.deb")

	if err := runCmd(ctx, "curl", "-fsSL", url, "-o", debPath); err != nil {
		// Fallback to package manager
		if err := installPkg(ctx, "git-delta"); err != nil {
			core.Warn("Could not install delta automatically. Install from https://github.com/dandavison/delta/releases")
		}
		return nil
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
