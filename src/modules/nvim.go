package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// minNvim* is the lowest acceptable Neovim version: nvim-treesitter's main
// branch requires 0.12. Debian/Ubuntu apt repos ship far older, so we install
// the official prebuilt tarball when the system package is too old.
const (
	minNvimMajor = 0
	minNvimMinor = 12
)

// minTreeSitter* is the lowest tree-sitter CLI nvim-treesitter's main branch
// accepts; it shells out to the CLI to build every parser. Distro packages lag
// well behind (trixie 0.22, noble 0.20).
const (
	minTreeSitterMajor = 0
	minTreeSitterMinor = 26
)

var treeSitterVersionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// treeSitterCurrent reports whether bin (a name to look up on PATH, or a path)
// is a tree-sitter CLI new enough for nvim-treesitter.
func treeSitterCurrent(ctx context.Context, bin string) bool {
	out, err := runProbe(ctx, bin, "--version")
	if err != nil {
		return false
	}
	m := treeSitterVersionRe.FindSubmatch(out)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(string(m[1]))
	minor, _ := strconv.Atoi(string(m[2]))
	return major > minTreeSitterMajor || (major == minTreeSitterMajor && minor >= minTreeSitterMinor)
}

// treeSitterFilter selects the bare gzipped Linux binary, not the
// tree-sitter-cli-*.zip bundle beside it.
func treeSitterFilter(goarch string) assetFilter {
	return assetFilter{
		Contains:   "tree-sitter-linux-",
		ArchTokens: archTokensFor(goarch),
		Suffix:     ".gz",
		LinuxOnly:  true,
	}
}

// ensureTreeSitterCLI installs a current tree-sitter CLI: brew on macOS, the
// release binary into ~/.local/bin on Linux. It is one static binary, so it
// needs no root; ~/.local/bin is ahead of /usr/local/bin on PATH, so it also
// shadows a copy an older dfinstall put there.
func ensureTreeSitterCLI(ctx context.Context) error {
	// Checked by path too: on a first run ~/.local/bin may not be on PATH yet.
	dst := core.HomeTarget(".local", "bin", "tree-sitter")
	upgrade := !core.IsMac() && upgradeable(dst)
	if !upgrade && (treeSitterCurrent(ctx, "tree-sitter") || (dst != "" && treeSitterCurrent(ctx, dst))) {
		return nil
	}
	if core.IsMac() {
		return installPkg(ctx, "tree-sitter-cli")
	}
	if err := core.CheckTarget(dst); err != nil {
		return err
	}

	tag, assets, err := latestRelease(ctx, "tree-sitter/tree-sitter")
	if err != nil {
		if upgrade {
			core.Warn("tree-sitter upgrade check: %v", err)
			return nil
		}
		return err
	}
	if upgrade && releaseCurrent(ctx, dst, tag) {
		core.Ok("tree-sitter %s is the latest release", tag)
		return nil
	}
	asset, ok := pickAsset(assets, treeSitterFilter(runtime.GOARCH))
	if !ok {
		return fmt.Errorf("no tree-sitter release binary for %s", runtime.GOARCH)
	}

	tmp, err := os.MkdirTemp("", "tree-sitter-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	gz := filepath.Join(tmp, "tree-sitter.gz")
	if err := downloadVerified(ctx, asset, gz); err != nil {
		return fmt.Errorf("download tree-sitter: %w", err)
	}
	if err := runCmd(ctx, "gunzip", gz); err != nil {
		return fmt.Errorf("decompress tree-sitter: %w", err)
	}
	if err := runCmd(ctx, "install", "-D", "-m", "0755", filepath.Join(tmp, "tree-sitter"), dst); err != nil {
		return fmt.Errorf("install tree-sitter: %w", err)
	}
	core.Changed("Installed tree-sitter CLI from %s", asset.Name)
	return nil
}

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

// localNvimLink is where installPrebuiltNvimLinux links the nvim it manages.
func localNvimLink() string { return core.HomeTarget(".local", "bin", "nvim") }

// nvimBin returns the nvim to run: the one dfinstall manages in ~/.local/bin
// when present, else whatever PATH finds. Checked by path first because on a
// first run ~/.local/bin is often not on PATH yet, and PATH would then find an
// older distro nvim.
func nvimBin() (string, error) {
	if p := localNvimLink(); p != "" {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return exec.LookPath("nvim")
}

// detectNvim locates nvim and parses its version. Second return is false when
// nvim isn't installed at all.
func detectNvim() (nvimInfo, bool) {
	p, err := nvimBin()
	if err != nil {
		return nvimInfo{}, false
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		real = p
	}
	info := nvimInfo{path: real}
	// No context here: this probe is shared with Status()/doctor, which have
	// none to inherit. ProbeTimeout still bounds it.
	out, err := runProbe(context.Background(), p, "--version")
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

// nvimOwner classifies how the existing nvim was installed. Returns one of:
// "local-prebuilt" (ours, in ~/.local/opt), "opt-prebuilt" (an older
// dfinstall's, in /opt), "apt:<package>", "manual".
func nvimOwner(realPath string) string {
	if opt := core.HomeTarget(".local", "opt"); opt != "" && strings.HasPrefix(realPath, opt+"/nvim-linux-") {
		return "local-prebuilt"
	}
	if strings.HasPrefix(realPath, "/opt/nvim-linux-") {
		return "opt-prebuilt"
	}
	if out, err := runProbe(context.Background(), "dpkg", "-S", realPath); err == nil {
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
// is found, asks the user before installing the prebuilt in front of it.
func ensureNvim(ctx context.Context) error {
	info, present := detectNvim()
	if present && info.atLeast(minNvimMajor, minNvimMinor) {
		if core.UpgradeMode && runtime.GOOS == "linux" && nvimOwner(info.path) == "local-prebuilt" {
			return upgradePrebuiltNvim(ctx, info.version)
		}
		return nil
	}

	if present {
		owner := nvimOwner(info.path)
		shown := info.version
		if !info.parsed {
			shown = "unknown version"
		}
		// Always-visible breadcrumb so the user sees that we noticed the
		// outdated nvim, regardless of whether the prompt fires or skips.
		core.AlwaysWarn("Neovim %s at %s is older than required %d.%d (nvim-treesitter's main branch requires it)", shown, info.path, minNvimMajor, minNvimMinor)

		if !confirmNvimUpgrade(shown, info.path, owner) {
			core.AlwaysWarn("Skipping Neovim upgrade; treesitter highlighting needs %d.%d and will not load", minNvimMajor, minNvimMinor)
			return nil
		}
		if runtime.GOOS == "linux" {
			if err := installPrebuiltNvimLinux(ctx); err != nil {
				return err
			}
			if hint := oldNvimRemoval(owner); hint != "" {
				core.Notice("The old Neovim at %s is still installed, shadowed on PATH. To remove it: %s", info.path, hint)
			}
			return nil
		}
	}

	if runtime.GOOS == "linux" {
		return installPrebuiltNvimLinux(ctx)
	}
	return installPkg(ctx, "neovim")
}

// upgradePrebuiltNvim replaces the prebuilt in ~/.local/opt when the latest
// release is a different version. Only the copy dfinstall manages is touched.
func upgradePrebuiltNvim(ctx context.Context, have string) error {
	tag, _, err := latestRelease(ctx, "neovim/neovim")
	if err != nil {
		core.Warn("Neovim upgrade check: %v", err)
		return nil
	}
	if semverRe.FindString(tag) == have {
		core.Ok("Neovim %s is the latest release", tag)
		return nil
	}
	core.Info("Updating Neovim %s to %s...", have, tag)
	return installPrebuiltNvimLinux(ctx)
}

// oldNvimRemoval is the command that removes a superseded nvim, or "" when
// there is nothing to remove (ours is replaced in place) or no safe command.
// dfinstall does not run it: the new nvim already wins on PATH, and removing
// a system copy is the only part that would need root.
func oldNvimRemoval(owner string) string {
	switch {
	case strings.HasPrefix(owner, "apt:"):
		return "sudo apt remove " + strings.TrimPrefix(owner, "apt:")
	case owner == "opt-prebuilt":
		return "sudo rm -rf /opt/nvim-linux-* /usr/local/bin/nvim"
	}
	return ""
}

// confirmNvimUpgrade asks the user whether to replace an outdated nvim.
// Returns false (skip) when stdin is not a terminal so unattended runs don't
// silently mutate a working-but-old install.
func confirmNvimUpgrade(version, path, owner string) bool {
	var srcDesc string
	if owner == "local-prebuilt" {
		srcDesc = "the previous prebuilt in ~/.local/opt will be replaced"
	} else {
		srcDesc = fmt.Sprintf("the current one at %s is left installed and shadowed on PATH by ~/.local/bin/nvim", path)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		core.AlwaysWarn("Can't prompt to upgrade Neovim; stdin isn't a terminal (running via MCP, piped input, or similar). Re-run 'dfinstall install nvim' interactively. Action would be: %s.",
			srcDesc)
		return false
	}

	title := fmt.Sprintf("Upgrade Neovim from %s?", version)
	desc := fmt.Sprintf("nvim-treesitter's main branch requires >= %d.%d.\nIf you confirm, %s, the latest prebuilt is installed to ~/.local/opt and linked as ~/.local/bin/nvim.",
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
		core.Warn("Confirm prompt failed: %v, skipping upgrade", err)
		return false
	}
	return confirm
}

// installPrebuiltNvimLinux installs the official prebuilt into ~/.local/opt and
// links it as ~/.local/bin/nvim. It is a self-contained tree, so it needs no
// root.
func installPrebuiltNvimLinux(ctx context.Context) error {
	asset := nvimPrebuiltAsset()
	if asset == "" {
		core.Warn("no prebuilt Neovim for arch %s; install manually", runtime.GOARCH)
		return nil
	}
	dir := strings.TrimSuffix(asset, ".tar.gz") // e.g. nvim-linux-x86_64

	assets, err := latestAssets(ctx, "neovim/neovim")
	if err != nil {
		return err
	}
	i := slices.IndexFunc(assets, func(a ghAsset) bool { return a.Name == asset })
	if i < 0 {
		return fmt.Errorf("latest Neovim release has no %s", asset)
	}

	optDir := core.HomeTarget(".local", "opt")
	link := localNvimLink()
	if err := core.CheckTarget(optDir); err != nil {
		return err
	}
	if fi, err := os.Lstat(link); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a symlink dfinstall made; move it aside to let dfinstall manage Neovim", link)
	}
	if err := os.MkdirAll(optDir, 0o755); err != nil {
		return err
	}

	// Staged inside optDir, so the final step is a rename on one filesystem
	// and a failed download or extraction never leaves a half-replaced nvim.
	stage, err := os.MkdirTemp(optDir, ".nvim-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	tarball := filepath.Join(stage, asset)

	core.Info("Installing prebuilt Neovim (%s)...", asset)
	if err := downloadVerified(ctx, assets[i], tarball); err != nil {
		return fmt.Errorf("download nvim: %w", err)
	}
	if err := runCmd(ctx, "tar", "-xzf", tarball, "-C", stage); err != nil {
		return fmt.Errorf("extract nvim: %w", err)
	}
	if _, err := os.Stat(filepath.Join(stage, dir, "bin", "nvim")); err != nil {
		return fmt.Errorf("extract nvim: %s has no %s/bin/nvim", asset, dir)
	}

	final := filepath.Join(optDir, dir)
	if err := core.RemoveManagedDir(final); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(stage, dir), final); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	_ = os.Remove(link)
	if err := os.Symlink(filepath.Join(final, "bin", "nvim"), link); err != nil {
		return fmt.Errorf("link nvim: %w", err)
	}
	core.Changed("Installed prebuilt Neovim to %s", final)
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
	{"nvim/notes.markdownlint.yaml", "notes.markdownlint.yaml"},

	// Custom lua
	{"nvim/lua/custom/keybinds.lua", "lua/custom/keybinds.lua"},
	{"nvim/lua/custom/plugins/init.lua", "lua/custom/plugins/init.lua"},
	{"nvim/lua/custom/plugins/colorizer.lua", "lua/custom/plugins/colorizer.lua"},
	{"nvim/lua/custom/plugins/comment.lua", "lua/custom/plugins/comment.lua"},
	{"nvim/lua/custom/plugins/harpoon.lua", "lua/custom/plugins/harpoon.lua"},
	{"nvim/lua/custom/plugins/undotree.lua", "lua/custom/plugins/undotree.lua"},
	{"nvim/lua/custom/plugins/oil.lua", "lua/custom/plugins/oil.lua"},
	{"nvim/lua/custom/plugins/flash.lua", "lua/custom/plugins/flash.lua"},
	{"nvim/lua/custom/plugins/markdown.lua", "lua/custom/plugins/markdown.lua"},
	{"nvim/lua/custom/plugins/notes.lua", "lua/custom/plugins/notes.lua"},
	{"nvim/lua/custom/plugins/smear-cursor.lua", "lua/custom/plugins/smear-cursor.lua"},
	{"nvim/lua/custom/plugins/diffview.lua", "lua/custom/plugins/diffview.lua"},

	// Filetype-specific editor options (native after/ftplugin)
	{"nvim/after/ftplugin/markdown.lua", "after/ftplugin/markdown.lua"},

	// Kickstart lua
	{"nvim/lua/kickstart/health.lua", "lua/kickstart/health.lua"},
	{"nvim/lua/kickstart/plugins/autopairs.lua", "lua/kickstart/plugins/autopairs.lua"},
	{"nvim/lua/kickstart/plugins/debug.lua", "lua/kickstart/plugins/debug.lua"},
	{"nvim/lua/kickstart/plugins/gitsigns.lua", "lua/kickstart/plugins/gitsigns.lua"},
	{"nvim/lua/kickstart/plugins/indent_line.lua", "lua/kickstart/plugins/indent_line.lua"},
	{"nvim/lua/kickstart/plugins/lint.lua", "lua/kickstart/plugins/lint.lua"},
	{"nvim/lua/kickstart/plugins/neo-tree.lua", "lua/kickstart/plugins/neo-tree.lua"},
}

func (NvimModule) Install(ctx context.Context) error {
	core.Info("Setting up Neovim config...")

	if core.DryRun {
		core.Info("would ensure Neovim >= %d.%d", minNvimMajor, minNvimMinor)
		if core.UpgradeMode {
			core.Info("would update Neovim and tree-sitter in ~/.local if newer releases exist")
		}
	} else if err := ensureNvim(ctx); err != nil {
		core.AlwaysWarn("Failed to ensure Neovim >= %d.%d: %v", minNvimMajor, minNvimMinor, err)
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
			core.Notice("Existing nvim git repo found, backing up to %s", bakDir)
			if err := os.Rename(nvimDir, bakDir); err != nil {
				core.Warn("Failed to back up nvim config: %v", err)
			}
		}
	}

	// Ensure directories
	dirs := []string{
		core.SubPath(nvimDir, "lua", "custom", "plugins"),
		core.SubPath(nvimDir, "lua", "kickstart", "plugins"),
	}
	for _, d := range dirs {
		if err := core.EnsureDir(d); err != nil {
			return err
		}
	}

	// Create all symlinks
	if err := (NvimModule{}).Links().Apply(); err != nil {
		return err
	}

	if core.DryRun {
		core.Info("would ensure tree-sitter CLI >= %d.%d", minTreeSitterMajor, minTreeSitterMinor)
	} else if err := ensureTreeSitterCLI(ctx); err != nil {
		core.AlwaysWarn("tree-sitter CLI: %v; treesitter parsers will not build", err)
	}

	// Sync plugins headlessly, then build the parsers init.lua lists in
	// vim.g.ts_parsers. nvim-treesitter's install is asynchronous, and a plain
	// `+qa` would quit before a single parser compiled.
	if !core.DryRun {
		if nvim, err := nvimBin(); err == nil {
			// install only adds missing plugins; sync also updates every one,
			// which is --upgrade's job, not a plain install's.
			lazyCmd := "install"
			if core.UpgradeMode {
				lazyCmd = "sync"
			}
			core.Info("Running Lazy %s...", lazyCmd)
			if err := runCmd(ctx, nvim, "--headless", "+Lazy! "+lazyCmd, "+qa"); err != nil {
				core.Warn("Plugin %s failed; run :Lazy %s manually in nvim", lazyCmd, lazyCmd)
			}
			core.Info("Building treesitter parsers...")
			if err := runCmd(ctx, nvim, "--headless",
				"+lua require('nvim-treesitter').install(vim.g.ts_parsers):wait(300000)", "+qa"); err != nil {
				core.Warn("Treesitter parser build failed; run :TSUpdate in nvim")
			}
		}
	}

	core.Ok("Neovim config done")
	return nil
}

func (m NvimModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("Neovim config uninstalled")
	return nil
}

func (NvimModule) Links() core.LinkSet {
	nvimDir := core.XDGTarget("nvim")
	ls := make(core.LinkSet, len(nvimLinks))
	for i, l := range nvimLinks {
		ls[i] = core.LinkPair{
			Src: core.ConfigPath(l.Src),
			Dst: filepath.Join(nvimDir, l.Dst),
		}
	}
	return ls
}

func (m NvimModule) Status() core.ModuleStatus {
	return m.Links().Status("nvim")
}
