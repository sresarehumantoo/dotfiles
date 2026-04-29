package modules

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// minNvim* is the lowest acceptable Neovim version. Bumped to 0.11 because
// telescope.nvim and other kickstart plugins now require it; Debian/Ubuntu
// apt repos still ship 0.7–0.10, so we install the official prebuilt tarball
// when the system package is too old.
const (
	minNvimMajor = 0
	minNvimMinor = 11
)

var nvimVersionRe = regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)

// nvimInfo describes the currently-installed Neovim, if any.
type nvimInfo struct {
	path    string // resolved real path of the nvim binary
	version string // e.g. "0.10.4"
	major   int
	minor   int
	patch   int
	parsed  bool // false when --version output couldn't be parsed
}

// detectNvim locates nvim on PATH and parses its version. Second return is
// false when nvim isn't installed at all.
func detectNvim() (nvimInfo, bool) {
	p, err := exec.LookPath("nvim")
	if err != nil {
		return nvimInfo{}, false
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		real = p
	}
	info := nvimInfo{path: real}
	out, err := exec.Command("nvim", "--version").Output()
	if err != nil {
		return info, true
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	m := nvimVersionRe.FindStringSubmatch(line)
	if len(m) >= 4 {
		info.major, _ = strconv.Atoi(m[1])
		info.minor, _ = strconv.Atoi(m[2])
		info.patch, _ = strconv.Atoi(m[3])
		info.version = fmt.Sprintf("%d.%d.%d", info.major, info.minor, info.patch)
		info.parsed = true
	}
	return info, true
}

func (i nvimInfo) atLeast(major, minor int) bool {
	if !i.parsed {
		return false
	}
	if i.major > major {
		return true
	}
	return i.major == major && i.minor >= minor
}

// nvimOwner classifies how the existing nvim was installed so we can remove
// it cleanly. Returns one of: "apt:<package>", "opt-prebuilt", "manual".
func nvimOwner(realPath string) string {
	if strings.HasPrefix(realPath, "/opt/nvim-linux-") {
		return "opt-prebuilt"
	}
	if out, err := exec.Command("dpkg", "-S", realPath).Output(); err == nil {
		if i := strings.IndexByte(string(out), ':'); i > 0 {
			return "apt:" + strings.TrimSpace(string(out)[:i])
		}
	}
	return "manual"
}

// nvimPrebuiltAsset returns the official Neovim release asset for the current
// Linux architecture, or "" if no prebuilt is published.
func nvimPrebuiltAsset() string {
	switch runtime.GOARCH {
	case "amd64":
		return "nvim-linux-x86_64.tar.gz"
	case "arm64":
		return "nvim-linux-arm64.tar.gz"
	}
	return ""
}

// ensureNvim makes sure a Neovim >= minNvim is available. If an older nvim
// is found, asks the user before removing it and installing the prebuilt.
func ensureNvim() error {
	info, present := detectNvim()
	if present && info.atLeast(minNvimMajor, minNvimMinor) {
		return nil
	}

	if present {
		owner := nvimOwner(info.path)
		shown := info.version
		if !info.parsed {
			shown = "unknown version"
		}
		if !confirmNvimUpgrade(shown, info.path, owner) {
			core.Notice("Keeping existing Neovim at %s — config plugins requiring %d.%d may fail", info.path, minNvimMajor, minNvimMinor)
			return nil
		}
		if err := removeOldNvim(info.path, owner); err != nil {
			core.Warn("Couldn't fully remove old Neovim: %v — continuing", err)
		}
	}

	if runtime.GOOS == "linux" {
		return installPrebuiltNvimLinux()
	}
	return installPkg("neovim")
}

// confirmNvimUpgrade asks the user whether to replace an outdated nvim.
// Returns false (skip) when stdin is not a terminal so unattended runs don't
// silently mutate a working-but-old install.
func confirmNvimUpgrade(version, path, owner string) bool {
	var srcDesc string
	switch {
	case strings.HasPrefix(owner, "apt:"):
		srcDesc = fmt.Sprintf("apt package '%s' will be removed via 'sudo apt remove'", strings.TrimPrefix(owner, "apt:"))
	case owner == "opt-prebuilt":
		srcDesc = "previous /opt prebuilt will be replaced"
	default:
		srcDesc = fmt.Sprintf("found at %s — won't be removed automatically (the new symlink at /usr/local/bin/nvim will shadow it on PATH)", path)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		core.Warn("Found Neovim %s at %s — older than %d.%d (telescope.nvim and other plugins require it). Stdin isn't a terminal so leaving it; re-run interactively to upgrade. Action would be: %s.",
			version, path, minNvimMajor, minNvimMinor, srcDesc)
		return false
	}

	title := fmt.Sprintf("Upgrade Neovim from %s?", version)
	desc := fmt.Sprintf("Telescope.nvim and other kickstart plugins require >= %d.%d.\nIf you confirm, %s, then the latest prebuilt is installed to /opt and symlinked into /usr/local/bin/nvim.",
		minNvimMajor, minNvimMinor, srcDesc)

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(desc).
				Affirmative("Yes, upgrade").
				Negative("No, keep current").
				Value(&confirm),
		),
	)
	core.PauseSpinner()
	defer core.ResumeSpinner()
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false
		}
		core.Warn("Confirm prompt failed: %v — skipping upgrade", err)
		return false
	}
	return confirm
}

// removeOldNvim cleanly removes the existing nvim based on how it was
// installed. opt-prebuilt is a no-op (installPrebuiltNvimLinux wipes the
// extraction dir before re-extracting). Manual installs are left alone with
// a warning.
func removeOldNvim(path, owner string) error {
	switch {
	case strings.HasPrefix(owner, "apt:"):
		pkg := strings.TrimPrefix(owner, "apt:")
		args := []string{"apt", "remove", "-y", pkg}
		if pkg != "neovim-runtime" && dpkgInstalled("neovim-runtime") {
			args = append(args, "neovim-runtime")
		}
		core.Info("Removing old Neovim apt packages: %v", args[3:])
		return runCmd("sudo", args...)
	case owner == "opt-prebuilt":
		return nil
	default:
		core.Warn("Old Neovim at %s isn't apt-managed — leaving it. New /usr/local/bin/nvim will take precedence on PATH.", path)
		return nil
	}
}

func installPrebuiltNvimLinux() error {
	asset := nvimPrebuiltAsset()
	if asset == "" {
		core.Warn("no prebuilt Neovim for arch %s — install manually", runtime.GOARCH)
		return nil
	}
	url := "https://github.com/neovim/neovim/releases/latest/download/" + asset
	dir := strings.TrimSuffix(asset, ".tar.gz") // e.g. nvim-linux-x86_64
	tmp := filepath.Join(os.TempDir(), asset)

	core.Info("Installing prebuilt Neovim (%s)...", asset)
	if err := runCmd("curl", "-fsSL", "-o", tmp, url); err != nil {
		return fmt.Errorf("download nvim: %w", err)
	}
	defer os.Remove(tmp)

	// Wipe any prior extraction so a partial download doesn't leak in.
	_ = runCmd("sudo", "rm", "-rf", filepath.Join("/opt", dir))
	if err := runCmd("sudo", "tar", "-xzf", tmp, "-C", "/opt"); err != nil {
		return fmt.Errorf("extract nvim: %w", err)
	}

	target := filepath.Join("/opt", dir, "bin", "nvim")
	if err := runCmd("sudo", "ln", "-sfn", target, "/usr/local/bin/nvim"); err != nil {
		return fmt.Errorf("symlink nvim: %w", err)
	}
	core.Ok("Installed prebuilt Neovim to /opt/%s", dir)
	return nil
}

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

	if core.DryRun {
		core.Info("would ensure Neovim >= %d.%d", minNvimMajor, minNvimMinor)
	} else if err := ensureNvim(); err != nil {
		core.Warn("Failed to ensure Neovim >= %d.%d: %v", minNvimMajor, minNvimMinor, err)
	}

	nvimDir := core.XDGTarget("nvim")

	// If existing nvim config is a git clone, back it up
	gitDir := filepath.Join(nvimDir, ".git")
	initLua := filepath.Join(nvimDir, "init.lua")
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		if li, err := os.Lstat(initLua); err == nil && li.Mode()&os.ModeSymlink == 0 {
			bakDir := nvimDir + ".bak"
			if _, err := os.Stat(bakDir); err == nil {
				core.Notice("Removing old nvim backup at %s", bakDir)
				os.RemoveAll(bakDir)
			}
			core.Notice("Existing nvim git repo found — backing up to %s", bakDir)
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
