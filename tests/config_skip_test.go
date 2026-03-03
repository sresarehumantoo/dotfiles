package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestIsModuleSkipped(t *testing.T) {
	orig := core.Cfg.SkipModules
	defer func() { core.Cfg.SkipModules = orig }()

	core.Cfg.SkipModules = []string{"wsl", "fonts"}

	if !core.IsModuleSkipped("wsl") {
		t.Error("expected wsl to be skipped")
	}
	if !core.IsModuleSkipped("fonts") {
		t.Error("expected fonts to be skipped")
	}
	if core.IsModuleSkipped("shell") {
		t.Error("expected shell to NOT be skipped")
	}
	if core.IsModuleSkipped("git") {
		t.Error("expected git to NOT be skipped")
	}
}

func TestIsModuleSkipped_Empty(t *testing.T) {
	orig := core.Cfg.SkipModules
	defer func() { core.Cfg.SkipModules = orig }()

	core.Cfg.SkipModules = nil

	if core.IsModuleSkipped("wsl") {
		t.Error("expected no modules to be skipped with empty list")
	}
}
