package modules

import (
	"os"
	"os/exec"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// Where a toolkit tool ends up on disk, by install method.
//
// This mapping used to be written out three times — in Status, in Uninstall,
// and in the menu's isToolInstalled — and all three had to agree or the tool
// would report as missing in one place, be skipped in another, and have the
// wrong path removed in the third. Uninstall acts on the answer with
// os.Remove/RemoveAll, so a disagreement there is destructive.

// toolArtifact locates a tool installed by a given method. Exactly one of Path
// and Bin is set: Path for tools we place ourselves, Bin for tools a package
// manager put somewhere on PATH.
type toolArtifact struct {
	Path  string // absolute path we installed to
	IsDir bool   // Path is a directory (a git clone) rather than a file
	Bin   string // binary name to resolve on PATH
}

// toolMethod is how t installs on this OS. On macOS its brew formula, when it
// has one, replaces the Linux method.
func toolMethod(t core.RegistryTool) string {
	if core.IsMac() && t.Brew != "" {
		return "brew"
	}
	return t.Method
}

// brewFormula is the formula a brew-installed tool uses: its macOS override,
// or the package of a tool whose method is brew.
func brewFormula(t core.RegistryTool) string {
	if t.Brew != "" {
		return t.Brew
	}
	return t.Package
}

// artifactFor returns where the tool lives once installed.
func artifactFor(t core.RegistryTool) toolArtifact {
	return artifactForMethod(toolMethod(t), t.Binary)
}

// artifactForMethod is artifactFor for callers that only have the method and
// binary name — the installers, which must write to exactly the path Status
// and Uninstall later look for.
func artifactForMethod(method, binary string) toolArtifact {
	switch method {
	case "appimage":
		return toolArtifact{Path: core.HomeTarget(".local", "bin", binary+".AppImage")}
	case "git_clone":
		// SubPath, not filepath.Join: an unresolved $HOME makes toolkitDir()
		// empty, and a relative Path is one the installer clones into and
		// Uninstall deletes from the current working directory.
		return toolArtifact{Path: core.SubPath(toolkitDir(), binary), IsDir: true}
	case "release_binary":
		return toolArtifact{Path: core.HomeTarget(".local", "bin", binary)}
	case "rustup":
		return toolArtifact{Path: core.HomeTarget(".cargo", "bin", "rustup")}
	default:
		// apt / brew / go / cargo / pipx / deb — the package manager decides the
		// location, so presence is a PATH lookup.
		return toolArtifact{Bin: binary}
	}
}

// Installed reports whether the artifact is present.
func (a toolArtifact) Installed() bool {
	if a.Path == "" {
		_, err := exec.LookPath(a.Bin)
		return err == nil
	}
	fi, err := os.Stat(a.Path)
	if err != nil {
		return false
	}
	if a.IsDir {
		return fi.IsDir()
	}
	return true
}

// isToolInstalled reports whether a registry tool is already on the system.
func isToolInstalled(t core.RegistryTool) bool {
	return artifactFor(t).Installed()
}
