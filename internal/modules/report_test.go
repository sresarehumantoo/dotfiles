package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
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
	orig, origTools := core.Cfg.SkipModules, core.Cfg.ToolkitTools
	defer func() { core.Cfg.SkipModules, core.Cfg.ToolkitTools = orig, origTools }()
	core.Cfg.SkipModules = []string{"skipme"}
	core.Cfg.ToolkitTools = nil

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
		{"toolkit, nothing selected", fakeModule{name: "toolkit"}, StateSkipped},
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
// With tools selected but none in place, toolkit is a missing install again.
func TestStatusRowToolkitWithSelection(t *testing.T) {
	orig := core.Cfg.ToolkitTools
	defer func() { core.Cfg.ToolkitTools = orig }()
	core.Cfg.ToolkitTools = []string{"ripgrep"}
	if got := statusRow(fakeModule{name: "toolkit", status: core.ModuleStatus{Missing: 1}}).State; got != StateNotInstalled {
		t.Errorf("state = %d, want StateNotInstalled", got)
	}
}

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

type linkedFake struct {
	fakeModule
	links core.LinkSet
}

func (l linkedFake) Links() core.LinkSet { return l.links }

// A partial module names the links that are not in place, so status can say
// what is missing without a trip to diff.
func TestStatusRowListsPartialProblems(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ok, gone := filepath.Join(dir, "ok"), filepath.Join(dir, "gone")
	if err := os.Symlink(src, ok); err != nil {
		t.Fatal(err)
	}
	m := linkedFake{
		fakeModule: fakeModule{name: "p", status: core.ModuleStatus{Linked: 1, Missing: 1}},
		links:      core.LinkSet{{Src: src, Dst: ok}, {Src: src, Dst: gone}},
	}
	r := statusRow(m)
	if r.State != StatePartial {
		t.Fatalf("state = %d, want StatePartial", r.State)
	}
	if len(r.Problems) != 1 || r.Problems[0].Dst != gone || r.Problems[0].State != "missing" {
		t.Errorf("Problems = %+v, want one missing %s", r.Problems, gone)
	}
}
