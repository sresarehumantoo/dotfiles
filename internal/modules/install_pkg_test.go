package modules

// In-package: installPkg and dpkgInstalled are unexported, and whether they
// reach sudo at all is the behavior under test.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// fakeApt puts apt-get, apt-cache, dpkg-query and sudo on PATH. dpkg-query
// reports each package in states with that db:Status-Status, and "not
// installed" for anything else; sudo logs its arguments without running them,
// including EnsureSudo's `-n true` probe.
func fakeApt(t *testing.T, states map[string]string) (sudoLog string) {
	t.Helper()
	if core.IsMac() || core.IsArchBased() {
		t.Skip("exercises the apt path")
	}
	dir := t.TempDir()
	sudoLog = filepath.Join(dir, "sudo.log")
	var cases strings.Builder
	for pkg, state := range states {
		cases.WriteString("  " + pkg + ") printf '" + state + "'; exit 0 ;;\n")
	}
	bins := map[string]string{
		"apt-get":    "exit 0\n",
		"apt-cache":  "echo \"$2 | 1.0 | http://deb.example trixie/main\"\n",
		"sudo":       "echo \"$*\" >> " + sudoLog + "\n",
		"dpkg-query": "case \"$3\" in\n" + cases.String() + "esac\necho \"dpkg-query: no packages found matching $3\" >&2; exit 1\n",
	}
	for name, body := range bins {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":/usr/bin:/bin")
	prev := aptUpdated
	aptUpdated = false
	t.Cleanup(func() { aptUpdated = prev })
	// Each test starts its own sudo session, so EnsureSudo's probe is logged.
	core.ReleaseSudo()
	t.Cleanup(core.ReleaseSudo)
	return sudoLog
}

func sudoCallsIn(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestInstallPkg_AllPresentNeverRunsSudo(t *testing.T) {
	log := fakeApt(t, map[string]string{"jq": "installed", "tree": "installed"})
	if err := installPkg(context.Background(), "jq", "tree"); err != nil {
		t.Fatal(err)
	}
	if got := sudoCallsIn(t, log); got != "" {
		t.Fatalf("sudo ran for packages that are all installed:\n%s", got)
	}
}

func TestInstallPkg_InstallsOnlyMissing(t *testing.T) {
	log := fakeApt(t, map[string]string{"jq": "installed", "nano": "config-files"})
	if err := installPkg(context.Background(), "jq", "tree", "nano"); err != nil {
		t.Fatal(err)
	}
	want := "-n true\napt-get update\napt-get install -y tree nano"
	if got := sudoCallsIn(t, log); got != want {
		t.Fatalf("sudo calls:\n%s\nwant:\n%s", got, want)
	}
}

func TestDpkgInstalled_ConfigFilesIsNotInstalled(t *testing.T) {
	fakeApt(t, map[string]string{"jq": "installed", "nano": "config-files"})
	if !dpkgInstalled("jq") {
		t.Error("jq (installed) reported missing")
	}
	if dpkgInstalled("nano") {
		t.Error("nano (removed, config files left) reported installed")
	}
	if dpkgInstalled("tree") {
		t.Error("tree (never installed) reported installed")
	}
}
