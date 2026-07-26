package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DryRun prevents filesystem modifications when true.
var DryRun bool

// LinkFile creates a symlink at dst pointing to src.
// - Existing correct symlink -> no-op
// - Existing wrong symlink -> repoint
// - Existing regular file -> backup via backup manager, then .bak fallback, then link
// - Missing parent dirs -> create them
func LinkFile(src, dst string) error {
	if err := checkTarget(dst); err != nil {
		return err
	}
	if DryRun {
		Info("would link: %s -> %s", dst, src)
		return nil
	}

	Debug("link: %s -> %s", src, dst)

	if err := BackupFile(dst); err != nil {
		Warn("backup failed for %s: %v", dst, err)
	}

	// Ensure parent directory exists
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	// Check if dst is already a symlink
	fi, err := os.Lstat(dst)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		current, err := os.Readlink(dst)
		if err == nil && current == src {
			Ok("already linked: %s", dst)
			return nil
		}
		Notice("repointing symlink: %s", dst)
		os.Remove(dst)
	} else if err == nil {
		// Regular file or directory exists — back it up
		bak := dst + ".bak"
		Notice("backing up existing: %s -> %s", dst, bak)
		if err := os.Rename(dst, bak); err != nil {
			return err
		}
	}

	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	Ok("linked: %s -> %s", dst, src)
	return nil
}

// CheckLink checks if dst is a symlink pointing to src.
// Returns: "ok", "wrong", "missing", or "file" (regular file exists).
func CheckLink(src, dst string) string {
	fi, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		return "missing"
	}
	if err != nil {
		return "missing"
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		current, err := os.Readlink(dst)
		if err == nil && current == src {
			return "ok"
		}
		return "wrong"
	}
	return "file"
}

// EnsureDir creates a directory (and parents) if it doesn't exist.
func EnsureDir(dir string) error {
	if err := checkTarget(dir); err != nil {
		return err
	}
	if DryRun {
		Debug("would create dir: %s", dir)
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// UnlinkFile removes the symlink at dst only if it points to src.
// Returns nil if dst is missing, not a symlink, or points elsewhere (with warning).
func UnlinkFile(src, dst string) error {
	if err := checkTarget(dst); err != nil {
		return err
	}

	fi, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		Warn("not a symlink, skipping: %s", dst)
		return nil
	}

	current, err := os.Readlink(dst)
	if err != nil {
		return fmt.Errorf("readlink %s: %w", dst, err)
	}

	if current != src {
		Warn("symlink points elsewhere (%s), skipping: %s", current, dst)
		return nil
	}

	if DryRun {
		Info("would unlink: %s", dst)
		return nil
	}

	if err := os.Remove(dst); err != nil {
		return fmt.Errorf("remove %s: %w", dst, err)
	}
	Ok("unlinked: %s", dst)
	return nil
}

// ConfigPath returns the absolute path to a file under config/.
func ConfigPath(parts ...string) string {
	args := append([]string{ConfigDir()}, parts...)
	return filepath.Join(args...)
}

// HomeTarget returns a path under $HOME.
//
// Returns "" when the home directory can't be resolved, rather than the
// relative path filepath.Join would produce from an empty home — that would
// silently retarget every managed file at the current working directory.
// checkTarget rejects the empty result at each operation.
func HomeTarget(parts ...string) string {
	home, err := HomeDir()
	if err != nil {
		warnNoHome(err)
		return ""
	}
	return filepath.Join(append([]string{home}, parts...)...)
}

// checkTarget rejects paths we must never act on. A path assembled from an
// unresolved $HOME comes out relative, so refusing non-absolute targets stops
// a link, unlink, or delete from landing in the current working directory.
func checkTarget(path string) error {
	if path == "" {
		return errors.New("empty target path (home directory unresolved?)")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("refusing to operate on non-absolute path %q", path)
	}
	return nil
}

// RemoveManagedDir removes a directory dfinstall owns, refusing anything that
// isn't an absolute path.
func RemoveManagedDir(path string) error {
	if err := checkTarget(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// XDGTarget returns a path under $XDG_CONFIG_HOME. Empty when the config home
// can't be resolved, for the same reason as HomeTarget.
func XDGTarget(parts ...string) string {
	base := XDGConfigHome()
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, parts...)...)
}
