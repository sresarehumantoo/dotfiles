package modules

import (
	"os"
	"path/filepath"
	"testing"
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
