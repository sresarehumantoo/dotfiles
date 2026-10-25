package core

// In-package rather than in tests/: the invariants here are about the
// spinner's internal state machine (pause depth, stop idempotence, the
// mutexes), and asserting them from outside would mean exporting that state
// purely for the test.

import (
	"strings"
	"sync"
	"testing"
)

// newTestSpinner returns a spinner that behaves as if attached to a terminal,
// so the drawing logic is exercised even when tests run with stdout redirected.
func newTestSpinner(t *testing.T) *Spinner {
	t.Helper()
	s := NewSpinner()
	s.mu.Lock()
	s.tty = true
	s.mu.Unlock()
	t.Cleanup(func() {
		s.Stop()
		setActiveSpinner(nil)
	})
	return s
}

// Stop used to close(s.done) unconditionally, so a second call — an error path
// plus a defer, say — panicked with "close of closed channel".
func TestSpinnerStopIsIdempotent(t *testing.T) {
	s := newTestSpinner(t)
	s.Start()
	s.Stop()
	s.Stop()
	s.Stop()
}

// Start had no re-entry guard, so a second call spawned a second render
// goroutine and the animation ran at double speed.
func TestSpinnerStartIsIdempotent(t *testing.T) {
	s := newTestSpinner(t)
	s.Start()
	s.Start()
	s.Start()

	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		t.Fatal("spinner not running after Start")
	}
	s.Stop()
}

// Pause/Resume were a single bool, so with nested pauses the inner Resume
// un-paused while the outer caller was still prompting — redrawing over a sudo
// password prompt. They must nest.
func TestSpinnerPauseNests(t *testing.T) {
	s := newTestSpinner(t)
	s.Start()

	if !s.drawing() {
		t.Fatal("spinner should be drawing after Start")
	}

	s.Pause()
	s.Pause()
	if s.drawing() {
		t.Fatal("spinner still drawing after Pause")
	}

	s.Resume() // inner
	if s.drawing() {
		t.Error("inner Resume un-paused while an outer Pause was still held")
	}

	s.Resume() // outer
	if !s.drawing() {
		t.Error("spinner did not resume after all pauses were released")
	}
}

// A stray Resume must not drive the depth negative, which would leave the next
// Pause unable to suspend.
func TestSpinnerUnbalancedResumeIsSafe(t *testing.T) {
	s := newTestSpinner(t)
	s.Start()

	s.Resume()
	s.Resume()

	s.Pause()
	if s.drawing() {
		t.Error("Pause failed to suspend after unbalanced Resume calls")
	}
	s.Resume()
	if !s.drawing() {
		t.Error("spinner did not resume")
	}
}

// PauseSpinner/ResumeSpinner are the package-level wrappers the modules use;
// they must nest the same way (packages.go pauses around a sudo run while
// runCmd's failure path pauses again to print output).
func TestPackageLevelPauseNests(t *testing.T) {
	s := newTestSpinner(t)
	s.Start()

	PauseSpinner()
	PauseSpinner()
	ResumeSpinner()
	if s.drawing() {
		t.Error("inner ResumeSpinner un-paused while an outer PauseSpinner was held")
	}
	ResumeSpinner()
	if !s.drawing() {
		t.Error("spinner did not resume after balanced ResumeSpinner calls")
	}
}

// Update truncates on rune boundaries; slicing bytes split multi-byte runes
// and rendered replacement characters.
func TestSpinnerUpdateDoesNotSplitRunes(t *testing.T) {
	s := newTestSpinner(t)

	s.Update("%s", strings.Repeat("é", 500))

	s.mu.Lock()
	got := s.text
	s.mu.Unlock()

	if strings.ContainsRune(got, '�') {
		t.Errorf("truncation split a multi-byte rune: %q", got)
	}
	for _, r := range got {
		if r != 'é' && r != '…' {
			t.Errorf("unexpected rune %q in truncated text %q", r, got)
			break
		}
	}
}

// A non-TTY spinner must not emit cursor escapes — piping `dfinstall install`
// to a file used to fill it with \r\033[K and braille frames.
func TestSpinnerSilentWhenNotATTY(t *testing.T) {
	s := NewSpinner()
	t.Cleanup(func() { s.Stop(); setActiveSpinner(nil) })

	s.mu.Lock()
	s.tty = false
	s.mu.Unlock()

	s.Start()
	if s.drawing() {
		t.Error("spinner reports drawing with no TTY")
	}
}

// The render goroutine and the logging path both write stdout and both touch
// spinner state. Exercised together so `go test -race` can see it.
func TestSpinnerConcurrentLoggingIsRaceFree(t *testing.T) {
	s := newTestSpinner(t)
	s.Start()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.Update("working %d/%d", n, j)
				SpinnerDetail("detail %d", j)
				PauseSpinner()
				ResumeSpinner()
				// A couple of real writes per worker so the race detector
				// sees emit() contending with the render goroutine, without
				// flooding the test log.
				if j%50 == 0 {
					Warn("worker %d checkpoint %d", n, j)
				}
			}
		}(i)
	}
	wg.Wait()
	s.Stop()

	FlushWarnings()
}
