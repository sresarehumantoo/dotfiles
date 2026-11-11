package core

import (
	"os"
	"path/filepath"
	"testing"
)

// A Homebrew bin/ entry is a symlink into the keg, and the content sits in
// the keg's share/, not beside the symlink.
func TestContentDirBeside_ResolvesKegSymlink(t *testing.T) {
	root := t.TempDir()
	keg := filepath.Join(root, "Cellar", "dfinstall", "9.0.0")
	share := filepath.Join(keg, "share", AppDirName)
	for _, d := range []string{filepath.Join(keg, "bin"), share, filepath.Join(root, "bin")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(keg, "bin", "dfinstall")
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "bin", "dfinstall")
	if err := os.Symlink(exe, link); err != nil {
		t.Fatal(err)
	}

	if got := contentDirBeside(link); got != "" {
		t.Errorf("found content %q before a stamp exists", got)
	}
	if err := os.WriteFile(filepath.Join(share, ContentStampName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := contentDirBeside(link); got != share {
		t.Errorf("contentDirBeside = %q, want the keg's %q", got, share)
	}
}
