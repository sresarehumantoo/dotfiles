package modules

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ulikunitz/xz"
)

// makeTarXz writes a .tar.xz at path holding the given name->content entries,
// exercising the exact xz+tar path extractFaces reads.
func makeTarXz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatalf("xz.NewWriter: %v", err)
	}
	tw := tar.NewWriter(xw)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := xw.Close(); err != nil {
		t.Fatalf("xz close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// TestExtractFaces_TakesOnlyCanonicalFaces builds an archive shaped like the
// real IosevkaTerm.tar.xz — the four wanted faces alongside the Mono and Propo
// variants, other weights, and a non-font file — and asserts extractFaces
// writes exactly the four and nothing else. This is the guard against ever
// installing the icon-clamping Mono build or the non-monospace Propo build.
func TestExtractFaces_TakesOnlyCanonicalFaces(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "IosevkaTerm.tar.xz")

	files := map[string]string{
		// wanted — the default proportional-metrics build, four canonical styles.
		// Nested under a dir to prove basename matching survives archive layout.
		"IosevkaTerm/IosevkaTermNerdFont-Regular.ttf":    "regular",
		"IosevkaTerm/IosevkaTermNerdFont-Bold.ttf":       "bold",
		"IosevkaTerm/IosevkaTermNerdFont-Italic.ttf":     "italic",
		"IosevkaTerm/IosevkaTermNerdFont-BoldItalic.ttf": "bolditalic",
		// unwanted lookalikes that must never be installed.
		"IosevkaTermNerdFontMono-Regular.ttf":     "mono",
		"IosevkaTermNerdFontPropo-Regular.ttf":    "propo",
		"IosevkaTermNerdFont-SemiBold.ttf":        "extra-weight",
		"IosevkaTermNerdFont-Thin.ttf":            "extra-weight",
		"IosevkaTermNerdFontMono-BoldItalic.ttf":  "mono",
		"IosevkaTermNerdFontPropo-BoldItalic.ttf": "propo",
		"README.md": "docs",
	}
	makeTarXz(t, archive, files)

	dest := filepath.Join(tmp, "dest")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractFaces(archive, dest, iosevkaTerm.faces); err != nil {
		t.Fatalf("extractFaces: %v", err)
	}

	got := dirEntries(t, dest)
	want := []string{
		"IosevkaTermNerdFont-Bold.ttf",
		"IosevkaTermNerdFont-BoldItalic.ttf",
		"IosevkaTermNerdFont-Italic.ttf",
		"IosevkaTermNerdFont-Regular.ttf",
	}
	if len(got) != len(want) {
		t.Fatalf("extracted %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extracted %v, want %v", got, want)
		}
	}

	// Content is the real face's bytes, not a Mono/Propo lookalike's.
	data, err := os.ReadFile(filepath.Join(dest, "IosevkaTermNerdFont-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "regular" {
		t.Fatalf("Regular content = %q, want %q (grabbed the wrong entry?)", data, "regular")
	}
}

// TestExtractFaces_MissingFaceIsError ensures a family that silently lacks a
// style fails loudly rather than leaving Ghostty to synthesize it.
func TestExtractFaces_MissingFaceIsError(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "partial.tar.xz")
	makeTarXz(t, archive, map[string]string{
		"IosevkaTermNerdFont-Regular.ttf": "regular",
		"IosevkaTermNerdFont-Bold.ttf":    "bold",
		// Italic and BoldItalic absent.
	})

	dest := filepath.Join(tmp, "dest")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractFaces(archive, dest, iosevkaTerm.faces); err == nil {
		t.Fatal("extractFaces succeeded on an archive missing two faces; want error")
	}
}

func TestParseSHALine(t *testing.T) {
	// Real format: "<64-hex>  <filename>", two spaces, many lines.
	listing := "aaaa  Hack.tar.xz\n" +
		"cad9da572d25e3413f7a15a319d2f3c9e7e915ee016baa99e0d88fc08cf5b781  IosevkaTerm.tar.xz\n" +
		"bbbb  IosevkaTerm.tar.xz.sig\n"

	sum, ok := parseSHALine(listing, "IosevkaTerm.tar.xz")
	if !ok {
		t.Fatal("expected to find IosevkaTerm.tar.xz")
	}
	if sum != "cad9da572d25e3413f7a15a319d2f3c9e7e915ee016baa99e0d88fc08cf5b781" {
		t.Fatalf("wrong sum: %s", sum)
	}

	// A sidecar whose name is a superstring of the asset must not match.
	if _, ok := parseSHALine(listing, "Iosevka.tar.xz"); ok {
		t.Fatal("matched a filename that is only a substring of a listed one")
	}
	if _, ok := parseSHALine(listing, "Absent.tar.xz"); ok {
		t.Fatal("matched an asset not in the listing")
	}
}
