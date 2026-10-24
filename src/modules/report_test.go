package modules

import (
	"context"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type fakeModule struct {
	name        string
	status      core.ModuleStatus
	unsupported string
	statusRan   *bool
}

func (f fakeModule) Name() string                  { return f.name }
func (f fakeModule) Install(context.Context) error { return nil }
func (f fakeModule) Unsupported() string           { return f.unsupported }
func (f fakeModule) Status() core.ModuleStatus {
	if f.statusRan != nil {
		*f.statusRan = true
	}
	s := f.status
	s.Name = f.name
	return s
}

func TestStatusRowState(t *testing.T) {
	orig := core.Cfg.SkipModules
	defer func() { core.Cfg.SkipModules = orig }()
	core.Cfg.SkipModules = []string{"skipme"}

	tests := []struct {
		name string
		m    fakeModule
		want StatusState
	}{
		{"all in place", fakeModule{name: "a", status: core.ModuleStatus{Linked: 3}}, StateInstalled},
		{"some missing", fakeModule{name: "b", status: core.ModuleStatus{Linked: 2, Missing: 1}}, StatePartial},
		{"all missing", fakeModule{name: "c", status: core.ModuleStatus{Missing: 4}}, StateNotInstalled},
		{"nothing reported", fakeModule{name: "d"}, StateNotInstalled},
		{"skip_modules", fakeModule{name: "skipme", status: core.ModuleStatus{Linked: 1}}, StateSkipped},
		{"other platform", fakeModule{name: "e", unsupported: "macOS only"}, StateUnavailable},
		{"other platform wins over skip", fakeModule{name: "skipme", unsupported: "WSL only"}, StateUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusRow(tt.m).State; got != tt.want {
				t.Errorf("state = %d, want %d", got, tt.want)
			}
		})
	}
}

// A module for another platform may probe tools that platform alone has, so
// Status must not run for it at all.
func TestStatusRowSkipsStatusWhenUnsupported(t *testing.T) {
	ran := false
	r := statusRow(fakeModule{name: "mac", unsupported: "macOS only", statusRan: &ran})
	if ran {
		t.Error("Status ran for an unsupported module")
	}
	if r.Reason != "macOS only" {
		t.Errorf("Reason = %q, want %q", r.Reason, "macOS only")
	}
}
