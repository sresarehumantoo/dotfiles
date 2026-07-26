package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func writeRegistry(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return path
}

// `registry validate` is a read-only check, and is routinely pointed at a file
// the user is merely reviewing. Caching it there is enough to change what the
// next `install toolkit` installs, because LoadOrFetchRegistry prefers the
// cache — apt/go/cargo packages and git clones nobody selected.
func TestInspectRegistry_DoesNotWriteCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := writeRegistry(t, validRegistryJSON)

	reg, err := core.InspectRegistry(context.Background(), src)
	if err != nil {
		t.Fatalf("InspectRegistry: %v", err)
	}
	if len(reg.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(reg.Tools))
	}

	if _, err := os.Stat(core.RegistryCachePath()); !os.IsNotExist(err) {
		t.Errorf("InspectRegistry wrote %s; validation must not replace the cache", core.RegistryCachePath())
	}
}

// FetchRegistry still caches — otherwise the test above would pass simply
// because caching had been removed everywhere.
func TestFetchRegistry_WritesCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := writeRegistry(t, validRegistryJSON)

	if _, err := core.FetchRegistry(context.Background(), src); err != nil {
		t.Fatalf("FetchRegistry: %v", err)
	}
	if _, err := os.Stat(core.RegistryCachePath()); err != nil {
		t.Errorf("FetchRegistry should have cached to %s: %v", core.RegistryCachePath(), err)
	}
}

// Validation is a security boundary, and it has to run on the inspect path too.
func TestInspectRegistry_StillValidates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := writeRegistry(t, `{"version":1,"tools":[
	  {"name":"evil","description":"x","category":"c","method":"git_clone","git_repo":"ext::sh -c whoami","binary":"evil"}
	]}`)

	if _, err := core.InspectRegistry(context.Background(), src); err == nil {
		t.Error("InspectRegistry accepted a non-https git_repo; validation must run on this path")
	}
}
