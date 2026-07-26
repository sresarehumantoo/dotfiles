package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// SkipInAll is the single skip policy for `install all`. The MCP server used
// to call IsModuleSkipped instead, which ignores the windev opt-in and so
// installed a heavy toolchain on machines that never enabled it.
func TestSkipInAll_WindevRequiresOptIn(t *testing.T) {
	origSkip := core.Cfg.SkipModules
	origWindev := core.Cfg.WindevEnabled
	defer func() {
		core.Cfg.SkipModules = origSkip
		core.Cfg.WindevEnabled = origWindev
	}()

	core.Cfg.SkipModules = nil

	core.Cfg.WindevEnabled = false
	if !core.SkipInAll("windev") {
		t.Error("windev must be skipped in `install all` until explicitly enabled")
	}
	if core.SkipInAll("shell") {
		t.Error("shell should not be skipped")
	}

	core.Cfg.WindevEnabled = true
	if core.SkipInAll("windev") {
		t.Error("windev should be included once opted in")
	}
}

func TestSkipInAll_HonorsSkipModules(t *testing.T) {
	origSkip := core.Cfg.SkipModules
	origWindev := core.Cfg.WindevEnabled
	defer func() {
		core.Cfg.SkipModules = origSkip
		core.Cfg.WindevEnabled = origWindev
	}()

	core.Cfg.SkipModules = []string{"fonts"}
	core.Cfg.WindevEnabled = true

	if !core.SkipInAll("fonts") {
		t.Error("expected fonts to be skipped via SkipModules")
	}
	if core.SkipInAll("git") {
		t.Error("expected git to NOT be skipped")
	}
}

// BeginInstall owns the backup decision for both the CLI and the MCP server.
func TestBeginInstall_BackupPolicy(t *testing.T) {
	cases := []struct {
		name       string
		dryRun     bool
		force      bool
		cfgExists  bool
		skipBackup bool
		want       bool
	}{
		{"dry run never snapshots", true, false, true, false, false},
		{"--backup overrides skip_backup", false, true, true, true, true},
		{"first run auto-snapshots", false, false, false, true, true},
		{"skip_backup false snapshots", false, false, true, false, true},
		{"skip_backup true does not", false, false, true, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			// Keep SaveConfig away from the real repo's .config.yaml.
			t.Setenv("DOTFILES", t.TempDir())
			core.ResetDotfilesDir()
			defer core.ResetDotfilesDir()

			origDry, origExists, origCfg := core.DryRun, core.CfgFileExists, core.Cfg
			defer func() {
				core.DryRun, core.CfgFileExists, core.Cfg = origDry, origExists, origCfg
			}()

			core.DryRun = tc.dryRun
			core.CfgFileExists = tc.cfgExists
			core.Cfg.SkipBackup = tc.skipBackup

			sess, err := core.BeginInstall(core.InstallOptions{ForceBackup: tc.force})
			if err != nil {
				t.Fatalf("BeginInstall: %v", err)
			}
			if got := sess.DidBackup(); got != tc.want {
				t.Errorf("DidBackup() = %v, want %v", got, tc.want)
			}
			sess.Finish()
		})
	}
}
