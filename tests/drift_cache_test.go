package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// drift.zsh printed, at the prompt after an install, the drift it had cached
// from before it: "managed keys differ: hooks" for the very settings the run
// had just merged.
func TestClearDriftCache(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	dir := filepath.Join(state, core.AppDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := func() {
		for _, f := range []string{"drift", "drift.checked", "drift.shown"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("stale\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	exists := func(f string) bool {
		_, err := os.Stat(filepath.Join(dir, f))
		return err == nil
	}

	orig := core.DryRun
	t.Cleanup(func() { core.DryRun = orig })

	seed()
	core.DryRun = true
	core.ClearDriftCache()
	if !exists("drift") || !exists("drift.checked") {
		t.Error("a dry run cleared the drift cache")
	}

	core.DryRun = false
	core.ClearDriftCache()
	if exists("drift") || exists("drift.checked") {
		t.Error("the cached result or its timestamp survived")
	}
	// What was last printed stays, so drift.zsh still reports a new finding once.
	if !exists("drift.shown") {
		t.Error("drift.shown was removed")
	}
}
