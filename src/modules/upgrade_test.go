package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

const (
	fixtureGhostty   = "font-size = 12\nwindow-width = 100\nkeybind = ctrl+a=copy_to_clipboard\n"
	fixtureZshrc     = "# zshrc\nexport EDITOR=nvim\n"
	fixtureGitconfig = "[init]\n\tdefaultBranch = main\n[pull]\n\trebase = false\n"
)

// upgradeFixture is an upstream repo shaped like the dotfiles and a clone of
// it, with HOME and XDG_CONFIG_HOME in a temp dir.
func upgradeFixture(t *testing.T) (up, clone string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	up = t.TempDir()
	gitIn(t, up, "init", "-q", "-b", "main")
	gitIn(t, up, "config", "user.email", "test@example.com")
	gitIn(t, up, "config", "user.name", "Test")
	putFile(t, up, "config/ghostty/config", fixtureGhostty)
	putFile(t, up, "config/shell/zshrc", fixtureZshrc)
	putFile(t, up, "config/git/gitconfig", fixtureGitconfig)
	putFile(t, up, "src/main.go", "package main\n")
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "first")

	clone = filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "-q", up, clone).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	gitIn(t, clone, "config", "user.email", "test@example.com")
	gitIn(t, clone, "config", "user.name", "Test")
	return up, clone
}

func putFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func commitUpstream(t *testing.T, up, rel, content string) {
	t.Helper()
	putFile(t, up, rel, content)
	gitIn(t, up, "commit", "-q", "-am", "upstream change to "+rel)
}

func TestPlanMove(t *testing.T) {
	cases := []struct {
		name, rel, edit string
		movable         bool
		want            string
	}{
		{"ghostty changed scalar", "config/ghostty/config",
			strings.Replace(fixtureGhostty, "window-width = 100", "window-width = 120", 1),
			true, "window-width = 120"},
		{"ghostty added key", "config/ghostty/config",
			fixtureGhostty + "\n# mine\nfont-thicken = true\n",
			true, "# mine\nfont-thicken = true"},
		{"ghostty changed repeatable key", "config/ghostty/config",
			strings.Replace(fixtureGhostty, "ctrl+a", "ctrl+b", 1),
			false, ""},
		{"ghostty removed key", "config/ghostty/config",
			strings.Replace(fixtureGhostty, "font-size = 12\n", "", 1),
			false, ""},
		{"zsh addition", "config/shell/zshrc",
			fixtureZshrc + "alias k=kubectl\n",
			true, "alias k=kubectl"},
		{"zsh changed line", "config/shell/zshrc",
			strings.Replace(fixtureZshrc, "nvim", "vim", 1),
			false, ""},
		{"zsh comment removed", "config/shell/zshrc",
			strings.Replace(fixtureZshrc, "# zshrc\n", "", 1) + "export PAGER=less\n",
			true, "export PAGER=less"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, clone := upgradeFixture(t)
			putFile(t, clone, c.rel, c.edit)
			o, ok := overrideFor(c.rel)
			if !ok {
				t.Fatalf("no override for %s", c.rel)
			}
			edit, movable := planMove(context.Background(), clone, c.rel, o.kind)
			if movable != c.movable {
				t.Fatalf("movable = %v, want %v", movable, c.movable)
			}
			if movable && edit.String() != c.want {
				t.Errorf("moved:\n%s\nwant:\n%s", edit, c.want)
			}
		})
	}
}

func TestPlanGitMove(t *testing.T) {
	ctx := context.Background()
	_, clone := upgradeFixture(t)
	rel := "config/git/gitconfig"

	putFile(t, clone, rel, fixtureGitconfig+"[core]\n\tpager = less\n")
	edit, ok := planMove(ctx, clone, rel, overrideGit)
	if !ok || !slices.Equal(edit.entries, [][2]string{{"core.pager", "less"}}) {
		t.Fatalf("new key: got %v, %v", edit.entries, ok)
	}

	o, _ := overrideFor(rel)
	if err := applyMove(ctx, o, rel, edit); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, clone, "config", "--file", o.path, "core.pager"); got != "less" {
		t.Errorf("override has core.pager = %q", got)
	}

	// The include sits at the top of gitconfig, so the main file would still win.
	putFile(t, clone, rel, strings.Replace(fixtureGitconfig, "rebase = false", "rebase = true", 1))
	if _, ok := planMove(ctx, clone, rel, overrideGit); ok {
		t.Error("changed key reported movable")
	}
}

func TestApplyMoveAppends(t *testing.T) {
	_, clone := upgradeFixture(t)
	o, _ := overrideFor("config/shell/zshrc")
	putFile(t, filepath.Dir(o.path), filepath.Base(o.path), "export MINE=1")
	edit := movedEdit{lines: []string{"alias k=kubectl"}}
	if err := applyMove(context.Background(), o, "config/shell/zshrc", edit); err != nil {
		t.Fatal(err)
	}
	got := getFile(t, filepath.Dir(o.path), filepath.Base(o.path))
	want := "export MINE=1\n\n# moved from config/shell/zshrc by dfinstall install --upgrade\nalias k=kubectl\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	_ = clone
}

func TestConvergeClone_FastForwardKeepsEdits(t *testing.T) {
	ctx := context.Background()
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "config/shell/zshrc", fixtureZshrc+"export PAGER=less\n")
	commitUpstream(t, up, "config/ghostty/config", "font-size = 13\nwindow-width = 100\nkeybind = ctrl+a=copy_to_clipboard\n")

	// Edits to a file upstream did not touch, and to one it did but elsewhere.
	putFile(t, clone, "config/git/gitconfig", strings.Replace(fixtureGitconfig, "rebase = false", "rebase = true", 1))
	putFile(t, clone, "config/ghostty/config", strings.Replace(fixtureGhostty, "keybind = ctrl+a", "keybind = ctrl+b", 1))

	srcChanged, err := ConvergeClone(ctx, clone)
	if err != nil {
		t.Fatal(err)
	}
	if srcChanged {
		t.Error("srcChanged with no change under src/")
	}
	if a, b := gitIn(t, clone, "rev-parse", "HEAD"), gitIn(t, up, "rev-parse", "HEAD"); a != b {
		t.Fatalf("clone at %s, upstream at %s", a, b)
	}
	if got := getFile(t, clone, "config/ghostty/config"); got != "font-size = 13\nwindow-width = 100\nkeybind = ctrl+b=copy_to_clipboard\n" {
		t.Errorf("ghostty config after pull:\n%s", got)
	}
	if !strings.Contains(getFile(t, clone, "config/git/gitconfig"), "rebase = true") {
		t.Error("gitconfig edit lost")
	}
	if got := gitIn(t, clone, "stash", "list"); got != "" {
		t.Errorf("stash left behind: %s", got)
	}
}

func TestConvergeClone_ConflictingEditStopsPull(t *testing.T) {
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "config/ghostty/config", strings.Replace(fixtureGhostty, "window-width = 100", "window-width = 90", 1))
	mine := strings.Replace(fixtureGhostty, "window-width = 100", "window-width = 120", 1)
	putFile(t, clone, "config/ghostty/config", mine)
	before := gitIn(t, clone, "rev-parse", "HEAD")

	if _, err := ConvergeClone(context.Background(), clone); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, clone, "rev-parse", "HEAD"); got != before {
		t.Error("pulled despite a conflicting edit")
	}
	if got := getFile(t, clone, "config/ghostty/config"); got != mine {
		t.Errorf("edit changed:\n%s", got)
	}
}

func TestConvergeClone_LocalCommitsStopPull(t *testing.T) {
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "config/shell/zshrc", fixtureZshrc+"export PAGER=less\n")
	putFile(t, clone, "config/shell/zshrc", fixtureZshrc+"alias k=kubectl\n")
	gitIn(t, clone, "commit", "-q", "-am", "local")
	before := gitIn(t, clone, "rev-parse", "HEAD")

	if _, err := ConvergeClone(context.Background(), clone); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, clone, "rev-parse", "HEAD"); got != before {
		t.Error("diverged clone was moved")
	}
}

func TestConvergeClone_ReportsSourceChange(t *testing.T) {
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "src/main.go", "package main\n\nfunc main() {}\n")
	changed, err := ConvergeClone(context.Background(), clone)
	if err != nil || !changed {
		t.Fatalf("srcChanged = %v, %v; want true", changed, err)
	}
}

func TestConvergeClone_DryRunChangesNothing(t *testing.T) {
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "config/shell/zshrc", fixtureZshrc+"export PAGER=less\n")
	gitIn(t, clone, "fetch", "-q")
	before := gitIn(t, clone, "rev-parse", "HEAD")

	core.DryRun = true
	t.Cleanup(func() { core.DryRun = false })
	if _, err := ConvergeClone(context.Background(), clone); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, clone, "rev-parse", "HEAD"); got != before {
		t.Error("dry run moved the clone")
	}
}

func TestSplitDiff(t *testing.T) {
	diff := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n--- old\n+++ new\n@@ -5,0 +6 @@\n+added\n"
	added, removed := splitDiff(diff)
	if !slices.Equal(added, []string{"++ new", "added"}) || !slices.Equal(removed, []string{"-- old"}) {
		t.Errorf("added %q, removed %q", added, removed)
	}
}
