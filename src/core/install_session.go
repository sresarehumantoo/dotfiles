package core

import "fmt"

// InstallOptions configures an install session.
type InstallOptions struct {
	// All is true for `install all`, which additionally adopts the invoking
	// clone as this machine's canonical dotfiles dir.
	All bool
	// ForceBackup corresponds to --backup: snapshot regardless of the
	// skip_backup config preference.
	ForceBackup bool
}

// InstallSession owns the setup and teardown shared by every install path —
// canonical adoption, the backup lifecycle, and config persistence — so the CLI
// and the MCP server cannot drift apart on it. It performs side effects and
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

	// CanonicalPrev is the previously-recorded canonical clone when an
	// `install all` moved the pointer; empty when it was already correct or
	// had never been set. CanonicalNow is the clone now recorded.
	CanonicalPrev string
	CanonicalNow  string
}

// BeginInstall performs the pre-install work shared by all install paths.
func BeginInstall(opt InstallOptions) (*InstallSession, error) {
	s := &InstallSession{}

	if opt.All {
		// Adopting first means DotfilesDir() resolves here for the rest of the
		// run, so the module loop repoints symlinks left pointing at other
		// clones — `install all` both switches canonical and consolidates a
		// machine whose links had drifted.
		invoking := InvokingCloneDir()
		if prev, changed := AdoptCanonical(invoking); changed {
			s.CanonicalPrev = prev
			s.CanonicalNow = invoking
		}
	}

	s.doBackup, s.firstRun = shouldBackup(opt.ForceBackup)
	if s.doBackup {
		if err := StartBackup(); err != nil {
			return nil, fmt.Errorf("start backup: %w", err)
		}
	}
	return s, nil
}

// DidBackup reports whether a restorable snapshot was taken.
func (s *InstallSession) DidBackup() bool { return s.doBackup }

// Finish closes the backup and persists config. Safe to defer.
func (s *InstallSession) Finish() {
	if s.doBackup {
		if err := FinishBackup(); err != nil {
			Warn("failed to finish backup: %v", err)
		}
	}
	s.saveConfig()
}

func (s *InstallSession) saveConfig() {
	// A preview must never mutate persisted state.
	if DryRun {
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
	// WindevMode belongs here alongside the other two: omitting it meant
	// `dfinstall install windev` recorded the opt-in only in verbose mode.
	if ExtendedMode || ToolkitMode || WindevMode {
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
		Info("first run detected — creating automatic backup")
		return true, true
	}
	// Config exists — respect the skip_backup preference.
	if !Cfg.SkipBackup {
		return true, false
	}
	return false, false
}
