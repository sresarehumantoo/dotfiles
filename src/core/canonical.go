package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The "canonical dotfiles dir" is a machine-global pointer recording which
// clone of the repo is authoritative on this host. It lives outside any clone
// (under the dfinstall config dir) so it survives switching between clones, and
// it makes DotfilesDir() resolve to a single checkout regardless of which
// clone's binary is invoked — which is what stops symlinks drifting across
// multiple clones. See DotfilesDir in env.go for the resolution order.

// CanonicalPointerPath returns the machine-global file that records the
// authoritative dotfiles clone for this host.
func CanonicalPointerPath() string {
	return filepath.Join(XDGConfigHome(), "dfinstall", "dotfiles-dir")
}

// looksLikeDotfilesRepo reports whether dir is plausibly a dotfiles checkout:
// it must exist and contain a config/ subdirectory (the source of every managed
// symlink). Used to ignore a stale pointer to a moved/deleted clone.
func looksLikeDotfilesRepo(dir string) bool {
	if dir == "" {
		return false
	}
	fi, err := os.Stat(filepath.Join(dir, "config"))
	return err == nil && fi.IsDir()
}

// ReadCanonicalDir returns the recorded canonical dotfiles dir, or "" if it is
// unset, unreadable, or no longer a valid checkout. The validity check makes
// the pointer self-healing: a stale path (clone deleted or renamed) is ignored
// so DotfilesDir() falls through to the invoking clone, and the next
// `install all` rewrites it.
func ReadCanonicalDir() string {
	data, err := os.ReadFile(CanonicalPointerPath())
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(data))
	if !looksLikeDotfilesRepo(dir) {
		return ""
	}
	return dir
}

// WriteCanonicalDir records dir as the authoritative dotfiles clone for this
// machine, writing atomically so a crash can't leave a half-written pointer.
func WriteCanonicalDir(dir string) error {
	path := CanonicalPointerPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dfinstall config dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(dir+"\n"), 0o644); err != nil {
		return fmt.Errorf("write canonical pointer: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename canonical pointer: %w", err)
	}
	return nil
}
