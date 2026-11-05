package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
	"github.com/sresarehumantoo/dotfiles/v2/internal/modules"
)

func writeMode(t *testing.T, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func modeOf(t *testing.T, p string) os.FileMode {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestEnsureExecutable_SetsMissingBits(t *testing.T) {
	p := writeMode(t, 0o644)
	core.EnsureExecutable(p)
	if got := modeOf(t, p); got != 0o755 {
		t.Errorf("mode = %o, want 755", got)
	}
}

// Packaged content is read-only (0555), so an executable source must be left
// exactly as shipped rather than chmodded back to 0755.
func TestEnsureExecutable_LeavesExecutableAlone(t *testing.T) {
	p := writeMode(t, 0o555)
	core.EnsureExecutable(p)
	if got := modeOf(t, p); got != 0o555 {
		t.Errorf("mode = %o, want 555 untouched", got)
	}
}

func TestEnsureExecutable_DryRun(t *testing.T) {
	core.DryRun = true
	defer func() { core.DryRun = false }()
	p := writeMode(t, 0o644)
	core.EnsureExecutable(p)
	if got := modeOf(t, p); got != 0o644 {
		t.Errorf("mode = %o, want 644 under dry run", got)
	}
}

func TestEnsureExecutable_MissingIsSilent(t *testing.T) {
	core.EnsureExecutable(filepath.Join(t.TempDir(), "absent"))
}

// Every script linked into ~/.local/bin must ship executable, because content
// that is read-only or root-owned cannot be chmodded at install time.
func TestLinkedScriptsShipExecutable(t *testing.T) {
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTFILES", repo)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	modules.RegisterAllModules()

	bin := core.HomeTarget(".local", "bin") + string(filepath.Separator)
	checked := 0
	for _, m := range core.AllModules() {
		le, ok := m.(core.LinkExporter)
		if !ok {
			continue
		}
		for _, l := range le.Links() {
			if !strings.HasPrefix(l.Dst, bin) {
				continue
			}
			checked++
			if mode := modeOf(t, l.Src); mode&0o111 != 0o111 {
				t.Errorf("%s: %s is %o, want executable", m.Name(), strings.TrimPrefix(l.Src, repo+"/"), mode)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no links into ~/.local/bin found; the test checked nothing")
	}
}
