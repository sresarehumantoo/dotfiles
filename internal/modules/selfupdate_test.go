package modules

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type tarEntry struct {
	name, body string
	typ        byte
}

func makeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Mode: 0o755, Size: int64(len(e.body)), Typeflag: typ}
		if typ == tar.TypeSymlink {
			hdr.Size, hdr.Linkname = 0, "/etc/passwd"
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
	return buf.Bytes()
}

func TestReleaseArchiveName(t *testing.T) {
	if got, want := releaseArchiveName("v2.0.0", "linux", "amd64"), "dfinstall_2.0.0_linux_amd64.tar.gz"; got != want {
		t.Errorf("releaseArchiveName = %q, want %q (GoReleaser's default template)", got, want)
	}
}

func TestNewerRelease(t *testing.T) {
	cases := []struct {
		tag, current string
		want         bool
	}{
		{"v1.10.0", "v1.9.2", true},
		{"v1.10.0", "v1.10.0", false},
		{"v1.9.2", "v1.10.0", false},
		{"v2.0.0", "1.99.99", true},
		{"v1.10.0", "v1.2.0-216-g28b8943-dirty", true},
		{"v1.10.0", "dev", true},
		{"", "v1.0.0", false},
		{"nightly", "v1.0.0", false},
	}
	for _, c := range cases {
		if got := newerRelease(c.tag, c.current); got != c.want {
			t.Errorf("newerRelease(%q, %q) = %v, want %v", c.tag, c.current, got, c.want)
		}
	}
}

// Only the named binaries are written, by base name, whatever path or type
// the archive gives them.
func TestExtractBinaries_WritesOnlyKnownBinaries(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	data := makeTarGz(t, []tarEntry{
		{name: "README.md", body: "readme"},
		{name: "dfinstall-mcp", typ: tar.TypeSymlink},
		{name: "../../outside/dfinstall", body: "cli"},
		{name: "nested/dfinstall-mcp", body: "mcp"},
		{name: "dfinstall", body: "second copy"},
	})
	if err := os.WriteFile(archive, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := extractBinaries(archive, out)
	if err != nil {
		t.Fatalf("extractBinaries: %v", err)
	}
	for name, body := range map[string]string{"dfinstall": "cli", "dfinstall-mcp": "mcp"} {
		b, err := os.ReadFile(got[name])
		if err != nil || string(b) != body {
			t.Errorf("%s = %q (err=%v), want %q", name, b, err, body)
		}
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 2 {
		t.Errorf("wrote %d files, want 2", len(entries))
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "outside")); err == nil {
		t.Error("an archive path escaped the target directory")
	}
}

// serveRelease serves archive at /<name> and returns the matching asset.
func serveRelease(t *testing.T, name string, archive []byte, digest string) ghAsset {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+name {
			http.NotFound(w, r)
			return
		}
		w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	if digest == "" {
		sum := sha256.Sum256(archive)
		digest = "sha256:" + hex.EncodeToString(sum[:])
	}
	return ghAsset{Name: name, URL: srv.URL + "/" + name, Digest: digest}
}

func noGitHubMirror(t *testing.T) {
	githubMirrorOnce = sync.Once{}
	githubMirrorOnce.Do(func() { githubMirrorSeen = githubMirror{} })
	t.Cleanup(func() { githubMirrorOnce = sync.Once{}; githubMirrorSeen = githubMirror{} })
}

func installedBin(t *testing.T, files map[string]string) (dir, exe string) {
	t.Helper()
	dir = t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir, filepath.Join(dir, "dfinstall")
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(b)
}

func TestInstallRelease_ReplacesInstalledBinaries(t *testing.T) {
	noGitHubMirror(t)
	name := releaseArchiveName("v9.0.0", runtime.GOOS, runtime.GOARCH)
	asset := serveRelease(t, name, makeTarGz(t, []tarEntry{
		{name: "dfinstall", body: "new cli"},
		{name: "dfinstall-mcp", body: "new mcp"},
	}), "")
	dir, exe := installedBin(t, map[string]string{"dfinstall": "old cli", "dfinstall-mcp": "old mcp"})

	if err := installRelease(context.Background(), "v9.0.0", []ghAsset{{Name: "other.tar.gz"}, asset}, exe); err != nil {
		t.Fatalf("installRelease: %v", err)
	}
	if got := readFile(t, exe); got != "new cli" {
		t.Errorf("dfinstall = %q, want the new release", got)
	}
	if got := readFile(t, filepath.Join(dir, "dfinstall-mcp")); got != "new mcp" {
		t.Errorf("dfinstall-mcp = %q, want the new release", got)
	}
	if fi, err := os.Stat(exe); err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("updated binary is not executable: %v %v", fi.Mode(), err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".dfinstall-update-") {
			t.Errorf("scratch dir left behind: %s", e.Name())
		}
	}
}

// The MCP server is only replaced where it was installed.
func TestInstallRelease_LeavesMissingSiblingAbsent(t *testing.T) {
	noGitHubMirror(t)
	name := releaseArchiveName("v9.0.0", runtime.GOOS, runtime.GOARCH)
	asset := serveRelease(t, name, makeTarGz(t, []tarEntry{
		{name: "dfinstall", body: "new cli"},
		{name: "dfinstall-mcp", body: "new mcp"},
	}), "")
	dir, exe := installedBin(t, map[string]string{"dfinstall": "old cli"})

	if err := installRelease(context.Background(), "v9.0.0", []ghAsset{asset}, exe); err != nil {
		t.Fatalf("installRelease: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dfinstall-mcp")); err == nil {
		t.Error("dfinstall-mcp installed where it was not before")
	}
}

func TestInstallRelease_DigestMismatchKeepsOldBinary(t *testing.T) {
	noGitHubMirror(t)
	name := releaseArchiveName("v9.0.0", runtime.GOOS, runtime.GOARCH)
	asset := serveRelease(t, name, makeTarGz(t, []tarEntry{{name: "dfinstall", body: "tampered"}}),
		"sha256:"+strings.Repeat("0", 64))
	_, exe := installedBin(t, map[string]string{"dfinstall": "old cli"})

	if err := installRelease(context.Background(), "v9.0.0", []ghAsset{asset}, exe); err == nil {
		t.Fatal("installRelease accepted an archive that does not match its digest")
	}
	if got := readFile(t, exe); got != "old cli" {
		t.Errorf("dfinstall = %q after a failed update, want the old binary", got)
	}
}

func TestInstallRelease_NoArchiveForPlatform(t *testing.T) {
	_, exe := installedBin(t, map[string]string{"dfinstall": "old cli"})
	err := installRelease(context.Background(), "v9.0.0", []ghAsset{{Name: "dfinstall_9.0.0_plan9_mips.tar.gz"}}, exe)
	if err == nil || !strings.Contains(err.Error(), "has no") {
		t.Errorf("installRelease = %v, want a missing-archive error", err)
	}
	if got := readFile(t, exe); got != "old cli" {
		t.Errorf("dfinstall = %q, want it untouched", got)
	}
}

// An archive without the running binary must not leave it half-replaced.
func TestInstallRelease_ArchiveMissingRunningBinary(t *testing.T) {
	noGitHubMirror(t)
	name := releaseArchiveName("v9.0.0", runtime.GOOS, runtime.GOARCH)
	asset := serveRelease(t, name, makeTarGz(t, []tarEntry{{name: "dfinstall-mcp", body: "new mcp"}}), "")
	dir, exe := installedBin(t, map[string]string{"dfinstall": "old cli", "dfinstall-mcp": "old mcp"})

	if err := installRelease(context.Background(), "v9.0.0", []ghAsset{asset}, exe); err == nil {
		t.Fatal("installRelease succeeded without the running binary in the archive")
	}
	if got := readFile(t, filepath.Join(dir, "dfinstall-mcp")); got != "old mcp" {
		t.Errorf("dfinstall-mcp = %q, want it untouched", got)
	}
}
