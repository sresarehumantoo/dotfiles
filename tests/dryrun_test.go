package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestLinkFile_DryRun(t *testing.T) {
	core.DryRun = true
	defer func() { core.DryRun = false }()

	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	os.WriteFile(src, []byte("hello"), 0644)

	dst := filepath.Join(tmp, "dest.txt")

	if err := core.LinkFile(src, dst); err != nil {
		t.Fatalf("LinkFile dry-run failed: %v", err)
	}

	// No symlink should be created
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Error("expected no symlink in dry-run mode")
	}
}

func TestEnsureDir_DryRun(t *testing.T) {
	core.DryRun = true
	defer func() { core.DryRun = false }()

	tmp := t.TempDir()
	dir := filepath.Join(tmp, "a", "b", "c")

	if err := core.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir dry-run failed: %v", err)
	}

	// Directory should not be created
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected no directory in dry-run mode")
	}
}

func TestUnlinkFile_DryRun(t *testing.T) {
	core.DryRun = true
	defer func() { core.DryRun = false }()

	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	os.WriteFile(src, []byte("hello"), 0644)

	dst := filepath.Join(tmp, "link")
	os.Symlink(src, dst)

	if err := core.UnlinkFile(src, dst); err != nil {
		t.Fatalf("UnlinkFile dry-run failed: %v", err)
	}

	// Symlink should still exist
	if _, err := os.Lstat(dst); err != nil {
		t.Error("expected symlink to still exist in dry-run mode")
	}
}
