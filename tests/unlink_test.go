package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestUnlinkFile_CorrectSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	os.WriteFile(src, []byte("hello"), 0644)

	dst := filepath.Join(tmp, "link")
	os.Symlink(src, dst)

	if err := core.UnlinkFile(src, dst); err != nil {
		t.Fatalf("UnlinkFile failed: %v", err)
	}

	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Error("expected symlink to be removed")
	}
}

func TestUnlinkFile_WrongSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	other := filepath.Join(tmp, "other.txt")
	os.WriteFile(src, []byte("hello"), 0644)
	os.WriteFile(other, []byte("other"), 0644)

	dst := filepath.Join(tmp, "link")
	os.Symlink(other, dst)

	if err := core.UnlinkFile(src, dst); err != nil {
		t.Fatalf("UnlinkFile failed: %v", err)
	}

	// Symlink should still exist (wrong target, not removed)
	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatal("expected symlink to still exist")
	}
	if target != other {
		t.Errorf("symlink target = %q, want %q", target, other)
	}
}

func TestUnlinkFile_Missing(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	dst := filepath.Join(tmp, "nonexistent")

	if err := core.UnlinkFile(src, dst); err != nil {
		t.Fatalf("UnlinkFile on missing path should return nil, got: %v", err)
	}
}

func TestUnlinkFile_RegularFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	os.WriteFile(src, []byte("hello"), 0644)

	dst := filepath.Join(tmp, "regular.txt")
	os.WriteFile(dst, []byte("data"), 0644)

	if err := core.UnlinkFile(src, dst); err != nil {
		t.Fatalf("UnlinkFile failed: %v", err)
	}

	// Regular file should still exist
	if _, err := os.Stat(dst); err != nil {
		t.Error("expected regular file to still exist")
	}
}
