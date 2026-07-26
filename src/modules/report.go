package modules

import (
	"fmt"
	"io"
	"os"

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

// DiffProblem is one link that isn't in the state we expect.
type DiffProblem struct {
	Dst   string
	State string // "missing", "wrong", or "file" — see core.CheckLink
}

// DiffEntry is one module's contribution to the diff report.
type DiffEntry struct {
	Name    string
	Skipped bool

	// Links and Problems describe modules that export their links.
	Exported bool
	Links    int
	Problems []DiffProblem

	// Missing and Extra are the fallback for modules that don't, taken from
	// Status() — less precise, since it can only report a count.
	Missing int
	Extra   string
}

// DiffReport is the full comparison of config against the filesystem.
type DiffReport struct {
	Entries []DiffEntry
	Issues  int
	Drift   core.LinkDrift
}

// CollectDiff inspects every module and reports what differs from config.
func CollectDiff() DiffReport {
	var r DiffReport

	for _, m := range core.AllModules() {
		// SkipInAll, not IsModuleSkipped: this report ends with "run install
		// all to fix", so it must use the same predicate `install all` does.
		// Otherwise an opt-out module (windev) is listed as fixable drift that
		// no command will ever fix.
		if core.SkipInAll(m.Name()) {
			r.Entries = append(r.Entries, DiffEntry{Name: m.Name(), Skipped: true})
			continue
		}

		e := DiffEntry{Name: m.Name()}
		if le, ok := m.(core.LinkExporter); ok {
			links := le.Links()
			e.Exported = true
			e.Links = len(links)
			for _, lp := range links {
				if state := core.CheckLink(lp.Src, lp.Dst); state != "ok" {
					e.Problems = append(e.Problems, DiffProblem{Dst: lp.Dst, State: state})
					r.Issues++
				}
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
