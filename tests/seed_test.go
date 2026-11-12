package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// seedHome isolates HOME and returns a shipped file at version v1 and the
// destination a seed of it goes to.
func seedHome(t *testing.T) (src, dst string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	src = filepath.Join(t.TempDir(), "htoprc")
	ship(t, src, "v1\n")
	return src, filepath.Join(home, ".config", "htop", "htoprc")
}

func ship(t *testing.T, path, body string) {
	t.Helper()
	os.Chmod(path, 0o644)
	if err := os.WriteFile(path, []byte(body), 0o444); err != nil {
		t.Fatal(err)
	}
}

func body(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(data)
}

func seed(t *testing.T, src, dst string) {
	t.Helper()
	if err := core.SeedFile(src, dst); err != nil {
		t.Fatalf("SeedFile: %v", err)
	}
}

func TestSeedFile_CopiesAUserOwnedFile(t *testing.T) {
	src, dst := seedHome(t)
	seed(t, src, dst)
	fi, err := os.Lstat(dst)
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("dst is not a regular file: %v", err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %o, want 0644 so the program can rewrite it", fi.Mode().Perm())
	}
	if body(t, dst) != "v1\n" {
		t.Errorf("dst = %q", body(t, dst))
	}
	if got := core.CheckSeed(dst); got != "ok" {
		t.Errorf("CheckSeed = %q", got)
	}
}

func TestSeedFile_KeepsExecutableBit(t *testing.T) {
	src, dst := seedHome(t)
	os.Chmod(src, 0o555)
	seed(t, src, dst)
	if fi, _ := os.Stat(dst); fi.Mode().Perm() != 0o755 {
		t.Errorf("mode %o, want 0755 from an executable source", fi.Mode().Perm())
	}
}

// Upgrade table, first row: an untouched copy takes the new version.
func TestSeedFile_UpdatesAnUntouchedCopy(t *testing.T) {
	src, dst := seedHome(t)
	seed(t, src, dst)
	ship(t, src, "v2\n")
	seed(t, src, dst)
	if body(t, dst) != "v2\n" {
		t.Errorf("dst = %q, want the new version", body(t, dst))
	}
	if len(core.SeedPending()) != 0 {
		t.Error("an untouched copy left a pending update")
	}
}

// Upgrade table, last row: an edited copy is kept and the new version waits
// under pending, never beside the file.
func TestSeedFile_KeepsAnEditedCopy(t *testing.T) {
	src, dst := seedHome(t)
	seed(t, src, dst)
	os.WriteFile(dst, []byte("mine\n"), 0o644)
	ship(t, src, "v2\n")
	seed(t, src, dst)

	if body(t, dst) != "mine\n" {
		t.Errorf("dst = %q, want the user's edit kept", body(t, dst))
	}
	pending := core.SeedPending()[dst]
	if pending == "" || body(t, pending) != "v2\n" {
		t.Fatalf("pending = %q (%q), want v2 staged", pending, body(t, pending))
	}
	if filepath.Dir(pending) == filepath.Dir(dst) {
		t.Errorf("pending update %s sits beside the live file", pending)
	}
	if want := filepath.Join(os.Getenv("HOME"), ".local/share/dfinstall/pending/.config/htop/htoprc"); pending != want {
		t.Errorf("pending at %s, want %s", pending, want)
	}

	// Merged by hand into exactly the new version: resolved.
	os.WriteFile(dst, []byte("v2\n"), 0o644)
	seed(t, src, dst)
	if len(core.SeedPending()) != 0 {
		t.Error("pending update left after the user took it")
	}
	// From here an untouched v2 copy follows the next upgrade again.
	ship(t, src, "v3\n")
	seed(t, src, dst)
	if body(t, dst) != "v3\n" {
		t.Errorf("dst = %q after resolving, want it to follow upgrades again", body(t, dst))
	}
}

// Reverting an edit to the version it came from makes it untouched again.
func TestSeedFile_RevertedEditFollowsUpgrades(t *testing.T) {
	src, dst := seedHome(t)
	seed(t, src, dst)
	os.WriteFile(dst, []byte("mine\n"), 0o644)
	ship(t, src, "v2\n")
	seed(t, src, dst)
	os.WriteFile(dst, []byte("v1\n"), 0o644)
	seed(t, src, dst)
	if body(t, dst) != "v2\n" || len(core.SeedPending()) != 0 {
		t.Errorf("dst = %q, pending %v; want v2 and nothing pending", body(t, dst), core.SeedPending())
	}
}

// A file that was there before dfinstall is the user's from the start.
func TestSeedFile_KeepsAPreexistingFile(t *testing.T) {
	src, dst := seedHome(t)
	os.MkdirAll(filepath.Dir(dst), 0o755)
	os.WriteFile(dst, []byte("theirs\n"), 0o644)
	seed(t, src, dst)
	if body(t, dst) != "theirs\n" {
		t.Errorf("dst = %q, want the existing file kept", body(t, dst))
	}
	if core.SeedPending()[dst] == "" {
		t.Error("shipped version not staged for a pre-existing file")
	}
}

// A link from a clone install is replaced by a copy of what it showed, which
// may carry an edit made in the clone.
func TestSeedFile_ReplacesALinkWithItsFile(t *testing.T) {
	src, dst := seedHome(t)
	clone := filepath.Join(t.TempDir(), "htoprc")
	os.WriteFile(clone, []byte("edited in the clone\n"), 0o644)
	os.MkdirAll(filepath.Dir(dst), 0o755)
	os.Symlink(clone, dst)

	seed(t, src, dst)
	if fi, _ := os.Lstat(dst); !fi.Mode().IsRegular() {
		t.Fatal("dst is still a link")
	}
	if body(t, dst) != "edited in the clone\n" {
		t.Errorf("dst = %q, want what the link showed", body(t, dst))
	}
	if body(t, clone) != "edited in the clone\n" {
		t.Error("the link's target was written through")
	}
	ship(t, src, "v2\n")
	seed(t, src, dst)
	if body(t, dst) != "edited in the clone\n" || core.SeedPending()[dst] == "" {
		t.Error("an edit carried over from the clone was not treated as the user's")
	}
}

func TestSeedFile_ReplacesADanglingLink(t *testing.T) {
	src, dst := seedHome(t)
	os.MkdirAll(filepath.Dir(dst), 0o755)
	os.Symlink(filepath.Join(t.TempDir(), "gone"), dst)
	seed(t, src, dst)
	if body(t, dst) != "v1\n" {
		t.Errorf("dst = %q, want the shipped file", body(t, dst))
	}
}

func TestUnseedFile(t *testing.T) {
	src, dst := seedHome(t)
	seed(t, src, dst)
	if err := core.UnseedFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dst); err == nil {
		t.Error("an untouched copy was left behind")
	}

	seed(t, src, dst)
	os.WriteFile(dst, []byte("mine\n"), 0o644)
	if err := core.UnseedFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if body(t, dst) != "mine\n" {
		t.Error("uninstall removed a copy the user changed")
	}
}

func TestSeedToLink(t *testing.T) {
	src, dst := seedHome(t)
	seed(t, src, dst)
	clone := filepath.Join(t.TempDir(), "htoprc")
	os.WriteFile(clone, []byte("v1\n"), 0o644)
	if ok, err := core.SeedToLink(clone, dst); !ok || err != nil {
		t.Fatalf("untouched copy: ok=%v err=%v", ok, err)
	}
	if target, _ := os.Readlink(dst); target != clone {
		t.Errorf("dst -> %q, want %q", target, clone)
	}

	// An edited copy goes back to a link only when the clone has the same
	// content, so nothing is lost.
	os.Remove(dst)
	seed(t, src, dst)
	os.WriteFile(dst, []byte("mine\n"), 0o644)
	if ok, _ := core.SeedToLink(clone, dst); ok || body(t, dst) != "mine\n" {
		t.Errorf("an edit the clone lacks was replaced (ok=%v)", ok)
	}
	os.WriteFile(clone, []byte("mine\n"), 0o644)
	if ok, _ := core.SeedToLink(clone, dst); !ok {
		t.Error("a copy matching the clone was not linked again")
	}
}

func TestSeed_DryRunWritesNothing(t *testing.T) {
	src, dst := seedHome(t)
	home := os.Getenv("HOME")
	saveGlobals(t)

	core.DryRun = true
	before := treeHash(t, home)
	seed(t, src, dst)
	if after := treeHash(t, home); after != before {
		t.Fatal("dry-run seed of a missing file wrote something")
	}

	core.DryRun = false
	seed(t, src, dst)
	os.WriteFile(dst, []byte("mine\n"), 0o644)
	ship(t, src, "v2\n")
	core.DryRun = true
	before = treeHash(t, home)
	seed(t, src, dst)
	core.UnseedFile(src, dst)
	core.SeedToLink(src, dst)
	if after := treeHash(t, home); after != before {
		t.Errorf("dry run changed HOME:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// A clone install links a seed file like any other; only packaged content
// copies it.
func TestLinkPair_SeedPolicyFollowsTheContentSource(t *testing.T) {
	noCloneHome(t)
	saveGlobals(t)
	clone := makeRepo(t, t.TempDir(), "clone")
	src := filepath.Join(clone, "config", "htop", "htoprc")
	os.MkdirAll(filepath.Dir(src), 0o755)
	os.WriteFile(src, []byte("x\n"), 0o644)
	if err := core.WriteCanonicalDir(clone); err != nil {
		t.Fatal(err)
	}
	core.ResetDotfilesDir()
	dst := core.XDGTarget("htop", "htoprc")
	pair := core.LinkPair{Src: src, Dst: dst, Policy: core.PolicySeed}
	if err := pair.Apply(); err != nil {
		t.Fatal(err)
	}
	if target, _ := os.Readlink(dst); target != src {
		t.Errorf("clone install: %s -> %q, want a link to %s", dst, target, src)
	}
	if s := (core.LinkSet{pair}).Status("htop"); s.Linked != 1 {
		t.Errorf("clone install Status = %+v", s)
	}

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
	core.ResetDotfilesDir()
	pair.Src = filepath.Join(core.ContentCurrentDir(), "config", "shell", "zshrc")
	if err := pair.Apply(); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(dst); !fi.Mode().IsRegular() {
		t.Error("packaged content linked a seed file instead of copying it")
	}
	if s := (core.LinkSet{pair}).Status("htop"); s.Linked != 1 || s.Missing != 0 {
		t.Errorf("packaged Status = %+v, want the seeded copy counted", s)
	}
}
