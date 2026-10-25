package modules

// In-package: nvimOwner and oldNvimRemoval are unexported, and decide whether
// an upgrade replaces dfinstall's own install or leaves a system one shadowed.

import (
	"path/filepath"
	"testing"
)

func TestNvimOwner_Prebuilts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	local := filepath.Join(home, ".local", "opt", "nvim-linux-x86_64", "bin", "nvim")
	if got := nvimOwner(local); got != "local-prebuilt" {
		t.Errorf("nvimOwner(%s) = %q, want local-prebuilt", local, got)
	}
	if got := nvimOwner("/opt/nvim-linux-x86_64/bin/nvim"); got != "opt-prebuilt" {
		t.Errorf("nvimOwner(/opt/...) = %q, want opt-prebuilt", got)
	}
}

func TestOldNvimRemoval(t *testing.T) {
	cases := map[string]string{
		"local-prebuilt": "",
		"manual":         "",
		"apt:neovim":     "sudo apt remove neovim",
		"opt-prebuilt":   "sudo rm -rf /opt/nvim-linux-* /usr/local/bin/nvim",
	}
	for owner, want := range cases {
		if got := oldNvimRemoval(owner); got != want {
			t.Errorf("oldNvimRemoval(%q) = %q, want %q", owner, got, want)
		}
	}
}
