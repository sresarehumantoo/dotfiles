package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An older dfinstall first on PATH (a source build left in ~/.local/bin after
// a package install) is reported with its version.
func TestBinaryOnPathResult_ReportsAShadowingBinary(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "dfinstall")
	if err := os.WriteFile(old, []byte("#!/bin/sh\necho 'dfinstall version v1.9.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := binaryOnPathResult()
	if r.OK {
		t.Fatalf("a different dfinstall first on PATH passed: %+v", r)
	}
	if !strings.Contains(r.Detail, old) || !strings.Contains(r.Detail, "v1.9.0") {
		t.Errorf("detail should name the binary and its version: %q", r.Detail)
	}
}

func TestBinaryOnPathResult_NotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if r := binaryOnPathResult(); !r.OK {
		t.Errorf("no dfinstall on PATH is not a problem: %+v", r)
	}
}
