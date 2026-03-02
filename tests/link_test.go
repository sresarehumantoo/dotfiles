package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestLinkFile_CreateNew(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "source.txt")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	dstFile := filepath.Join(tmp, "dest.txt")

	if err := core.LinkFile(srcFile, dstFile); err != nil {
		t.Fatalf("LinkFile failed: %v", err)
	}

	target, err := os.Readlink(dstFile)
	if err != nil {
		t.Fatalf("expected symlink, got error: %v", err)
	}
	if target != srcFile {
		t.Errorf("symlink target = %q, want %q", target, srcFile)
	}
}

func TestLinkFile_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "source.txt")
	os.WriteFile(srcFile, []byte("hello"), 0644)
	dstFile := filepath.Join(tmp, "dest.txt")

	// Link twice — should be a no-op the second time
	core.LinkFile(srcFile, dstFile)
	if err := core.LinkFile(srcFile, dstFile); err != nil {
		t.Fatalf("second LinkFile failed: %v", err)
	}

	target, _ := os.Readlink(dstFile)
	if target != srcFile {
		t.Errorf("symlink target = %q, want %q", target, srcFile)
	}
}

func TestLinkFile_Repoint(t *testing.T) {
	tmp := t.TempDir()
	oldSrc := filepath.Join(tmp, "old.txt")
	newSrc := filepath.Join(tmp, "new.txt")
	os.WriteFile(oldSrc, []byte("old"), 0644)
	os.WriteFile(newSrc, []byte("new"), 0644)

	dstFile := filepath.Join(tmp, "dest.txt")
	os.Symlink(oldSrc, dstFile)

	if err := core.LinkFile(newSrc, dstFile); err != nil {
		t.Fatalf("LinkFile repoint failed: %v", err)
	}

	target, _ := os.Readlink(dstFile)
	if target != newSrc {
		t.Errorf("symlink target = %q, want %q", target, newSrc)
	}
}

func TestLinkFile_BackupExisting(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "source.txt")
	os.WriteFile(srcFile, []byte("source"), 0644)

	dstFile := filepath.Join(tmp, "dest.txt")
	os.WriteFile(dstFile, []byte("existing"), 0644)

	if err := core.LinkFile(srcFile, dstFile); err != nil {
		t.Fatalf("LinkFile backup failed: %v", err)
	}

	// Check backup exists
	bakData, err := os.ReadFile(dstFile + ".bak")
	if err != nil {
		t.Fatal("expected .bak file")
	}
	if string(bakData) != "existing" {
		t.Errorf("backup content = %q, want %q", string(bakData), "existing")
	}

	// Check symlink is correct
	target, _ := os.Readlink(dstFile)
	if target != srcFile {
		t.Errorf("symlink target = %q, want %q", target, srcFile)
	}
}

func TestLinkFile_NestedDirs(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "source.txt")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	// Destination in nested non-existent directory
	dstFile := filepath.Join(tmp, "a", "b", "c", "dest.txt")

	if err := core.LinkFile(srcFile, dstFile); err != nil {
		t.Fatalf("LinkFile nested failed: %v", err)
	}

	target, _ := os.Readlink(dstFile)
	if target != srcFile {
		t.Errorf("symlink target = %q, want %q", target, srcFile)
	}
}

func TestCheckLink(t *testing.T) {
	tmp := t.TempDir()
	srcFile := filepath.Join(tmp, "source.txt")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	dstFile := filepath.Join(tmp, "dest.txt")

	// Missing
	if got := core.CheckLink(srcFile, dstFile); got != "missing" {
		t.Errorf("CheckLink missing = %q, want %q", got, "missing")
	}

	// Correct symlink
	os.Symlink(srcFile, dstFile)
	if got := core.CheckLink(srcFile, dstFile); got != "ok" {
		t.Errorf("CheckLink ok = %q, want %q", got, "ok")
	}

	// Wrong symlink
	os.Remove(dstFile)
	otherFile := filepath.Join(tmp, "other.txt")
	os.WriteFile(otherFile, []byte("other"), 0644)
	os.Symlink(otherFile, dstFile)
	if got := core.CheckLink(srcFile, dstFile); got != "wrong" {
		t.Errorf("CheckLink wrong = %q, want %q", got, "wrong")
	}

	// Regular file
	os.Remove(dstFile)
	os.WriteFile(dstFile, []byte("regular"), 0644)
	if got := core.CheckLink(srcFile, dstFile); got != "file" {
		t.Errorf("CheckLink file = %q, want %q", got, "file")
	}
}

func TestEnsureDir(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "a", "b", "c")

	if err := core.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !fi.IsDir() {
		t.Error("expected directory")
	}

	// Idempotent
	if err := core.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir idempotent failed: %v", err)
	}
}
