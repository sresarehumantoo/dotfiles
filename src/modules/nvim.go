package modules

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type NvimModule struct{}

func (NvimModule) Name() string { return "nvim" }

type nvimLink struct {
	Src string
	Dst string
}

var nvimLinks = []nvimLink{
	// Root files
	{"nvim/init.lua", "init.lua"},
	{"nvim/lazy-lock.json", "lazy-lock.json"},
	{"nvim/.stylua.toml", ".stylua.toml"},

	// Custom lua
	{"nvim/lua/custom/keybinds.lua", "lua/custom/keybinds.lua"},
	{"nvim/lua/custom/plugins/init.lua", "lua/custom/plugins/init.lua"},
	{"nvim/lua/custom/plugins/colorizer.lua", "lua/custom/plugins/colorizer.lua"},
	{"nvim/lua/custom/plugins/comment.lua", "lua/custom/plugins/comment.lua"},
	{"nvim/lua/custom/plugins/harpoon.lua", "lua/custom/plugins/harpoon.lua"},
	{"nvim/lua/custom/plugins/undotree.lua", "lua/custom/plugins/undotree.lua"},
	{"nvim/lua/custom/plugins/oil.lua", "lua/custom/plugins/oil.lua"},
	{"nvim/lua/custom/plugins/flash.lua", "lua/custom/plugins/flash.lua"},

	// Kickstart lua
	{"nvim/lua/kickstart/health.lua", "lua/kickstart/health.lua"},
	{"nvim/lua/kickstart/plugins/autopairs.lua", "lua/kickstart/plugins/autopairs.lua"},
	{"nvim/lua/kickstart/plugins/debug.lua", "lua/kickstart/plugins/debug.lua"},
	{"nvim/lua/kickstart/plugins/gitsigns.lua", "lua/kickstart/plugins/gitsigns.lua"},
	{"nvim/lua/kickstart/plugins/indent_line.lua", "lua/kickstart/plugins/indent_line.lua"},
	{"nvim/lua/kickstart/plugins/lint.lua", "lua/kickstart/plugins/lint.lua"},
	{"nvim/lua/kickstart/plugins/neo-tree.lua", "lua/kickstart/plugins/neo-tree.lua"},
}

func (NvimModule) Install() error {
	core.Info("Setting up Neovim config...")

	nvimDir := core.XDGTarget("nvim")

	// If existing nvim config is a git clone, back it up
	gitDir := filepath.Join(nvimDir, ".git")
	initLua := filepath.Join(nvimDir, "init.lua")
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		if li, err := os.Lstat(initLua); err == nil && li.Mode()&os.ModeSymlink == 0 {
			bakDir := nvimDir + ".bak"
			if _, err := os.Stat(bakDir); err == nil {
				core.Warn("Removing old nvim backup at %s", bakDir)
				os.RemoveAll(bakDir)
			}
			core.Warn("Existing nvim git repo found — backing up to %s", bakDir)
			if err := os.Rename(nvimDir, bakDir); err != nil {
				core.Warn("Failed to back up nvim config: %v", err)
			}
		}
	}

	// Ensure directories
	dirs := []string{
		filepath.Join(nvimDir, "lua", "custom", "plugins"),
		filepath.Join(nvimDir, "lua", "kickstart", "plugins"),
	}
	for _, d := range dirs {
		if err := core.EnsureDir(d); err != nil {
			return err
		}
	}

	// Create all symlinks
	for _, l := range nvimLinks {
		src := core.ConfigPath(l.Src)
		dst := filepath.Join(nvimDir, l.Dst)
		if err := core.LinkFile(src, dst); err != nil {
			return err
		}
	}

	// Sync plugins headlessly
	if !core.DryRun {
		if _, err := exec.LookPath("nvim"); err == nil {
			core.Info("Syncing Neovim plugins...")
			cmd := exec.Command("nvim", "--headless", "+Lazy! sync", "+qa")
			if core.Level >= core.LogVerbose {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}
			if err := cmd.Run(); err != nil {
				core.Warn("Plugin sync failed — run :Lazy sync manually in nvim")
			}
		}
	}

	core.Ok("Neovim config done")
	return nil
}

func (NvimModule) Uninstall() error {
	nvimDir := core.XDGTarget("nvim")
	for _, l := range nvimLinks {
		src := core.ConfigPath(l.Src)
		dst := filepath.Join(nvimDir, l.Dst)
		if err := core.UnlinkFile(src, dst); err != nil {
			return err
		}
	}
	core.Ok("Neovim config uninstalled")
	return nil
}

func (NvimModule) Links() []core.LinkPair {
	nvimDir := core.XDGTarget("nvim")
	pairs := make([]core.LinkPair, len(nvimLinks))
	for i, l := range nvimLinks {
		pairs[i] = core.LinkPair{
			Src: core.ConfigPath(l.Src),
			Dst: filepath.Join(nvimDir, l.Dst),
		}
	}
	return pairs
}

func (NvimModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "nvim"}
	nvimDir := core.XDGTarget("nvim")
	for _, l := range nvimLinks {
		src := core.ConfigPath(l.Src)
		dst := filepath.Join(nvimDir, l.Dst)
		if core.CheckLink(src, dst) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}
