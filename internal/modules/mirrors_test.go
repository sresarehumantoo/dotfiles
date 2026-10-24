package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNpmMirrored(t *testing.T) {
	for reg, want := range map[string]bool{
		"":                                    false,
		"https://registry.npmjs.org/":         false,
		"https://registry.npmjs.org":          false,
		"https://nexus.example.com/repo/npm/": true,
	} {
		if got := npmMirrored(reg); got != want {
			t.Errorf("npmMirrored(%q) = %v", reg, got)
		}
	}
}

func TestCargoReplacement(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) string {
		p := filepath.Join(dir, "config.toml")
		os.WriteFile(p, []byte(s), 0o644)
		return p
	}
	if got := cargoReplacement(write("[build]\njobs = 4\n")); got != "" {
		t.Errorf("no replacement: %q", got)
	}
	p := write("[source.crates-io]\nreplace-with = \"corp\"\n\n[source.corp]\nregistry = \"sparse+https://nexus.example.com/repository/cargo/\"\n")
	if got := cargoReplacement(p); got != "sparse+https://nexus.example.com/repository/cargo/" {
		t.Errorf("replacement: %q", got)
	}
	p = write("[source.crates-io]\nreplace-with = \"vendored\"\n[source.vendored]\ndirectory = \"vendor\"\n")
	if got := cargoReplacement(p); !strings.HasPrefix(got, "source replacement in ") {
		t.Errorf("replacement without registry: %q", got)
	}
}
