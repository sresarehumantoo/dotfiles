package modules

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// In-package rather than in tests/, because the dependency list and its binary
// mapping are unexported, and exporting them purely for a test is the pattern
// this repo deliberately avoids elsewhere.

func TestSwayPackagesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, pkg := range swayPackages {
		if strings.TrimSpace(pkg) == "" {
			t.Fatalf("swayPackages contains an empty entry")
		}
		if seen[pkg] {
			t.Errorf("swayPackages lists %q twice — installPkg would pass it to apt twice", pkg)
		}
		seen[pkg] = true

		if bin := swayPkgBinary(pkg); strings.TrimSpace(bin) == "" {
			t.Errorf("swayPkgBinary(%q) is empty; LookPath(\"\") always fails, so the "+
				"package would be reported missing forever and reinstalled every run", pkg)
		}
	}
}

// The list drives `apt-get install` on someone else's machine, so an entry
// that no config actually references is not a harmless extra — it is unasked-for
// software. This caught `foot` being added as a "fallback terminal" when the
// config sets `$term ghostty` and never mentions foot.
func TestSwayPackagesAreReferencedByConfig(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot locate source file")
	}
	repo := filepath.Join(filepath.Dir(thisFile), "..", "..")

	var corpus strings.Builder
	for _, rel := range []string{
		"config/sway/config",
		"config/waybar/config",
		"config/swaync/config.json",
		"config/sway/sway-powermenu",
		"config/sway/sway-quickpanel",
		"config/sway/sway-calendar",
		"docs/sway.md",
	} {
		b, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		corpus.WriteString(string(b))
		corpus.WriteByte('\n')
	}
	hay := corpus.String()

	for _, pkg := range swayPackages {
		// A package earns its place if either its package name or the binary
		// it provides is named somewhere in the desktop it configures.
		if strings.Contains(hay, pkg) || strings.Contains(hay, swayPkgBinary(pkg)) {
			continue
		}
		t.Errorf("swayPackages includes %q (binary %q) but neither appears in any sway "+
			"config or in docs/sway.md — it would be installed on every sway box for "+
			"no reason. Remove it, or document why it is required.",
			pkg, swayPkgBinary(pkg))
	}
}

// missingSwayPackages must decide on PATH, not on a package database: this repo
// runs on a box where sway lives in /opt/sway-next/bin, and the fonts module
// already taught us what gating on the wrong signal costs.
func TestMissingSwayPackagesResolvesViaPATH(t *testing.T) {
	dir := t.TempDir()
	// Provide exactly one of the mapped packages, under the BINARY name rather
	// than the package name — the distinction the mapping exists for.
	present := "swayosd"
	bin := filepath.Join(dir, swayPkgBinary(present))
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	t.Setenv("PATH", dir)

	missing := missingSwayPackages()
	missingSet := map[string]bool{}
	for _, m := range missing {
		missingSet[m] = true
	}

	if missingSet[present] {
		t.Errorf("%q is on PATH as %q but was reported missing — the package→binary "+
			"mapping is not being applied", present, swayPkgBinary(present))
	}
	if !missingSet["waybar"] {
		t.Errorf("waybar is absent from the sandboxed PATH but was not reported missing")
	}

	// Packages in swayPkgGlobs are decided on DISK, not PATH, so an emptied PATH
	// says nothing about them and they must be excluded from this count.
	want := -1 // swayosd, provided above
	for _, pkg := range swayPackages {
		globs, fileProbed := swayPkgGlobs[pkg]
		if !fileProbed {
			want++
			continue
		}
		if !pkgFilePresent(globs) {
			want++
		}
	}
	if len(missing) != want {
		t.Errorf("expected %d missing, got %d (%v)", want, len(missing), missing)
	}
}

// A typelib has no binary, so it cannot be probed on PATH. Without a file-based
// check every gir1.2-* package would be reported missing forever, and `install
// sway` would reinstall it on every run.
func TestPkgFilePresentDecidesOnDisk(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "x86_64-linux-gnu", "girepository-1.0")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	globs := []string{
		filepath.Join(dir, "*", "girepository-1.0", "Marker-0.1.typelib"),
		filepath.Join(dir, "girepository-1.0", "Marker-0.1.typelib"),
	}
	if pkgFilePresent(globs) {
		t.Fatal("reported present before the typelib exists")
	}

	// The multiarch glob must match through the wildcard directory component.
	marker := filepath.Join(nested, "Marker-0.1.typelib")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing marker: %v", err)
	}
	if !pkgFilePresent(globs) {
		t.Error("typelib exists under the multiarch path but was not found")
	}

	// PATH must be irrelevant to this decision.
	t.Setenv("PATH", "")
	if !pkgFilePresent(globs) {
		t.Error("an emptied PATH changed a file-based probe")
	}

	// Every package declared as file-probed must really be present on this box,
	// or the glob is wrong — this is the check that would have caught a typo in
	// the multiarch pattern.
	for pkg, g := range swayPkgGlobs {
		if !pkgFilePresent(g) {
			t.Logf("note: %s not found via %v (fine on a non-sway box)", pkg, g)
		}
	}
}

// A symlink inherits its target's mode, so a helper script that is not
// executable AT THE SOURCE lands in ~/.local/bin unrunnable — and sway execs
// these from `exec_always` lines and keybindings, where the failure is silent:
// a key that does nothing, or in sway-tray-filter's case a tray that never
// appears. `swayScripts` is the list Install() chmods before linking, so an
// entry missing from it is exactly that bug. This asserts the two halves agree.
func TestSwayScriptsCoverEveryLinkedScript(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot locate source file")
	}
	repo := filepath.Join(filepath.Dir(thisFile), "..", "..")

	declared := map[string]bool{}
	for _, name := range swayScripts {
		declared[name] = true
		// The chmod in Install() targets config/sway/<name>; a name that is not
		// a file there makes it a no-op that warns on every install.
		if _, err := os.Stat(filepath.Join(repo, "config", "sway", name)); err != nil {
			t.Errorf("swayScripts lists %q but config/sway/%s does not exist: %v", name, name, err)
		}
	}

	// Anything this module links INTO ~/.local/bin from config/sway must be in
	// that list. This is the direction that catches a newly added helper.
	for _, link := range (SwayModule{}).Links() {
		if !strings.Contains(link.Dst, filepath.Join(".local", "bin")) {
			continue
		}
		if !strings.Contains(link.Src, filepath.Join("config", "sway")+string(filepath.Separator)) {
			continue
		}
		name := filepath.Base(link.Src)
		if !declared[name] {
			t.Errorf("config/sway/%s is linked into ~/.local/bin but is missing from "+
				"swayScripts, so Install() never chmods it — the symlink inherits the "+
				"source mode and sway would run an unexecutable file, silently", name)
		}
	}
}
