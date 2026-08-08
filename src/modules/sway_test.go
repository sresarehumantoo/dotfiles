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
