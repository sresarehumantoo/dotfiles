package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// What an upgrade from an older dfinstall can leave behind, especially when the
// new one comes from a package rather than the clone: an older binary that PATH
// still runs, a checkout older than the binary, and a pre-2.0 settings file.

// upgradeDoctorResults reports each of them.
func upgradeDoctorResults() []DoctorResult {
	return []DoctorResult{binaryOnPathResult(), checkoutFilesResult(), settingsFileResult()}
}

// binaryOnPathResult reports when the dfinstall PATH finds is not this one,
// typically a source build left in ~/.local/bin ahead of a Homebrew or apt
// install.
func binaryOnPathResult() DoctorResult {
	const name = "dfinstall on PATH"
	self, err := os.Executable()
	if err != nil {
		return DoctorResult{Name: name, OK: true, Detail: "could not locate the running binary"}
	}
	self = resolvedPath(self)
	// The MCP server answers for the CLI installed beside it.
	if filepath.Base(self) != "dfinstall" {
		self = filepath.Join(filepath.Dir(self), "dfinstall")
	}
	found, err := exec.LookPath("dfinstall")
	if err != nil {
		return DoctorResult{Name: name, OK: true, Detail: "not on PATH; running " + self}
	}
	if resolvedPath(found) == self {
		return DoctorResult{Name: name, OK: true, Detail: found + " (" + core.Version + ", " + core.BuildChannel() + ")"}
	}
	// No context to inherit: doctor is a synchronous report.
	other := "version unknown"
	if out, err := runProbe(context.Background(), found, "--version"); err == nil {
		other = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "dfinstall version "))
	}
	return DoctorResult{Name: name, OK: false,
		Detail: fmt.Sprintf("%s (%s) runs, not this one (%s, %s)", found, other, self, core.Version),
		Extra: []string{"usually a source build left behind after installing dfinstall from a package",
			"remove " + found + " (and dfinstall-mcp beside it), or put " + filepath.Dir(self) + " earlier on PATH"}}
}

// checkoutFilesResult reports files this dfinstall links that the checkout
// does not have. LinkFile skips them rather than leave dangling links, so
// without this the gap is easy to miss.
func checkoutFilesResult() DoctorResult {
	const name = "checkout files"
	missing, first := 0, ""
	for _, m := range core.AllModules() {
		le, ok := m.(core.LinkExporter)
		if !ok {
			continue
		}
		for _, l := range le.Links() {
			if _, err := os.Stat(l.Src); err != nil {
				missing++
				if first == "" {
					first = l.Src
				}
			}
		}
	}
	if missing == 0 {
		return DoctorResult{Name: name, OK: true, Detail: "every file this dfinstall links is in " + core.DotfilesDir()}
	}
	return DoctorResult{Name: name, OK: false,
		Detail: fmt.Sprintf("%d file(s) this dfinstall links are missing from %s", missing, core.DotfilesDir()),
		Extra: []string{"the checkout is older than this dfinstall: pull it (git -C " + core.DotfilesDir() + " pull), then run 'dfinstall install all'",
			"first missing: " + first}}
}

// settingsFileResult says which settings file is in use, and when a pre-2.0
// one in the clone has been superseded and can go.
func settingsFileResult() DoctorResult {
	const name = "settings"
	current, legacy := core.ConfigFilePath(), core.LegacyConfigFilePath()
	hasCurrent, hasLegacy := fileExists(current), legacy != "" && fileExists(legacy)
	switch {
	case hasCurrent && hasLegacy:
		return DoctorResult{Name: name, OK: true,
			Detail: current + "; " + legacy + " is from before 2.0 and no longer read, delete it once no older dfinstall uses this checkout"}
	case hasLegacy:
		return DoctorResult{Name: name, OK: true, Detail: legacy + " (from before 2.0; moves to " + current + " on the next save)"}
	case hasCurrent:
		return DoctorResult{Name: name, OK: true, Detail: current}
	default:
		return DoctorResult{Name: name, OK: true, Detail: "defaults (no settings file yet)"}
	}
}

func resolvedPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
