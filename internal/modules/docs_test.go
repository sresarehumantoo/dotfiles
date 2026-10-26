package modules

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDevdocs(t *testing.T) {
	catalog := []devdocsEntry{
		{Slug: "bash"},
		{Slug: "python~3.15"},
		{Slug: "python~3.14"},
		{Slug: "pythonista"},
	}
	for name, want := range map[string]string{
		"bash":        "bash",
		"python":      "python~3.15",
		"python~3.14": "python~3.14",
		"pythonista":  "pythonista",
	} {
		got, ok := resolveDevdocs(catalog, name)
		if !ok || got.Slug != want {
			t.Errorf("resolveDevdocs(%q) = %q, %v; want %q", name, got.Slug, ok, want)
		}
	}
	if _, ok := resolveDevdocs(catalog, "py"); ok {
		t.Error("a bare prefix of a slug must not resolve")
	}
}

func writeTarGz(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	if err := os.WriteFile(path, makeTarGz(t, entries), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractDocTarball(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	writeTarGz(t, archive, []tarEntry{
		{name: "top/doc/ci/yaml/_index.md", body: "yaml"},
		{name: "top/doc/ci/index.md", body: "ci"},
		{name: "top/doc/user/start.md", body: "start"},
		{name: "top/doc/user/image.png", body: "png"},
		{name: "top/doc/user/link.md", typ: tar.TypeSymlink},
		{name: "top/other/skip.md", body: "skip"},
	})
	stage := filepath.Join(dir, "stage")
	if err := extractDocTarball(archive, "doc", stage); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(stage, docIndexFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"ci/yaml\tci/yaml/_index.md\tci",
		"ci\tci/index.md\tci",
		"user/start\tuser/start.md\tuser",
	} {
		if !strings.Contains(string(index), line+"\n") {
			t.Errorf("index lacks %q:\n%s", line, index)
		}
	}
	if strings.Count(string(index), "\n") != 3 {
		t.Errorf("index should hold only the three pages under doc/:\n%s", index)
	}
	if _, err := os.Stat(filepath.Join(stage, "user", "image.png")); err == nil {
		t.Error("non-markdown files must not be extracted")
	}
}

func TestExtractDocTarballRefusesEscape(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	writeTarGz(t, archive, []tarEntry{{name: "top/doc/../escape.md", body: "x"}})
	stage := filepath.Join(dir, "stage")
	if err := extractDocTarball(archive, "doc", stage); err == nil {
		t.Fatal("a page path leaving the stage must be refused")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.md")); err == nil {
		t.Fatal("escape.md was written outside the stage")
	}
}
