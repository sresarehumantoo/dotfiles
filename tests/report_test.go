package tests

import (
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func TestFormatStatusLine_ExactColumns(t *testing.T) {
	got := modules.FormatStatusLine(core.ModuleStatus{
		Name: "shell", Linked: 11, Missing: 0, Extra: "+2 preserved",
	})
	want := "shell                 11        0  +2 preserved"
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
	modules.WriteStatus(&b)
	out := b.String()

	if !strings.HasPrefix(out, "MODULE") {
		t.Fatalf("missing header:\n%s", out)
	}
	for _, row := range modules.StatusRows() {
		line := modules.FormatStatusLine(row)
		if !strings.Contains(out, line) {
			t.Errorf("rendered table is missing the FormatStatusLine output for %q:\n%q", row.Name, line)
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

func TestStatusRows_AnnotatesSkipped(t *testing.T) {
	modules.RegisterAllModules()

	orig := core.Cfg.SkipModules
	defer func() { core.Cfg.SkipModules = orig }()
	core.Cfg.SkipModules = []string{"fonts"}

	for _, r := range modules.StatusRows() {
		if r.Name == "fonts" && !strings.Contains(r.Extra, "skipped") {
			t.Errorf("fonts row not annotated as skipped: %+v", r)
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
		"ok-mod           ok (4 links)",
		"skip-mod         skipped",
		"count-mod        2 missing",
		"extra-mod        ok (not WSL)",
		"  missing: /a",
		"  wrong target: /b",
		"  regular file (not symlinked): /c",
		"3 issue(s) — FIXHINT",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
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
		"/home/u/projects/dotfiles (canonical) — 1 link(s)",
		"/home/u/dotfiles — 2 link(s)",
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
