package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Seeded files are the managed files that are copied into place instead of
// linked when links come from packaged content: files their program rewrites
// (htop's htoprc, `p10k configure`), and files with no include for local
// changes. The store is read-only, so a link would make every save fail.
//
// The copy is the user's. Each one's shipped digest is recorded, Debian ucf
// style, so an upgrade can tell an untouched copy, which it replaces, from an
// edited one, which it keeps while putting the new version under
// AppDataPath("pending") for the user to merge. Nothing is ever merged
// automatically.

// seedStatePath holds a map from each seeded destination to the digest of the
// version it was seeded from.
func seedStatePath() string { return AppDataPath("seeded.json") }

func loadSeeds() map[string]string {
	seeds := map[string]string{}
	if data, err := os.ReadFile(seedStatePath()); err == nil {
		if err := json.Unmarshal(data, &seeds); err != nil {
			Warn("ignoring unreadable %s: %v", seedStatePath(), err)
			return map[string]string{}
		}
	}
	return seeds
}

func saveSeeds(seeds map[string]string) error {
	p := seedStatePath()
	if err := checkTarget(p); err != nil {
		return err
	}
	data, err := json.MarshalIndent(seeds, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return writeAtomic(p, append(data, '\n'), 0o644)
}

// SeedPendingPath is where an upgrade leaves the new version of a seeded file
// the user changed. It mirrors dst's place under HOME, and never sits next to
// dst, where a config.d/* glob would load it.
func SeedPendingPath(dst string) string {
	home, err := HomeDir()
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(home, dst)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		rel = filepath.Base(dst)
	}
	return AppDataPath("pending", rel)
}

// SeedFile puts a copy of src at dst, or brings an earlier copy up to date:
//
//   - nothing at dst: copy src
//   - a link (from a clone install, or into the store): replace it with a copy
//     of the file it shows, which is the user's version
//   - a copy matching the recorded digest: untouched since seeding, so take src
//   - any other file: the user's; keep it and leave src as a pending update
func SeedFile(src, dst string) error {
	if err := checkTarget(dst); err != nil {
		return err
	}
	shipped, err := FileHash(src)
	if err != nil {
		Warn("not seeded %s: %s is not in the dotfiles content", dst, src)
		return nil
	}
	seeds := loadSeeds()
	recorded, known := seeds[dst]

	fi, err := os.Lstat(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if DryRun {
			Changed("would copy %s", dst)
			return nil
		}
		if err := copySeed(src, dst, src); err != nil {
			return err
		}
		Changed("copied %s", dst)

	case err != nil:
		return err

	case fi.Mode()&os.ModeSymlink != 0:
		shown := dst
		if _, err := os.Stat(dst); err != nil {
			shown = src // dangling: there is no user version to keep
		}
		if DryRun {
			Changed("would replace the link %s with a copy of its file", dst)
			return nil
		}
		if err := BackupFile(dst); err != nil {
			Warn("backup failed for %s: %v", dst, err)
		}
		if err := copySeed(shown, dst, src); err != nil {
			return err
		}
		Changed("replaced the link %s with a copy of its file", dst)
		if live, err := FileHash(dst); err == nil && live != shipped {
			if err := stagePending(src, dst, shipped); err != nil {
				return err
			}
		}

	case !fi.Mode().IsRegular():
		return fmt.Errorf("%s is not a file", dst)

	default:
		live, err := FileHash(dst)
		if err != nil {
			return err
		}
		switch {
		case live == shipped:
			removePending(dst)
			if recorded == shipped {
				return nil
			}
		case known && live == recorded:
			if DryRun {
				Changed("would update %s", dst)
				return nil
			}
			if err := BackupFile(dst); err != nil {
				Warn("backup failed for %s: %v", dst, err)
			}
			if err := copySeed(src, dst, src); err != nil {
				return err
			}
			removePending(dst)
			Changed("updated %s", dst)
		default:
			return keepEdited(src, dst, shipped, known, seeds)
		}
	}
	if DryRun {
		return nil
	}
	seeds[dst] = shipped
	return saveSeeds(seeds)
}

// keepEdited leaves the user's dst alone and stages src as its pending update.
// A file seeded for the first time over an existing one records src as its
// baseline, so it counts as edited from the start.
func keepEdited(src, dst, shipped string, known bool, seeds map[string]string) error {
	if err := stagePending(src, dst, shipped); err != nil || DryRun {
		return err
	}
	if !known {
		seeds[dst] = shipped
		return saveSeeds(seeds)
	}
	return nil
}

// stagePending puts src where the user can merge it into their dst, unless it
// is already there.
func stagePending(src, dst, shipped string) error {
	pending := SeedPendingPath(dst)
	if pending == "" {
		return fmt.Errorf("cannot place the pending update for %s", dst)
	}
	if h, err := FileHash(pending); err == nil && h == shipped {
		return nil
	}
	if DryRun {
		Changed("would keep your %s and put the shipped version at %s", dst, pending)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(pending), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := writeAtomic(pending, data, 0o644); err != nil {
		return err
	}
	Changed("kept your %s; the shipped version is at %s", dst, pending)
	return nil
}

// UnseedFile removes a seeded copy the user never changed. A changed one is
// left in place with a notice, and a leftover link is removed as UnlinkFile
// would.
func UnseedFile(src, dst string) error {
	if err := checkTarget(dst); err != nil {
		return err
	}
	seeds := loadSeeds()
	fi, err := os.Lstat(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case fi.Mode()&os.ModeSymlink != 0:
		return UnlinkFile(src, dst)
	case fi.Mode().IsRegular():
		live, err := FileHash(dst)
		if err != nil {
			return err
		}
		shipped, _ := FileHash(src)
		if live != seeds[dst] && live != shipped {
			Notice("kept %s, which you changed", dst)
			break
		}
		if DryRun {
			Info("would remove: %s", dst)
			return nil
		}
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("remove %s: %w", dst, err)
		}
		Ok("removed: %s", dst)
	}
	if DryRun {
		return nil
	}
	removePending(dst)
	if _, ok := seeds[dst]; ok {
		delete(seeds, dst)
		return saveSeeds(seeds)
	}
	return nil
}

// SeedToLink turns a seeded copy back into a link to src when nothing would be
// lost: it is untouched since seeding, or already matches src (a clone that
// carries the same edit). Reports whether it did; a copy that differs is left.
func SeedToLink(src, dst string) (bool, error) {
	fi, err := os.Lstat(dst)
	if err != nil || !fi.Mode().IsRegular() {
		return false, nil
	}
	seeds := loadSeeds()
	recorded, known := seeds[dst]
	if !known {
		return false, nil
	}
	live, err := FileHash(dst)
	if err != nil {
		return false, err
	}
	if target, _ := FileHash(src); live != recorded && live != target {
		Notice("kept %s, which differs from %s; merge it there, then run install", dst, src)
		return false, nil
	}
	if DryRun {
		Changed("would link %s again", dst)
		return true, nil
	}
	if err := BackupFile(dst); err != nil {
		Warn("backup failed for %s: %v", dst, err)
	}
	if err := os.Remove(dst); err != nil {
		return false, err
	}
	if err := LinkFile(src, dst); err != nil {
		return false, err
	}
	removePending(dst)
	delete(seeds, dst)
	return true, saveSeeds(seeds)
}

// CheckSeed reports a seeded destination in CheckLink's terms: "ok" for a
// file, "wrong" for a link still waiting to become a copy, "missing".
func CheckSeed(dst string) string {
	fi, err := os.Lstat(dst)
	switch {
	case err != nil:
		return "missing"
	case fi.Mode()&os.ModeSymlink != 0:
		return "wrong"
	case fi.Mode().IsRegular():
		return "ok"
	}
	return "file"
}

// SeedPending lists the seeded destinations with an update waiting to be
// merged, each with where the new version is.
func SeedPending() map[string]string {
	out := map[string]string{}
	for dst := range loadSeeds() {
		if p := SeedPendingPath(dst); p != "" && fileExists(p) {
			out[dst] = p
		}
	}
	return out
}

func removePending(dst string) {
	if DryRun {
		return
	}
	if p := SeedPendingPath(dst); p != "" {
		_ = os.Remove(p)
	}
}

// copySeed writes the contents of from to dst as a user-owned file, with the
// executable bits of modeOf (the shipped file) and owner write.
func copySeed(from, dst, modeOf string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(modeOf); err == nil && fi.Mode().Perm()&0o111 != 0 {
		mode = 0o755
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return writeAtomic(dst, data, mode)
}

// writeAtomic replaces path through a temp file in the same directory, so a
// reader sees the old file or the new one, and a link at path is replaced
// rather than written through.
func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".dfinstall-")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
