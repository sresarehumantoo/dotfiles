package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
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
