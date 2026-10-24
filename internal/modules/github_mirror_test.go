package modules

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNexusBase(t *testing.T) {
	for in, want := range map[string]string{
		"http://nexus:8081/repository/npm-proxy/":                   "http://nexus:8081/",
		"https://ci:s3cret@nexus.example.com/nexus/repository/npm/": "https://nexus.example.com/nexus/",
		"https://registry.npmjs.org/":                               "",
		"https://corp.jfrog.io/artifactory/api/npm/npm/":            "",
		"not a url": "",
	} {
		if got := nexusBase(in); got != want {
			t.Errorf("nexusBase(%q) = %q, want %q", in, got, want)
		}
	}
}

// The fixture is a real Nexus CE 3.96 listing, trimmed, from scripts/nexuslab.
func TestParseNexusRepos(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "nexus-repositories.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := parseNexusRepos("http://nexus.example.com/", data)
	want := githubMirror{
		api: "http://nexus.example.com/repository/github-api/",
		web: "http://nexus.example.com/repository/github/",
	}
	if got != want {
		t.Errorf("parseNexusRepos = %+v, want %+v", got, want)
	}
	if got := parseNexusRepos("http://x/", []byte(`{"error": "forbidden"}`)); got != (githubMirror{}) {
		t.Errorf("non-list body: %+v", got)
	}
}

func TestGitHubMirrorURLsVia(t *testing.T) {
	m := githubMirror{api: "http://n/repository/gh-api/", web: "http://n/repository/gh/"}
	cases := map[string][]string{
		"https://api.github.com/repos/a/b/releases/latest":  {"http://n/repository/gh-api/repos/a/b/releases/latest", "https://api.github.com/repos/a/b/releases/latest"},
		"https://github.com/a/b/releases/download/v1/x.tgz": {"http://n/repository/gh/a/b/releases/download/v1/x.tgz", "https://github.com/a/b/releases/download/v1/x.tgz"},
		"https://dot.net/v1/dotnet-install.sh":              {"https://dot.net/v1/dotnet-install.sh"},
	}
	for in, want := range cases {
		if got := m.urlsVia(in); !reflect.DeepEqual(got, want) {
			t.Errorf("urlsVia(%s) = %v, want %v", in, got, want)
		}
	}
	if got := (githubMirror{web: "http://n/gh/"}).urlsVia("https://api.github.com/x"); len(got) != 1 {
		t.Errorf("api URL with only a web mirror: %v", got)
	}
}

// A failing mirror hands over to GitHub; the order is mirror, then GitHub.
func TestGitHubDownloadFallsBack(t *testing.T) {
	githubMirrorOnce = sync.Once{}
	t.Cleanup(func() { githubMirrorOnce = sync.Once{}; githubMirrorSeen = githubMirror{} })
	githubMirrorOnce.Do(func() {
		githubMirrorSeen = githubMirror{web: "http://mirror.invalid/repository/github/"}
	})

	bin := t.TempDir()
	log := filepath.Join(bin, "log")
	// Fails for the mirror, succeeds for github.com; writes -o's argument.
	script := `#!/bin/sh
for a; do case "$a" in http*) echo "$a" >> "` + log + `"; url="$a";; esac; done
case "$url" in *mirror.invalid*) echo "curl: (7) refused" >&2; exit 7;; esac
while [ $# -gt 0 ]; do [ "$1" = -o ] && echo ok > "$2"; shift; done
`
	os.WriteFile(filepath.Join(bin, "curl"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dst := filepath.Join(t.TempDir(), "x.tgz")
	if err := githubDownload(context.Background(), "https://github.com/a/b/releases/download/v1/x.tgz", dst); err != nil {
		t.Fatalf("githubDownload: %v", err)
	}
	data, _ := os.ReadFile(log)
	got := strings.Fields(string(data))
	want := []string{"http://mirror.invalid/repository/github/a/b/releases/download/v1/x.tgz", "https://github.com/a/b/releases/download/v1/x.tgz"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fetch order = %v, want %v", got, want)
	}
}
