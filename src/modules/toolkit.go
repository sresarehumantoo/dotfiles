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

	for _, name := range tools {
		info, ok := lookup[name]
		if !ok {
			core.Warn("Unknown toolkit tool %q — skipping", name)
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
	if len(goTools) > 0 {
		if _, err := exec.LookPath("go"); err != nil {
			core.Warn("go not found — skipping %d go tools (install Go first)", len(goTools))
		} else {
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
	}

	// Install cargo tools
	for _, t := range cargoTools {
		if _, err := exec.LookPath(t.Binary); err == nil {
			core.Ok("%s already installed", t.Binary)
			continue
		}
		if _, err := exec.LookPath("cargo"); err != nil {
			core.Warn("cargo not found — skipping %s (install Rust toolchain first)", t.Binary)
			continue
		}
		core.Info("Installing %s via cargo install...", t.Binary)
		if err := runCmd("cargo", "install", t.Package); err != nil {
			core.Warn("Failed to install %s: %v", t.Binary, err)
		} else {
			core.Ok("%s installed", t.Binary)
		}
	}

	// Install pipx tools
	if len(pipxTools) > 0 {
		if _, err := exec.LookPath("pipx"); err != nil {
			core.Warn("pipx not found — skipping %d pipx tools (install pipx first)", len(pipxTools))
		} else {
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
