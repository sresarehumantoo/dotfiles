package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// A backup records a symlink's target as a STRING, so restoring one recreates a
// link to wherever it pointed when the snapshot was taken. If that path has
// since moved or been deleted, the restore succeeds and leaves a dangling link
// behind -- and it used to report success while doing it.
//
// Measured on a live machine 2026-09-12: a June snapshot restored 46 entries
// pointing at a spare clone that no longer existed, taking `shell` from 12
// linked to 1 and leaving ~/.zshrc and ~/.gitconfig resolving to nothing, while
// the restore printed "Restored backup ... successfully."
func TestRestoreReportsDanglingLinks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTFILES", home)
	core.Cfg.BackupDirP = filepath.Join(home, "backups")

	// A file that exists now and will be displaced by the restore.
	live := filepath.Join(home, ".livefile")
	if err := os.WriteFile(live, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Snapshot it while it is a symlink to a target we then delete, which is
	// exactly the shape of the real failure.
	gone := filepath.Join(home, "since-deleted", "zshrc")
	if err := os.MkdirAll(filepath.Dir(gone), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gone, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(live)
	if err := os.Symlink(gone, live); err != nil {
		t.Fatal(err)
	}

	if err := core.StartBackup(); err != nil {
		t.Fatalf("StartBackup: %v", err)
	}
	if err := core.BackupFile(live); err != nil {
		t.Fatalf("BackupFile: %v", err)
	}
	if err := core.FinishBackup(); err != nil {
		t.Fatalf("FinishBackup: %v", err)
	}
	backups, err := core.ListBackups()
	if err != nil || len(backups) == 0 {
		t.Fatalf("ListBackups: %v (%d found)", err, len(backups))
	}
	ts := backups[0].Timestamp

	// The clone goes away, as a moved or deleted checkout would.
	if err := os.RemoveAll(filepath.Dir(gone)); err != nil {
		t.Fatal(err)
	}
	os.Remove(live)

	// ⚠ NOT an error: reproducing a link that already pointed at something
	// absent is a faithful restore, and failing it would break a restore that
	// did its job (see TestRestoreRoundTrip, which snapshots exactly that).
	// What must not happen is reporting a bare success.
	res, err := core.RestoreBackup(ts)
	if err != nil {
		t.Fatalf("a faithful restore must not fail just because a link dangles: %v", err)
	}
	if len(res.Dangling) != 1 {
		t.Fatalf("Dangling = %d links, want 1 (%v)", len(res.Dangling), res.Dangling)
	}
	if res.Dangling[0] != live {
		t.Errorf("Dangling[0] = %s, want %s", res.Dangling[0], live)
	}

	summary := res.Summary(ts)
	if strings.Contains(summary, "successfully") {
		t.Errorf("summary claims success over a machine whose links dangle: %q", summary)
	}
	if !strings.Contains(summary, "no longer exist") {
		t.Errorf("summary should say what is wrong, got: %q", summary)
	}

	// The link must still have been restored: this is a report fix, not a
	// refusal to restore.
	if _, lerr := os.Lstat(live); lerr != nil {
		t.Errorf("the link should still be restored: %v", lerr)
	}

	// And a clean restore must still read as a success.
	clean := core.RestoreResult{Total: 3}
	if !strings.Contains(clean.Summary(ts), "successfully") {
		t.Errorf("a restore with nothing dangling should report success, got: %q", clean.Summary(ts))
	}
}
