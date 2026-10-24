package modules

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// An installed switcher must count its agent and binary, or status files it
// under "Not installed".
func TestAlttabStatusCountsInstall(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("alttab reports nothing off macOS")
	}
	t.Setenv("HOME", t.TempDir())
	touch := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if s := (AlttabModule{}).Status(); s.Linked != 0 || s.Missing != 0 {
		t.Errorf("not installed: got %d/%d, want 0/0", s.Linked, s.Missing)
	}

	touch(alttabAgent())
	if s := (AlttabModule{}).Status(); s.Linked != 1 || s.Missing != 1 {
		t.Errorf("agent without binary: got linked %d missing %d, want 1 and 1", s.Linked, s.Missing)
	}

	touch(alttabBinary())
	s := (AlttabModule{}).Status()
	if s.Linked != 2 || s.Missing != 0 {
		t.Errorf("installed: got linked %d missing %d, want 2 and 0", s.Linked, s.Missing)
	}
	if got := statusRow(AlttabModule{}).State; got != StateInstalled {
		t.Errorf("installed alttab is in state %d, want StateInstalled", got)
	}
}
