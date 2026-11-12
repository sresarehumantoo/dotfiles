package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// noCloneHome isolates HOME and XDG, unsets $DOTFILES, and runs the test from
// an empty directory: the situation of a released binary with nothing cloned.
func noCloneHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("DOTFILES", "")
	t.Chdir(t.TempDir())
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	return home
}

func TestInvokingCloneDir_EmptyOutsideAnyClone(t *testing.T) {
	noCloneHome(t)
	if got := core.InvokingCloneDir(); got != "" {
		t.Errorf("InvokingCloneDir = %q from a directory that is not a clone, want \"\"", got)
	}
	if err := core.RequireDotfiles(); err == nil || !strings.Contains(err.Error(), "dfinstall init") {
		t.Errorf("RequireDotfiles = %v, want an error pointing at init", err)
	}
	if got := core.ConfigPath("htop", "htoprc"); got != "" {
		t.Errorf("ConfigPath = %q with no checkout, want \"\" rather than a relative path", got)
	}
}

func TestInvokingCloneDir_FindsCloneInWorkingDir(t *testing.T) {
	noCloneHome(t)
	clone := makeRepo(t, t.TempDir(), "clone")
	t.Chdir(clone)
	if got := core.InvokingCloneDir(); got != clone {
		t.Errorf("InvokingCloneDir = %q, want %q", got, clone)
	}
}

// A package's content tree (/usr/share/dfinstall) has a config/ directory too,
// but links into it would dangle on the next package upgrade.
func TestInvokingCloneDir_IgnoresPackagedContentInWorkingDir(t *testing.T) {
	noCloneHome(t)
	pkg := makeRepo(t, t.TempDir(), "pkg")
	if err := os.WriteFile(filepath.Join(pkg, core.ContentStampName), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(pkg)
	if got := core.InvokingCloneDir(); got != "" {
		t.Errorf("InvokingCloneDir = %q from packaged content, want \"\"", got)
	}
}

// install all from a binary that runs from no clone must keep the checkout
// init recorded. It used to adopt the working directory instead.
func TestBeginInstallAll_KeepsCheckoutWhenRunFromNoClone(t *testing.T) {
	noCloneHome(t)
	saveGlobals(t)
	canon := makeRepo(t, t.TempDir(), "canon")
	if err := core.WriteCanonicalDir(canon); err != nil {
		t.Fatal(err)
	}

	sess, err := core.BeginInstall(core.InstallOptions{All: true})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	sess.Finish()

	if got := core.ReadCanonicalDir(); got != canon {
		t.Errorf("canonical pointer = %q after install all, want %q", got, canon)
	}
	if sess.CanonicalNow != "" {
		t.Errorf("session reported adopting %q", sess.CanonicalNow)
	}
}

// Packaged content is only left by an explicit init --dir. install all run
// from inside the old clone used to adopt it again.
func TestBeginInstallAll_KeepsPackagedContentWhenRunFromAClone(t *testing.T) {
	noCloneHome(t)
	saveGlobals(t)
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.SyncContentDir(contentFixture(t, "v1.0.0"), "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	sess.Finish()
	t.Cleanup(func() { removeSealed(core.ContentStoreDir()) })
	if err := core.WriteCanonicalDir(core.ContentCurrentDir()); err != nil {
		t.Fatal(err)
	}
	clone := makeRepo(t, t.TempDir(), "clone")
	t.Chdir(clone)

	sess, err = core.BeginInstall(core.InstallOptions{All: true})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	sess.Finish()
	if got := core.ReadCanonicalDir(); got != core.ContentCurrentDir() {
		t.Errorf("canonical pointer = %q, want packaged content kept", got)
	}

	// $DOTFILES is an explicit choice, so it still switches.
	t.Setenv("DOTFILES", clone)
	sess, err = core.BeginInstall(core.InstallOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	sess.Finish()
	if got := core.ReadCanonicalDir(); got != clone {
		t.Errorf("canonical pointer = %q with $DOTFILES set, want %q", got, clone)
	}
}
