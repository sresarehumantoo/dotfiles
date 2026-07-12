package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// makeRepo creates a dir that looks like a dotfiles checkout (has config/).
func makeRepo(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("makeRepo: %v", err)
	}
	return dir
}

func TestLinkRoot(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"/home/owen/dotfiles/config/tmux/tmux.conf", "/home/owen/dotfiles"},
		{"/home/owen/projects/dotfiles/config/nvim/init.lua", "/home/owen/projects/dotfiles"},
		{"/opt/df/config/git/gitconfig", "/opt/df"},
		{"/etc/hosts", ""},                // not a managed target
		{"relative/config/x", "relative"}, // marker still matches
		{"/no-config-here/file", ""},      // no "/config/" segment
	}
	for _, c := range cases {
		if got := core.LinkRoot(c.target); got != c.want {
			t.Errorf("LinkRoot(%q) = %q, want %q", c.target, got, c.want)
		}
	}
}

func TestCanonicalReadWriteAndSelfHeal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Nothing written yet.
	if got := core.ReadCanonicalDir(); got != "" {
		t.Fatalf("expected empty canonical, got %q", got)
	}

	// Write a valid repo -> reads back.
	repo := makeRepo(t, tmp, "repo")
	if err := core.WriteCanonicalDir(repo); err != nil {
		t.Fatalf("WriteCanonicalDir: %v", err)
	}
	if got := core.ReadCanonicalDir(); got != repo {
		t.Fatalf("ReadCanonicalDir = %q, want %q", got, repo)
	}

	// Pointer at a non-existent / non-repo dir is ignored (self-healing).
	if err := core.WriteCanonicalDir(filepath.Join(tmp, "gone")); err != nil {
		t.Fatalf("WriteCanonicalDir(stale): %v", err)
	}
	if got := core.ReadCanonicalDir(); got != "" {
		t.Fatalf("stale pointer should read as empty, got %q", got)
	}
}

func TestDotfilesDirPrecedence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	defer core.ResetDotfilesDir()

	canon := makeRepo(t, tmp, "canon")
	if err := core.WriteCanonicalDir(canon); err != nil {
		t.Fatalf("WriteCanonicalDir: %v", err)
	}

	// $DOTFILES beats the canonical pointer.
	t.Setenv("DOTFILES", "/explicit/override")
	core.ResetDotfilesDir()
	if got := core.DotfilesDir(); got != "/explicit/override" {
		t.Fatalf("with $DOTFILES set, DotfilesDir = %q, want /explicit/override", got)
	}

	// Without $DOTFILES, the canonical pointer wins over the invoking clone.
	t.Setenv("DOTFILES", "")
	core.ResetDotfilesDir()
	if got := core.DotfilesDir(); got != canon {
		t.Fatalf("DotfilesDir = %q, want canonical %q", got, canon)
	}

	// A stale canonical pointer falls through to the invoking clone.
	if err := core.WriteCanonicalDir(filepath.Join(tmp, "vanished")); err != nil {
		t.Fatalf("WriteCanonicalDir(stale): %v", err)
	}
	core.ResetDotfilesDir()
	if got := core.DotfilesDir(); got == filepath.Join(tmp, "vanished") {
		t.Fatalf("stale canonical should be ignored, but DotfilesDir = %q", got)
	}
}

func TestLinkDriftSplit(t *testing.T) {
	// Single root == canonical -> no drift.
	d := core.LinkDrift{
		Canonical: "/home/owen/dotfiles",
		Roots:     map[string][]string{"/home/owen/dotfiles": {"~/.tmux.conf"}},
	}
	if d.Split() {
		t.Error("single canonical root should not be split")
	}

	// Two roots -> drift.
	d = core.LinkDrift{
		Canonical: "/home/owen/dotfiles",
		Roots: map[string][]string{
			"/home/owen/dotfiles":          {"~/.tmux.conf"},
			"/home/owen/projects/dotfiles": {"~/.config/nvim/init.lua"},
		},
	}
	if !d.Split() {
		t.Error("two roots should be split")
	}
	if roots := d.SortedRoots(); len(roots) != 2 || roots[0] != "/home/owen/dotfiles" {
		t.Errorf("SortedRoots = %v, want stable sorted order", roots)
	}

	// Single root that isn't canonical -> drift (e.g. after adopting a new
	// canonical but before re-linking).
	d = core.LinkDrift{
		Canonical: "/home/owen/projects/dotfiles",
		Roots:     map[string][]string{"/home/owen/dotfiles": {"~/.tmux.conf"}},
	}
	if !d.Split() {
		t.Error("single non-canonical root should be split")
	}
}
