package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type ToolkitModule struct{}

func (ToolkitModule) Name() string { return "toolkit" }

func (ToolkitModule) Install() error {
	if core.DryRun {
		tools := core.Cfg.ToolkitTools
		if len(tools) == 0 {
			core.Info("would install toolkit tools (none configured — run with --toolkit to select)")
			return nil
		}
		core.Info("would install toolkit tools: %s", strings.Join(tools, ", "))
		return nil
	}

	tools := core.Cfg.ToolkitTools
	if len(tools) == 0 {
		core.Info("No toolkit tools configured — run with --toolkit to select")
		return nil
	}

	// Load registry — force fetch only when --toolkit menu was shown
	reg, err := core.LoadOrFetchRegistry(core.ToolkitMode)
	if err != nil {
		// Try cache as fallback
		reg, err = core.LoadCachedRegistry()
		if err != nil {
			core.Warn("Toolkit registry not available — run with --toolkit to fetch")
			return nil
		}
	}

	// Build lookup map
	lookup := make(map[string]core.RegistryTool, len(reg.Tools))
	for _, t := range reg.Tools {
		lookup[t.Name] = t
	}

	// Group by install method
	var aptPkgs []string
	var goTools []core.RegistryTool
	var pipxTools []core.RegistryTool
	var appImageTools []core.RegistryTool
	var cargoTools []core.RegistryTool
	var gitCloneTools []core.RegistryTool
	var debTools []core.RegistryTool
	var releaseBinaryTools []core.RegistryTool

	for _, name := range tools {
		info, ok := lookup[name]
		if !ok {
			core.Warn("Unknown toolkit tool %q — skipping", name)
			continue
		}
		if !core.ToolMatchesDistro(info) {
			core.Debug("skipping %s — not available on this distro", name)
			continue
		}
		// Skip already-installed tools at gather so we don't re-send them
		// to apt's bulk install (idempotent but noisy in logs and spinner)
		// and don't redundantly hit pipx/cargo/go/git/curl per skipped tool.
		// The pre-install summary already reported these as "Tools installed".
		if isToolInstalled(info) {
			core.Debug("skipping %s — already installed", name)
			continue
		}
		switch info.Method {
		case "apt":
			aptPkgs = append(aptPkgs, info.Package)
		case "go":
			goTools = append(goTools, info)
		case "pipx":
			pipxTools = append(pipxTools, info)
		case "appimage":
			appImageTools = append(appImageTools, info)
		case "cargo":
			cargoTools = append(cargoTools, info)
		case "git_clone":
			gitCloneTools = append(gitCloneTools, info)
		case "deb":
			debTools = append(debTools, info)
		case "release_binary":
			releaseBinaryTools = append(releaseBinaryTools, info)
		}
	}

	// Install apt packages in bulk
	if len(aptPkgs) > 0 {
		core.Info("Installing apt packages: %s", strings.Join(aptPkgs, ", "))
		if err := installPkg(aptPkgs...); err != nil {
			core.Warn("Some apt packages may have failed: %v", err)
		}
		core.Ok("apt packages done")
	}

	// Install go tools
	if len(goTools) > 0 && ensureToolchain("go", "golang", len(goTools), "go tools") {
		for _, t := range goTools {
			if _, err := exec.LookPath(t.Binary); err == nil {
				core.Ok("%s already installed", t.Binary)
				continue
			}
			core.Info("Installing %s via go install...", t.Binary)
			if err := runCmd("go", "install", t.Package); err != nil {
				core.Warn("Failed to install %s: %v", t.Binary, err)
			} else {
				core.Ok("%s installed", t.Binary)
			}
		}
	}

	// Install cargo tools
	if len(cargoTools) > 0 && ensureToolchain("cargo", "cargo", len(cargoTools), "cargo tools") {
		for _, t := range cargoTools {
			if _, err := exec.LookPath(t.Binary); err == nil {
				core.Ok("%s already installed", t.Binary)
				continue
			}
			core.Info("Installing %s via cargo install...", t.Binary)
			if err := runCmd("cargo", "install", t.Package); err != nil {
				core.Warn("Failed to install %s: %v", t.Binary, err)
			} else {
				core.Ok("%s installed", t.Binary)
			}
		}
	}

	// Install pipx tools
	if len(pipxTools) > 0 && ensureToolchain("pipx", "pipx", len(pipxTools), "pipx tools") {
		for _, t := range pipxTools {
			if pipxHasPkg(t.Package) {
				core.Ok("%s already installed via pipx", t.Package)
				continue
			}
			core.Info("Installing %s via pipx...", t.Package)
			if err := runCmd("pipx", "install", t.Package); err != nil {
				core.Warn("Failed to install %s: %v", t.Package, err)
			} else {
				core.Ok("%s installed", t.Package)
			}
		}
	}

	// Install git clone tools
	if len(gitCloneTools) > 0 {
		if _, err := exec.LookPath("git"); err != nil {
			core.Warn("git not found — skipping %d git-clone tools", len(gitCloneTools))
		} else {
			for _, t := range gitCloneTools {
				if err := installGitClone(t.Binary, t.GitRepo); err != nil {
					core.Warn("Failed to clone %s: %v", t.Binary, err)
				}
			}
		}
	}

	// Install AppImage tools
	if len(appImageTools) > 0 {
		if _, err := exec.LookPath("curl"); err != nil {
			core.Warn("curl not found — skipping %d AppImage tools", len(appImageTools))
		} else {
			for _, t := range appImageTools {
				if err := installAppImage(t.Binary, t.AppRepo); err != nil {
					core.Warn("Failed to install %s AppImage: %v", t.Binary, err)
				}
			}
		}
	}

	// Install deb packages from GitHub releases
	if len(debTools) > 0 {
		if _, err := exec.LookPath("curl"); err != nil {
			core.Warn("curl not found — skipping %d deb tools", len(debTools))
		} else {
			for _, t := range debTools {
				if err := installDeb(t.Binary, t.DebRepo); err != nil {
					core.Warn("Failed to install %s deb: %v", t.Binary, err)
				}
			}
		}
	}

	// Install single binaries from GitHub releases
	if len(releaseBinaryTools) > 0 {
		if _, err := exec.LookPath("curl"); err != nil {
			core.Warn("curl not found — skipping %d release-binary tools", len(releaseBinaryTools))
		} else {
			for _, t := range releaseBinaryTools {
				if err := installReleaseBinary(t.Binary, t.ReleaseRepo, t.AssetPattern); err != nil {
					core.Warn("Failed to install %s: %v", t.Binary, err)
				}
			}
		}
	}

	// Clean registry cache — tool names should not persist on disk
	core.CleanRegistryCache()

	return nil
}

func (ToolkitModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "toolkit"}

	tools := core.Cfg.ToolkitTools
	if len(tools) == 0 {
		s.Extra = "run --toolkit to configure"
		return s
	}

	reg, err := core.LoadCachedRegistry()
	if err != nil {
		s.Extra = "registry not fetched"
		return s
	}

	lookup := make(map[string]core.RegistryTool, len(reg.Tools))
	for _, t := range reg.Tools {
		lookup[t.Name] = t
	}

	home, _ := os.UserHomeDir()

	for _, name := range tools {
		info, ok := lookup[name]
		if !ok {
			s.Missing++
			continue
		}
		if !core.ToolMatchesDistro(info) {
			continue
		}
		switch info.Method {
		case "appimage":
			appPath := filepath.Join(home, ".local", "bin", info.Binary+".AppImage")
			if _, err := os.Stat(appPath); err == nil {
				s.Linked++
			} else {
				s.Missing++
			}
		case "git_clone":
			clonePath := filepath.Join(home, ".local", "share", "toolkit", info.Binary)
			if fi, err := os.Stat(clonePath); err == nil && fi.IsDir() {
				s.Linked++
			} else {
				s.Missing++
			}
		case "release_binary":
			binPath := filepath.Join(home, ".local", "bin", info.Binary)
			if _, err := os.Stat(binPath); err == nil {
				s.Linked++
			} else {
				s.Missing++
			}
		default:
			if _, err := exec.LookPath(info.Binary); err == nil {
				s.Linked++
			} else {
				s.Missing++
			}
		}
	}

	s.Extra = fmt.Sprintf("%d/%d tools", s.Linked, s.Linked+s.Missing)
	return s
}

func (ToolkitModule) Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	reg, err := core.LoadCachedRegistry()
	if err != nil {
		core.Warn("Registry not available — can only remove known paths")
		return nil
	}

	lookup := make(map[string]core.RegistryTool, len(reg.Tools))
	for _, t := range reg.Tools {
		lookup[t.Name] = t
	}

	for _, name := range core.Cfg.ToolkitTools {
		info, ok := lookup[name]
		if !ok {
			continue
		}

		switch info.Method {
		case "appimage":
			appPath := filepath.Join(home, ".local", "bin", info.Binary+".AppImage")
			if _, err := os.Stat(appPath); err == nil {
				if core.DryRun {
					core.Info("would remove %s", appPath)
					continue
				}
				if err := os.Remove(appPath); err != nil {
					core.Warn("Failed to remove %s: %v", appPath, err)
				} else {
					core.Ok("Removed %s", appPath)
				}
			}
		case "git_clone":
			clonePath := filepath.Join(home, ".local", "share", "toolkit", info.Binary)
			if _, err := os.Stat(clonePath); err == nil {
				if core.DryRun {
					core.Info("would remove %s", clonePath)
					continue
				}
				if err := os.RemoveAll(clonePath); err != nil {
					core.Warn("Failed to remove %s: %v", clonePath, err)
				} else {
					core.Ok("Removed %s", clonePath)
				}
			}
		case "deb":
			if _, err := exec.LookPath(info.Binary); err == nil {
				if core.DryRun {
					core.Info("would run: sudo dpkg -r %s", info.Name)
					continue
				}
				core.Info("Removing %s via dpkg...", info.Name)
				if err := runCmd("sudo", "dpkg", "-r", info.Name); err != nil {
					core.Warn("Failed to remove %s: %v", info.Name, err)
				} else {
					core.Ok("Removed %s", info.Name)
				}
			}
		case "release_binary":
			binPath := filepath.Join(home, ".local", "bin", info.Binary)
			if _, err := os.Stat(binPath); err == nil {
				if core.DryRun {
					core.Info("would remove %s", binPath)
					continue
				}
				if err := os.Remove(binPath); err != nil {
					core.Warn("Failed to remove %s: %v", binPath, err)
				} else {
					core.Ok("Removed %s", binPath)
				}
			}
		}
	}

	core.Info("apt/go/cargo/pipx tools should be removed manually if no longer needed")
	return nil
}

// pipxHasPkg checks if a package is already installed via pipx.
func pipxHasPkg(pkg string) bool {
	out, err := exec.Command("pipx", "list", "--short").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == pkg {
			return true
		}
	}
	return false
}

// toolkitDir returns the base directory for git-cloned toolkit repos.
func toolkitDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "toolkit")
}

// installGitClone clones a git repository to ~/.local/share/toolkit/<name>.
func installGitClone(name, repoURL string) error {
	destDir := toolkitDir()
	destPath := filepath.Join(destDir, name)

	// Skip if already present
	if fi, err := os.Stat(destPath); err == nil && fi.IsDir() {
		core.Ok("%s already cloned", name)
		return nil
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	core.Info("Cloning %s...", name)
	if err := runCmd("git", "clone", "--depth=1", repoURL, destPath); err != nil {
		os.RemoveAll(destPath)
		return fmt.Errorf("clone %s: %w", name, err)
	}

	core.Ok("%s cloned to %s", name, destPath)
	return nil
}

// installDeb downloads a .deb from a GitHub release and installs it via dpkg.
func installDeb(name, repo string) error {
	// Skip if already installed
	if _, err := exec.LookPath(name); err == nil {
		core.Ok("%s already installed", name)
		return nil
	}

	core.Info("Downloading %s .deb from GitHub...", name)

	// Query GitHub releases API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	out, err := exec.Command("curl", "-fsSL", apiURL).Output()
	if err != nil {
		return fmt.Errorf("fetch releases for %s: %w", repo, err)
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &release); err != nil {
		return fmt.Errorf("parse releases JSON for %s: %w", repo, err)
	}

	// Find the right .deb for the current architecture
	arch := runtime.GOARCH
	archPatterns := map[string][]string{
		"amd64": {"amd64", "x86_64", "x64"},
		"arm64": {"arm64", "aarch64"},
	}
	patterns, ok := archPatterns[arch]
	if !ok {
		patterns = []string{arch}
	}

	var downloadURL string
	for _, asset := range release.Assets {
		lower := strings.ToLower(asset.Name)
		if !strings.HasSuffix(lower, ".deb") {
			continue
		}
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
		if downloadURL != "" {
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no .deb found for %s/%s", arch, name)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp("", name+"-*.deb")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := runCmd("curl", "-fsSL", "-o", tmpPath, downloadURL); err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}

	// Install with dpkg, then fix any missing dependencies with apt
	if err := runCmd("sudo", "dpkg", "-i", tmpPath); err != nil {
		core.Info("Fixing dependencies for %s...", name)
		bin := core.AptBin()
		if bin == "" {
			return fmt.Errorf("install %s (dpkg failed and no apt binary available): %w", name, err)
		}
		if fixErr := runCmd("sudo", bin, "install", "-f", "-y"); fixErr != nil {
			return fmt.Errorf("install %s (dpkg failed and apt fix failed): dpkg: %w, apt: %v", name, err, fixErr)
		}
	}

	core.Ok("%s installed via deb", name)
	return nil
}

// installAppImage downloads an AppImage from a GitHub release to ~/.local/bin/.
func installAppImage(name, repo string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	destDir := filepath.Join(home, ".local", "bin")
	destPath := filepath.Join(destDir, name+".AppImage")

	// Skip if already present
	if _, err := os.Stat(destPath); err == nil {
		core.Ok("%s AppImage already installed", name)
		return nil
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	core.Info("Downloading %s AppImage from GitHub...", name)

	// Query GitHub releases API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	out, err := exec.Command("curl", "-fsSL", apiURL).Output()
	if err != nil {
		return fmt.Errorf("fetch releases for %s: %w", repo, err)
	}

	// Parse JSON to find AppImage URL
	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &release); err != nil {
		return fmt.Errorf("parse releases JSON for %s: %w", repo, err)
	}

	// Find the right AppImage for the current architecture
	arch := runtime.GOARCH
	archPatterns := map[string][]string{
		"amd64": {"x86_64", "amd64", "x64"},
		"arm64": {"aarch64", "arm64"},
	}
	patterns, ok := archPatterns[arch]
	if !ok {
		patterns = []string{arch}
	}

	var downloadURL string
	for _, asset := range release.Assets {
		lower := strings.ToLower(asset.Name)
		if !strings.HasSuffix(lower, ".appimage") {
			continue
		}
		for _, p := range patterns {
			if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(p)) {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
		if downloadURL != "" {
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no AppImage found for %s/%s", arch, name)
	}

	// Download
	if err := runCmd("curl", "-fsSL", "-o", destPath, downloadURL); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("download %s: %w", name, err)
	}

	// Make executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("chmod %s: %w", destPath, err)
	}

	core.Ok("%s AppImage installed to %s", name, destPath)
	return nil
}

// installReleaseBinary downloads a binary (raw or inside a tarball) from a
// GitHub release and places it at ~/.local/bin/<name>. Asset selection: the
// optional `pattern` is a substring filter applied first (e.g. "linux-musl"),
// then the asset must contain a token matching the current arch. If the asset
// is a .tar.gz/.tgz, it's extracted and the file named <name> inside is
// promoted to the destination.
func installReleaseBinary(name, repo, pattern string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	destDir := filepath.Join(home, ".local", "bin")
	destPath := filepath.Join(destDir, name)

	if _, err := os.Stat(destPath); err == nil {
		core.Ok("%s already installed", name)
		return nil
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	core.Info("Downloading %s from GitHub release...", name)

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	out, err := exec.Command("curl", "-fsSL", apiURL).Output()
	if err != nil {
		return fmt.Errorf("fetch releases for %s: %w", repo, err)
	}
	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(out, &release); err != nil {
		return fmt.Errorf("parse releases JSON for %s: %w", repo, err)
	}

	arch := runtime.GOARCH
	archPatterns := map[string][]string{
		"amd64": {"x86_64", "amd64", "x64"},
		"arm64": {"aarch64", "arm64"},
	}
	patterns, ok := archPatterns[arch]
	if !ok {
		patterns = []string{arch}
	}

	patternLower := strings.ToLower(pattern)
	var (
		downloadURL string
		assetName   string
	)
	for _, asset := range release.Assets {
		lower := strings.ToLower(asset.Name)
		// Skip checksum / signature / source-archive noise
		if strings.HasSuffix(lower, ".sha256") || strings.HasSuffix(lower, ".sig") ||
			strings.HasSuffix(lower, ".asc") || strings.HasSuffix(lower, ".sbom") ||
			strings.HasSuffix(lower, ".pem") {
			continue
		}
		if patternLower != "" && !strings.Contains(lower, patternLower) {
			continue
		}
		archMatched := false
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				archMatched = true
				break
			}
		}
		if !archMatched {
			continue
		}
		// Prefer Linux assets (skip darwin/windows when both ship)
		if strings.Contains(lower, "darwin") || strings.Contains(lower, "windows") ||
			strings.Contains(lower, ".exe") {
			continue
		}
		downloadURL = asset.BrowserDownloadURL
		assetName = asset.Name
		break
	}

	if downloadURL == "" {
		return fmt.Errorf("no release asset matched for %s (arch=%s, pattern=%q)", name, arch, pattern)
	}

	tmpDir, err := os.MkdirTemp("", "release-"+name+"-")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpPath := filepath.Join(tmpDir, assetName)
	if err := runCmd("curl", "-fsSL", "-o", tmpPath, downloadURL); err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}

	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		if err := runCmd("tar", "-xzf", tmpPath, "-C", tmpDir); err != nil {
			return fmt.Errorf("extract %s: %w", assetName, err)
		}
		found, ferr := findExtractedBinary(tmpDir, name)
		if ferr != nil {
			return ferr
		}
		if err := os.Rename(found, destPath); err != nil {
			return fmt.Errorf("move %s: %w", name, err)
		}
	default:
		// Raw binary
		if err := os.Rename(tmpPath, destPath); err != nil {
			return fmt.Errorf("move %s: %w", name, err)
		}
	}
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("chmod %s: %w", destPath, err)
	}
	core.Ok("%s installed to %s", name, destPath)
	return nil
}

// ensureToolchain makes sure the given binary is on PATH; if not, it tries
// to install the corresponding system package via the detected package
// manager and re-checks. Returns true when the toolchain is usable, false
// when the bootstrap failed (the per-method install loop should skip).
// Used to auto-resolve missing cargo/pipx/go before running per-tool
// install commands so users don't have to do a separate prereq install.
func ensureToolchain(binName, pkgName string, n int, label string) bool {
	if _, err := exec.LookPath(binName); err == nil {
		return true
	}
	core.Info("%s not found — installing %q (required for %d %s)...", binName, pkgName, n, label)
	if err := installPkg(pkgName); err != nil {
		core.AlwaysWarn("Failed to install %s: %v — skipping %d %s", pkgName, err, n, label)
		return false
	}
	if _, err := exec.LookPath(binName); err != nil {
		core.AlwaysWarn("Installed %s but %s still not on PATH — skipping %d %s (open a new shell or check PATH)", pkgName, binName, n, label)
		return false
	}
	core.Ok("%s installed via %s", binName, pkgName)
	return true
}

// findExtractedBinary walks an extracted archive directory looking for a
// regular file named exactly `name` (typical for one-binary releases).
func findExtractedBinary(root, name string) (string, error) {
	var found string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(p) == name {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", root, err)
	}
	if found == "" {
		return "", fmt.Errorf("binary %q not found in archive", name)
	}
	return found, nil
}
