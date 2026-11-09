package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Changed records something an install actually did to this machine, for the
// report at the end of the run, and prints it like Ok under -v. A step that
// found the machine already right reports that with Ok instead, so a module
// that never calls Changed is reported as unchanged. Under DryRun, callers
// word it as what would change.
func Changed(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	var first func()
	changesMu.Lock()
	if activeSession != nil && activeSession.current != nil {
		o := activeSession.current
		if len(o.Changes) == 0 {
			first = o.firstChange
		}
		o.Changes = append(o.Changes, formatted)
	}
	changesMu.Unlock()
	if first != nil {
		first()
	}
	if Level >= LogVerbose {
		emit(okSymbol(G.Ok), formatted)
	}
}

var (
	changesMu     sync.Mutex
	activeSession *InstallSession
)

// ModuleOutcome is what one module did during an install session.
type ModuleOutcome struct {
	Name    string
	Changes []string
	Skipped bool
	Failed  bool

	firstChange func()
}

// SkipError is what Install returns when the module decided not to run on
// this machine, such as oh-my-zsh where git cannot run. Run reports the module
// as skipped, not failed, and returns nil.
type SkipError struct{ Reason string }

func (e *SkipError) Error() string { return e.Reason }

// Run installs m, attributing every Changed call it makes to it.
func (s *InstallSession) Run(ctx context.Context, m Module) error {
	return s.RunWithProgress(ctx, m, nil)
}

// RunWithProgress is Run, calling firstChange when m first changes something,
// so a spinner can say "Checking" until a module actually installs or updates.
func (s *InstallSession) RunWithProgress(ctx context.Context, m Module, firstChange func()) error {
	o := &ModuleOutcome{Name: m.Name(), firstChange: firstChange}
	changesMu.Lock()
	s.outcomes = append(s.outcomes, o)
	s.current = o
	changesMu.Unlock()

	err := m.Install(ctx)
	var skip *SkipError
	skipped := errors.As(err, &skip)
	if skipped {
		Notice("%s skipped: %s", m.Name(), skip.Reason)
		err = nil
	}

	changesMu.Lock()
	s.current = nil
	o.Failed = err != nil
	o.Skipped = skipped
	changesMu.Unlock()
	return err
}

// CurrentModule names the module the active install session is running, or
// "" outside one.
func CurrentModule() string {
	changesMu.Lock()
	defer changesMu.Unlock()
	if activeSession != nil && activeSession.current != nil {
		return activeSession.current.Name
	}
	return ""
}

// Skip records a module the run left out, so the report can say so.
func (s *InstallSession) Skip(name string) {
	changesMu.Lock()
	s.outcomes = append(s.outcomes, &ModuleOutcome{Name: name, Skipped: true})
	changesMu.Unlock()
}

// Report returns what each module did, in the order they ran.
func (s *InstallSession) Report() InstallReport {
	changesMu.Lock()
	defer changesMu.Unlock()
	r := InstallReport{DryRun: DryRun}
	for _, o := range s.outcomes {
		c := *o
		c.Changes = append([]string(nil), o.Changes...)
		c.firstChange = nil
		r.Modules = append(r.Modules, c)
	}
	return r
}

// InstallReport says, per module, what an install changed, and which modules
// it found already in place or skipped.
type InstallReport struct {
	Modules []ModuleOutcome
	DryRun  bool
}

// Outcome is the end of the run's "Done" line: how many modules changed
// something, which tells an install that did nothing from one that did.
func (r InstallReport) Outcome() string {
	n := 0
	for _, o := range r.Modules {
		if !o.Skipped && len(o.Changes) > 0 {
			n++
		}
	}
	return fmt.Sprintf("installed, %d changed", n)
}

// maxChangesShown caps each module's lines in the report; -v prints them all
// as they happen anyway.
const maxChangesShown = 6

// Write renders the report. full lists every change rather than capping each
// module at maxChangesShown, and names the unchanged and skipped modules
// instead of counting them.
func (r InstallReport) Write(w io.Writer, full bool) {
	var unchanged, skipped []string
	width := 0
	for _, o := range r.Modules {
		if len(o.Changes) > 0 && len(o.Name) > width {
			width = len(o.Name)
		}
	}

	heading := "Changed"
	if r.DryRun {
		heading = "Would change"
	}
	wroteHeading := false
	for _, o := range r.Modules {
		switch {
		case o.Skipped:
			skipped = append(skipped, o.Name)
			continue
		case len(o.Changes) == 0:
			if !o.Failed {
				unchanged = append(unchanged, o.Name)
			}
			continue
		}
		if !wroteHeading {
			fmt.Fprintf(w, "  %s:\n", heading)
			wroteHeading = true
		}
		shown := o.Changes
		if !full && len(shown) > maxChangesShown {
			shown = shown[:maxChangesShown-1]
		}
		for i, c := range shown {
			name := ""
			if i == 0 {
				name = o.Name
			}
			fmt.Fprintf(w, "    %-*s  %s\n", width, name, TildePath(c))
		}
		if more := len(o.Changes) - len(shown); more > 0 {
			fmt.Fprintf(w, "    %-*s  ... and %d more (-v lists them)\n", width, "", more)
		}
	}
	if !full {
		var parts []string
		if len(unchanged) > 0 {
			parts = append(parts, fmt.Sprintf("%d unchanged", len(unchanged)))
		}
		if len(skipped) > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", len(skipped)))
		}
		if len(parts) > 0 {
			fmt.Fprintf(w, "  %s; -v lists them\n", strings.Join(parts, ", "))
		}
		return
	}
	if len(unchanged) > 0 {
		// Several modules skip their package and network steps under DryRun
		// rather than probe them, so silence there is not proof.
		label := "Unchanged"
		if r.DryRun {
			label = "Unchanged, as far as a dry run can tell"
		}
		fmt.Fprintf(w, "  %s: %s\n", label, strings.Join(unchanged, ", "))
	}
	if len(skipped) > 0 {
		fmt.Fprintf(w, "  Skipped: %s\n", strings.Join(skipped, ", "))
	}
}

// TildePath shortens every occurrence of the home directory in s to ~.
func TildePath(s string) string {
	home, err := HomeDir()
	if err != nil || home == "" || home == "/" {
		return s
	}
	return strings.ReplaceAll(s, home+"/", "~/")
}
