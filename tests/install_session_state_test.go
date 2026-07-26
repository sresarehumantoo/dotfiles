package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// sandboxHome points HOME, XDG_CONFIG_HOME and DOTFILES at throwaway dirs so a
// session's side effects land somewhere disposable instead of the real machine.
func sandboxHome(t *testing.T) (home, clone string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	clone = makeRepo(t, t.TempDir(), "clone")
	t.Setenv("DOTFILES", clone)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	return home, clone
}

// saveGlobals restores the process-global install state the sessions mutate.
func saveGlobals(t *testing.T) {
	t.Helper()
	origDry, origExists, origCfg := core.DryRun, core.CfgFileExists, core.Cfg
	t.Cleanup(func() {
		core.DryRun, core.CfgFileExists, core.Cfg = origDry, origExists, origCfg
	})
}

// A preview must not touch the machine-global canonical pointer. Adopting
// during --dry-run silently made whichever clone you previewed from
// authoritative, so every later partial install linked at the wrong checkout.
func TestBeginInstall_DryRunDoesNotWriteCanonicalPointer(t *testing.T) {
	saveGlobals(t)
	_, clone := sandboxHome(t)

	core.DryRun = true
	core.CfgFileExists = true

	sess, err := core.BeginInstall(core.InstallOptions{All: true})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	sess.Finish()

	if _, err := os.Stat(core.CanonicalPointerPath()); !os.IsNotExist(err) {
		got, _ := os.ReadFile(core.CanonicalPointerPath())
		t.Errorf("--dry-run wrote the canonical pointer (%q); it must not exist", got)
	}

	// The preview should still *report* the adoption it would perform.
	if sess.CanonicalNow != clone {
		t.Errorf("CanonicalNow = %q, want %q — the preview must still say what it would do", sess.CanonicalNow, clone)
	}
}

// The non-dry-run counterpart, so the test above can't pass by the adoption
// having been removed outright.
func TestBeginInstall_RealRunWritesCanonicalPointer(t *testing.T) {
	saveGlobals(t)
	_, clone := sandboxHome(t)

	core.DryRun = false
	core.CfgFileExists = true
	core.Cfg.SkipBackup = true

	sess, err := core.BeginInstall(core.InstallOptions{All: true})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	sess.Finish()

	if got := core.ReadCanonicalDir(); got != clone {
		t.Errorf("canonical pointer = %q, want %q", got, clone)
	}
}

// A failed first run has to stay a first run. Persisting skip_backup: true here
// disarms the automatic backup, so the *next* install — the one that actually
// replaces ~/.zshrc — silently has nothing to restore from.
func TestInstallSession_FailedFirstRunDoesNotPersistSkipBackup(t *testing.T) {
	saveGlobals(t)
	sandboxHome(t)

	core.DryRun = false
	core.CfgFileExists = false // first run
	core.Cfg.SkipBackup = false

	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	if !sess.DidBackup() {
		t.Fatal("first run should have taken a backup")
	}
	sess.MarkFailed()
	sess.Finish()

	if _, err := os.Stat(core.ConfigFilePath()); !os.IsNotExist(err) {
		t.Errorf("a failed first run wrote %s; the next run must still count as a first run", core.ConfigFilePath())
	}
}

// The success path still records the first run, or every install would keep
// re-taking the automatic backup.
func TestInstallSession_SuccessfulFirstRunPersistsSkipBackup(t *testing.T) {
	saveGlobals(t)
	sandboxHome(t)

	core.DryRun = false
	core.CfgFileExists = false
	core.Cfg.SkipBackup = false

	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	sess.Finish()

	if _, err := os.Stat(core.ConfigFilePath()); err != nil {
		t.Fatalf("successful first run should have written %s: %v", core.ConfigFilePath(), err)
	}
	if !core.Cfg.SkipBackup {
		t.Error("successful first run should set skip_backup")
	}
}

// Finish releases the install lock, and does so exactly once — a second call
// must not unlock a mutex it no longer holds.
func TestInstallSession_FinishIsIdempotent(t *testing.T) {
	saveGlobals(t)
	sandboxHome(t)

	core.DryRun = false
	core.CfgFileExists = true
	core.Cfg.SkipBackup = true

	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	sess.Finish()
	sess.Finish() // must not panic on a double unlock

	// The lock is free, so a subsequent session can start.
	next, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatalf("second BeginInstall: %v", err)
	}
	next.Finish()
}
