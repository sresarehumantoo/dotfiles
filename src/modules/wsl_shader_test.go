package modules

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// The override file has TWO writers: this package (at install time) and the
// `ghostty-shader` script (for later toggling). They must agree byte for byte.
//
// The coupling is not cosmetic. Both the script's `reset` and this package's
// Uninstall refuse to touch a file that does not carry the managed header, so a
// drifted header means each tool stops recognizing the other's file and quietly
// leaves it behind. Same hazard as a second copy of the toolkit artifact map.
func TestGhosttyOverrideMatchesShellScript(t *testing.T) {
	script := core.ConfigPath("devtools", "ghostty-shader")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("script not found: %v", err)
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	for _, tc := range []struct {
		action  string
		enabled bool
	}{{"on", true}, {"off", false}} {
		t.Run(tc.action, func(t *testing.T) {
			goHome := t.TempDir()
			t.Setenv("HOME", goHome)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(goHome, ".config"))
			if err := writeGhosttyShaderOverride(tc.enabled); err != nil {
				t.Fatalf("go writer: %v", err)
			}
			fromGo, err := os.ReadFile(ghosttyLocalPath())
			if err != nil {
				t.Fatalf("reading go output: %v", err)
			}

			shXDG := filepath.Join(t.TempDir(), "config")
			cmd := exec.CommandContext(t.Context(), "bash", script, tc.action)
			cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+shXDG)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("script failed: %v\n%s", err, out)
			}
			fromSh, err := os.ReadFile(filepath.Join(shXDG, "ghostty", "ghostty.local"))
			if err != nil {
				t.Fatalf("reading script output: %v", err)
			}

			if string(fromGo) != string(fromSh) {
				t.Errorf("writers disagree for %q\n--- go ---\n%s\n--- shell ---\n%s",
					tc.action, fromGo, fromSh)
			}
		})
	}
}
