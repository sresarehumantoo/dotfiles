package tests

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written. core.emit writes via fmt.Printf, so swapping os.Stdout catches
// it — there is no cached writer to work around.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		_, _ = io.Copy(&sb, r)
		done <- sb.String()
	}()

	fn()

	os.Stdout = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// RunDoctor must print its results at the DEFAULT log level.
//
// It used to build its output from core.Ok and core.Warn: Ok is suppressed
// below LogVerbose, and Warn is buffered until a FlushWarnings that RunDoctor
// never called. So `dfinstall doctor` printed its header and nothing else, and
// every check — pass or fail — was silently discarded. Only `doctor -v` worked.
// This test fails against that code.
func TestRunDoctorPrintsAtDefaultLogLevel(t *testing.T) {
	prev := core.Level
	core.Level = core.LogQuiet
	defer func() { core.Level = prev }()

	out := captureStdout(t, modules.RunDoctor)

	if !strings.Contains(out, "Running health checks") {
		t.Fatalf("header missing; capture probably failed:\n%s", out)
	}

	// The header alone is what the bug produced. Require actual check lines.
	lines := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	if lines < 5 {
		t.Errorf("doctor printed only %d non-empty lines at default level; results were swallowed:\n%s", lines, out)
	}

	// Every run has a verdict line, whichever way it goes.
	if !strings.Contains(out, "All checks passed!") && !strings.Contains(out, "Some checks failed") {
		t.Errorf("no verdict line in output:\n%s", out)
	}
}

// Quiet and verbose should show the same checks. -v may add unrelated chatter
// from elsewhere, so this compares the check lines rather than raw equality.
func TestRunDoctorSameChecksQuietAndVerbose(t *testing.T) {
	prev := core.Level
	defer func() { core.Level = prev }()

	core.Level = core.LogQuiet
	quiet := captureStdout(t, modules.RunDoctor)

	core.Level = core.LogVerbose
	verbose := captureStdout(t, modules.RunDoctor)

	count := func(s string) int {
		n := 0
		for _, l := range strings.Split(s, "\n") {
			if strings.Contains(l, "✓") || strings.Contains(l, "⚠") {
				n++
			}
		}
		return n
	}

	q, v := count(quiet), count(verbose)
	if q == 0 || v == 0 {
		t.Fatalf("expected check lines in both (quiet=%d verbose=%d)", q, v)
	}
	if q != v {
		t.Errorf("quiet showed %d check lines, verbose showed %d — they must agree", q, v)
	}
}
