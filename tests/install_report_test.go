package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// fileModule links one file and writes one generated file, the two primitives
// that record changes on their own.
type fileModule struct{ src, link, gen string }

func (fileModule) Name() string { return "fake" }

func (m fileModule) Install(context.Context) error {
	if err := core.LinkFile(m.src, m.link); err != nil {
		return err
	}
	_, err := core.WriteFileIfChanged(m.gen, []byte("generated\n"), 0o644)
	return err
}

func (fileModule) Status() core.ModuleStatus { return core.ModuleStatus{Name: "fake"} }

func runReport(t *testing.T, m core.Module, skip ...string) core.InstallReport {
	t.Helper()
	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		t.Fatalf("BeginInstall: %v", err)
	}
	defer sess.Finish()
	for _, s := range skip {
		sess.Skip(s)
	}
	if err := sess.Run(context.Background(), m); err != nil {
		t.Fatalf("Install: %v", err)
	}
	return sess.Report()
}

func reportSandbox(t *testing.T) fileModule {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTFILES", t.TempDir())
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	origDry, origExists, origCfg := core.DryRun, core.CfgFileExists, core.Cfg
	t.Cleanup(func() { core.DryRun, core.CfgFileExists, core.Cfg = origDry, origExists, origCfg })
	core.CfgFileExists = true
	core.Cfg.SkipBackup = true

	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return fileModule{src: src, link: filepath.Join(home, ".linked"), gen: filepath.Join(home, ".config", "gen")}
}

// A second install of an unchanged machine must report nothing, which is the
// whole point of the report: it is only worth reading if "changed" is true.
func TestInstallReport_SecondRunIsUnchanged(t *testing.T) {
	m := reportSandbox(t)

	first := runReport(t, m)
	if got := first.Modules[0].Changes; len(got) != 2 {
		t.Fatalf("first run changes = %v, want a link and a write", got)
	}

	fi, err := os.Stat(m.gen)
	if err != nil {
		t.Fatal(err)
	}
	second := runReport(t, m, "skipped-one")
	if got := second.Modules[1].Changes; len(got) != 0 {
		t.Errorf("second run changes = %v, want none", got)
	}
	if fi2, _ := os.Stat(m.gen); !fi2.ModTime().Equal(fi.ModTime()) {
		t.Error("identical generated file was rewritten")
	}

	var b strings.Builder
	second.Write(&b, false)
	if out := b.String(); !strings.Contains(out, "Unchanged: fake") || !strings.Contains(out, "Skipped: skipped-one") || strings.Contains(out, "Changed") {
		t.Errorf("report:\n%s", out)
	}
}

// --dry-run used to print "would link" for every link, correct or not, so the
// preview could not tell you what an install would actually do.
func TestInstallReport_DryRunListsOnlyRealChanges(t *testing.T) {
	m := reportSandbox(t)
	runReport(t, m)

	core.DryRun = true
	if got := runReport(t, m).Modules[0].Changes; len(got) != 0 {
		t.Errorf("dry run over an installed machine = %v, want none", got)
	}

	if err := os.Remove(m.link); err != nil {
		t.Fatal(err)
	}
	got := runReport(t, m).Modules[0].Changes
	if len(got) != 1 || !strings.HasPrefix(got[0], "would link") {
		t.Errorf("dry run with a missing link = %v, want one would-link", got)
	}
	if _, err := os.Lstat(m.link); !os.IsNotExist(err) {
		t.Error("dry run created the link")
	}
}
