package modules

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// A second install must not report rustup as installed again. On macOS it
// did, every run, so `install all` never came back with nothing changed.
func TestInstallRustupBrew_ReportsOnlyARealChange(t *testing.T) {
	prefix := t.TempDir()
	state := filepath.Join(t.TempDir(), "toolchain-set")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(prefix, "bin", "brew"), "#!/bin/sh\nexit 0\n")
	// `rustup default` fails until `rustup default stable` has run once.
	write(filepath.Join(prefix, "opt", "rustup", "bin", "rustup"), `#!/bin/sh
if [ "$1" = default ] && [ $# -eq 1 ]; then test -f "`+state+`"; exit; fi
if [ "$1" = default ]; then touch "`+state+`"; fi
exit 0
`)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", os.Getenv("PATH"))
	write(core.BrewPrefixFile(), prefix+"\n")
	level := core.Level
	core.Level = core.LogVerbose
	t.Cleanup(func() { core.Level = level })

	run := func() string {
		var b bytes.Buffer
		stop := core.CaptureOutput(&b)
		ok := installRustupBrew(t.Context())
		stop()
		if !ok {
			t.Fatalf("installRustupBrew failed:\n%s", b.String())
		}
		return b.String()
	}

	if out := run(); !strings.Contains(out, "rustup installed") {
		t.Errorf("setting the toolchain was not reported as a change:\n%s", out)
	}
	if out := run(); strings.Contains(out, "rustup installed") {
		t.Errorf("a second run reported rustup as installed again:\n%s", out)
	}
}
