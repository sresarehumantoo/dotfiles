package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
	"github.com/sresarehumantoo/dotfiles/v3/internal/modules"
)

// Registering the set again, as many tests here do, adds nothing: every
// module is registered once, in install order.
func TestRegistryOrder(t *testing.T) {
	modules.RegisterAllModules()
	modules.RegisterAllModules()

	expected := []string{
		"locale", "packages", "tealdeer", "extras", "toolkit", "delta", "fonts", "omz",
		"shell", "devtools", "git", "nvim", "windev", "tmux",
		"konsole", "ghostty", "sway", "aerospace", "sketchybar", "mackeys", "alttab", "htop", "newsboat", "docs", "claude", "opencode", "wsl", "vmguest", "defaultshell",
	}

	names := core.ModuleNames()
	if len(names) != len(expected) {
		t.Fatalf("got %d modules, want %d: %v", len(names), len(expected), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("module %d = %q, want %q", i, name, expected[i])
		}
	}
}

func TestGetModule_Found(t *testing.T) {
	modules.RegisterAllModules()

	m, ok := core.GetModule("shell")
	if !ok {
		t.Fatal("expected to find 'shell' module")
	}
	if m.Name() != "shell" {
		t.Errorf("module name = %q, want %q", m.Name(), "shell")
	}
}

func TestGetModule_NotFound(t *testing.T) {
	_, ok := core.GetModule("nonexistent_module_xyz")
	if ok {
		t.Error("expected not to find 'nonexistent_module_xyz' module")
	}
}
