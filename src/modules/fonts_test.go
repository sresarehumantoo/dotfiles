package modules

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
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

// fakeInstall lays out a font directory exactly as installDownloadedFont leaves
// it: the four faces plus the tag stamp. Returns the family dir.
func fakeInstall(t *testing.T, dataHome, tag string) string {
	t.Helper()
	dir := filepath.Join(dataHome, "fonts", iosevkaTerm.dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, face := range iosevkaTerm.faces {
		if err := os.WriteFile(filepath.Join(dir, face), []byte("ttf"), 0644); err != nil {
			t.Fatalf("write face: %v", err)
		}
	}
	if tag != "" {
		if err := os.WriteFile(filepath.Join(dir, tagStamp), []byte(tag+"\n"), 0644); err != nil {
			t.Fatalf("write stamp: %v", err)
		}
	}
	return dir
}

// TestFontInstalled_ChecksOwnedArtifactNotFontconfig pins the detection contract
// that install correctness depends on. The old gate asked fontconfig whether the
// *family name* resolved, which is true for a hand-installed copy anywhere on
// the system and version-blind — so the download was skipped forever, a tag bump
// no-oped, and uninstall reported success while the font stayed.
func TestFontInstalled_ChecksOwnedArtifactNotFontconfig(t *testing.T) {
	data := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", data)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)

	if present, _ := fontInstalled(iosevkaTerm); present {
		t.Error("empty data home: want not present")
	}

	// A copy installed flat in the fonts dir — visible to fontconfig, but not
	// the directory this module owns. Must still read as not installed.
	flat := filepath.Join(data, "fonts")
	if err := os.MkdirAll(flat, 0755); err != nil {
		t.Fatal(err)
	}
	for _, face := range iosevkaTerm.faces {
		if err := os.WriteFile(filepath.Join(flat, face), []byte("ttf"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if present, _ := fontInstalled(iosevkaTerm); present {
		t.Error("unmanaged flat copy must not count as installed")
	}

	// Properly installed at the pinned tag.
	dir := fakeInstall(t, data, nerdFontsTag)
	present, tag := fontInstalled(iosevkaTerm)
	if !present || tag != nerdFontsTag {
		t.Errorf("fontInstalled = (%v, %q), want (true, %q)", present, tag, nerdFontsTag)
	}

	// A stale tag is present-but-wrong, so a bump is actionable.
	if err := os.WriteFile(filepath.Join(dir, tagStamp), []byte("v3.0.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if present, tag := fontInstalled(iosevkaTerm); !present || tag == nerdFontsTag {
		t.Errorf("stale stamp: got (%v, %q), want present with a different tag", present, tag)
	}
	if len(fontNotes()) == 0 {
		t.Error("a stale tag must be reported by fontNotes")
	}

	// One missing face is not an install — Ghostty would synthesize the rest.
	if err := os.Remove(filepath.Join(dir, iosevkaTerm.faces[0])); err != nil {
		t.Fatal(err)
	}
	if present, _ := fontInstalled(iosevkaTerm); present {
		t.Error("missing face must not count as installed")
	}
}

// TestDoctorFontsCheck_PassesOnCorrectInstall is the regression guard for the
// migration defect: doctor kept checking for HackNerdFont-Regular.ttf, so a
// correctly migrated machine reported "fonts — not found" and was told to run
// `install all`, which could never fix it, while a machine still littered with
// stale Hack files passed.
//
// Deliberately driven through RunDoctorChecks rather than checkFonts directly,
// so it compiles against the pre-fix module too — run there, it fails, which is
// the only thing that makes it a regression test rather than a restatement.
func TestDoctorFontsCheck_PassesOnCorrectInstall(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, ".local", "share")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)

	fontsCheck := func() (DoctorResult, bool) {
		for _, r := range RunDoctorChecks() {
			if r.Name == "fonts" {
				return r, true
			}
		}
		return DoctorResult{}, false
	}

	if r, ok := fontsCheck(); !ok {
		t.Fatal("no \"fonts\" check in RunDoctorChecks")
	} else if r.OK {
		t.Fatal("empty home: fonts check should fail")
	}

	// Reproduce exactly what a correct install leaves behind: the vendored
	// floor linked, and the downloaded family present at the pinned tag.
	fakeInstall(t, data, nerdFontsTag)
	for _, l := range (FontsModule{}).Links() {
		if err := os.MkdirAll(filepath.Dir(l.Dst), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(l.Src, l.Dst); err != nil {
			t.Fatal(err)
		}
	}

	r, ok := fontsCheck()
	if !ok {
		t.Fatal("no \"fonts\" check in RunDoctorChecks")
	}
	if !r.OK {
		t.Errorf("correctly installed fonts: doctor reported %q, want the check to pass", r.Detail)
	}
}

// TestLegacyArtifacts_NarrowAndContentChecked covers the migration cleanup: the
// old module's Hack faces go, and a displaced .bak goes only when it duplicates
// the vendored font that replaced it. The .bak matters because fontconfig
// identifies files by content, not extension — one left in a font directory is
// a permanently registered duplicate face, not an inert backup.
func TestLegacyArtifacts_NarrowAndContentChecked(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, ".local", "share")
	fontDir := filepath.Join(data, "fonts")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	if err := os.MkdirAll(fontDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Point the source root at a fixture rather than the real repo, so the test
	// is hermetic and can't silently t.Skip when the vendored font isn't where
	// this process happens to resolve it.
	srcRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcRoot, "config", "fonts"), 0755); err != nil {
		t.Fatal(err)
	}
	// DotfilesDir caches its first resolution process-wide, so $DOTFILES alone
	// is not enough once another test has resolved it.
	t.Setenv("DOTFILES", srcRoot)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)

	m := FontsModule{}
	link := m.Links()[0]
	if err := os.WriteFile(link.Src, []byte("vendored-meslo-bytes"), 0644); err != nil {
		t.Fatalf("seed vendored font at %s: %v", link.Src, err)
	}

	write := func(name, content string) string {
		p := filepath.Join(fontDir, name)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Hack faces the old module installed, including the banned builds its
	// `unzip -qo Hack.zip` path dumped flat into this directory.
	write("HackNerdFont-Regular.ttf", "hack")
	write("HackNerdFontMono-Bold.ttf", "hack")
	write("HackNerdFontPropo-Italic.ttf", "hack")
	// Generic unzip leftovers — too commonly named to claim. Must survive.
	keepReadme := write("README.md", "readme")

	vendored, err := os.ReadFile(link.Src)
	if err != nil {
		t.Fatalf("vendored font unreadable: %v", err)
	}
	// A .bak byte-identical to the vendored font: ours, and a duplicate.
	dupBak := link.Dst + ".bak"
	if err := os.WriteFile(dupBak, vendored, 0644); err != nil {
		t.Fatal(err)
	}

	got := m.legacyArtifacts()
	want := map[string]bool{
		filepath.Join(fontDir, "HackNerdFont-Regular.ttf"):     true,
		filepath.Join(fontDir, "HackNerdFontMono-Bold.ttf"):    true,
		filepath.Join(fontDir, "HackNerdFontPropo-Italic.ttf"): true,
		dupBak: true,
	}
	if len(got) != len(want) {
		t.Fatalf("legacyArtifacts = %v, want %d entries", got, len(want))
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("legacyArtifacts included unexpected %q", p)
		}
	}

	if !m.cleanLegacyArtifacts() {
		t.Error("cleanLegacyArtifacts should report a change")
	}
	for p := range want {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", p)
		}
	}
	if _, err := os.Stat(keepReadme); err != nil {
		t.Errorf("README.md must be left alone, got %v", err)
	}

	// A .bak that does NOT match the vendored font is the user's: never deleted.
	if err := os.WriteFile(dupBak, []byte("something else entirely"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := m.legacyArtifacts(); len(got) != 0 {
		t.Errorf("non-matching .bak must not be claimed, got %v", got)
	}
	m.cleanLegacyArtifacts()
	if _, err := os.Stat(dupBak); err != nil {
		t.Errorf("non-matching .bak must survive, got %v", err)
	}
}
