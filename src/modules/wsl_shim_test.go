package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// newFakeWindows stands up a fake Windows tree plus a `wslpath` on PATH that
// translates into it, so the real winToLinux/winBinary/installWinShims code
// runs unmodified. Returns the fake mount root.
//
// This is the only way to exercise the shim path off WSL, and it is worth
// doing: the shims are what keeps appendWindowsPath=false from breaking
// demorec, wsl-restart and the module's own interop.
func newFakeWindows(t *testing.T, exes map[string]string) string {
	t.Helper()

	root := t.TempDir()
	mnt := filepath.Join(root, "mnt", "c")

	for winRel, name := range exes {
		// Keys are written in Windows form for readability. filepath.Join does
		// NOT split on backslashes on Linux, so translate first or this creates
		// one directory literally named `Windows\System32`.
		dir := filepath.Join(mnt, filepath.Join(strings.Split(winRel, `\`)...))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		// ⚠ The fake target must NOT start with the bytes "MZ". This box (and
		// any Debian with wine installed) registers a binfmt_misc handler on
		// magic 4d5a, so an MZ-prefixed file handed to exec() is routed to
		// run-detectors/wine, which hung this test for 150s before it was
		// killed. A plain shell script both avoids that and lets the exec check
		// below actually prove argument forwarding.
		body := "#!/bin/sh\necho \"fake " + name + " args: $*\"\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write fake exe: %v", err)
		}
	}

	// Minimal wslpath: C:\a\b -> <mnt>/a/b
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	script := "#!/bin/sh\n" +
		"p=\"$2\"\n" +
		"p=$(printf '%s' \"$p\" | sed 's|^C:||' | tr '\\\\' '/')\n" +
		"printf '%s%s\\n' \"" + mnt + "\" \"$p\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "wslpath"), []byte(script), 0o755); err != nil {
		t.Fatalf("write wslpath: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// winToLinux memoizes; a stale entry from another test would poison this one.
	winPathCacheMu.Lock()
	winPathCache = map[string]string{}
	winPathCacheMu.Unlock()
	t.Cleanup(func() {
		winPathCacheMu.Lock()
		winPathCache = map[string]string{}
		winPathCacheMu.Unlock()
	})

	return mnt
}

func TestWinBinaryResolvesThroughWslpath(t *testing.T) {
	mnt := newFakeWindows(t, map[string]string{
		`Windows\System32`:                        "cmd.exe",
		`Windows\System32\WindowsPowerShell\v1.0`: "powershell.exe",
	})

	got := winBinary("cmd.exe")
	want := filepath.Join(mnt, "Windows", "System32", "cmd.exe")
	if got != want {
		t.Errorf("winBinary(cmd.exe) = %q, want %q", got, want)
	}

	// powershell.exe is NOT in System32; resolving it proves the search walks
	// past the first directory rather than giving up.
	got = winBinary("powershell.exe")
	want = filepath.Join(mnt, "Windows", "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if got != want {
		t.Errorf("winBinary(powershell.exe) = %q, want %q", got, want)
	}

	if got := winBinary("definitely-not-here.exe"); got != "" {
		t.Errorf("expected empty for a missing binary, got %q", got)
	}
}

func TestWinShimLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mnt := newFakeWindows(t, map[string]string{
		`Windows\System32`: "cmd.exe",
		`Windows`:          "explorer.exe",
	})

	written := installWinShims()
	if len(written) == 0 {
		t.Fatal("no shims written")
	}

	shim := filepath.Join(home, ".local", "bin", "cmd.exe")
	data, err := os.ReadFile(shim)
	if err != nil {
		t.Fatalf("reading shim: %v", err)
	}
	body := string(data)

	target := filepath.Join(mnt, "Windows", "System32", "cmd.exe")
	if !strings.Contains(body, target) {
		t.Errorf("shim does not point at the resolved target %q:\n%s", target, body)
	}
	if !strings.Contains(body, shimHeader) {
		t.Error("shim missing managed header")
	}

	fi, err := os.Stat(shim)
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("shim is not executable (mode %v)", fi.Mode().Perm())
	}

	// The shim must actually run and forward its arguments through to the
	// target. This is the property that keeps `taskkill.exe /PID x /T /F` and
	// `powershell.exe -NoProfile -Command ...` working once the Windows PATH
	// is gone, so it is worth executing rather than just reading the file.
	out, err := runProbe(t.Context(), shim, "one", "two three")
	if err != nil {
		t.Fatalf("shim failed to execute: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "fake cmd.exe args: one two three" {
		t.Errorf("shim did not forward arguments: %q", got)
	}

	// Idempotent: a second install must not churn the file.
	before, _ := os.ReadFile(shim)
	installWinShims()
	after, _ := os.ReadFile(shim)
	if string(before) != string(after) {
		t.Error("second install rewrote an identical shim")
	}

	present, expected := countWinShims()
	if expected == 0 || present != expected {
		t.Errorf("countWinShims() = (%d, %d), want present == expected > 0", present, expected)
	}

	if n := removeWinShims(); n == 0 {
		t.Error("removeWinShims removed nothing")
	}
	if _, err := os.Stat(shim); !os.IsNotExist(err) {
		t.Error("shim survived removal")
	}
}

// A user's own wrapper in ~/.local/bin must survive both install and uninstall.
// Clobbering it would be silent data loss in a directory dfinstall shares with
// hand-written scripts.
func TestWinShimsDoNotClobberForeignFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	newFakeWindows(t, map[string]string{`Windows\System32`: "cmd.exe"})

	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(binDir, "cmd.exe")
	const userContent = "#!/bin/sh\n# my own wrapper, hands off\n"
	if err := os.WriteFile(foreign, []byte(userContent), 0o755); err != nil {
		t.Fatal(err)
	}

	installWinShims()
	got, _ := os.ReadFile(foreign)
	if string(got) != userContent {
		t.Errorf("install overwrote a foreign file:\n%s", got)
	}

	removeWinShims()
	if _, err := os.Stat(foreign); err != nil {
		t.Error("uninstall deleted a foreign file it did not create")
	}
}

// --dry-run must not write, remove, or otherwise touch anything. Install()
// returns before reaching these today, but os.WriteFile is not guarded the way
// core.LinkFile is, so the guard belongs on the functions themselves.
func TestWinShimsRespectDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	newFakeWindows(t, map[string]string{`Windows\System32`: "cmd.exe"})

	core.DryRun = true
	defer func() { core.DryRun = false }()

	if written := installWinShims(); len(written) != 0 {
		t.Errorf("dry run reported writing shims: %v", written)
	}
	shim := filepath.Join(home, ".local", "bin", "cmd.exe")
	if _, err := os.Stat(shim); !os.IsNotExist(err) {
		t.Error("dry run created a shim on disk")
	}

	// And a dry-run uninstall must not delete a real one.
	core.DryRun = false
	installWinShims()
	if _, err := os.Stat(shim); err != nil {
		t.Fatalf("setup: shim not created: %v", err)
	}
	core.DryRun = true
	if n := removeWinShims(); n != 0 {
		t.Errorf("dry run reported removing %d shim(s)", n)
	}
	if _, err := os.Stat(shim); err != nil {
		t.Error("dry run deleted a shim")
	}
}

func TestGhosttyShaderOverrideRespectsDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	core.DryRun = true
	defer func() { core.DryRun = false }()

	if err := writeGhosttyShaderOverride(false); err != nil {
		t.Fatalf("dry run returned an error: %v", err)
	}
	if _, err := os.Stat(ghosttyLocalPath()); !os.IsNotExist(err) {
		t.Error("dry run wrote the ghostty override")
	}
}

// With no wslpath and no Windows tree, nothing should be written and nothing
// should panic. This is the every-non-WSL-machine case.
func TestWinShimsNoopWithoutWindows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // empty PATH: no wslpath

	winPathCacheMu.Lock()
	winPathCache = map[string]string{}
	winPathCacheMu.Unlock()

	if written := installWinShims(); len(written) != 0 {
		t.Errorf("wrote shims with no Windows present: %v", written)
	}
	if present, expected := countWinShims(); present != 0 || expected != 0 {
		t.Errorf("countWinShims() = (%d, %d), want (0, 0)", present, expected)
	}
}
