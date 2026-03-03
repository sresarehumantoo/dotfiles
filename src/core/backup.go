package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupEntry records the pre-install state of a single path.
type BackupEntry struct {
	Path          string `json:"path"`
	Type          string `json:"type"` // "file", "symlink", "missing"
	SymlinkTarget string `json:"symlink_target,omitempty"`
	BackupFile    string `json:"backup_file,omitempty"`
	Hash          string `json:"hash,omitempty"`
}

// BackupManifest is the serialized form of one backup session.
type BackupManifest struct {
	Timestamp string        `json:"timestamp"`
	Entries   []BackupEntry `json:"entries"`
}

// BackupListEntry is the summary returned by ListBackups.
type BackupListEntry struct {
	Timestamp string
	Count     int
}

// backupManager tracks the active backup session.
type backupManager struct {
	dir      string
	filesDir string
	entries  []BackupEntry
	seen     map[string]bool
	ts       string
}

var activeBackup *backupManager

// BackupDir returns the base directory for all backups.
func BackupDir() string {
	if Cfg.BackupDirP != "" {
		return Cfg.BackupDirP
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "dfinstall", "backups")
}

// StartBackup begins a new backup session.
func StartBackup() error {
	ts := time.Now().Format("20060102-150405")
	dir := filepath.Join(BackupDir(), ts)
	filesDir := filepath.Join(dir, "files")

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	activeBackup = &backupManager{
		dir:      dir,
		filesDir: filesDir,
		seen:     make(map[string]bool),
		ts:       ts,
	}

	Info("backup session started: %s", ts)
	return nil
}

// BackupActive returns true if a backup session is running.
func BackupActive() bool {
	return activeBackup != nil
}

// BackupFile records the current state of dst before LinkFile modifies it.
// No-op if no session is active. Skips /etc/ paths.
func BackupFile(dst string) error {
	if activeBackup == nil {
		return nil
	}

	if DryRun {
		return nil
	}

	if IsSystemPath(dst) {
		Debug("backup: skipping system path %s", dst)
		return nil
	}

	// Deduplicate
	if activeBackup.seen[dst] {
		return nil
	}
	activeBackup.seen[dst] = true

	fi, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		activeBackup.entries = append(activeBackup.entries, BackupEntry{
			Path: dst,
			Type: "missing",
		})
		Debug("backup: %s (missing)", dst)
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(dst)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", dst, err)
		}
		activeBackup.entries = append(activeBackup.entries, BackupEntry{
			Path:          dst,
			Type:          "symlink",
			SymlinkTarget: target,
		})
		Debug("backup: %s (symlink -> %s)", dst, target)
		return nil
	}

	// Regular file — copy into backup
	flat := FlattenPath(dst)
	backupDst := filepath.Join(activeBackup.filesDir, flat)

	if err := copyFile(dst, backupDst); err != nil {
		return fmt.Errorf("copy %s: %w", dst, err)
	}

	hash, _ := FileHash(dst)

	activeBackup.entries = append(activeBackup.entries, BackupEntry{
		Path:       dst,
		Type:       "file",
		BackupFile: flat,
		Hash:       hash,
	})
	Debug("backup: %s (file, hash=%s)", dst, hash[:12])
	return nil
}

// FinishBackup writes the manifest and cleans up empty sessions.
func FinishBackup() error {
	if activeBackup == nil {
		return nil
	}
	defer func() { activeBackup = nil }()

	// If nothing was recorded, clean up the empty directory
	if len(activeBackup.entries) == 0 {
		Info("backup: no entries recorded, cleaning up")
		if err := os.RemoveAll(activeBackup.dir); err != nil {
			Warn("backup: failed to clean up empty backup dir: %v", err)
		}
		return nil
	}

	manifest := BackupManifest{
		Timestamp: activeBackup.ts,
		Entries:   activeBackup.entries,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(activeBackup.dir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	Info("backup saved: %s (%d entries)", activeBackup.ts, len(activeBackup.entries))
	return nil
}

// ListBackups returns available backups, newest first.
func ListBackups() ([]BackupListEntry, error) {
	baseDir := BackupDir()
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result []BackupListEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest, err := loadManifest(filepath.Join(baseDir, e.Name()))
		if err != nil {
			continue
		}
		result = append(result, BackupListEntry{
			Timestamp: e.Name(),
			Count:     len(manifest.Entries),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp > result[j].Timestamp
	})
	return result, nil
}

// RestoreBackup restores the state recorded in a backup.
func RestoreBackup(ts string) error {
	dir := filepath.Join(BackupDir(), ts)
	manifest, err := loadManifest(dir)
	if err != nil {
		return fmt.Errorf("load backup %s: %w", ts, err)
	}

	var failures int
	for _, entry := range manifest.Entries {
		var restoreErr error

		switch entry.Type {
		case "missing":
			// Path didn't exist before — remove whatever dfinstall placed
			restoreErr = os.Remove(entry.Path)
			if os.IsNotExist(restoreErr) {
				restoreErr = nil // already gone
			}
			if restoreErr == nil {
				Ok("removed: %s", entry.Path)
			}

		case "symlink":
			// Restore original symlink
			os.Remove(entry.Path)
			restoreErr = os.Symlink(entry.SymlinkTarget, entry.Path)
			if restoreErr == nil {
				Ok("restored symlink: %s -> %s", entry.Path, entry.SymlinkTarget)
			}

		case "file":
			// Restore original file from backup
			src := filepath.Join(dir, "files", entry.BackupFile)
			os.Remove(entry.Path)
			if err := EnsureDir(filepath.Dir(entry.Path)); err != nil {
				restoreErr = err
			} else {
				restoreErr = copyFile(src, entry.Path)
			}
			if restoreErr == nil {
				Ok("restored file: %s", entry.Path)
			}
		}

		if restoreErr != nil {
			Warn("restore failed for %s: %v", entry.Path, restoreErr)
			failures++
		}
	}

	total := len(manifest.Entries)
	if failures > 0 {
		return fmt.Errorf("restored %d/%d entries (%d failures)", total-failures, total, failures)
	}

	Info("restored %d entries from backup %s", total, ts)
	return nil
}

// FlattenPath converts a filesystem path to a flat filename (/ -> --).
func FlattenPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	return strings.ReplaceAll(p, "/", "--")
}

// IsSystemPath returns true for paths that should be skipped (e.g. /etc/).
func IsSystemPath(p string) bool {
	return strings.HasPrefix(p, "/etc/")
}

// loadManifest reads a manifest.json from a backup directory.
func loadManifest(dir string) (*BackupManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m BackupManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	fi, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
