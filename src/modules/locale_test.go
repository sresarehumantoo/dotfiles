package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// TestLocaleGenerated_Spellings pins both `locale -a` spellings. glibc prints
// "en_US.utf8" but macOS prints "en_US.UTF-8", and only the target used to be
// stripped of its hyphen, so on a Mac the locale never matched.
func TestLocaleGenerated_Spellings(t *testing.T) {
	for _, listed := range []string{"en_US.utf8", "en_US.UTF-8"} {
		t.Run(listed, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\nprintf 'C\\nPOSIX\\n%s\\n' '" + listed + "'\n"
			if err := os.WriteFile(filepath.Join(dir, "locale"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

			if !localeGenerated("en_US.UTF-8") {
				t.Errorf("locale -a listing %q: en_US.UTF-8 not found", listed)
			}
			if localeGenerated("de_DE.UTF-8") {
				t.Errorf("locale -a listing %q: de_DE.UTF-8 reported present", listed)
			}
		})
	}
}

// TestLocale_NoRootOnABareImage: a minimal image lists only C.utf8. That is
// enough for UTF-8 shells, so the module reports it and runs nothing as root.
func TestLocale_NoRootOnABareImage(t *testing.T) {
	if core.IsMac() {
		t.Skip("macOS always lists en_US.UTF-8")
	}
	cases := []struct {
		listing     string
		wantLinked  int
		wantMissing int
	}{
		{"C\\nC.utf8\\nPOSIX\\n", 1, 0},
		{"C\\nPOSIX\\n", 0, 1},
	}
	for _, c := range cases {
		dir := t.TempDir()
		sudoLog := filepath.Join(dir, "sudo.log")
		files := map[string]string{
			"locale": "#!/bin/sh\nprintf '" + c.listing + "'\n",
			"sudo":   "#!/bin/sh\necho \"$*\" >> " + sudoLog + "\n",
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

		if err := (LocaleModule{}).Install(context.Background()); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(sudoLog); err == nil {
			t.Errorf("listing %q: sudo ran: %s", c.listing, data)
		}
		s := (LocaleModule{}).Status()
		if s.Linked != c.wantLinked || s.Missing != c.wantMissing {
			t.Errorf("listing %q: status linked=%d missing=%d, want %d/%d", c.listing, s.Linked, s.Missing, c.wantLinked, c.wantMissing)
		}
	}
}
