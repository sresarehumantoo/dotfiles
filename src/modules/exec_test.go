package modules

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// TestProbeErr_CarriesStderr pins that a failed probe explains itself. Cmd.Output()
// stashes stderr on ExitError but renders only "exit status N", so the fonts
// module's release fetch used to fail with a bare `exit status 22` — and curl
// returns 22 for every HTTP status >= 400 alike, so the exit code alone
// distinguishes nothing. Run against the pre-fix tree this fails.
func TestProbeErr_CarriesStderr(t *testing.T) {
	ctx := context.Background()

	_, err := runProbe(ctx, "sh", "-c", "echo 'curl: (22) The requested URL returned error: 404' >&2; exit 22")
	if err == nil {
		t.Fatal("want an error from a failing probe")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not carry the stderr reason", err)
	}
	// The exit status must still be visible — the reason supplements it.
	if !strings.Contains(err.Error(), "exit status 22") {
		t.Errorf("error %q lost the exit status", err)
	}

	// Same for the network probe, which is what the release fetch actually uses.
	_, err = runNetProbe(ctx, "sh", "-c", "echo 'boom-from-net' >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "boom-from-net") {
		t.Errorf("runNetProbe error = %v, want it to carry stderr", err)
	}

	// A silent failure must not gain noise, and success must stay clean.
	if _, err := runProbe(ctx, "sh", "-c", "exit 3"); err == nil ||
		strings.Contains(err.Error(), ":") && strings.Count(err.Error(), ":") > 1 {
		t.Errorf("silent failure should stay a bare exit error, got %v", err)
	}
	out, err := runProbe(ctx, "sh", "-c", "echo hello")
	if err != nil || strings.TrimSpace(string(out)) != "hello" {
		t.Errorf("successful probe: out=%q err=%v", out, err)
	}
}

// TestRunNetCmd_UsesNetworkDeadline guards the timeout class a download runs
// under. The font archive fetch went through runCmd, so a stalled mirror was
// bounded by InstallTimeout (45m) rather than NetworkTimeout (10m) — long
// enough that the run looks hung.
func TestRunNetCmd_UsesNetworkDeadline(t *testing.T) {
	if core.NetworkTimeout >= core.InstallTimeout {
		t.Fatalf("NetworkTimeout (%v) must be tighter than InstallTimeout (%v)",
			core.NetworkTimeout, core.InstallTimeout)
	}

	// A command that outlives its deadline is killed rather than waited on.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- runCmdTimeout(ctx, 150*time.Millisecond, "sleep", "30") }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want an error when the deadline passes")
		}
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Errorf("took %v — deadline was not enforced", elapsed)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("runCmdTimeout ignored its deadline")
	}
}

// TestCurlArgs_HardensTheFetch pins the flags that make a fetch fail fast and
// loudly: -f so an HTTP error isn't saved as content, -S so the reason survives
// -s and reaches stderr, retries and a connect timeout.
func TestCurlArgs_HardensTheFetch(t *testing.T) {
	got := strings.Join(curlArgs("https://example.invalid/a.tar.xz", "-o", "/tmp/x"), " ")
	for _, want := range []string{"-fsSL", "--connect-timeout", "--retry", "--retry-connrefused"} {
		if !strings.Contains(got, want) {
			t.Errorf("curlArgs missing %s: %s", want, got)
		}
	}
	// url must precede the caller's extra args, and --max-time is deliberately absent.
	if !strings.Contains(got, "https://example.invalid/a.tar.xz -o /tmp/x") {
		t.Errorf("url/extra ordering wrong: %s", got)
	}
	if strings.Contains(got, "--max-time") {
		t.Errorf("--max-time fights the retries and duplicates the context deadline: %s", got)
	}
}
