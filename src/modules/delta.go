package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type DeltaModule struct{}

func (DeltaModule) Name() string { return "delta" }

func (DeltaModule) Install() error {
	if _, err := exec.LookPath("delta"); err == nil {
		core.Ok("delta already installed")
		return nil
	}

	if core.DryRun {
		core.Info("would install delta")
		return nil
	}

	core.Info("Installing delta...")

	if _, err := exec.LookPath("apt-get"); err == nil {
		return installDeltaDeb()
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return installPkg("git-delta")
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return installPkg("git-delta")
	}
	if _, err := exec.LookPath("brew"); err == nil {
		return installPkg("git-delta")
	}

	core.Warn("Install delta manually from https://github.com/dandavison/delta/releases")
	return nil
}

func installDeltaDeb() error {
	tmp, err := os.MkdirTemp("", "delta-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Prefer dpkg for accurate arch detection, fall back to GOARCH
	arch := runtime.GOARCH
	if out, err := exec.Command("dpkg", "--print-architecture").Output(); err == nil {
		arch = string(out[:len(out)-1]) // trim newline
	}

	url := fmt.Sprintf("https://github.com/dandavison/delta/releases/latest/download/git-delta_%s.deb", arch)
	debPath := filepath.Join(tmp, "git-delta.deb")

	cmd := exec.Command("curl", "-fsSL", url, "-o", debPath)
	if err := cmd.Run(); err != nil {
		// Fallback to package manager
		if err := installPkg("git-delta"); err != nil {
			core.Warn("Could not install delta automatically. Install from https://github.com/dandavison/delta/releases")
		}
		return nil
	}

	core.PauseSpinner()
	cmd = core.SudoCmd("dpkg", "-i", debPath)
	if core.Level >= core.LogVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		fixCmd := core.SudoCmd("apt-get", "install", "-f", "-y")
		if core.Level >= core.LogVerbose {
			fixCmd.Stdout = os.Stdout
			fixCmd.Stderr = os.Stderr
		}
		if fixErr := fixCmd.Run(); fixErr != nil {
			core.Warn("apt-get install -f failed: %v", fixErr)
		}
	}
	core.ResumeSpinner()

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
