package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func TestLinkSet_ApplyStatusRemove(t *testing.T) {
	src := t.TempDir()
	dstRoot := t.TempDir()

	srcA := filepath.Join(src, "a.conf")
	srcB := filepath.Join(src, "b.conf")
	for _, p := range []string{srcA, srcB} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// A nested destination proves Apply relies on LinkFile's parent-dir
	// creation rather than needing its own EnsureDir.
	ls := core.LinkSet{
		{Src: srcA, Dst: filepath.Join(dstRoot, "a.conf")},
		{Src: srcB, Dst: filepath.Join(dstRoot, "deep", "nested", "b.conf")},
	}

	if s := ls.Status("t"); s.Linked != 0 || s.Missing != 2 {
		t.Fatalf("before Apply: linked=%d missing=%d, want 0/2", s.Linked, s.Missing)
	}

	if err := ls.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	s := ls.Status("t")
	if s.Name != "t" || s.Linked != 2 || s.Missing != 0 {
		t.Fatalf("after Apply: name=%q linked=%d missing=%d, want t/2/0", s.Name, s.Linked, s.Missing)
	}
	for _, l := range ls {
		target, err := os.Readlink(l.Dst)
		if err != nil {
			t.Fatalf("readlink %s: %v", l.Dst, err)
		}
		if target != l.Src {
			t.Errorf("%s -> %s, want %s", l.Dst, target, l.Src)
		}
	}

	if err := ls.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if s := ls.Status("t"); s.Linked != 0 || s.Missing != 2 {
		t.Errorf("after Remove: linked=%d missing=%d, want 0/2", s.Linked, s.Missing)
	}
}

// Remove must not delete a file it doesn't own.
func TestLinkSet_RemoveLeavesForeignFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := filepath.Join(src, "a.conf")
	if err := os.WriteFile(srcFile, []byte("ours"), 0644); err != nil {
		t.Fatal(err)
	}
	usersFile := filepath.Join(dst, "a.conf")
	if err := os.WriteFile(usersFile, []byte("the user's own file"), 0644); err != nil {
		t.Fatal(err)
	}

	ls := core.LinkSet{{Src: srcFile, Dst: usersFile}}
	if err := ls.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	data, err := os.ReadFile(usersFile)
	if err != nil {
		t.Fatalf("Remove deleted a file it did not create: %v", err)
	}
	if string(data) != "the user's own file" {
		t.Errorf("file contents changed: %q", data)
	}
}

// Every module's Status must count exactly the links it exports. These used to
// be written out separately and could disagree — a path changed in Install and
// Links but not Status left Status silently reporting on the wrong set.
//
// Modules that are inactive (ghostty when ghostty isn't installed, windev when
// not opted in) legitimately report 0/0, so those are allowed too.
func TestModuleStatusMatchesExportedLinks(t *testing.T) {
	modules.RegisterAllModules()

	for _, m := range core.AllModules() {
		le, ok := m.(core.LinkExporter)
		if !ok {
			continue
		}
		t.Run(m.Name(), func(t *testing.T) {
			want := len(le.Links())
			s := m.Status()
			counted := s.Linked + s.Missing

			if counted == 0 && want > 0 {
				t.Skipf("module inactive (%s) — reports 0 links", s.Extra)
			}
			if counted != want {
				t.Errorf("Status counts %d links (%d linked + %d missing) but Links() exports %d",
					counted, s.Linked, s.Missing, want)
			}
		})
	}
}
