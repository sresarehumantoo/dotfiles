package core

import (
	"fmt"
	"sync"
)

// installMu serializes whole install sessions. The backup session, the
// canonical pointer, and Cfg are all process-global, so two installs running at
// once corrupt each other: the second StartBackup replaces the first's manager,
// and the first FinishBackup then writes its manifest into the second's
// directory, leaving the first's copied files unlisted and unrestorable.
//
// One process runs one install at a time; a second BeginInstall queues here.
var installMu sync.Mutex

// InstallOptions configures an install session.
type InstallOptions struct {
	// All is true for `install all`, which additionally adopts the invoking
	// clone as this machine's canonical dotfiles dir.
	All bool
	// ForceBackup corresponds to --backup: snapshot regardless of the
	// skip_backup config preference.
	ForceBackup bool
	// ContentOnly is for init, which installs content but replaces nothing
	// in the user's home: no snapshot unless ForceBackup asks for one, and a
	// first run's automatic backup stays armed for the install that does.
	ContentOnly bool
}

// InstallSession owns the setup and teardown shared by every install path —
// canonical adoption, the backup lifecycle, and config persistence. It performs side effects and
// reports what it did; rendering stays with the caller.
//
// Usage:
//
//	sess, err := core.BeginInstall(core.InstallOptions{All: true})
//	if err != nil { return err }
//	defer sess.Finish()
type InstallSession struct {
	doBackup bool
	firstRun bool
	failed   bool
	finished bool

	// CanonicalPrev is the previously-recorded canonical clone when an
	// `install all` moved the pointer; empty when it was already correct or
	// had never been set. CanonicalNow is the clone now recorded.
	CanonicalPrev string
	CanonicalNow  string

	outcomes []*ModuleOutcome
	current  *ModuleOutcome
}

// BeginInstall performs the pre-install work shared by all install paths.
func BeginInstall(opt InstallOptions) (*InstallSession, error) {
	installMu.Lock()
	s := &InstallSession{}

	// Published so a module can tell "you asked for me specifically" from
	// "you asked for everything". Set here rather than in the CLI because
	// this is the one path every install goes through.
	//
	// The sway module is the current consumer: installing a compositor and a
	// bar is right when someone types `install sway`, and wrong to do
	// silently to a WSL box or a server during `install all`.
	InstallingAll = opt.All

	// A binary running from no clone (a release or package) keeps whichever
	// checkout `init` recorded.
	if dir, _ := invokingClone(); opt.All && dir != "" && AllInstallClone() == "" {
		Notice("using packaged content, not the clone at %s; to switch to it: dfinstall init --dir %s", dir, dir)
	}
	if invoking := AllInstallClone(); opt.All && invoking != "" {
		switch {
		case DryRun:
			// The canonical pointer is machine-global, so adopting during a
			// preview would make whichever clone you previewed from
			// authoritative for every later run. Report the change instead of
			// making it; links below still preview against the current
			// canonical clone, which is where an unconfirmed run leaves them.
			if prev := ReadCanonicalDir(); prev != invoking {
				s.CanonicalPrev = prev
				s.CanonicalNow = invoking
			}
		default:
			// Adopting first means DotfilesDir() resolves here for the rest of
			// the run, so the module loop repoints symlinks left pointing at
			// other clones — `install all` both switches canonical and
			// consolidates a machine whose links had drifted.
			if prev, changed := AdoptCanonical(invoking); changed {
				s.CanonicalPrev = prev
				s.CanonicalNow = invoking
			}
		}
	}

	s.doBackup, s.firstRun = shouldBackup(opt.ForceBackup)
	if opt.ContentOnly && !opt.ForceBackup {
		s.doBackup, s.firstRun = false, false
	}
	if s.doBackup {
		if err := StartBackup(); err != nil {
			// No session is returned, so no Finish will run to release this.
			installMu.Unlock()
			return nil, fmt.Errorf("start backup: %w", err)
		}
	}
	changesMu.Lock()
	activeSession = s
	changesMu.Unlock()
	return s, nil
}

// AllInstallClone is the clone `install all` adopts: the one it runs from,
// unless that is only the working directory while packaged content is in use.
// Packaged content is left by an explicit `init --dir`, never because install
// all ran from inside an old clone. "" when there is none to adopt.
func AllInstallClone() string {
	dir, fromCwd := invokingClone()
	if fromCwd && ReadCanonicalDir() == ContentCurrentDir() {
		return ""
	}
	return dir
}

// DidBackup reports whether a restorable snapshot was taken.
func (s *InstallSession) DidBackup() bool { return s.doBackup }

// MarkFailed records that the install did not succeed, so Finish leaves
// persisted state alone. Without it a failed first run still writes
// skip_backup: true, and the next run — the one that actually replaces the
// user's dotfiles — silently takes no backup.
//
// **Every install path must call this on its error path**, including the
// install-all loops, which must call it when *any* module failed. The
// asymmetry is deliberate: an unnecessary extra backup costs some disk, a
// missing one costs the user's dotfiles. `install all` was missed when this
// was introduced, so a first run where every module failed still disarmed the
// backup.
func (s *InstallSession) MarkFailed() { s.failed = true }

// Finish closes the backup, persists config, and releases the install lock.
// Safe to defer, and idempotent so a double Finish can't unlock twice.
func (s *InstallSession) Finish() {
	if s.finished {
		return
	}
	s.finished = true
	defer installMu.Unlock()
	// Reset with the lock still held. Leaving it set would make the NEXT
	// session inherit this one's answer.
	defer func() { InstallingAll = false }()
	defer func() {
		changesMu.Lock()
		activeSession = nil
		changesMu.Unlock()
	}()

	if s.doBackup {
		if err := FinishBackup(); err != nil {
			Warn("failed to finish backup: %v", err)
		}
	}
	s.saveConfig()
	ClearDriftCache()
}

func (s *InstallSession) saveConfig() {
	// A preview must never mutate persisted state.
	if DryRun {
		return
	}

	// Neither must a run that accomplished nothing: a failed first run has to
	// stay a first run so its automatic backup is still armed next time.
	if s.failed {
		return
	}

	if s.firstRun {
		Cfg.SkipBackup = true
		if err := SaveConfig(); err != nil {
			Warn("failed to save config: %v", err)
		} else {
			Info("config saved: %s (skip_backup: true)", ConfigFilePath())
		}
		return
	}

	// The opt-in modes mutate Cfg in memory; persist so the choice sticks.
	// OptInMode belongs here alongside the other two: omitting it meant
	// `dfinstall install windev` recorded the opt-in only in verbose mode.
	if ExtendedMode || ToolkitMode || OptInMode {
		if err := SaveConfig(); err != nil {
			Warn("failed to save config: %v", err)
		}
	}
}

// shouldBackup decides whether to snapshot and whether this is a first run.
func shouldBackup(force bool) (doBackup, firstRun bool) {
	if DryRun {
		return false, false
	}
	// --backup always wins.
	if force {
		return true, false
	}
	// No config file → first run, auto-backup.
	if !CfgFileExists {
		Info("first run detected, creating automatic backup")
		return true, true
	}
	// Config exists — respect the skip_backup preference.
	if !Cfg.SkipBackup {
		return true, false
	}
	return false, false
}
