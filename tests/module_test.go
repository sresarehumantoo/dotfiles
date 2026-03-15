package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func TestRegistryOrder(t *testing.T) {
	modules.RegisterAllModules()

	expected := []string{
		"packages", "extras", "toolkit", "delta", "fonts", "omz",
		"shell", "devtools", "git", "nvim", "tmux",
		"konsole", "ghostty", "htop", "wsl", "defaultshell",
	}

	names := core.ModuleNames()

	// The registry may have been populated by other tests calling RegisterAllModules,
	// so we check that the expected modules appear in the correct relative order.
	if len(names) < len(expected) {
		t.Fatalf("got %d modules, want at least %d", len(names), len(expected))
	}

	// Check the tail of the names list matches expected order
	tail := names[len(names)-len(expected):]
	for i, name := range tail {
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
