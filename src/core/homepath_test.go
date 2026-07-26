package core

// In-package: checkTarget is the guard under test and is deliberately
// unexported.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// With $HOME unset, os.UserHomeDir returns ("", err) and the old code joined
// it anyway — filepath.Join("", ".oh-my-zsh") is the *relative* ".oh-my-zsh",
// so every managed path silently retargeted at the current working directory.
func TestHomeTarget_EmptyRatherThanRelative(t *testing.T) {
	t.Setenv("HOME", "")

	if got := HomeTarget(".oh-my-zsh"); got != "" {
		t.Errorf("HomeTarget with no $HOME = %q, want \"\" (a relative path here would target the CWD)", got)
	}

	// Sanity: this is what the old behaviour produced.
	if rel := filepath.Join("", ".oh-my-zsh"); filepath.IsAbs(rel) {
		t.Fatal("premise wrong: Join with an empty home is absolute")
	}
}

func TestHomeTarget_NormalCase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	want := filepath.Join(home, ".config", "x")
	if got := HomeTarget(".config", "x"); got != want {
		t.Errorf("HomeTarget = %q, want %q", got, want)
	}
}

func TestHomeDir_ErrorsWhenUnset(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := HomeDir(); err == nil {
		t.Error("HomeDir returned no error with $HOME unset")
	}
}

// Every filesystem-mutating entry point must reject a path that isn't
// absolute, so an unresolved home can't cause work in the CWD.
func TestMutatingOpsRejectNonAbsolutePaths(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func(string) error
	}{
		{"LinkFile", func(p string) error { return LinkFile(src, p) }},
		{"UnlinkFile", func(p string) error { return UnlinkFile(src, p) }},
		{"EnsureDir", EnsureDir},
		{"RemoveManagedDir", RemoveManagedDir},
	}

	for _, tc := range cases {
		for _, bad := range []string{"", ".oh-my-zsh", ".tmux/plugins", "relative/path"} {
			t.Run(tc.name+"/"+bad, func(t *testing.T) {
				err := tc.call(bad)
				if err == nil {
					t.Fatalf("%s(%q) succeeded; must refuse non-absolute targets", tc.name, bad)
				}
				if !strings.Contains(err.Error(), "non-absolute") && !strings.Contains(err.Error(), "empty target") {
					t.Errorf("%s(%q) error = %v, want a rejection", tc.name, bad, err)
				}
			})
		}
	}
}

// The guard must not get in the way of a normal absolute path.
func TestRemoveManagedDir_RemovesAbsolutePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugins")
	if err := os.MkdirAll(filepath.Join(dir, "tpm"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveManagedDir(dir); err != nil {
		t.Fatalf("RemoveManagedDir: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("directory still present: %v", err)
	}
}
