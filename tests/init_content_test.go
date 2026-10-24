package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

// gitRepo creates a committed git repository, with a config/ tree when
// withConfig is set, and returns its path for use as a clone URL.
func gitRepo(t *testing.T, withConfig bool) string {
	t.Helper()
	dir := t.TempDir()
	file := "README"
	if withConfig {
		file = filepath.Join("config", "htop", "htoprc")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "init"},
		{"tag", "v9.9.9"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestInitContent_ClonesAndRecordsCheckout(t *testing.T) {
	noCloneHome(t)
	repo := gitRepo(t, true)

	dir, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: repo})
	if err != nil {
		t.Fatalf("InitContent: %v", err)
	}
	if dir != modules.DefaultContentDir() {
		t.Errorf("checkout at %q, want the default %q", dir, modules.DefaultContentDir())
	}
	if _, err := os.Stat(filepath.Join(dir, "config", "htop", "htoprc")); err != nil {
		t.Errorf("checkout has no config tree: %v", err)
	}
	if got := core.ReadCanonicalDir(); got != dir {
		t.Errorf("canonical pointer = %q, want %q", got, dir)
	}
	core.ResetDotfilesDir()
	if got := core.DotfilesDir(); got != dir {
		t.Errorf("DotfilesDir = %q after init, want %q", got, dir)
	}

	// Running it again is a no-op, whatever repo it is handed.
	again, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: "https://example.invalid/none.git"})
	if err != nil || again != dir {
		t.Errorf("second InitContent = (%q, %v), want (%q, nil)", again, err, dir)
	}
}

func TestInitContent_DryRunChangesNothing(t *testing.T) {
	noCloneHome(t)
	saveGlobals(t)
	core.DryRun = true

	if _, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: gitRepo(t, true)}); err != nil {
		t.Fatalf("InitContent: %v", err)
	}
	if _, err := os.Stat(modules.DefaultContentDir()); !os.IsNotExist(err) {
		t.Errorf("dry run created %s (err=%v)", modules.DefaultContentDir(), err)
	}
	if _, err := os.Stat(core.CanonicalPointerPath()); !os.IsNotExist(err) {
		t.Errorf("dry run wrote the canonical pointer (err=%v)", err)
	}
}

func TestInitContent_RefusesNonEmptyDir(t *testing.T) {
	noCloneHome(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: gitRepo(t, true), Dir: dir})
	if err == nil {
		t.Fatal("InitContent cloned into a non-empty directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep")); err != nil {
		t.Errorf("existing file disturbed: %v", err)
	}
}

func TestInitContent_RejectsRepoWithoutConfig(t *testing.T) {
	noCloneHome(t)
	dir := filepath.Join(t.TempDir(), "checkout")

	if _, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: gitRepo(t, false), Dir: dir}); err == nil {
		t.Fatal("InitContent accepted a repository with no config/")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("rejected clone left behind at %s", dir)
	}
	if got := core.ReadCanonicalDir(); got != "" {
		t.Errorf("canonical pointer = %q after a rejected clone", got)
	}
}

func TestInitContent_AdoptsExistingCheckout(t *testing.T) {
	noCloneHome(t)
	clone := makeRepo(t, t.TempDir(), "clone")

	dir, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: "https://example.invalid/none.git", Dir: clone})
	if err != nil || dir != clone {
		t.Fatalf("InitContent = (%q, %v), want (%q, nil)", dir, err, clone)
	}
	if got := core.ReadCanonicalDir(); got != clone {
		t.Errorf("canonical pointer = %q, want %q", got, clone)
	}
}

func TestInitContent_ChecksOutRef(t *testing.T) {
	noCloneHome(t)
	dir := filepath.Join(t.TempDir(), "checkout")

	if _, err := modules.InitContent(context.Background(), modules.InitOptions{Repo: gitRepo(t, true), Ref: "v9.9.9", Dir: dir}); err != nil {
		t.Fatalf("InitContent: %v", err)
	}
	out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--exact-match").Output()
	if err != nil || strings.TrimSpace(string(out)) != "v9.9.9" {
		t.Errorf("checkout is at %q (err=%v), want tag v9.9.9", out, err)
	}
}
