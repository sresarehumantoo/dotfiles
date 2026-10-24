package modules

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs a git command in dir, failing the test if it does not succeed.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	// gpgsign off so a developer's global signing config cannot make this
	// hang on a passphrase prompt or fail outright.
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	// Isolate from the developer's global gitconfig; identity is set locally.
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	gitIn(t, dir, "init", "-b", "main")
	gitIn(t, dir, "config", "user.email", "test@example.com")
	gitIn(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "file")
	gitIn(t, dir, "commit", "-m", "first")
	return dir
}

// The question does not apply to every directory, and a check that reports on
// one that isn't a checkout would be noise on a tarball install.
func TestCloneFreshness_NotApplicable(t *testing.T) {
	if r := cloneFreshness(""); r != nil {
		t.Errorf("empty dir: want nil, got %+v", r)
	}
	if r := cloneFreshness(t.TempDir()); r != nil {
		t.Errorf("non-git dir: want nil, got %+v", r)
	}
}

func TestCloneFreshness_ReportsWorktreeAndUpstreamState(t *testing.T) {
	dir := newTestRepo(t)

	// No remote yet.
	r := cloneFreshness(dir)
	if r == nil {
		t.Fatal("want a result for a real checkout")
	}
	if !r.OK {
		t.Error("the check must never fail a run: a dirty or ahead clone is the normal state of a machine being worked on, and doctor's footer would then give advice that cannot fix it")
	}
	if !strings.Contains(r.Detail, "main") || !strings.Contains(r.Detail, "no upstream") {
		t.Errorf("want branch and no-upstream note, got %q", r.Detail)
	}

	// Uncommitted work.
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cloneFreshness(dir).Detail; !strings.Contains(got, "1 uncommitted") {
		t.Errorf("want the uncommitted count, got %q", got)
	}
	gitIn(t, dir, "checkout", "--", "file")

	// Give it an upstream it is level with.
	remote := t.TempDir()
	gitIn(t, remote, "init", "--bare", "-b", "main")
	gitIn(t, dir, "remote", "add", "origin", remote)
	gitIn(t, dir, "push", "-u", "origin", "main")

	got := cloneFreshness(dir).Detail
	if !strings.Contains(got, "clean") || !strings.Contains(got, "level with origin/main") {
		t.Errorf("want clean and level, got %q", got)
	}

	// Committed but not pushed: the state that means this host holds work
	// nothing else has.
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "commit", "-am", "second")
	if got := cloneFreshness(dir).Detail; !strings.Contains(got, "1 unpushed to origin/main") {
		t.Errorf("want the unpushed count, got %q", got)
	}

	// Dirty AND level: the committed history is shared even though the
	// worktree is not, and both halves must be said.
	gitIn(t, dir, "push")
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("four\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = cloneFreshness(dir).Detail
	if !strings.Contains(got, "1 uncommitted") || !strings.Contains(got, "level with origin/main") {
		t.Errorf("want both the dirty count and the upstream comparison, got %q", got)
	}
}

func TestParseAheadBehind(t *testing.T) {
	// `git rev-list --left-right --count @{u}...HEAD` prints behind then ahead.
	for _, tc := range []struct {
		in            string
		behind, ahead int
		ok            bool
	}{
		{"0\t0", 0, 0, true},
		{"3\t2", 3, 2, true},
		{"  0   5  ", 0, 5, true},
		{"", 0, 0, false},
		{"1", 0, 0, false},
		{"a\tb", 0, 0, false},
		{"1\t2\t3", 0, 0, false},
	} {
		behind, ahead, ok := parseAheadBehind(tc.in)
		if ok != tc.ok || behind != tc.behind || ahead != tc.ahead {
			t.Errorf("%q: got (%d,%d,%v), want (%d,%d,%v)", tc.in, behind, ahead, ok, tc.behind, tc.ahead, tc.ok)
		}
	}
}
