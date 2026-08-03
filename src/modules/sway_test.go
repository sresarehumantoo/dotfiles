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
	if len(missing) != len(swayPackages)-1 {
		t.Errorf("expected %d missing, got %d (%v)", len(swayPackages)-1, len(missing), missing)
	}
}
