package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/internal/core"
)

const (
	fixtureGhostty   = "font-size = 12\nwindow-width = 100\nkeybind = ctrl+a=copy_to_clipboard\n"
	fixtureZshrc     = "# zshrc\nexport EDITOR=nvim\n"
	fixtureGitconfig = "[init]\n\tdefaultBranch = main\n[pull]\n\trebase = false\n"
)

// upgradeFixture is an upstream repo shaped like the dotfiles and a clone of
// it, with HOME and the XDG dirs in a temp dir.
func upgradeFixture(t *testing.T) (up, clone string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	up = t.TempDir()
	gitIn(t, up, "init", "-q", "-b", "main")
	gitIn(t, up, "config", "user.email", "test@example.com")
	gitIn(t, up, "config", "user.name", "Test")
	putFile(t, up, "config/ghostty/config", fixtureGhostty)
	putFile(t, up, "config/shell/zshrc", fixtureZshrc)
	putFile(t, up, "config/git/gitconfig", fixtureGitconfig)
	putFile(t, up, "cmd/dfinstall/main.go", "package main\n")
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
	rel := "config/git/gitconfig"
	cases := []struct {
		name, edit string
		movable    bool
		want       [][2]string
	}{
		{"new key", fixtureGitconfig + "[core]\n\tpager = less\n", true, [][2]string{{"core.pager", "less"}}},
		// ~/.gitconfig.user is included last, so a changed value wins there.
		{"changed key", strings.Replace(fixtureGitconfig, "rebase = false", "rebase = true", 1), true, [][2]string{{"pull.rebase", "true"}}},
		{"reformatted", "[init]\n    defaultBranch = main\n\n[pull]\n    rebase = false\n", true, nil},
		{"removed key", strings.Replace(fixtureGitconfig, "\trebase = false\n", "", 1), false, nil},
		{"list grows", fixtureGitconfig + "[credential]\n\thelper = store\n", true, [][2]string{{"credential.helper", "store"}}},
		{"list rewritten", fixtureGitconfig, false, nil}, // set up below
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			up, clone := upgradeFixture(t)
			if c.name == "list rewritten" {
				commitUpstream(t, up, rel, fixtureGitconfig+"[credential]\n\thelper = \n\thelper = cache\n")
				gitIn(t, clone, "pull", "-q")
				c.edit = fixtureGitconfig + "[credential]\n\thelper = \n\thelper = store\n"
			}
			putFile(t, clone, rel, c.edit)
			edit, ok := planMove(ctx, clone, rel, overrideGit)
			if ok != c.movable || (ok && !slices.Equal(edit.entries, c.want)) {
				t.Errorf("got %v, %v; want %v, %v", edit.entries, ok, c.want, c.movable)
			}
		})
	}
}

func TestMoveGitEditSkipsWhatIsThere(t *testing.T) {
	ctx := context.Background()
	_, clone := upgradeFixture(t)
	rel := "config/git/gitconfig"
	o, _ := overrideFor(rel)
	if !strings.HasSuffix(o.path, "/.gitconfig.user") {
		t.Fatalf("gitconfig override is %s", o.path)
	}
	putFile(t, filepath.Dir(o.path), filepath.Base(o.path), "[pull]\n\trebase = merges\n[core]\n\tpager = less\n[credential]\n\thelper = store\n")

	putFile(t, clone, rel, strings.Replace(fixtureGitconfig, "rebase = false", "rebase = true", 1)+
		"[core]\n\tpager = less\n\teditor = nvim\n[credential]\n\thelper = store\n")
	edit, ok := planMove(ctx, clone, rel, overrideGit)
	if !ok {
		t.Fatal("not movable")
	}
	edit = withoutPresent(ctx, o, edit)
	// pager and the helper are already there; rebase is there with another value.
	want := [][2]string{{"pull.rebase", "true"}, {"core.editor", "nvim"}}
	if !slices.Equal(edit.entries, want) {
		t.Fatalf("to write: %v, want %v", edit.entries, want)
	}
	if err := applyMove(ctx, o, rel, edit); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, clone, "config", "--file", o.path, "--get-all", "pull.rebase"); got != "true" {
		t.Errorf("pull.rebase in override: %q, want the one replaced value", got)
	}
	if got := gitIn(t, clone, "config", "--file", o.path, "--get-all", "credential.helper"); got != "store" {
		t.Errorf("credential.helper duplicated: %q", got)
	}
}

func TestMoveLinesSkipsWhatIsThere(t *testing.T) {
	_, clone := upgradeFixture(t)
	o, _ := overrideFor("config/shell/zshrc")
	putFile(t, filepath.Dir(o.path), filepath.Base(o.path), "alias k=kubectl\n")
	putFile(t, clone, "config/shell/zshrc", fixtureZshrc+"alias k=kubectl\nexport PAGER=less\n")
	edit, _ := planMove(context.Background(), clone, "config/shell/zshrc", overrideLines)
	if got := withoutPresent(context.Background(), o, edit).String(); got != "export PAGER=less" {
		t.Errorf("to write: %q", got)
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
		t.Error("srcChanged with no change under cmd/ or internal/")
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
	commitUpstream(t, up, "cmd/dfinstall/main.go", "package main\n\nfunc main() {}\n")
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

// The MCP server has no terminal to print to, so a dry run's report has to
// reach CaptureOutput at the default quiet level.
func TestUpgradeClone_DryRunReportsWhatItWouldDo(t *testing.T) {
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "cmd/dfinstall/main.go", "package main\n\nfunc main() {}\n")
	gitIn(t, clone, "fetch", "-q")

	core.DryRun = true
	t.Cleanup(func() { core.DryRun = false })
	var b strings.Builder
	stop := core.CaptureOutput(&b)
	exe := UpgradeClone(context.Background(), clone)
	stop()

	if exe != "" {
		t.Errorf("dry run rebuilt %s", exe)
	}
	for _, want := range []string{"would fast-forward 1 commit(s)", "would rebuild dfinstall from " + clone} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, b.String())
		}
	}
}

func TestSplitDiff(t *testing.T) {
	diff := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n--- old\n+++ new\n@@ -5,0 +6 @@\n+added\n"
	added, removed := splitDiff(diff)
	if !slices.Equal(added, []string{"++ new", "added"}) || !slices.Equal(removed, []string{"-- old"}) {
		t.Errorf("added %q, removed %q", added, removed)
	}
}

func TestRescueReplacedLinks(t *testing.T) {
	_, clone := upgradeFixture(t)
	home := os.Getenv("HOME")
	root := t.TempDir() // the canonical clone the links name
	links := core.LinkSet{
		{Src: filepath.Join(root, "config/ghostty/config"), Dst: filepath.Join(home, ".config/ghostty/config")},
		{Src: filepath.Join(root, "config/shell/zshrc"), Dst: filepath.Join(home, ".zshrc")},
		{Src: filepath.Join(root, "config/git/gitconfig"), Dst: filepath.Join(home, ".gitconfig")},
	}
	mine := strings.Replace(fixtureGhostty, "100", "140", 1)
	putFile(t, home, ".config/ghostty/config", mine)
	putFile(t, home, ".zshrc", fixtureZshrc) // a file, but identical
	if err := os.Symlink(filepath.Join(clone, "config/git/gitconfig"), filepath.Join(home, ".gitconfig")); err != nil {
		t.Fatal(err)
	}

	rescueReplacedLinks(context.Background(), clone, root, links)

	if got := getFile(t, clone, "config/ghostty/config"); got != mine {
		t.Errorf("replaced link not adopted into the clone:\n%s", got)
	}
	if got := gitIn(t, clone, "status", "--porcelain"); got != "M config/ghostty/config" {
		t.Errorf("clone status %q, want only the adopted file", got)
	}
}

func TestUpdateClone(t *testing.T) {
	up, clone := upgradeFixture(t)
	commitUpstream(t, up, "config/shell/zshrc", fixtureZshrc+"export PAGER=less\n")

	core.UpgradeMode = false
	updateClone(context.Background(), "plugin", clone)
	if gitIn(t, clone, "rev-parse", "HEAD") == gitIn(t, up, "rev-parse", "HEAD") {
		t.Fatal("updated without --upgrade")
	}

	core.UpgradeMode = true
	t.Cleanup(func() { core.UpgradeMode = false })
	updateClone(context.Background(), "plugin", clone)
	if a, b := gitIn(t, clone, "rev-parse", "HEAD"), gitIn(t, up, "rev-parse", "HEAD"); a != b {
		t.Errorf("clone at %s, upstream at %s", a, b)
	}
}

func TestBinaryStale(t *testing.T) {
	ctx := context.Background()
	_, clone := upgradeFixture(t)
	built := gitIn(t, clone, "rev-parse", "--short", "HEAD")
	gitIn(t, clone, "tag", "v9.9.9")
	version := "v9.9.8-1-g" + built

	for _, v := range []string{version, version + "-dirty", "v9.9.9", "v9.9.9-dirty"} {
		if BinaryStale(ctx, clone, v) {
			t.Errorf("%s: stale at its own commit", v)
		}
	}
	for _, v := range []string{"dev", "", "v0.0.0-1-gdeadbee"} {
		if !BinaryStale(ctx, clone, v) {
			t.Errorf("%s: not stale, but names no commit here", v)
		}
	}

	// Pulled by hand: config-only commits leave the binary current, a source
	// change does not.
	putFile(t, clone, "config/shell/zshrc", fixtureZshrc+"export PAGER=less\n")
	gitIn(t, clone, "commit", "-q", "-am", "config")
	if BinaryStale(ctx, clone, version) {
		t.Error("stale after a config-only commit")
	}
	putFile(t, clone, "cmd/dfinstall/main.go", "package main\n\nfunc main() {}\n")
	gitIn(t, clone, "commit", "-q", "-am", "src")
	if !BinaryStale(ctx, clone, version) {
		t.Error("not stale after a source commit")
	}

	if BinaryStale(ctx, t.TempDir(), "dev") {
		t.Error("stale in a directory that is not a checkout")
	}
}
