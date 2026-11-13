package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
)

func TestContentArchiveName(t *testing.T) {
	// The name v2.5.0 actually published.
	if got, want := contentArchiveName("v2.5.0"), "dfinstall-content_2.5.0.tar.gz"; got != want {
		t.Errorf("contentArchiveName = %q, want %q (the content archive's name_template)", got, want)
	}
}

// releaseContent builds a content archive for tag the way the release does.
func releaseContent(t *testing.T, tag string) []byte {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "config", "shell", "zshrc")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("# "+tag+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := core.GenerateContentStamp(root, tag)
	if err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(root, core.ContentStampName)
	if err := core.WriteContentStamp(stamp, st); err != nil {
		t.Fatal(err)
	}
	top := "dfinstall-content_" + strings.TrimPrefix(tag, "v") + "/"
	return makeTarGz(t, []tarEntry{
		{name: top + core.ContentStampName, body: readFile(t, stamp)},
		{name: top + "config/shell/zshrc", body: readFile(t, p)},
	})
}

func contentSession(t *testing.T) *core.InstallSession {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sess.Finish)
	t.Cleanup(func() {
		filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				os.Chmod(p, 0o755)
			}
			return nil
		})
	})
	return sess
}

func TestInstallReleaseContent(t *testing.T) {
	noGitHubMirror(t)
	sess := contentSession(t)
	name := contentArchiveName("v9.0.0")
	asset := serveRelease(t, name, releaseContent(t, "v9.0.0"), "")

	if err := installReleaseContent(context.Background(), sess, "v9.0.0", []ghAsset{asset}); err != nil {
		t.Fatalf("installReleaseContent: %v", err)
	}
	if got := readFile(t, filepath.Join(core.ContentCurrentDir(), "config/shell/zshrc")); got != "# v9.0.0\n" {
		t.Errorf("zshrc = %q", got)
	}
	if !core.ContentUpToDate("v9.0.0") {
		t.Error("ContentUpToDate false after install")
	}
}

func TestInstallReleaseContent_RefusesBadDigest(t *testing.T) {
	noGitHubMirror(t)
	sess := contentSession(t)
	name := contentArchiveName("v9.0.0")
	asset := serveRelease(t, name, releaseContent(t, "v9.0.0"), "sha256:"+strings.Repeat("0", 64))

	err := installReleaseContent(context.Background(), sess, "v9.0.0", []ghAsset{asset})
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("err = %v, want a digest mismatch", err)
	}
	if _, err := os.Lstat(core.ContentStoreDir()); err == nil {
		t.Error("store created from an unverified download")
	}
}

func TestInstallReleaseContent_MissingAsset(t *testing.T) {
	sess := contentSession(t)
	err := installReleaseContent(context.Background(), sess, "v9.0.0", []ghAsset{{Name: "dfinstall_9.0.0_linux_amd64.tar.gz"}})
	if err == nil || !strings.Contains(err.Error(), contentArchiveName("v9.0.0")) {
		t.Fatalf("err = %v, want one naming the missing archive", err)
	}
}
