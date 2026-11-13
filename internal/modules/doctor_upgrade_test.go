package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// An older dfinstall first on PATH (a source build left in ~/.local/bin after
// a package install) is reported with its version.
func TestBinaryOnPathResult_ReportsAShadowingBinary(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "dfinstall")
	if err := os.WriteFile(old, []byte("#!/bin/sh\necho 'dfinstall version v1.9.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := binaryOnPathResult()
	if r.OK {
		t.Fatalf("a different dfinstall first on PATH passed: %+v", r)
	}
	if !strings.Contains(r.Detail, old) || !strings.Contains(r.Detail, "v1.9.0") {
		t.Errorf("detail should name the binary and its version: %q", r.Detail)
	}
}

func TestBinaryOnPathResult_NotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if r := binaryOnPathResult(); !r.OK {
		t.Errorf("no dfinstall on PATH is not a problem: %+v", r)
	}
}

// A link whose source is another managed file in the home, not the checkout,
// is not a missing checkout file before that module is installed.
func TestCheckoutFilesResult_OnlyCountsTheCheckout(t *testing.T) {
	isolate(t)
	registerOnce.Do(RegisterAllModules)
	root := t.TempDir()
	t.Setenv("DOTFILES", root)
	core.ResetDotfilesDir()
	for _, m := range core.AllModules() {
		if le, ok := m.(core.LinkExporter); ok {
			for _, l := range le.Links() {
				if strings.HasPrefix(l.Src, root+string(filepath.Separator)) {
					if err := os.MkdirAll(filepath.Dir(l.Src), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(l.Src, nil, 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	if r := checkoutFilesResult(); !r.OK {
		t.Errorf("a complete checkout failed: %+v", r)
	}
}
