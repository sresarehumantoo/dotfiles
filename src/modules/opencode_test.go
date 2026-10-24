package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpencodeTargetFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		avx2         bool
		want         string
		ok           bool
	}{
		{"linux", "amd64", true, "linux-x64", true},
		{"linux", "amd64", false, "linux-x64-baseline", true},
		{"linux", "arm64", false, "linux-arm64", true},
		{"darwin", "arm64", false, "darwin-arm64", true},
		{"darwin", "amd64", true, "darwin-x64", true},
		{"windows", "amd64", true, "", false},
		{"linux", "386", false, "", false},
	}
	for _, c := range cases {
		got, ok := opencodeTargetFor(c.goos, c.goarch, c.avx2)
		if got != c.want || ok != c.ok {
			t.Errorf("opencodeTargetFor(%s, %s, %v) = %q, %v; want %q, %v", c.goos, c.goarch, c.avx2, got, ok, c.want, c.ok)
		}
	}
	if got := opencodeAssetName("darwin-arm64"); got != "opencode-darwin-arm64.zip" {
		t.Errorf("darwin asset = %s", got)
	}
	if got := opencodeAssetName("linux-x64-baseline"); got != "opencode-linux-x64-baseline.tar.gz" {
		t.Errorf("linux asset = %s", got)
	}
}

// A mirrored npm goes first; the public registry is only a fallback behind
// the GitHub release; no npm means GitHub alone.
func TestOpencodeSourcesOrder(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	names := func() string {
		var n []string
		for _, s := range opencodeSources(context.Background()) {
			n = append(n, s.name)
		}
		return strings.Join(n, " | ")
	}

	if got := names(); got != "the GitHub release" {
		t.Errorf("no npm: %s", got)
	}
	fakeNpm := func(registry string) {
		os.WriteFile(filepath.Join(bin, "npm"), []byte("#!/bin/sh\necho '"+registry+"'\n"), 0o755)
	}
	fakeNpm("https://registry.npmjs.org/")
	if got := names(); got != "the GitHub release | npm (https://registry.npmjs.org/)" {
		t.Errorf("public npm: %s", got)
	}
	fakeNpm("https://ci:s3cret@nexus.example.com/repository/npm-group/")
	if got := names(); got != "npm (https://nexus.example.com/repository/npm-group/) | the GitHub release" {
		t.Errorf("mirrored npm: %s", got)
	}
}

func TestZshQuote(t *testing.T) {
	if got := zshQuote(`{"a":"it's"}`); got != `'{"a":"it'\''s"}'` {
		t.Errorf("zshQuote = %s", got)
	}
}
