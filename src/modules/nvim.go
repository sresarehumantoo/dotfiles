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
	if treeSitterCurrent(ctx, "tree-sitter") || (dst != "" && treeSitterCurrent(ctx, dst)) {
		return nil
	}
	if core.IsMac() {
		return installPkg(ctx, "tree-sitter-cli")
	}
	if err := core.CheckTarget(dst); err != nil {
		return err
	}

	assets, err := latestAssets(ctx, "tree-sitter/tree-sitter")
	if err != nil {
		return err
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
	core.Ok("Installed tree-sitter CLI from %s", asset.Name)
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
	// No context here: this probe is shared with Status()/doctor, which have
	// none to inherit. ProbeTimeout still bounds it.
	out, err := runProbe(context.Background(), "nvim", "--version")
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
// is found, asks the user before removing it and installing the prebuilt.
func ensureNvim(ctx context.Context) error {
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
		// Always-visible breadcrumb so the user sees that we noticed the
		// outdated nvim, regardless of whether the prompt fires or skips.
		core.AlwaysWarn("Neovim %s at %s is older than required %d.%d (nvim-treesitter's main branch requires it)", shown, info.path, minNvimMajor, minNvimMinor)

		if !confirmNvimUpgrade(shown, info.path, owner) {
			core.AlwaysWarn("Skipping Neovim upgrade — treesitter highlighting needs %d.%d and will not load", minNvimMajor, minNvimMinor)
			return nil
		}
		if err := removeOldNvim(ctx, info.path, owner); err != nil {
			core.AlwaysWarn("Couldn't fully remove old Neovim: %v — continuing", err)
		}
	}

	if runtime.GOOS == "linux" {
		return installPrebuiltNvimLinux(ctx)
	}
	return installPkg(ctx, "neovim")
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
		core.AlwaysWarn("Can't prompt to upgrade Neovim — stdin isn't a terminal (running via MCP, piped input, or similar). Re-run 'dfinstall install nvim' interactively. Action would be: %s.",
			srcDesc)
		return false
	}

	title := fmt.Sprintf("Upgrade Neovim from %s?", version)
	desc := fmt.Sprintf("nvim-treesitter's main branch requires >= %d.%d.\nIf you confirm, %s, then the latest prebuilt is installed to /opt and symlinked into /usr/local/bin/nvim.",
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
func removeOldNvim(ctx context.Context, path, owner string) error {
	switch {
	case strings.HasPrefix(owner, "apt:"):
		pkg := strings.TrimPrefix(owner, "apt:")
		args := []string{"apt", "remove", "-y", pkg}
		if pkg != "neovim-runtime" && dpkgInstalled("neovim-runtime") {
			args = append(args, "neovim-runtime")
		}
		core.Info("Removing old Neovim apt packages: %v", args[3:])
		return runCmd(ctx, "sudo", args...)
	case owner == "opt-prebuilt":
		return nil
	default:
		core.Warn("Old Neovim at %s isn't apt-managed — leaving it. New /usr/local/bin/nvim will take precedence on PATH.", path)
		return nil
	}
}

func installPrebuiltNvimLinux(ctx context.Context) error {
	asset := nvimPrebuiltAsset()
	if asset == "" {
		core.Warn("no prebuilt Neovim for arch %s — install manually", runtime.GOARCH)
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

	// A private directory, so nothing else on the machine can replace the
	// tarball between the download and root extracting it.
	tmpDir, err := os.MkdirTemp("", "nvim-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmp := filepath.Join(tmpDir, asset)

	core.Info("Installing prebuilt Neovim (%s)...", asset)
	if err := downloadVerified(ctx, assets[i], tmp); err != nil {
		return fmt.Errorf("download nvim: %w", err)
	}

	// Wipe any prior extraction so a partial download doesn't leak in.
	_ = runCmd(ctx, "sudo", "rm", "-rf", filepath.Join("/opt", dir))
	// tar running as root keeps the archive's owners by default, which left
	// the binary root runs owned by uid 1001 (GitHub's build user), i.e.
	// whichever local account happens to hold that uid.
	if err := runCmd(ctx, "sudo", "tar", "--no-same-owner", "--no-same-permissions", "-xzf", tmp, "-C", "/opt"); err != nil {
		return fmt.Errorf("extract nvim: %w", err)
	}

	target := filepath.Join("/opt", dir, "bin", "nvim")
	if err := runCmd(ctx, "sudo", "ln", "-sfn", target, "/usr/local/bin/nvim"); err != nil {
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
			core.Notice("Existing nvim git repo found — backing up to %s", bakDir)
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
		core.AlwaysWarn("tree-sitter CLI: %v — treesitter parsers will not build", err)
	}

	// Sync plugins headlessly, then build the parsers init.lua lists in
	// vim.g.ts_parsers. nvim-treesitter's install is asynchronous, and a plain
	// `+qa` would quit before a single parser compiled.
	if !core.DryRun {
		if _, err := exec.LookPath("nvim"); err == nil {
			core.Info("Syncing Neovim plugins...")
			if err := runCmd(ctx, "nvim", "--headless", "+Lazy! sync", "+qa"); err != nil {
				core.Warn("Plugin sync failed — run :Lazy sync manually in nvim")
			}
			core.Info("Building treesitter parsers...")
			if err := runCmd(ctx, "nvim", "--headless",
				"+lua require('nvim-treesitter').install(vim.g.ts_parsers):wait(300000)", "+qa"); err != nil {
				core.Warn("Treesitter parser build failed — run :TSUpdate in nvim")
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
