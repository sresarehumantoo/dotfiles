package tests

import (
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// HomeTarget returns "" when $HOME can't be resolved, but that only helps if
// callers propagate the emptiness. filepath.Join("", "custom") is "custom" — a
// relative path that never reaches checkTarget, because git clone, os.WriteFile
// and os.MkdirAll don't go through LinkFile. That is how an unresolved $HOME
// cloned oh-my-zsh into the current working directory.
func TestSubPath_PropagatesEmptyBase(t *testing.T) {
	if got := core.SubPath("", "custom", "plugins"); got != "" {
		t.Errorf("SubPath(\"\", ...) = %q, want \"\" — an empty base must not become a relative path", got)
	}
	// Guard against the bug being "fixed" by making SubPath a no-op.
	if got, want := core.SubPath("/home/u/.oh-my-zsh", "custom"), filepath.Join("/home/u/.oh-my-zsh", "custom"); got != want {
		t.Errorf("SubPath = %q, want %q", got, want)
	}
	if got := core.SubPath("/home/u"); got != "/home/u" {
		t.Errorf("SubPath with no parts = %q, want /home/u", got)
	}
}

// CheckTarget is the refusal LinkFile applies, exported so callers that act on
// a path directly can make it too.
func TestCheckTarget_RefusesEmptyAndRelative(t *testing.T) {
	if err := core.CheckTarget(""); err == nil {
		t.Error("CheckTarget(\"\") should fail — home directory unresolved")
	}
	if err := core.CheckTarget("custom/plugins"); err == nil {
		t.Error("CheckTarget on a relative path should fail — it would land in the CWD")
	}
	if err := core.CheckTarget("/home/u/.config"); err != nil {
		t.Errorf("CheckTarget on an absolute path should pass, got %v", err)
	}
}

// With $HOME unset every home- and XDG-derived path must come out empty rather
// than relative, all the way through the helpers modules build paths with.
func TestPathHelpers_EmptyWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	if got := core.HomeTarget(".oh-my-zsh"); got != "" {
		t.Errorf("HomeTarget = %q, want \"\"", got)
	}
	if got := core.XDGTarget("dfinstall", "plugins.zsh"); got != "" {
		t.Errorf("XDGTarget = %q, want \"\"", got)
	}
	if got := core.SubPath(core.HomeTarget(".oh-my-zsh"), "custom", "plugins"); got != "" {
		t.Errorf("SubPath over an unresolved home = %q, want \"\"", got)
	}
	// The canonical pointer is the worst case: a relative path here would be
	// unreadable from any other directory, so each run would re-adopt whichever
	// clone it happened to be invoked from.
	if got := core.CanonicalPointerPath(); got != "" {
		t.Errorf("CanonicalPointerPath = %q, want \"\"", got)
	}
	if err := core.WriteCanonicalDir("/some/clone"); err == nil {
		t.Error("WriteCanonicalDir should refuse to write when the pointer path is unresolvable")
	}
}
