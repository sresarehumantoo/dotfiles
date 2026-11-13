package tests

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// contentHome gives the test its own HOME, so each starts with no store, and
// an install session to sync inside.
func contentHome(t *testing.T) (*core.InstallSession, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	t.Cleanup(sess.Finish)
	t.Cleanup(func() { removeSealed(home) })
	return sess, home
}

// removeSealed restores write on the read-only directories of a content
// store, so TempDir's cleanup can remove them.
func removeSealed(root string) {
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0o755)
		}
		return nil
	})
}

// contentFixture writes a stamped content tree for version and returns its
// root, the way a package's share/dfinstall looks.
func contentFixture(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string, mode os.FileMode) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("config/shell/zshrc", "# zshrc "+version+"\n", 0o644)
	write("config/devtools/tool", "#!/bin/sh\necho "+version+"\n", 0o755)
	write("README", "not content\n", 0o644)
	st, err := core.GenerateContentStamp(root, version)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.WriteContentStamp(filepath.Join(root, core.ContentStampName), st); err != nil {
		t.Fatal(err)
	}
	return root
}

type tarEntry struct {
	name     string
	body     string
	typ      byte
	mode     int64
	linkname string
}

func writeTarball(t *testing.T, entries []tarEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ, mode := e.typ, e.mode
		if typ == 0 {
			typ = tar.TypeReg
		}
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typ, Mode: mode, Linkname: e.linkname}
		if typ == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "content.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// fixtureEntries lists a fixture tree the way the release archive packs it:
// every file under one top directory, no directory entries.
func fixtureEntries(t *testing.T, root, version string) []tarEntry {
	t.Helper()
	top := "dfinstall-content_" + strings.TrimPrefix(version, "v") + "/"
	var entries []tarEntry
	add := func(rel string) {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(filepath.Join(root, rel))
		entries = append(entries, tarEntry{name: top + filepath.ToSlash(rel), body: string(data), mode: int64(info.Mode().Perm())})
	}
	add(core.ContentStampName)
	filepath.WalkDir(filepath.Join(root, "config"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			add(rel)
		}
		return nil
	})
	return entries
}

// tarballFrom packs a fixture tree, with each extra entry replacing the
// fixture's entry of the same name or, failing that, appended.
func tarballFrom(t *testing.T, root, version string, extra ...tarEntry) string {
	t.Helper()
	entries := fixtureEntries(t, root, version)
	for _, e := range extra {
		i := slices.IndexFunc(entries, func(f tarEntry) bool { return f.name == e.name })
		if i < 0 {
			entries = append(entries, e)
		} else {
			entries[i] = e
		}
	}
	return writeTarball(t, entries)
}

// treeHash summarizes every path, type, mode and file body under root, so a
// test can assert that nothing at all changed.
func treeHash(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, _ := os.Lstat(p)
		line := p + " " + info.Mode().String()
		if info.Mode().IsRegular() {
			data, _ := os.ReadFile(p)
			sum := sha256.Sum256(data)
			line += " " + hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func readCurrent(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(core.ContentCurrentDir(), rel))
	if err != nil {
		t.Fatalf("read current %s: %v", rel, err)
	}
	return string(data)
}

func currentTarget(t *testing.T) string {
	t.Helper()
	target, err := os.Readlink(core.ContentCurrentDir())
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	return target
}

func storeEntries(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(core.ContentStoreDir())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestContentSync_FromDir(t *testing.T) {
	sess, _ := contentHome(t)
	src := contentFixture(t, "v1.0.0")
	before := treeHash(t, src)

	if err := sess.SyncContentDir(src, "v1.0.0"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := currentTarget(t); got != "v1.0.0" {
		t.Errorf("current -> %q, want the relative name v1.0.0", got)
	}
	if got := readCurrent(t, "config/shell/zshrc"); got != "# zshrc v1.0.0\n" {
		t.Errorf("zshrc = %q", got)
	}
	if _, err := os.Stat(filepath.Join(core.ContentCurrentDir(), "README")); err == nil {
		t.Error("a file outside config/ was copied into the store")
	}
	modes := map[string]os.FileMode{
		"config/shell/zshrc":   0o444,
		"config/devtools/tool": 0o555,
		"config/shell":         0o555,
		"config":               0o555,
		".":                    0o555,
		core.ContentStampName:  0o444,
	}
	for rel, want := range modes {
		info, err := os.Stat(filepath.Join(core.ContentCurrentDir(), rel))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", rel, got, want)
		}
	}
	if !core.ContentUpToDate("v1.0.0") {
		t.Error("ContentUpToDate false right after a sync")
	}
	if after := treeHash(t, src); after != before {
		t.Error("the source tree was modified")
	}

	// A second sync of the same version changes nothing.
	store := treeHash(t, core.ContentStoreDir())
	if err := sess.SyncContentDir(src, "v1.0.0"); err != nil {
		t.Fatalf("resync: %v", err)
	}
	if treeHash(t, core.ContentStoreDir()) != store {
		t.Error("a resync of the current version rewrote the store")
	}
}

func TestContentSync_FromTarball(t *testing.T) {
	sess, _ := contentHome(t)
	archive := tarballFrom(t, contentFixture(t, "v1.0.0"), "v1.0.0")
	if err := sess.SyncContentTarball(archive, "v1.0.0"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := readCurrent(t, "config/devtools/tool"); !strings.Contains(got, "v1.0.0") {
		t.Errorf("tool = %q", got)
	}
	if got := storeEntries(t); strings.Join(got, ",") != "current,v1.0.0" {
		t.Errorf("store holds %v, want only current and v1.0.0", got)
	}
}

func TestContentSync_RefusesVersionMismatch(t *testing.T) {
	sess, _ := contentHome(t)
	src := contentFixture(t, "v1.0.0")
	err := sess.SyncContentDir(src, "v2.0.0")
	if err == nil || !strings.Contains(err.Error(), "v1.0.0") {
		t.Fatalf("err = %v, want a refusal naming the stamped version", err)
	}
	if err := sess.SyncContentTarball(tarballFrom(t, src, "v1.0.0"), "v2.0.0"); err == nil {
		t.Fatal("tarball stamped v1.0.0 accepted as v2.0.0")
	}
	if _, err := os.Lstat(core.ContentCurrentDir()); err == nil {
		t.Error("current exists after a refused sync")
	}
}

func TestContentSync_RefusesTamperedFile(t *testing.T) {
	sess, _ := contentHome(t)
	src := contentFixture(t, "v1.0.0")
	os.WriteFile(filepath.Join(src, "config/shell/zshrc"), []byte("tampered\n"), 0o644)
	err := sess.SyncContentDir(src, "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "config/shell/zshrc") {
		t.Fatalf("err = %v, want a refusal naming the file", err)
	}
	if got := storeEntries(t); len(got) != 0 {
		t.Errorf("store holds %v after a refused sync", got)
	}
}

func TestContentSync_RefusesUnstampedAndMissingFiles(t *testing.T) {
	sess, _ := contentHome(t)
	extra := contentFixture(t, "v1.0.0")
	os.WriteFile(filepath.Join(extra, "config/shell/extra"), []byte("x"), 0o644)
	if err := sess.SyncContentDir(extra, "v1.0.0"); err == nil {
		t.Error("a file missing from the stamp was accepted")
	}
	missing := contentFixture(t, "v1.0.0")
	os.Remove(filepath.Join(missing, "config/devtools/tool"))
	if err := sess.SyncContentDir(missing, "v1.0.0"); err == nil {
		t.Error("a stamped file missing from the tree was accepted")
	}
}

func TestContentSync_RefusesSymlinkInDir(t *testing.T) {
	sess, _ := contentHome(t)
	src := contentFixture(t, "v1.0.0")
	os.Symlink("/etc/passwd", filepath.Join(src, "config/shell/link"))
	if err := sess.SyncContentDir(src, "v1.0.0"); err == nil {
		t.Fatal("a symlink in the source tree was accepted")
	}
}

func TestContentSync_RefusesHostileTarballs(t *testing.T) {
	src := contentFixture(t, "v1.0.0")
	top := "dfinstall-content_1.0.0/"
	cases := map[string][]tarEntry{
		"parent traversal": {{name: top + "../../evil", body: "x"}},
		"nested traversal": {{name: top + "config/../../evil", body: "x"}},
		"absolute path":    {{name: "/tmp/evil", body: "x"}},
		"symlink":          {{name: top + "config/link", typ: tar.TypeSymlink, linkname: "/etc/passwd"}},
		"hardlink":         {{name: top + "config/hard", typ: tar.TypeLink, linkname: top + "config/shell/zshrc"}},
		"setuid":           {{name: top + "config/devtools/tool", body: "#!/bin/sh\necho v1.0.0\n", mode: 0o4755}},
		"device":           {{name: top + "config/dev", typ: tar.TypeChar}},
		"second top dir":   {{name: "other/config/x", body: "x"}},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			sess, home := contentHome(t)
			archive := tarballFrom(t, src, "v1.0.0", extra...)
			if err := sess.SyncContentTarball(archive, "v1.0.0"); err == nil {
				t.Fatal("hostile archive accepted")
			}
			if _, err := os.Lstat(core.ContentCurrentDir()); err == nil {
				t.Error("current exists after a refused archive")
			}
			if got := storeEntries(t); len(got) != 0 {
				t.Errorf("store holds %v after a refused archive", got)
			}
			if _, err := os.Lstat(filepath.Join(home, ".local/share/evil")); err == nil {
				t.Error("traversal wrote outside the stage")
			}
		})
	}
	t.Run("duplicate entry", func(t *testing.T) {
		sess, _ := contentHome(t)
		entries := fixtureEntries(t, src, "v1.0.0")
		archive := writeTarball(t, append(entries, entries[len(entries)-1]))
		if err := sess.SyncContentTarball(archive, "v1.0.0"); err == nil {
			t.Fatal("archive with a duplicate entry accepted")
		}
	})
	t.Run("no top directory", func(t *testing.T) {
		sess, _ := contentHome(t)
		archive := writeTarball(t, []tarEntry{{name: core.ContentStampName, body: "{}"}})
		if err := sess.SyncContentTarball(archive, "v1.0.0"); err == nil {
			t.Fatal("archive without a top directory accepted")
		}
	})
}

func TestContentSync_PruneKeepsCurrentAndPrevious(t *testing.T) {
	sess, _ := contentHome(t)
	for _, v := range []string{"v1.0.0", "v2.0.0", "v3.0.0"} {
		if err := sess.SyncContentDir(contentFixture(t, v), v); err != nil {
			t.Fatalf("sync %s: %v", v, err)
		}
	}
	if got := strings.Join(storeEntries(t), ","); got != "current,v2.0.0,v3.0.0" {
		t.Errorf("store = %s, want current plus v3.0.0 and the v2.0.0 it replaced", got)
	}
	if got := currentTarget(t); got != "v3.0.0" {
		t.Errorf("current -> %s", got)
	}

	// Rolling back to an older package keeps the newer copy as the previous.
	if err := sess.SyncContentDir(contentFixture(t, "v2.0.0"), "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(storeEntries(t), ","); got != "current,v2.0.0,v3.0.0" {
		t.Errorf("after rollback store = %s", got)
	}
	if got := currentTarget(t); got != "v2.0.0" {
		t.Errorf("current -> %s after rollback", got)
	}
}

// An interrupted run that renamed its version into place but never swapped
// current leaves the old version live, and the next run finishes the job.
func TestContentSync_InterruptedBeforeSwap(t *testing.T) {
	sess, _ := contentHome(t)
	if err := sess.SyncContentDir(contentFixture(t, "v1.0.0"), "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := sess.SyncContentDir(contentFixture(t, "v2.0.0"), "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	cur := core.ContentCurrentDir()
	os.Remove(cur)
	os.Symlink("v1.0.0", cur)

	if got := readCurrent(t, "config/shell/zshrc"); got != "# zshrc v1.0.0\n" {
		t.Fatalf("current unusable between rename and swap: %q", got)
	}
	v2 := filepath.Join(core.ContentStoreDir(), "v2.0.0")
	// Interrupted between the rename and sealing the version directory.
	os.Chmod(v2, 0o755)
	before, _ := os.Stat(v2)
	if err := sess.SyncContentDir(contentFixture(t, "v2.0.0"), "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if got := currentTarget(t); got != "v2.0.0" {
		t.Errorf("current -> %s", got)
	}
	after, _ := os.Stat(v2)
	if !os.SameFile(before, after) {
		t.Error("the intact v2.0.0 copy was rebuilt instead of reused")
	}
	if got := after.Mode().Perm(); got != 0o555 {
		t.Errorf("reused v2.0.0 mode = %o, want it sealed", got)
	}
}

func TestContentSync_ClearsLeftoverStage(t *testing.T) {
	sess, _ := contentHome(t)
	stage := filepath.Join(core.ContentStoreDir(), "v1.0.0.tmp")
	os.MkdirAll(filepath.Join(stage, "config/junk"), 0o755)
	os.WriteFile(filepath.Join(stage, "config/junk/file"), []byte("x"), 0o444)
	os.Chmod(filepath.Join(stage, "config/junk"), 0o555)

	if err := sess.SyncContentDir(contentFixture(t, "v1.0.0"), "v1.0.0"); err != nil {
		t.Fatalf("sync over a leftover stage: %v", err)
	}
	if _, err := os.Lstat(stage); err == nil {
		t.Error("leftover stage still present")
	}
}

func TestContentSync_RepairsTamperedStore(t *testing.T) {
	sess, _ := contentHome(t)
	src := contentFixture(t, "v1.0.0")
	if err := sess.SyncContentDir(src, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(core.ContentCurrentDir(), "config/shell/zshrc")
	os.Chmod(live, 0o644)
	os.WriteFile(live, []byte("edited\n"), 0o644)
	if core.ContentUpToDate("v1.0.0") {
		t.Fatal("ContentUpToDate true for an edited store")
	}
	if err := sess.SyncContentDir(src, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if got := readCurrent(t, "config/shell/zshrc"); got != "# zshrc v1.0.0\n" {
		t.Errorf("zshrc = %q after repair", got)
	}
}

func TestContentSync_DryRunWritesNothing(t *testing.T) {
	sess, home := contentHome(t)
	src := contentFixture(t, "v1.0.0")
	core.DryRun = true
	t.Cleanup(func() { core.DryRun = false })

	before := treeHash(t, home)
	if err := sess.SyncContentDir(src, "v1.0.0"); err != nil {
		t.Fatalf("dry-run sync: %v", err)
	}
	if err := sess.SyncContentTarball(tarballFrom(t, src, "v1.0.0"), "v1.0.0"); err != nil {
		t.Fatalf("dry-run tarball sync: %v", err)
	}
	if after := treeHash(t, home); after != before {
		t.Errorf("dry run changed HOME:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestContentSync_RequiresOpenSession(t *testing.T) {
	sess, _ := contentHome(t)
	sess.Finish()
	if err := sess.SyncContentDir(contentFixture(t, "v1.0.0"), "v1.0.0"); err == nil {
		t.Fatal("sync ran on a finished session")
	}
	var none *core.InstallSession
	if err := none.SyncContentDir(contentFixture(t, "v1.0.0"), "v1.0.0"); err == nil {
		t.Fatal("sync ran with no session")
	}
}

func TestContentSync_RejectsStoreNames(t *testing.T) {
	for _, v := range []string{"", "current", "v1.tmp", "../x", "a/b", ".", "-v1"} {
		t.Run(v, func(t *testing.T) {
			sess, _ := contentHome(t)
			if err := sess.SyncContentTarball(filepath.Join(t.TempDir(), "none"), v); err == nil {
				t.Errorf("version %q accepted", v)
			}
		})
	}
}

// The canonical pointer accepts the store's current as a dotfiles dir and
// keeps the unresolved path, so links built from it survive a version swap.
func TestContentSync_CurrentIsACanonicalDir(t *testing.T) {
	sess, _ := contentHome(t)
	if err := sess.SyncContentDir(contentFixture(t, "v1.0.0"), "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	cur := core.ContentCurrentDir()
	if err := core.WriteCanonicalDir(cur); err != nil {
		t.Fatal(err)
	}
	if got := core.ReadCanonicalDir(); got != cur {
		t.Fatalf("ReadCanonicalDir = %q, want %q", got, cur)
	}
	if err := sess.SyncContentDir(contentFixture(t, "v2.0.0"), "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if got := core.ReadCanonicalDir(); got != cur {
		t.Errorf("after a swap ReadCanonicalDir = %q, want the unchanged %q", got, cur)
	}
	if got := readCurrent(t, "config/shell/zshrc"); got != "# zshrc v2.0.0\n" {
		t.Errorf("current serves %q after the swap", got)
	}
}
