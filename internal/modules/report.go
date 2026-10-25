package modules

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
	"golang.org/x/term"
)

// Status and diff reporting, shared by the CLI and the MCP server.
//
// Both used to carry their own copy of this — identical logic with fmt.Printf
// swapped for fmt.Fprintf into a strings.Builder. Copies of the same report
// are how the two surfaces drifted apart before (see core.SkipInAll), so the
// collection lives here and the callers only choose a writer and the wording
// of the remediation hint.

// StatusState is where a module stands on this machine.
type StatusState int

const (
	StateInstalled StatusState = iota
	StatePartial
	StateNotInstalled
	StateSkipped
	StateUnavailable
)

// statusGroups is the order WriteStatus renders the states in.
var statusGroups = []struct {
	state StatusState
	title string
	color *color.Color
}{
	{StateInstalled, "Installed", color.New(color.FgGreen, color.Bold)},
	{StatePartial, "Partially installed", color.New(color.FgYellow, color.Bold)},
	{StateNotInstalled, "Not installed", color.New(color.FgRed, color.Bold)},
	{StateSkipped, "Skipped", color.New(color.FgCyan)},
	{StateUnavailable, "Not available here", color.New(color.Faint)},
}

func stateSymbol(s StatusState) string {
	switch s {
	case StateInstalled:
		return core.G.Ok
	case StatePartial:
		return core.G.Partial
	case StateNotInstalled:
		return core.G.Fail
	case StateSkipped:
		return core.G.Skip
	}
	return core.G.Off
}

// StatusRow is one module's status with its state decided.
type StatusRow struct {
	core.ModuleStatus
	State StatusState
	// Reason says why a module is skipped or unavailable.
	Reason string
	// Problems lists a partially installed module's links that are not in
	// place. Empty for modules that do not export their links.
	Problems []DiffProblem
}

// platformGated is a module that only applies on some platforms. Unsupported
// returns "" where it applies, and otherwise says where it does ("macOS only").
type platformGated interface {
	Unsupported() string
}

func macOnly() string {
	if core.IsMac() {
		return ""
	}
	return "macOS only"
}

func linuxOnly() string {
	if core.IsMac() {
		return "Linux only"
	}
	return ""
}

// StatusRows returns one row per module, in install order.
func StatusRows() []StatusRow {
	all := core.AllModules()
	rows := make([]StatusRow, 0, len(all))
	for _, m := range all {
		rows = append(rows, statusRow(m))
	}
	return rows
}

func statusRow(m core.Module) StatusRow {
	// Unavailable is decided before Status runs: a module for another
	// platform has nothing to report, and some probe tools that are absent.
	if pg, ok := m.(platformGated); ok {
		if why := pg.Unsupported(); why != "" {
			return StatusRow{ModuleStatus: core.ModuleStatus{Name: m.Name()}, State: StateUnavailable, Reason: why}
		}
	}
	if a, ok := m.(appModule); ok && !a.AppInstalled() {
		return StatusRow{ModuleStatus: core.ModuleStatus{Name: m.Name()}, State: StateUnavailable, Reason: m.Name() + " not found"}
	}

	r := StatusRow{ModuleStatus: m.Status()}
	switch {
	case core.IsModuleSkipped(m.Name()):
		r.State, r.Reason = StateSkipped, "skip_modules"
	case core.SkipInAll(m.Name()):
		r.State, r.Reason = StateSkipped, "opt-in: dfinstall install "+m.Name()
	// No tools chosen is a choice, not a missing install: install all has
	// nothing to do for it, and only the menu can change that.
	case m.Name() == "toolkit" && len(core.Cfg.ToolkitTools) == 0:
		r.State, r.Reason = StateSkipped, "nothing selected: dfinstall install toolkit --toolkit"
	case r.Missing == 0 && r.Linked > 0:
		r.State = StateInstalled
	case r.Missing > 0 && r.Linked > 0:
		r.State = StatePartial
		if le, ok := m.(core.LinkExporter); ok {
			for _, lp := range le.Links() {
				if state := core.CheckLink(lp.Src, lp.Dst); state != "ok" {
					r.Problems = append(r.Problems, DiffProblem{Dst: lp.Dst, State: state})
				}
			}
		}
	default:
		r.State = StateNotInstalled
	}
	return r
}

// FormatStatusLine formats a single module status line, without the symbol.
func FormatStatusLine(r StatusRow) string {
	var count string
	if r.State != StateUnavailable && r.Linked+r.Missing > 0 {
		count = fmt.Sprintf("%d/%d", r.Linked, r.Linked+r.Missing)
	}
	// A skipped module's own note restates why it was skipped ("disabled").
	info := r.Extra
	switch {
	case r.State == StateSkipped:
		info = r.Reason
	case r.Reason != "" && info != "":
		info = r.Reason + ", " + info
	case r.Reason != "":
		info = r.Reason
	}
	return strings.TrimRight(fmt.Sprintf("%-14s %7s  %s", r.Name, count, info), " ")
}

// linkProblemLabel names a core.CheckLink state, for status and diff alike.
func linkProblemLabel(state string) string {
	switch state {
	case "missing":
		return "missing"
	case "wrong":
		return "wrong target"
	case "file":
		return "regular file (not symlinked)"
	}
	return state
}

// tildePath shortens a path under $HOME for display.
func tildePath(p string) string {
	home, err := core.HomeDir()
	if err != nil || home == "" {
		return p
	}
	if rel, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
		return filepath.Join("~", rel)
	}
	return p
}

// StatusHints is how a surface words its remediation commands: the CLI names
// `dfinstall install`, the MCP server its own tools.
type StatusHints struct {
	// Install is a format string taking a module name.
	Install string
	// InstallAll and Diff are complete commands.
	InstallAll, Diff string
}

// CLIStatusHints are the hints for the dfinstall command line.
var CLIStatusHints = StatusHints{
	Install:    "dfinstall install %s",
	InstallAll: "dfinstall install all",
	Diff:       "dfinstall diff",
}

// WriteStatus renders the status report, grouped by state, ending with the
// commands that fix whatever is not installed. It colors the output only when
// w is a terminal color is enabled for.
func WriteStatus(w io.Writer, hints StatusHints) {
	paint := painter(w)

	rows := StatusRows()
	counts := map[StatusState]int{}
	var fix []string
	for _, r := range rows {
		counts[r.State]++
		if r.State == StatePartial || r.State == StateNotInstalled {
			fix = append(fix, r.Name)
		}
	}

	var summary []string
	for _, g := range statusGroups {
		if n := counts[g.state]; n > 0 {
			summary = append(summary, paint(g.color, fmt.Sprintf("%d %s", n, strings.ToLower(g.title))))
		}
	}
	fmt.Fprintf(w, "%s  %s\n", paint(bold, "Modules on "+platformName()), strings.Join(summary, ", "))

	for _, g := range statusGroups {
		if counts[g.state] == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s\n", paint(bold, fmt.Sprintf("%s (%d)", g.title, counts[g.state])))
		for _, r := range rows {
			if r.State != g.state {
				continue
			}
			line := FormatStatusLine(r)
			if g.state == StateUnavailable {
				line = paint(g.color, line)
			}
			fmt.Fprintf(w, "  %s %s\n", paint(g.color, stateSymbol(g.state)), line)
			for _, p := range r.Problems {
				fmt.Fprintf(w, "      %s\n", paint(faint, linkProblemLabel(p.State)+": "+tildePath(p.Dst)))
			}
		}
	}

	if len(fix) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", paint(bold, "To fix"))
	var steps [][2]string
	// Past a handful, one command per module is a wall; install all is the
	// command a fresh machine wants anyway.
	if len(fix) <= 3 {
		for _, name := range fix {
			steps = append(steps, [2]string{fmt.Sprintf(hints.Install, name), ""})
		}
	} else {
		steps = append(steps,
			[2]string{hints.InstallAll, "everything above"},
			[2]string{fmt.Sprintf(hints.Install, "<module>"), "one module"})
	}
	steps = append(steps, [2]string{hints.Diff, "every path that differs"})
	width := 0
	for _, st := range steps {
		width = max(width, len(st[0]))
	}
	for _, st := range steps {
		if st[1] == "" {
			fmt.Fprintf(w, "  %s\n", st[0])
			continue
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, st[0], paint(faint, st[1]))
	}
}

var (
	bold  = color.New(color.Bold)
	faint = color.New(color.Faint)
	red   = color.New(color.FgRed, color.Bold)
	amber = color.New(color.FgYellow, color.Bold)
)

// painter colors text only when w is a terminal color is enabled for, so a
// pipe, a file or the MCP server's buffer gets plain text.
func painter(w io.Writer) func(*color.Color, string) string {
	if f, ok := w.(*os.File); ok && !color.NoColor && term.IsTerminal(int(f.Fd())) {
		return func(c *color.Color, s string) string { return c.Sprint(s) }
	}
	return func(_ *color.Color, s string) string { return s }
}

func platformName() string {
	switch {
	case core.IsMac():
		return "macOS"
	case core.IsWSL():
		return "WSL"
	}
	return "Linux"
}

// PrintStatus prints the status report to stdout.
func PrintStatus() { WriteStatus(os.Stdout, CLIStatusHints) }

// DiffProblem is one managed path that isn't in the state we expect.
type DiffProblem struct {
	Dst string
	// "missing", "wrong" or "file" (see core.CheckLink), "content", or
	// "edited": linked correctly, but the clone's copy has uncommitted edits,
	// which is where anything writing through the link puts its change.
	State string
	// Detail says what differs for a "content" problem, and names the clone
	// file for an "edited" one.
	Detail string
	// MoveTo is the override file install --upgrade can move an "edited"
	// problem's change into; empty when it cannot (the change rewrites or
	// removes something the override, loaded later, has no way to undo).
	MoveTo string
	// Changes summarizes an "edited" problem's edit, a line each.
	Changes []string
}

// appModule is a module for an application a machine may not have, such as
// sway or ghostty. install all skips it there and Status reports nothing, so
// diff must not call its links missing. Links() still declares them, so an
// uninstall after the application is removed still finds them.
type appModule interface {
	AppInstalled() bool
}

// contentDrifter is a module that manages content it does not link, such as
// keys merged into a file a program also writes. Link checks cannot see that
// drift, so the module reports it.
type contentDrifter interface {
	ContentDrift() []DiffProblem
}

// DiffEntry is one module's contribution to the diff report.
type DiffEntry struct {
	Name    string
	Skipped bool

	// Links and Problems describe modules that export their links.
	Exported bool
	Links    int
	Problems []DiffProblem

	// Absent is a module that does not apply here: another platform's, or
	// one for an application this machine does not have. Reason says which.
	Absent bool
	Reason string

	// Missing and Extra are the fallback for modules that don't, taken from
	// Status() — less precise, since it can only report a count.
	Missing int
	Extra   string
}

// DiffReport is the full comparison of config against the filesystem.
type DiffReport struct {
	Entries []DiffEntry
	Issues  int
	// Edited counts "edited" problems. They are not Issues: install leaves
	// them alone, so "run install all to fix" would not.
	Edited int
	// Root is the clone the links point into.
	Root  string
	Drift core.LinkDrift
}

// CollectDiff inspects every module and reports what differs from config.
func CollectDiff() DiffReport {
	root := core.DotfilesDir()
	r := DiffReport{Root: root}
	edited := editedInClone(root)

	for _, m := range core.AllModules() {
		if pg, ok := m.(platformGated); ok {
			if why := pg.Unsupported(); why != "" {
				r.Entries = append(r.Entries, DiffEntry{Name: m.Name(), Absent: true, Reason: why})
				continue
			}
		}
		// SkipInAll, not IsModuleSkipped: this report ends with "run install
		// all to fix", so it must use the same predicate `install all` does.
		// Otherwise an opt-out module (windev) is listed as fixable drift that
		// no command will ever fix.
		if core.SkipInAll(m.Name()) {
			r.Entries = append(r.Entries, DiffEntry{Name: m.Name(), Skipped: true})
			continue
		}

		if a, ok := m.(appModule); ok && !a.AppInstalled() {
			r.Entries = append(r.Entries, DiffEntry{Name: m.Name(), Absent: true, Reason: m.Name() + " not found"})
			continue
		}

		e := DiffEntry{Name: m.Name()}
		if le, ok := m.(core.LinkExporter); ok {
			links := le.Links()
			e.Exported = true
			e.Links = len(links)
			for _, lp := range links {
				state := core.CheckLink(lp.Src, lp.Dst)
				switch {
				case state != "ok":
					e.Problems = append(e.Problems, DiffProblem{Dst: lp.Dst, State: state})
					r.Issues++
				case len(edited) > 0:
					if rel := editedUnder(edited, root, lp.Src); rel != "" {
						e.Problems = append(e.Problems, DiffProblem{Dst: lp.Dst, State: "edited", Detail: rel, MoveTo: movableTo(root, rel), Changes: describeEdit(root, rel)})
						r.Edited++
					}
				}
			}
			if cd, ok := m.(contentDrifter); ok {
				drift := cd.ContentDrift()
				e.Problems = append(e.Problems, drift...)
				r.Issues += len(drift)
			}
		} else {
			s := m.Status()
			e.Missing, e.Extra = s.Missing, s.Extra
			r.Issues += s.Missing
		}
		r.Entries = append(r.Entries, e)
	}

	r.Drift = core.DetectLinkDrift()
	return r
}

// editedInClone lists the files in the clone at root that differ from HEAD,
// relative to root. Empty when root is not a git checkout.
func editedInClone(root string) []string {
	if root == "" {
		return nil
	}
	// context.Background: CollectDiff has no context, as Status() has none;
	// this is a local git read bounded by runProbe's timeout.
	out, err := runProbe(context.Background(), "git", "-C", root, "diff", "--name-only", "HEAD")
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

// movableTo returns the override file install --upgrade would move rel's
// edits into, or "" when it would keep them in the clone.
func movableTo(root, rel string) string {
	o, ok := overrideFor(rel)
	if !ok {
		return ""
	}
	// context.Background: as in editedInClone.
	edit, ok := planMove(context.Background(), root, rel, o.kind)
	if !ok || edit.empty() {
		return ""
	}
	return o.path
}

// describeEdit summarizes the uncommitted edit to rel: changed keys for a git
// config, else the added and removed lines, at most ten.
func describeEdit(root, rel string) []string {
	// context.Background: as in editedInClone.
	ctx := context.Background()
	var out []string
	if o, ok := overrideFor(rel); ok && o.kind == overrideGit {
		before, after, ok := gitConfigVersions(ctx, root, rel)
		if !ok {
			return nil
		}
		for _, c := range gitConfigChanges(before, after) {
			switch {
			case len(c.before) == 0:
				out = append(out, fmt.Sprintf("+ %s = %s", c.key, strings.Join(c.after, ", ")))
			case len(c.after) == 0:
				out = append(out, fmt.Sprintf("- %s (was %s)", c.key, strings.Join(c.before, ", ")))
			default:
				out = append(out, fmt.Sprintf("~ %s: %s -> %s", c.key, strings.Join(c.before, ", "), strings.Join(c.after, ", ")))
			}
		}
	} else {
		diff, err := runProbe(ctx, "git", "-C", root, "diff", "--no-color", "--no-ext-diff", "-U0", "HEAD", "--", rel)
		if err != nil {
			return nil
		}
		added, removed := splitDiff(string(diff))
		for _, l := range removed {
			if strings.TrimSpace(l) != "" {
				out = append(out, "- "+l)
			}
		}
		for _, l := range added {
			if strings.TrimSpace(l) != "" {
				out = append(out, "+ "+l)
			}
		}
	}
	if len(out) > 10 {
		out = append(out[:10], fmt.Sprintf("... %d more", len(out)-10))
	}
	return out
}

// editedUnder returns the first edited file at src or, for a linked
// directory, under it.
func editedUnder(edited []string, root, src string) string {
	rel, err := filepath.Rel(root, src)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	for _, f := range edited {
		if f == rel || strings.HasPrefix(f, rel+"/") {
			return f
		}
	}
	return ""
}

// Write renders the report.
//
// The two hints are the only thing that differs between surfaces: the CLI
// tells the user to run `dfinstall install all`, the MCP server names its own
// tool. They're separate strings because the original wording quoted the
// command differently in each sentence.
func (r DiffReport) Write(w io.Writer, fixHint, consolidateCmd string) {
	paint := painter(w)
	var inSync, skipped, absent int
	for _, e := range r.Entries {
		switch {
		case e.Skipped:
			skipped++
			continue
		case e.Absent:
			absent++
			continue
		case len(e.Problems) == 0 && e.Missing == 0:
			inSync++
			continue
		}

		// An edit made through a link is the only problem install leaves
		// alone, so a module with nothing else is flagged, not failed.
		sym, c := core.G.Partial, amber
		for _, p := range e.Problems {
			if p.State != "edited" {
				sym, c = core.G.Fail, red
			}
		}
		if e.Missing > 0 {
			sym, c = core.G.Fail, red
			fmt.Fprintf(w, "  %s %-14s %d missing\n", paint(c, sym), e.Name, e.Missing)
			continue
		}
		fmt.Fprintf(w, "  %s %s\n", paint(c, sym), e.Name)
		for _, p := range e.Problems {
			switch p.State {
			case "missing", "wrong", "file":
				fmt.Fprintf(w, "      %s: %s\n", linkProblemLabel(p.State), tildePath(p.Dst))
			case "content":
				fmt.Fprintf(w, "      %s: %s\n", tildePath(p.Dst), p.Detail)
			case "edited":
				fmt.Fprintf(w, "      edited in the clone (uncommitted %s): %s\n", p.Detail, tildePath(p.Dst))
				for _, ch := range p.Changes {
					fmt.Fprintf(w, "          %s\n", ch)
				}
				if p.MoveTo != "" {
					fmt.Fprintf(w, "        %s\n", paint(faint, "install all --upgrade can move the edit into "+tildePath(p.MoveTo)))
				} else {
					fmt.Fprintf(w, "        %s\n", paint(faint, fmt.Sprintf("commit it to keep it on every machine, or revert it: git -C %s checkout -- %s", r.Root, p.Detail)))
				}
			}
		}
	}

	summary := []string{fmt.Sprintf("%d in sync", inSync)}
	if skipped > 0 {
		summary = append(summary, fmt.Sprintf("%d skipped", skipped))
	}
	if absent > 0 {
		summary = append(summary, fmt.Sprintf("%d not available here", absent))
	}
	if r.Issues > 0 || r.Edited > 0 {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%s\n\n", paint(faint, "Modules: "+strings.Join(summary, ", ")))

	if r.Issues == 0 {
		fmt.Fprintln(w, "No drift detected.")
	} else {
		fmt.Fprintf(w, "%d issue(s): %s\n", r.Issues, fixHint)
	}
	if r.Edited > 0 {
		fmt.Fprintf(w, "%d managed file(s) edited through their link; install all leaves them as they are\n", r.Edited)
	}

	// Flag the multi-clone case explicitly: symlinks split across dotfiles
	// clones each show as a "wrong target" above, but the root cause is drift.
	if r.Drift.Split() {
		fmt.Fprintf(w, "\nSymlinks split across %d dotfiles clone(s):\n", len(r.Drift.Roots))
		for _, root := range r.Drift.SortedRoots() {
			marker := ""
			if root == r.Drift.Canonical {
				marker = " (canonical)"
			}
			fmt.Fprintf(w, "  %s%s: %d link(s)\n", root, marker, len(r.Drift.Roots[root]))
		}
		fmt.Fprintf(w, "Run %s from %s to consolidate.\n", consolidateCmd, r.Drift.Canonical)
	}
}

// WriteBrief prints one line per managed path that something changed after
// install: a link replaced by a file or pointing elsewhere, managed content
// edited, or a linked file edited through its link. Nothing when there is none. It is for the shell's drift check, so
// it leaves out what may be deliberate on a given machine: missing links
// (a module never installed there), and modules that do not export links,
// whose counts include packages a machine may lack on purpose.
func (r DiffReport) WriteBrief(w io.Writer) {
	short := tildePath
	for _, e := range r.Entries {
		for _, p := range e.Problems {
			switch p.State {
			case "wrong":
				fmt.Fprintf(w, "%s links somewhere else\n", short(p.Dst))
			case "file":
				fmt.Fprintf(w, "%s was replaced by a file\n", short(p.Dst))
			case "content":
				fmt.Fprintf(w, "%s: %s\n", short(p.Dst), p.Detail)
			case "edited":
				if p.MoveTo != "" {
					fmt.Fprintf(w, "%s: edited in the clone (%s); install all --upgrade can move it to %s\n", short(p.Dst), p.Detail, short(p.MoveTo))
				} else {
					fmt.Fprintf(w, "%s: edited in the clone (%s); commit or revert it there\n", short(p.Dst), p.Detail)
				}
			}
		}
	}
}
