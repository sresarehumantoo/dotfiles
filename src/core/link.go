package core

import (
	"os"
	"path/filepath"
)

// LinkFile creates a symlink at dst pointing to src.
// - Existing correct symlink -> no-op
// - Existing wrong symlink -> repoint
// - Existing regular file -> backup to .bak, then link
// - Missing parent dirs -> create them
func LinkFile(src, dst string) error {
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
		Warn("repointing symlink: %s", dst)
		os.Remove(dst)
	} else if err == nil {
		// Regular file or directory exists — back it up
		bak := dst + ".bak"
		Warn("backing up existing: %s -> %s", dst, bak)
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
	return os.MkdirAll(dir, 0755)
}

// ConfigPath returns the absolute path to a file under config/.
func ConfigPath(parts ...string) string {
	args := append([]string{ConfigDir()}, parts...)
	return filepath.Join(args...)
}

// HomeTarget returns a path under $HOME.
func HomeTarget(parts ...string) string {
	home, _ := os.UserHomeDir()
	args := append([]string{home}, parts...)
	return filepath.Join(args...)
}

// XDGTarget returns a path under $XDG_CONFIG_HOME.
func XDGTarget(parts ...string) string {
	args := append([]string{XDGConfigHome()}, parts...)
	return filepath.Join(args...)
}
