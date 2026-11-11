package modules

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

var registerOnce sync.Once

// packagedFixture writes a stamped content tree for version, the shape a
// package's share/dfinstall has, and makes initPackaged sync from it.
func packagedFixture(t *testing.T, version string) string {
	t.Helper()
	registerOnce.Do(RegisterAllModules)
	root := t.TempDir()
	putFile(t, root, "config/htop/htoprc", "# htoprc "+version+"\n")
	putFile(t, root, "config/shell/zshrc", "# zshrc "+version+"\n")
	st, err := core.GenerateContentStamp(root, version)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.WriteContentStamp(filepath.Join(root, core.ContentStampName), st); err != nil {
		t.Fatal(err)
	}
	old := packagedSync
	packagedSync = func(_ context.Context, sess *core.InstallSession) error { return sess.SyncContentDir(root, version) }
	t.Cleanup(func() { packagedSync = old })
	return root
}

// isolate gives the test its own HOME with no $DOTFILES and no recorded
// content, and restores the read-only store for cleanup.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("DOTFILES", "")
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	if err := core.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		filepath.WalkDir(home, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				os.Chmod(p, 0o755)
			}
			return nil
		})
	})
	return home
}

// linkAt makes the link a clone install would have made for htop.
func linkAt(t *testing.T, root string) string {
	t.Helper()
	dst := core.XDGTarget("htop", "htoprc")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Remove(dst)
	if err := os.Symlink(filepath.Join(root, "config", "htop", "htoprc"), dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func readLink(t *testing.T, p string) string {
	t.Helper()
	target, err := os.Readlink(p)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestInitPackaged_NewInstall(t *testing.T) {
	isolate(t)
	packagedFixture(t, "v1.0.0")

	dir, err := InitContent(context.Background(), InitOptions{Packaged: true})
	if err != nil {
		t.Fatalf("init --packaged: %v", err)
	}
	if dir != core.ContentCurrentDir() || core.ReadCanonicalDir() != dir {
		t.Fatalf("content at %q, pointer %q, want both %q", dir, core.ReadCanonicalDir(), core.ContentCurrentDir())
	}
	if !core.UsingPackagedContent() {
		t.Error("UsingPackagedContent false after init --packaged")
	}
	// init links nothing, so the first install's automatic backup must
	// still be armed: no settings file yet means a first run.
	if _, err := os.Stat(core.ConfigFilePath()); err == nil {
		t.Error("init --packaged wrote the settings file, disarming the first install's backup")
	}
	if _, err := os.Stat(core.AppDataPath("backups")); err == nil {
		t.Error("init --packaged took a backup of nothing")
	}

	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Errorf("second init --packaged: %v", err)
	}
}

func TestInitPackaged_RejectsCloneOptions(t *testing.T) {
	isolate(t)
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true, Dir: t.TempDir()}); err == nil {
		t.Error("--packaged with --dir accepted")
	}
}

func TestInitPackaged_RefusesUnderDotfilesEnv(t *testing.T) {
	isolate(t)
	packagedFixture(t, "v1.0.0")
	t.Setenv("DOTFILES", t.TempDir())
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err == nil || !strings.Contains(err.Error(), "DOTFILES") {
		t.Errorf("err = %v, want a refusal naming $DOTFILES", err)
	}
}

func TestInitPackaged_MigratesAndRollsBack(t *testing.T) {
	up, clone := upgradeFixture(t)
	isolate(t)
	putFile(t, up, "config/htop/htoprc", "# clone htoprc\n")
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "htop")
	gitIn(t, clone, "pull", "-q")
	if err := core.WriteCanonicalDir(clone); err != nil {
		t.Fatal(err)
	}
	dst := linkAt(t, clone)
	packagedFixture(t, "v1.0.0")

	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatalf("migration: %v", err)
	}
	want := filepath.Join(core.ContentCurrentDir(), "config", "htop", "htoprc")
	if got := readLink(t, dst); got != want {
		t.Errorf("htoprc -> %q after migration, want %q (unresolved current)", got, want)
	}
	if got := getFile(t, core.AppDataPath(), "previous-clone"); got != clone+"\n" {
		t.Errorf("previous-clone = %q", got)
	}
	if _, err := os.Stat(filepath.Join(clone, "config", "htop", "htoprc")); err != nil {
		t.Errorf("the clone was touched: %v", err)
	}
	backups, _ := os.ReadDir(core.AppDataPath("backups"))
	if len(backups) == 0 {
		t.Error("migration took no backup")
	}

	// init --dir <clone> undoes it in one step.
	if _, err := InitContent(context.Background(), InitOptions{Dir: clone}); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := core.ReadCanonicalDir(); got != clone {
		t.Errorf("pointer = %q after rollback, want %q", got, clone)
	}
	if got := readLink(t, dst); got != filepath.Join(clone, "config", "htop", "htoprc") {
		t.Errorf("htoprc -> %q after rollback, want the clone again", got)
	}
}

func TestInitPackaged_RefusesCloneOnlyChanges(t *testing.T) {
	_, clone := upgradeFixture(t)
	isolate(t)
	if err := core.WriteCanonicalDir(clone); err != nil {
		t.Fatal(err)
	}
	dst := linkAt(t, clone)
	packagedFixture(t, "v1.0.0")
	putFile(t, clone, "config/shell/zshrc", "# edited on this machine\n")

	_, err := InitContent(context.Background(), InitOptions{Packaged: true})
	if err == nil || !strings.Contains(err.Error(), "config/shell/zshrc") {
		t.Fatalf("err = %v, want a refusal naming the edit", err)
	}
	if got := core.ReadCanonicalDir(); got != clone {
		t.Errorf("pointer moved to %q on a refusal", got)
	}
	if got := readLink(t, dst); !strings.HasPrefix(got, clone) {
		t.Errorf("link moved to %q on a refusal", got)
	}
	if _, err := os.Lstat(core.ContentStoreDir()); err == nil {
		t.Error("store written on a refusal")
	}

	if _, err := InitContent(context.Background(), InitOptions{Packaged: true, Force: true}); err != nil {
		t.Fatalf("--force: %v", err)
	}
	if !core.UsingPackagedContent() {
		t.Error("--force did not switch")
	}
}

func TestInitPackaged_DryRunChangesNothing(t *testing.T) {
	_, clone := upgradeFixture(t)
	home := isolate(t)
	if err := core.WriteCanonicalDir(clone); err != nil {
		t.Fatal(err)
	}
	dst := linkAt(t, clone)
	packagedFixture(t, "v1.0.0")
	core.DryRun = true
	t.Cleanup(func() { core.DryRun = false })

	before := snapshot(t, home)
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if after := snapshot(t, home); after != before {
		t.Errorf("dry run changed HOME:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	if got := readLink(t, dst); !strings.HasPrefix(got, clone) {
		t.Errorf("dry run moved the link to %q", got)
	}
}

func TestCloneOnlyChanges(t *testing.T) {
	up, clone := upgradeFixture(t)
	ctx := context.Background()
	got := func() string {
		kept, err := cloneOnlyChanges(ctx, clone)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(kept, "; ")
	}
	if s := got(); s != "" {
		t.Errorf("clean clone reports %q", s)
	}
	putFile(t, clone, "cmd/dfinstall/main.go", "package main // edited\n")
	if s := got(); s != "" {
		t.Errorf("an edit outside config/ reports %q", s)
	}
	putFile(t, clone, "config/tmux/tmux.conf", "# edited\n")
	if s := got(); s != "edited config/tmux/tmux.conf" {
		t.Errorf("got %q", s)
	}
	gitIn(t, clone, "commit", "-q", "-am", "local")
	if s := got(); !strings.Contains(s, "1 commit(s)") {
		t.Errorf("an unpushed commit reports %q", s)
	}
	if kept, err := cloneOnlyChanges(ctx, t.TempDir()); err != nil || len(kept) != 0 {
		t.Errorf("not a checkout: %v, %v", kept, err)
	}
	_ = up
}

// snapshot lists every path, link target and file body under root.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		b.WriteString(p)
		if target, err := os.Readlink(p); err == nil {
			b.WriteString(" -> " + target)
		} else if d.Type().IsRegular() {
			data, _ := os.ReadFile(p)
			b.WriteString(" " + string(data))
		}
		b.WriteString("\n")
		return nil
	})
	return b.String()
}

// Where git's own policy denies the clone, a plain init uses packaged content
// when this build has some, and otherwise fails naming the policy. Nothing
// here reaches the network: the deny is decided locally.
func TestInitContent_FallsBackWhenGitIsDenied(t *testing.T) {
	isolate(t)
	t.Setenv("GIT_ALLOW_PROTOCOL", "ssh")
	saved := core.Channel
	t.Cleanup(func() { core.Channel = saved })

	core.Channel = core.ChannelSource
	_, err := InitContent(context.Background(), InitOptions{})
	if err == nil || !strings.Contains(err.Error(), "GIT_ALLOW_PROTOCOL") || !strings.Contains(err.Error(), "no packaged content") {
		t.Fatalf("source build: err = %v, want the policy and no packaged content named", err)
	}

	core.Channel = core.ChannelRelease
	packagedFixture(t, "v1.0.0")
	if _, err := InitContent(context.Background(), InitOptions{Repo: DefaultContentRepo}); err == nil || !strings.Contains(err.Error(), "cannot clone") {
		t.Fatalf("explicit --repo: err = %v, want no fallback", err)
	}
	if _, err := InitContent(context.Background(), InitOptions{}); err != nil {
		t.Fatalf("plain init: %v", err)
	}
	if !core.UsingPackagedContent() {
		t.Error("plain init did not fall back to packaged content")
	}
}

// After a package upgrade the store is behind the binary; the next install
// syncs it, and links keep their path through current.
func TestPrepareContent_SyncsToTheBinarysVersion(t *testing.T) {
	isolate(t)
	saved := core.Version
	t.Cleanup(func() { core.Version = saved })
	packagedFixture(t, "v1.0.0")
	core.Version = "v1.0.0"
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatal(err)
	}

	packagedFixture(t, "v2.0.0")
	core.Version = "v2.0.0"
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Finish()
	if err := PrepareContent(context.Background(), sess); err != nil {
		t.Fatalf("PrepareContent: %v", err)
	}
	if got := core.CurrentContentVersion(); got != "v2.0.0" {
		t.Errorf("current = %s, want v2.0.0", got)
	}
	if got := getFile(t, core.ContentCurrentDir(), "config/htop/htoprc"); got != "# htoprc v2.0.0\n" {
		t.Errorf("htoprc = %q", got)
	}
	if got := core.ReadCanonicalDir(); got != core.ContentCurrentDir() {
		t.Errorf("pointer = %q", got)
	}
}

func TestPrepareContent_LeavesACloneAlone(t *testing.T) {
	_, clone := upgradeFixture(t)
	isolate(t)
	if err := core.WriteCanonicalDir(clone); err != nil {
		t.Fatal(err)
	}
	packagedFixture(t, "v1.0.0")
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Finish()
	if err := PrepareContent(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(core.ContentStoreDir()); err == nil {
		t.Error("a clone install got a content store")
	}
}

// packagedInstall sets up packaged content v1.0.0 with this binary at
// version, and htop linked from it.
func packagedInstall(t *testing.T, version string) string {
	t.Helper()
	isolate(t)
	saved := core.Version
	t.Cleanup(func() { core.Version = saved })
	packagedFixture(t, "v1.0.0")
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatal(err)
	}
	core.Version = version
	return linkAt(t, core.ContentCurrentDir())
}

func editStore(t *testing.T, rel, body string) {
	t.Helper()
	p := filepath.Join(core.ContentCurrentDir(), rel)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContentDoctor(t *testing.T) {
	packagedInstall(t, "v1.0.0")
	if r := contentSourceResult(); !strings.Contains(r.Detail, "packaged content v1.0.0") {
		t.Errorf("content source = %q", r.Detail)
	}
	if r := contentVersionResult(); !r.OK {
		t.Errorf("content version failed on a match: %q", r.Detail)
	}

	core.Version = "v2.0.0"
	r := contentVersionResult()
	if r.OK || !strings.Contains(r.Detail, "v1.0.0") || !strings.Contains(strings.Join(r.Extra, " "), "install all") {
		t.Errorf("behind: %+v, want a failure naming the version and the fix", r)
	}

	core.Version = "v1.0.0"
	editStore(t, "config/htop/htoprc", "edited\n")
	r = contentVersionResult()
	if r.OK || !strings.Contains(strings.Join(r.Extra, " "), "config/htop/htoprc") {
		t.Errorf("edited: %+v, want a failure listing the file", r)
	}
}

func TestContentDiff(t *testing.T) {
	packagedInstall(t, "v2.0.0")
	editStore(t, "config/htop/htoprc", "edited\n")

	r := CollectDiff()
	if !r.Packaged || r.ContentVersion != "v1.0.0" {
		t.Fatalf("Packaged=%v ContentVersion=%q", r.Packaged, r.ContentVersion)
	}
	var brief strings.Builder
	r.WriteBrief(&brief)
	for _, want := range []string{
		"packaged content is v1.0.0 but dfinstall is v2.0.0",
		"edited in packaged content (config/htop/htoprc); install all restores it",
	} {
		if !strings.Contains(brief.String(), want) {
			t.Errorf("brief missing %q:\n%s", want, brief.String())
		}
	}
	if r.Edited != 0 || r.Issues < 2 {
		t.Errorf("Issues=%d Edited=%d, want the edit and the version counted as fixable issues", r.Issues, r.Edited)
	}
	var full strings.Builder
	r.Write(&full, "run install all", "install all")
	if strings.Contains(full.String(), "git -C") {
		t.Errorf("packaged report gives git advice:\n%s", full.String())
	}
}
