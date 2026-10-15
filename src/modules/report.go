package modules

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// Status and diff reporting, shared by the CLI and the MCP server.
//
// Both used to carry their own copy of this — identical logic with fmt.Printf
// swapped for fmt.Fprintf into a strings.Builder. Copies of the same report
// are how the two surfaces drifted apart before (see core.SkipInAll), so the
// collection lives here and the callers only choose a writer and the wording
// of the remediation hint.

// StatusRows returns one row per module, with the "skipped" annotation applied.
func StatusRows() []core.ModuleStatus {
	all := core.AllModules()
	rows := make([]core.ModuleStatus, 0, len(all))
	for _, m := range all {
		s := m.Status()
		if core.IsModuleSkipped(m.Name()) {
			if s.Extra != "" {
				s.Extra += ", skipped"
			} else {
				s.Extra = "skipped"
			}
		}
		rows = append(rows, s)
	}
	return rows
}

// FormatStatusLine formats a single module status line.
func FormatStatusLine(s core.ModuleStatus) string {
	return fmt.Sprintf("%-15s  %7d  %7d  %s", s.Name, s.Linked, s.Missing, s.Extra)
}

// WriteStatus renders the status table.
func WriteStatus(w io.Writer) {
	fmt.Fprintf(w, "%-15s  %7s  %7s  %s\n", "MODULE", "LINKED", "MISSING", "INFO")
	fmt.Fprintf(w, "%-15s  %7s  %7s  %s\n", "------", "------", "-------", "----")
	for _, s := range StatusRows() {
		fmt.Fprintln(w, FormatStatusLine(s))
	}
}

// PrintStatus prints the status table to stdout.
func PrintStatus() { WriteStatus(os.Stdout) }

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

	// Absent is a module for an application this machine does not have.
	Absent bool

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
		// SkipInAll, not IsModuleSkipped: this report ends with "run install
		// all to fix", so it must use the same predicate `install all` does.
		// Otherwise an opt-out module (windev) is listed as fixable drift that
		// no command will ever fix.
		if core.SkipInAll(m.Name()) {
			r.Entries = append(r.Entries, DiffEntry{Name: m.Name(), Skipped: true})
			continue
		}

		if a, ok := m.(appModule); ok && !a.AppInstalled() {
			r.Entries = append(r.Entries, DiffEntry{Name: m.Name(), Absent: true})
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
						e.Problems = append(e.Problems, DiffProblem{Dst: lp.Dst, State: "edited", Detail: rel, MoveTo: movableTo(root, rel)})
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
	for _, e := range r.Entries {
		switch {
		case e.Skipped:
			fmt.Fprintf(w, "%-15s  skipped\n", e.Name)

		case e.Absent:
			fmt.Fprintf(w, "%-15s  not installed here\n", e.Name)

		case e.Exported && len(e.Problems) == 0:
			fmt.Fprintf(w, "%-15s  ok (%d links)\n", e.Name, e.Links)

		case e.Exported:
			fmt.Fprintf(w, "%-15s\n", e.Name)
			for _, p := range e.Problems {
				switch p.State {
				case "missing":
					fmt.Fprintf(w, "  missing: %s\n", p.Dst)
				case "wrong":
					fmt.Fprintf(w, "  wrong target: %s\n", p.Dst)
				case "file":
					fmt.Fprintf(w, "  regular file (not symlinked): %s\n", p.Dst)
				case "content":
					fmt.Fprintf(w, "  %s: %s\n", p.Dst, p.Detail)
				case "edited":
					fmt.Fprintf(w, "  edited in the clone (uncommitted %s): %s\n", p.Detail, p.Dst)
					if p.MoveTo != "" {
						fmt.Fprintf(w, "    install all --upgrade can move the edit into %s\n", p.MoveTo)
					} else {
						fmt.Fprintf(w, "    commit it to keep it on every machine, or revert it: git -C %s checkout -- %s\n", r.Root, p.Detail)
					}
				}
			}

		case e.Missing > 0:
			fmt.Fprintf(w, "%-15s  %d missing\n", e.Name, e.Missing)

		default:
			extra := ""
			if e.Extra != "" {
				extra = " (" + e.Extra + ")"
			}
			fmt.Fprintf(w, "%-15s  ok%s\n", e.Name, extra)
		}
	}

	fmt.Fprintln(w)
	if r.Issues == 0 {
		fmt.Fprintln(w, "No drift detected.")
	} else {
		fmt.Fprintf(w, "%d issue(s) — %s\n", r.Issues, fixHint)
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
			fmt.Fprintf(w, "  %s%s — %d link(s)\n", root, marker, len(r.Drift.Roots[root]))
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
	home, _ := core.HomeDir()
	short := func(p string) string {
		if home != "" && strings.HasPrefix(p, home+"/") {
			return "~" + p[len(home):]
		}
		return p
	}
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
