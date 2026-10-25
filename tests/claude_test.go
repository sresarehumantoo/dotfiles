package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
	"github.com/sresarehumantoo/dotfiles/v2/internal/modules"
)

func claudeSandbox(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	dots := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTFILES", dots)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)
	if err := os.MkdirAll(filepath.Join(dots, "config", "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := `{"attribution": {"commit": "", "pr": ""}, "includeCoAuthoredBy": false}`
	os.WriteFile(filepath.Join(dots, "config", "claude", "settings.json"), []byte(base), 0o644)
	os.WriteFile(filepath.Join(dots, "config", "claude", "CLAUDE.md"), []byte("# rules\n"), 0o644)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestClaudeMergeKeepsUnmanagedKeys(t *testing.T) {
	home := claudeSandbox(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	live := `{"includeCoAuthoredBy": true, "model": "x", "cleanupPeriodDays": 9007199254740993, "autoMode": {"environment": ["a <b> & c"]}}`
	if err := os.WriteFile(settings, []byte(live), 0o640); err != nil {
		t.Fatal(err)
	}

	m := modules.ClaudeModule{}
	if s := m.Status(); s.Extra != "settings.json: 2 managed keys differ" {
		t.Fatalf("Status().Extra = %q, want both managed keys reported stale", s.Extra)
	}
	if err := m.Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, _ := os.ReadFile(settings)
	got := string(data)
	for _, want := range []string{
		`"includeCoAuthoredBy": false`,
		`"commit": ""`,
		`"model": "x"`,
		`9007199254740993`,
		`"a <b> & c"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("merged settings missing %s:\n%s", want, got)
		}
	}
	if fi, _ := os.Stat(settings); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640 preserved", fi.Mode().Perm())
	}
	if s := m.Status(); s.Extra != "" || s.Linked != 1 {
		t.Errorf("after Install: linked=%d extra=%q, want 1 and in sync", s.Linked, s.Extra)
	}
}

func TestClaudeMergeRefusesUnparsableSettings(t *testing.T) {
	home := claudeSandbox(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	broken := `{"model": "x",`
	os.WriteFile(settings, []byte(broken), 0o600)

	if err := (modules.ClaudeModule{}).Install(context.Background()); err == nil {
		t.Fatal("Install succeeded over an unparsable settings.json")
	}
	if data, _ := os.ReadFile(settings); string(data) != broken {
		t.Errorf("unparsable settings.json was rewritten: %q", data)
	}
}

func TestClaudeMergeCreatesMissingSettings(t *testing.T) {
	home := claudeSandbox(t)
	if err := (modules.ClaudeModule{}).Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	settings := filepath.Join(home, ".claude", "settings.json")
	fi, err := os.Stat(settings)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("new settings.json mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestClaudeMergeDryRunWritesNothing(t *testing.T) {
	home := claudeSandbox(t)
	core.DryRun = true
	t.Cleanup(func() { core.DryRun = false })

	if err := (modules.ClaudeModule{}).Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".claude", "settings.json")); err == nil {
		t.Error("dry run created settings.json")
	}
}

func TestClaudeContentDrift(t *testing.T) {
	home := claudeSandbox(t)
	m := modules.ClaudeModule{}
	if d := m.ContentDrift(); d != nil {
		t.Errorf("no settings.json yet: want no drift, got %v", d)
	}
	if err := m.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d := m.ContentDrift(); d != nil {
		t.Errorf("after install: want no drift, got %v", d)
	}
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"attribution": {"commit": "", "pr": ""}, "includeCoAuthoredBy": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := m.ContentDrift()
	if len(d) != 1 || d[0].Dst != settings || d[0].Detail != "managed keys differ: includeCoAuthoredBy" {
		t.Errorf("edited key: got %+v", d)
	}
}

// A home that never had settings.json is not stale, it is not merged yet.
func TestClaudeStatusExtra(t *testing.T) {
	home := claudeSandbox(t)
	m := modules.ClaudeModule{}
	if got := m.Status().Extra; got != "settings.json not merged yet" {
		t.Errorf("no settings.json: Extra = %q", got)
	}
	if err := m.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := m.Status().Extra; got != "" {
		t.Errorf("after install: Extra = %q, want empty", got)
	}
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.WriteFile(settings, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := m.Status().Extra; got != "settings.json: 2 managed keys differ" {
		t.Errorf("both keys removed: Extra = %q", got)
	}
}
