package modules

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
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

// linkAt makes the link an install from root would have made for ~/.zshrc,
// a linked file.
func linkAt(t *testing.T, root string) string {
	t.Helper()
	return linkFrom(t, root, "config/shell/zshrc", core.HomeTarget(".zshrc"))
}

// seedLinkAt makes the link a clone install has for htoprc, which packaged
// content seeds instead.
func seedLinkAt(t *testing.T, root string) string {
	t.Helper()
	return linkFrom(t, root, "config/htop/htoprc", core.XDGTarget("htop", "htoprc"))
}

func linkFrom(t *testing.T, root, rel, dst string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Remove(dst)
	if err := os.Symlink(filepath.Join(root, filepath.FromSlash(rel)), dst); err != nil {
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
	seed := seedLinkAt(t, clone)
	packagedFixture(t, "v1.0.0")

	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatalf("migration: %v", err)
	}
	want := filepath.Join(core.ContentCurrentDir(), "config", "shell", "zshrc")
	if got := readLink(t, dst); got != want {
		t.Errorf(".zshrc -> %q after migration, want %q (unresolved current)", got, want)
	}
	if fi, err := os.Lstat(seed); err != nil || !fi.Mode().IsRegular() {
		t.Errorf("htoprc is not a seeded copy after migration: %v", err)
	} else if got := getFile(t, filepath.Dir(seed), "htoprc"); got != "# clone htoprc\n" {
		t.Errorf("seeded htoprc = %q, want the clone's version it was showing", got)
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
	if got := readLink(t, dst); got != filepath.Join(clone, "config", "shell", "zshrc") {
		t.Errorf(".zshrc -> %q after rollback, want the clone again", got)
	}
	if got := readLink(t, seed); got != filepath.Join(clone, "config", "htop", "htoprc") {
		t.Errorf("htoprc -> %q after rollback, want a link to the clone again", got)
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

// A plain init uses packaged content wherever the build has some, without
// asking git anything, and leaves a recorded clone alone.
func TestInitContent_DefaultsToPackaged(t *testing.T) {
	isolate(t)
	saved := core.Channel
	t.Cleanup(func() { core.Channel = saved })
	core.Channel = core.ChannelRelease
	packagedFixture(t, "v1.0.0")
	// git works but the remote is unreachable, so a probe would fail init.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+filepath.Join(t.TempDir(), "none")+"/.insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/")

	if _, err := InitContent(context.Background(), InitOptions{}); err != nil {
		t.Fatalf("plain init: %v", err)
	}
	if !core.UsingPackagedContent() {
		t.Error("plain init did not use packaged content")
	}
}

// Where git's own policy denies the clone, --clone fails naming the policy
// and points at packaged content when the build has some; a build with none
// fails the same way from a plain init. Nothing here reaches the network: the
// deny is decided locally.
func TestInitContent_RefusesADeniedClone(t *testing.T) {
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
	for _, o := range []InitOptions{{Clone: true}, {Repo: DefaultContentRepo}} {
		_, err := InitContent(context.Background(), o)
		if err == nil || !strings.Contains(err.Error(), "GIT_ALLOW_PROTOCOL") || !strings.Contains(err.Error(), "init --packaged") {
			t.Errorf("%+v: err = %v, want the policy named and --packaged suggested", o, err)
		}
	}
	if core.ReadCanonicalDir() != "" {
		t.Error("a refused clone recorded dotfiles content")
	}
}

// From packaged content, --clone goes on to clone rather than reporting the
// content already set up.
func TestInitContent_CloneLeavesPackaged(t *testing.T) {
	isolate(t)
	saved := core.Channel
	t.Cleanup(func() { core.Channel = saved })
	core.Channel = core.ChannelRelease
	packagedFixture(t, "v1.0.0")
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_ALLOW_PROTOCOL", "ssh")
	if _, err := InitContent(context.Background(), InitOptions{}); err != nil {
		t.Errorf("plain init over packaged content: %v", err)
	}
	if _, err := InitContent(context.Background(), InitOptions{Clone: true}); err == nil || !strings.Contains(err.Error(), "cannot clone") {
		t.Errorf("--clone from packaged content: err = %v, want a clone attempt", err)
	}
	if _, err := InitContent(context.Background(), InitOptions{Clone: true, Packaged: true}); err == nil {
		t.Error("--clone with --packaged was accepted")
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
	editStore(t, "config/shell/zshrc", "edited\n")

	r := CollectDiff()
	if !r.Packaged || r.ContentVersion != "v1.0.0" {
		t.Fatalf("Packaged=%v ContentVersion=%q", r.Packaged, r.ContentVersion)
	}
	var brief strings.Builder
	r.WriteBrief(&brief)
	for _, want := range []string{
		"packaged content is v1.0.0 but dfinstall is v2.0.0",
		"edited in packaged content (config/shell/zshrc); install all restores it",
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

// An edit to a seeded file does not block the migration: the file is copied,
// edit and all.
func TestInitPackaged_KeepsASeededFileEdit(t *testing.T) {
	up, clone := upgradeFixture(t)
	isolate(t)
	putFile(t, up, "config/htop/htoprc", "# upstream htoprc\n")
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "htop")
	gitIn(t, clone, "pull", "-q")
	if err := core.WriteCanonicalDir(clone); err != nil {
		t.Fatal(err)
	}
	seed := seedLinkAt(t, clone)
	packagedFixture(t, "v1.0.0")
	putFile(t, clone, "config/htop/htoprc", "# saved by htop on this machine\n")

	kept, err := cloneOnlyChanges(context.Background(), clone)
	if err != nil || len(kept) != 0 {
		t.Fatalf("cloneOnlyChanges = %v, %v; want a seeded file's edit not counted", kept, err)
	}
	if _, err := InitContent(context.Background(), InitOptions{Packaged: true}); err != nil {
		t.Fatalf("migration: %v", err)
	}
	if got := getFile(t, filepath.Dir(seed), "htoprc"); got != "# saved by htop on this machine\n" {
		t.Errorf("htoprc = %q, want the edit carried into the copy", got)
	}
	if core.SeedPending()[seed] == "" {
		t.Error("the shipped htoprc was not staged beside the user's edited copy")
	}
}

func TestSeedPendingReported(t *testing.T) {
	packagedInstall(t, "v1.0.0")
	pair := HtopModule{}.Links()[0]
	if err := pair.Apply(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(pair.Dst); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("htoprc not seeded as a copy: %v", err)
	}
	if err := os.WriteFile(pair.Dst, []byte("saved by htop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packagedFixture(t, "v2.0.0")
	core.Version = "v2.0.0"
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareContent(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if err := (HtopModule{}).Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	sess.Finish()
	if got := getFile(t, filepath.Dir(pair.Dst), "htoprc"); got != "saved by htop\n" {
		t.Fatalf("upgrade replaced the user's htoprc: %q", got)
	}

	r := CollectDiff()
	var brief strings.Builder
	r.WriteBrief(&brief)
	if !strings.Contains(brief.String(), "htoprc: your copy kept; merge the shipped version from") {
		t.Errorf("brief does not report the pending update:\n%s", brief.String())
	}
	for _, e := range r.Entries {
		if e.Name != "htop" {
			continue
		}
		if len(e.Problems) != 1 || e.Problems[0].State != "pending" {
			t.Errorf("htop problems = %+v, want only the pending update", e.Problems)
		}
	}
	if r.Edited < 1 {
		t.Errorf("Edited = %d; a pending update is counted with what install leaves alone", r.Edited)
	}
	if d := seededFilesResult(); !d.OK || !strings.Contains(strings.Join(d.Extra, " "), "htoprc") {
		t.Errorf("doctor seeded files = %+v", d)
	}
	// The row passes, and doctor printed only a failing row's extra lines,
	// so it said an update was waiting without saying which file.
	if out := doctorOutput(t); !strings.Contains(out, "htoprc: merge from") {
		t.Errorf("doctor does not name the file with an update waiting:\n%s", out)
	}
}

func doctorOutput(t *testing.T) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		_, _ = io.Copy(&sb, r)
		done <- sb.String()
	}()
	RunDoctor()
	os.Stdout = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}
