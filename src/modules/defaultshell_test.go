package modules

// In-package: accountShell is unexported. It decides whether chsh runs at all,
// so reading $SHELL instead re-ran chsh, and its password prompt, every time
// until the next login.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestAccountShell_ReadsPasswdNotSHELL(t *testing.T) {
	if core.IsMac() {
		t.Skip("reads dscl on macOS")
	}
	dir := t.TempDir()
	getent := "#!/bin/sh\n[ \"$1 $2\" = \"passwd tester\" ] && echo 'tester:x:1000:1000::/home/tester:/usr/bin/zsh'\n"
	if err := os.WriteFile(filepath.Join(dir, "getent"), []byte(getent), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("USER", "tester")
	t.Setenv("SHELL", "/bin/bash")

	if got := accountShell(context.Background()); got != "/usr/bin/zsh" {
		t.Fatalf("accountShell = %q, want the passwd entry's /usr/bin/zsh", got)
	}

	t.Setenv("USER", "nobody-here")
	if got := accountShell(context.Background()); got != "/bin/bash" {
		t.Fatalf("with no passwd entry, accountShell = %q, want $SHELL", got)
	}
}
