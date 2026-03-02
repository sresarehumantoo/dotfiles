package modules

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type FontsModule struct{}

func (FontsModule) Name() string { return "fonts" }

func (FontsModule) Install() error {
	core.Info("Installing fonts...")

	fontDir := core.HomeTarget(".local", "share", "fonts")
	bundledDir := core.ConfigPath("fonts")
	needCache := false

	if err := core.EnsureDir(fontDir); err != nil {
		return err
	}

	// Install from bundled fonts/ directory
	entries, err := os.ReadDir(bundledDir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			ext := filepath.Ext(name)
			if ext != ".ttf" && ext != ".otf" {
				continue
			}
			src := filepath.Join(bundledDir, name)
			dst := filepath.Join(fontDir, name)
			if _, err := os.Stat(dst); err == nil {
				if core.FilesMatch(src, dst) {
					core.Ok("font already installed: %s", name)
					continue
				}
				core.Info("updating font: %s", name)
			}
			data, err := os.ReadFile(src)
			if err != nil {
				core.Warn("could not read font %s: %v", name, err)
				continue
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				core.Warn("could not install font %s: %v", name, err)
				continue
			}
			core.Ok("installed font: %s", name)
			needCache = true
		}
	}

	// Fallback: download Hack Nerd Font if still missing
	hackFont := filepath.Join(fontDir, "HackNerdFont-Regular.ttf")
	if _, err := os.Stat(hackFont); os.IsNotExist(err) {
		core.Info("Downloading Hack Nerd Font...")
		tmp, err := os.MkdirTemp("", "fonts-*")
		if err == nil {
			defer os.RemoveAll(tmp)
			zipPath := filepath.Join(tmp, "Hack.zip")
			url := "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/Hack.zip"
			cmd := exec.Command("curl", "-fsSL", url, "-o", zipPath)
			if err := cmd.Run(); err == nil {
				cmd = exec.Command("unzip", "-qo", zipPath, "-d", fontDir)
				if err := cmd.Run(); err == nil {
					needCache = true
					core.Ok("Hack Nerd Font downloaded")
				}
			} else {
				core.Warn("Could not download Hack Nerd Font. Install manually from https://github.com/ryanoasis/nerd-fonts/releases")
			}
		}
	}

	if needCache {
		if _, err := exec.LookPath("fc-cache"); err == nil {
			exec.Command("fc-cache", "-f", fontDir).Run()
		}
	}

	core.Ok("Fonts done")
	return nil
}

func (FontsModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "fonts"}
	fontDir := core.HomeTarget(".local", "share", "fonts")
	bundledDir := core.ConfigPath("fonts")
	fonts := []string{"HackNerdFont-Regular.ttf", "MesloLGS NF Regular.ttf"}
	for _, f := range fonts {
		dst := filepath.Join(fontDir, f)
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			s.Missing++
			continue
		}
		// If bundled source exists, verify content matches
		src := filepath.Join(bundledDir, f)
		if _, err := os.Stat(src); err == nil && !core.FilesMatch(src, dst) {
			s.Missing++
			continue
		}
		s.Linked++
	}
	return s
}
