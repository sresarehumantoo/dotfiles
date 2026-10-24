package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInHomebrewCellar(t *testing.T) {
	cases := map[string]bool{
		"/opt/homebrew/Cellar/dfinstall/2.0.0/bin/dfinstall":              true,
		"/home/linuxbrew/.linuxbrew/Cellar/dfinstall/2.0.0/bin/dfinstall": true,
		"/usr/local/bin/dfinstall":                                        false,
		"/home/u/.local/bin/dfinstall":                                    false,
		"/home/u/Cellarish/dfinstall":                                     false,
	}
	for exe, want := range cases {
		if got := InHomebrewCellar(exe); got != want {
			t.Errorf("InHomebrewCellar(%q) = %v, want %v", exe, got, want)
		}
	}
}

// Homebrew puts a symlink into <prefix>/bin; the Cellar is only visible once
// it is resolved.
func TestInHomebrewCellar_ResolvesBinSymlink(t *testing.T) {
	prefix := t.TempDir()
	real := filepath.Join(prefix, "Cellar", "dfinstall", "2.0.0", "bin", "dfinstall")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(prefix, "bin", "dfinstall")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if !InHomebrewCellar(link) {
		t.Errorf("InHomebrewCellar(%q) = false for a link into the Cellar", link)
	}
}
