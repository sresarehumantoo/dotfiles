package core

import (
	"strings"
	"testing"
)

func TestSudoReason(t *testing.T) {
	if got := sudoReason([]string{"bash", "-c", "set -e\n  echo x | sudo tee /etc/foo"}); got != "This step needs sudo to run: bash -c set -e echo x | sudo tee /etc/foo" {
		t.Errorf("script not collapsed to one line: %q", got)
	}
	if got := sudoReason(nil); got != "This step needs sudo" {
		t.Errorf("no command: %q", got)
	}
	long := sudoReason([]string{"apt-get", "install", "-y", strings.Repeat("pkg ", 60)})
	if !strings.HasSuffix(long, "…") || len([]rune(long)) > 140 {
		t.Errorf("long command not cut short: %q", long)
	}

	changesMu.Lock()
	activeSession = &InstallSession{current: &ModuleOutcome{Name: "packages"}}
	changesMu.Unlock()
	t.Cleanup(func() {
		changesMu.Lock()
		activeSession = nil
		changesMu.Unlock()
	})
	if got := sudoReason([]string{"apt-get", "update"}); got != "The packages module needs sudo to run: apt-get update" {
		t.Errorf("module not named: %q", got)
	}
}
