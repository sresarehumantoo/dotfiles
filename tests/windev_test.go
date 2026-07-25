package tests

import (
	"context"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

// TestWindevModule_DryRunInstall_NoOp verifies the dry-run guard short-circuits
// Install() before any real package or download work, returning nil.
func TestWindevModule_DryRunInstall_NoOp(t *testing.T) {
	core.DryRun = true
	defer func() { core.DryRun = false }()

	m := modules.WindevModule{}
	if err := m.Install(context.Background()); err != nil {
		t.Fatalf("dry-run Install: %v", err)
	}
}

// TestWindevModule_Status_Disabled verifies Status() reports "disabled" with
// zero linked/missing when the opt-in flag isn't set — keeping `status` and
// `diff` quiet for users who haven't enabled the module.
func TestWindevModule_Status_Disabled(t *testing.T) {
	saved := core.Cfg.WindevEnabled
	core.Cfg.WindevEnabled = false
	defer func() { core.Cfg.WindevEnabled = saved }()

	s := modules.WindevModule{}.Status()
	if s.Name != "windev" {
		t.Errorf("Status.Name = %q, want %q", s.Name, "windev")
	}
	if s.Linked != 0 || s.Missing != 0 {
		t.Errorf("Status counts when disabled = %d linked / %d missing, want 0/0", s.Linked, s.Missing)
	}
	if s.Extra != "disabled" {
		t.Errorf("Status.Extra when disabled = %q, want %q", s.Extra, "disabled")
	}
}
