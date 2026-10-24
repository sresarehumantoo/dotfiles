package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func TestFormatStatusLine_ExactColumns(t *testing.T) {
	got := modules.FormatStatusLine(modules.StatusRow{
		ModuleStatus: core.ModuleStatus{Name: "shell", Linked: 11, Extra: "+2 preserved"},
		State:        modules.StateInstalled,
	})
	want := "shell            11/11  +2 preserved"
	if got != want {
		t.Errorf("FormatStatusLine =\n%q\nwant\n%q", got, want)
	}
}

// WriteStatus must render through FormatStatusLine, not its own copy of the
// format string — the two used to be separate, so the tested function wasn't
// the one production called and they were free to drift.
func TestWriteStatusUsesFormatStatusLine(t *testing.T) {
	modules.RegisterAllModules()

	var b strings.Builder
	modules.WriteStatus(&b, modules.CLIStatusHints)
	out := b.String()

	if !strings.HasPrefix(out, "Modules on ") {
		t.Fatalf("missing summary line:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("a non-terminal writer got ANSI escapes:\n%q", out)
	}
	for _, row := range modules.StatusRows() {
		line := modules.FormatStatusLine(row)
		if !strings.Contains(out, line) {
			t.Errorf("rendered report is missing the FormatStatusLine output for %q:\n%q", row.Name, line)
		}
	}
}

// With the ASCII set, the report must carry none of the Unicode status glyphs.
func TestWriteStatus_ASCIIGlyphs(t *testing.T) {
	modules.RegisterAllModules()
	orig := core.G
	defer func() { core.G = orig }()
	core.UseASCII(true)

	var b strings.Builder
	modules.WriteStatus(&b, modules.CLIStatusHints)
	for _, g := range []string{"✓", "✗", "·", "⚠"} {
		if strings.Contains(b.String(), g) {
			t.Errorf("ASCII report contains %q:\n%s", g, b.String())
		}
	}
}

func TestStatusRows_CoversEveryModule(t *testing.T) {
	modules.RegisterAllModules()

	rows := modules.StatusRows()
	if len(rows) != len(core.AllModules()) {
		t.Fatalf("StatusRows returned %d rows for %d modules", len(rows), len(core.AllModules()))
	}
}

func TestStatusRows_MarksSkipped(t *testing.T) {
	modules.RegisterAllModules()

	orig := core.Cfg.SkipModules
	defer func() { core.Cfg.SkipModules = orig }()
	core.Cfg.SkipModules = []string{"fonts"}

	for _, r := range modules.StatusRows() {
		if r.Name == "fonts" && r.State != modules.StateSkipped {
			t.Errorf("fonts row not marked skipped: %+v", r)
		}
	}
}

// The CLI and the MCP server render the same DiffReport and differ only in the
// remediation hint. Rendering one report through both must produce identical
// output apart from that phrase — this is the property that stops the two
// surfaces drifting apart again.
func TestDiffReport_OnlyTheFixHintDiffers(t *testing.T) {
	report := modules.DiffReport{
		Entries: []modules.DiffEntry{
			{Name: "shell", Exported: true, Links: 11},
			{Name: "nvim", Exported: true, Links: 2, Problems: []modules.DiffProblem{
				{Dst: "/home/u/.config/nvim/init.lua", State: "missing"},
				{Dst: "/home/u/.config/nvim/x.lua", State: "wrong"},
			}},
			{Name: "windev", Skipped: true},
			{Name: "packages", Missing: 3},
			{Name: "locale", Extra: "en_US.UTF-8"},
		},
		Issues: 5,
	}

	var cli, mcp strings.Builder
	report.Write(&cli, "run dfinstall install all to fix", "'dfinstall install all'")
	report.Write(&mcp, "run dfinstall_install with module 'all' to fix", "dfinstall_install with module 'all'")

	normalize := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s,
			"run dfinstall install all to fix", "<HINT>"),
			"run dfinstall_install with module 'all' to fix", "<HINT>")
	}
	if normalize(cli.String()) != normalize(mcp.String()) {
		t.Errorf("CLI and MCP renderings differ beyond the hint:\n--- cli ---\n%s\n--- mcp ---\n%s",
			cli.String(), mcp.String())
	}
}

func TestDiffReport_RendersEachEntryShape(t *testing.T) {
	report := modules.DiffReport{
		Entries: []modules.DiffEntry{
			{Name: "ok-mod", Exported: true, Links: 4},
			{Name: "skip-mod", Skipped: true},
			{Name: "count-mod", Missing: 2},
			{Name: "extra-mod", Extra: "not WSL"},
			{Name: "mac-mod", Absent: true, Reason: "macOS only"},
			{Name: "edit-mod", Exported: true, Links: 1, Problems: []modules.DiffProblem{
				{Dst: "/e", State: "edited", Detail: "config/e"},
			}},
			{Name: "bad-mod", Exported: true, Links: 3, Problems: []modules.DiffProblem{
				{Dst: "/a", State: "missing"},
				{Dst: "/b", State: "wrong"},
				{Dst: "/c", State: "file"},
			}},
		},
		Issues: 3,
	}

	var b strings.Builder
	report.Write(&b, "FIXHINT", "CONSOLIDATECMD")
	out := b.String()

	for _, want := range []string{
		core.G.Fail + " count-mod      2 missing",
		core.G.Fail + " bad-mod\n",
		"      missing: /a",
		"      wrong target: /b",
		"      regular file (not symlinked): /c",
		core.G.Partial + " edit-mod\n",
		"Modules: 2 in sync, 1 skipped, 1 not available here",
		"3 issue(s): FIXHINT",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Only modules with problems get a line of their own.
	for _, quiet := range []string{"ok-mod", "skip-mod", "extra-mod", "mac-mod"} {
		if strings.Contains(out, quiet) {
			t.Errorf("output lists %q, which has nothing to fix:\n%s", quiet, out)
		}
	}
}

// The multi-clone drift block renders only when links are actually split, so
// no real machine here exercises it — construct one. Both surfaces must render
// it with their own command name.
func TestDiffReport_RendersSplitCloneDrift(t *testing.T) {
	report := modules.DiffReport{
		Entries: []modules.DiffEntry{{Name: "shell", Exported: true, Links: 2}},
		Drift: core.LinkDrift{
			Canonical: "/home/u/projects/dotfiles",
			Roots: map[string][]string{
				"/home/u/projects/dotfiles": {"/home/u/.zshrc"},
				"/home/u/dotfiles":          {"/home/u/.aliases", "/home/u/.p10k.zsh"},
			},
		},
	}

	var cli, mcp strings.Builder
	report.Write(&cli, "run dfinstall install all to fix", "'dfinstall install all'")
	report.Write(&mcp, "run dfinstall_install with module 'all' to fix", "dfinstall_install with module 'all'")

	for _, want := range []string{
		"Symlinks split across 2 dotfiles clone(s):",
		"/home/u/projects/dotfiles (canonical): 1 link(s)",
		"/home/u/dotfiles: 2 link(s)",
	} {
		if !strings.Contains(cli.String(), want) {
			t.Errorf("CLI drift block missing %q:\n%s", want, cli.String())
		}
		if !strings.Contains(mcp.String(), want) {
			t.Errorf("MCP drift block missing %q:\n%s", want, mcp.String())
		}
	}

	if !strings.Contains(cli.String(), "Run 'dfinstall install all' from /home/u/projects/dotfiles to consolidate.") {
		t.Errorf("CLI consolidate line wrong:\n%s", cli.String())
	}
	if !strings.Contains(mcp.String(), "Run dfinstall_install with module 'all' from /home/u/projects/dotfiles to consolidate.") {
		t.Errorf("MCP consolidate line wrong:\n%s", mcp.String())
	}
}

func TestDiffReport_CleanReportSaysNoDrift(t *testing.T) {
	var b strings.Builder
	modules.DiffReport{
		Entries: []modules.DiffEntry{{Name: "shell", Exported: true, Links: 3}},
	}.Write(&b, "unused", "unused")

	if !strings.Contains(b.String(), "No drift detected.") {
		t.Errorf("clean report should say so:\n%s", b.String())
	}
}

// The shell's drift check prints whatever this writes, so a clean report and
// links that were never installed must print nothing.
func TestDiffReport_WriteBrief(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	var b strings.Builder
	modules.DiffReport{
		Entries: []modules.DiffEntry{
			{Name: "shell", Exported: true, Links: 3, Problems: []modules.DiffProblem{
				{Dst: "/home/u/.zshrc", State: "missing"},
				{Dst: "/home/u/.config/ghostty/config", State: "file"},
				{Dst: "/home/u/.gitconfig", State: "wrong"},
			}},
			{Name: "claude", Exported: true, Links: 1, Problems: []modules.DiffProblem{
				{Dst: "/home/u/.claude/settings.json", State: "content", Detail: "managed keys differ: tui"},
			}},
			{Name: "packages", Missing: 2},
		},
	}.WriteBrief(&b)

	want := "~/.config/ghostty/config was replaced by a file\n" +
		"~/.gitconfig links somewhere else\n" +
		"~/.claude/settings.json: managed keys differ: tui\n"
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}

	b.Reset()
	modules.DiffReport{Entries: []modules.DiffEntry{{Name: "shell", Exported: true, Links: 3}}}.WriteBrief(&b)
	if b.String() != "" {
		t.Errorf("clean report printed %q", b.String())
	}
}

// A WSL box or a server has no sway, and install all never links its desktop
// there, so diff calling 21 links missing was noise no command would fix.
func TestCollectDiff_AbsentAppIsNotDrift(t *testing.T) {
	modules.RegisterAllModules()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOTFILES", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)

	for _, e := range modules.CollectDiff().Entries {
		if e.Name != "sway" && e.Name != "ghostty" {
			continue
		}
		if len(e.Problems) > 0 || !e.Absent {
			t.Errorf("%s with no %s on PATH: absent=%v, %d problem(s)", e.Name, e.Name, e.Absent, len(e.Problems))
		}
	}
}

// A module for another platform is not drift: diff must say it does not apply
// here rather than count its links, or call it ok.
func TestCollectDiff_OtherPlatformIsAbsent(t *testing.T) {
	modules.RegisterAllModules()
	name, want := "aerospace", "macOS only"
	if core.IsMac() {
		name, want = "konsole", "Linux only"
	}
	for _, e := range modules.CollectDiff().Entries {
		if e.Name == name && (!e.Absent || e.Reason != want) {
			t.Errorf("%s: absent=%v reason=%q, want absent with %q", e.Name, e.Absent, e.Reason, want)
		}
	}
}

// A script running `git config --global` writes through ~/.gitconfig into the
// clone: the link stays intact, so only the clone's git status shows it.
func TestCollectDiff_EditThroughLinkIsReported(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	modules.RegisterAllModules()
	home, dots := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTFILES", dots)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)

	src := filepath.Join(dots, "config", "git", "gitconfig")
	os.MkdirAll(filepath.Dir(src), 0o755)
	os.WriteFile(src, []byte("[core]\n\tpager = delta\n"), 0o644)
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "x"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dots}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	dst := filepath.Join(home, ".gitconfig")
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}

	gitEntry := func(r modules.DiffReport) modules.DiffEntry {
		for _, e := range r.Entries {
			if e.Name == "git" {
				return e
			}
		}
		t.Fatal("no git entry")
		return modules.DiffEntry{}
	}
	if e := gitEntry(modules.CollectDiff()); len(e.Problems) != 0 {
		t.Fatalf("clean clone: %+v", e.Problems)
	}

	os.WriteFile(dst, []byte("[core]\n\tpager = less\n"), 0o644)
	r := modules.CollectDiff()
	e := gitEntry(r)
	if len(e.Problems) != 1 || e.Problems[0].State != "edited" || e.Problems[0].Detail != "config/git/gitconfig" {
		t.Fatalf("edit through the link: %+v", e.Problems)
	}
	// A changed key moves to ~/.gitconfig.user, which is included last and
	// wins, and diff shows the change itself.
	var b strings.Builder
	r.WriteBrief(&b)
	if want := "~/.gitconfig: edited in the clone (config/git/gitconfig); install all --upgrade can move it to ~/.gitconfig.user\n"; uniqueLines(b.String()) != want {
		t.Errorf("brief for a changed key:\n%s\nwant:\n%s", b.String(), want)
	}
	b.Reset()
	r.Write(&b, "fix", "consolidate")
	if !strings.Contains(b.String(), "~ core.pager: delta -> less") {
		t.Errorf("diff does not show the change:\n%s", b.String())
	}

	// A removed key cannot: nothing loaded later can unset it. The advice must
	// not offer --upgrade.
	os.WriteFile(dst, []byte("[alias]\n\tst = status\n"), 0o644)
	r = modules.CollectDiff()
	b.Reset()
	r.WriteBrief(&b)
	if want := "~/.gitconfig: edited in the clone (config/git/gitconfig); commit or revert it there\n"; uniqueLines(b.String()) != want {
		t.Errorf("brief for a removed key:\n%s\nwant:\n%s", b.String(), want)
	}
	b.Reset()
	r.Write(&b, "fix", "consolidate")
	if !strings.Contains(b.String(), "git -C "+dots+" checkout -- config/git/gitconfig") || strings.Contains(b.String(), "can move") {
		t.Errorf("diff for a removed key:\n%s", b.String())
	}
}

// uniqueLines drops repeated lines: other tests in this package call
// RegisterAllModules too, so a module can be registered more than once.
func uniqueLines(s string) string {
	var out strings.Builder
	seen := map[string]bool{}
	for _, l := range strings.SplitAfter(s, "\n") {
		if l != "" && !seen[l] {
			seen[l] = true
			out.WriteString(l)
		}
	}
	return out.String()
}
