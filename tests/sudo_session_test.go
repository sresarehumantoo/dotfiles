package tests

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/internal/core"
)

// fakeSudo puts a sudo on PATH that logs its arguments and answers `-n true`
// with cachedExit, so EnsureSudo's decisions can be read back from the log.
func fakeSudo(t *testing.T, cachedExit int) (logPath string) {
	t.Helper()
	if core.IsMac() {
		t.Skip("EnsureSudo never primes on macOS")
	}
	if os.Geteuid() == 0 {
		t.Skip("EnsureSudo never primes as root")
	}
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$*\" >> " + logPath + "\n" +
		"[ \"$*\" = \"-n true\" ] && exit " + strconv.Itoa(cachedExit) + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Cleanup(core.ReleaseSudo)
	return logPath
}

func sudoCalls(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestEnsureSudo_NoEscalationNoCalls(t *testing.T) {
	log := fakeSudo(t, 1)
	core.ReleaseSudo()
	if calls := sudoCalls(t, log); len(calls) != 0 {
		t.Fatalf("a run that never escalates ran sudo: %v", calls)
	}
}

func TestEnsureSudo_PromptsOnceAndDropsItsOwnCredentials(t *testing.T) {
	log := fakeSudo(t, 1)
	ctx := context.Background()
	core.EnsureSudo(ctx)
	core.EnsureSudo(ctx)
	core.ReleaseSudo()
	got := strings.Join(sudoCalls(t, log), "|")
	if want := "-n true|-v|-k"; got != want {
		t.Fatalf("sudo calls = %q, want %q", got, want)
	}
}

func TestEnsureSudo_LeavesPreexistingCredentials(t *testing.T) {
	log := fakeSudo(t, 0)
	core.EnsureSudo(context.Background())
	core.ReleaseSudo()
	got := strings.Join(sudoCalls(t, log), "|")
	if want := "-n true"; got != want {
		t.Fatalf("sudo calls = %q, want %q (credentials it did not obtain must not be dropped)", got, want)
	}
}

func TestEnsureSudo_NonInteractiveNeverPrompts(t *testing.T) {
	log := fakeSudo(t, 1)
	prev := core.Interactive
	core.Interactive = false
	t.Cleanup(func() { core.Interactive = prev })
	core.EnsureSudo(context.Background())
	core.ReleaseSudo()
	got := strings.Join(sudoCalls(t, log), "|")
	if want := "-n true"; got != want {
		t.Fatalf("sudo calls = %q, want %q", got, want)
	}
}

func TestEnsureSudo_RePrimesAfterRelease(t *testing.T) {
	log := fakeSudo(t, 0)
	ctx := context.Background()
	core.EnsureSudo(ctx)
	core.ReleaseSudo()
	core.EnsureSudo(ctx)
	got := strings.Join(sudoCalls(t, log), "|")
	if want := "-n true|-n true"; got != want {
		t.Fatalf("sudo calls = %q, want %q (a long-lived MCP server must re-prime)", got, want)
	}
}

func TestSudoCmd_PrimesLazily(t *testing.T) {
	log := fakeSudo(t, 0)
	cmd := core.SudoCmd(context.Background(), "true")
	if got := strings.Join(sudoCalls(t, log), "|"); got != "-n true" {
		t.Fatalf("SudoCmd should prime before returning; sudo calls = %q", got)
	}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

// The password prompt must say what it is for: it used to print only "This
// step requires sudo access", leaving nothing to judge the request by.
func TestEnsureSudo_PromptNamesTheCommand(t *testing.T) {
	fakeSudo(t, 1)
	out := captureStdout(t, func() {
		core.SudoCmd(context.Background(), "apt-get", "install", "-y", "fzf")
	})
	if want := "needs sudo to run: apt-get install -y fzf"; !strings.Contains(out, want) {
		t.Fatalf("prompt output %q does not contain %q", out, want)
	}
}
