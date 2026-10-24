package core

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeBrew makes a prefix holding bin/brew and a Cellar, read-only when ro.
func fakeBrew(t *testing.T, ro bool) string {
	t.Helper()
	p := t.TempDir()
	if err := os.MkdirAll(filepath.Join(p, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "bin", "brew"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cellar := filepath.Join(p, "Cellar")
	if err := os.Mkdir(cellar, 0o755); err != nil {
		t.Fatal(err)
	}
	if ro {
		for _, d := range []string{cellar, p} {
			if err := os.Chmod(d, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(d, 0o755) })
		}
	}
	return p
}

// A managed Mac can have an IT-owned Homebrew this user cannot write to beside
// one of their own; the writable one has to win, and a recorded choice wins
// over both.
func TestBrewPrefixFrom(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere, so read-only prefixes cannot be simulated")
	}
	itOwned := fakeBrew(t, true)
	own := fakeBrew(t, false)
	empty := t.TempDir()

	cases := []struct {
		name       string
		recorded   string
		candidates []string
		want       string
	}{
		{"writable beats an earlier read-only one", "", []string{itOwned, own}, own},
		{"read-only is the fallback", "", []string{empty, itOwned}, itOwned},
		{"recorded wins even when read-only", itOwned, []string{own}, itOwned},
		{"recorded without brew is ignored", empty, []string{itOwned, own}, own},
		{"empty and missing candidates are skipped", "", []string{"", empty, filepath.Join(empty, "nope")}, ""},
	}
	for _, c := range cases {
		if got := brewPrefixFrom(c.recorded, c.candidates); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
