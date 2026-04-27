package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// withFakeBins replaces PATH with a temp dir containing executable stubs
// for the given names. Returns the directory so tests can add more stubs.
func withFakeBins(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatalf("write fake bin %s: %v", n, err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

func TestAptBin_PrefersAptGet(t *testing.T) {
	withFakeBins(t, "apt-get", "apt")
	if got := core.AptBin(); got != "apt-get" {
		t.Errorf("expected apt-get when both are present, got %q", got)
	}
}

func TestAptBin_FallsBackToApt(t *testing.T) {
	withFakeBins(t, "apt")
	if got := core.AptBin(); got != "apt" {
		t.Errorf("expected apt when apt-get is absent, got %q", got)
	}
}

func TestAptBin_EmptyWhenNeitherPresent(t *testing.T) {
	withFakeBins(t) // empty dir on PATH
	if got := core.AptBin(); got != "" {
		t.Errorf("expected empty string when neither is on PATH, got %q", got)
	}
}
