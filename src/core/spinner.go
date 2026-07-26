package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	spinColor = color.New(color.FgCyan, color.Bold).SprintFunc()
	doneColor = color.New(color.FgGreen, color.Bold).SprintFunc()
	failColor = color.New(color.FgYellow, color.Bold).SprintFunc()
	hintColor = color.New(color.FgHiBlack).SprintFunc()
)

// Spinner renders an animated progress indicator on the terminal.
//
// All of its state lives behind mu. An earlier version split it across three
// places — s.active, a package-level spinnerRunning, and a spinnerPaused
// atomic — which let them disagree: calling s.Pause() directly cleared two of
// them but not the third, so the matching ResumeSpinner silently did nothing.
type Spinner struct {
	mu sync.Mutex

	text  string
	frame int

	// running is true between Start and Stop. pauseDepth counts nested
	// Pause calls; the animation draws only at depth 0.
	running    bool
	pauseDepth int
	stopped    bool

	// tty is false when stdout is redirected, in which case we never emit
	// cursor escapes — they'd fill a log file with control characters.
	tty bool

	done chan struct{}
	wg   sync.WaitGroup
}

// activeSpinner holds the current spinner so callers can pause it around
// interactive prompts. Guarded by activeMu rather than being a bare global.
var (
	activeMu      sync.Mutex
	activeSpinner *Spinner
)

func setActiveSpinner(s *Spinner) {
	activeMu.Lock()
	activeSpinner = s
	activeMu.Unlock()
}

func getActiveSpinner() *Spinner {
	activeMu.Lock()
	defer activeMu.Unlock()
	return activeSpinner
}

// NewSpinner creates a new Spinner (call Start to begin).
func NewSpinner() *Spinner {
	s := &Spinner{
		done: make(chan struct{}),
		tty:  term.IsTerminal(int(os.Stdout.Fd())),
	}
	setActiveSpinner(s)
	return s
}

// Start begins the spinner animation in a background goroutine. Calling it
// twice is a no-op rather than starting a second goroutine drawing at double
// the frame rate.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running || s.stopped {
		s.mu.Unlock()
		return
	}
	s.running = true
	tty := s.tty
	s.mu.Unlock()

	if !tty {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				// Snapshot under s.mu, then write under the output lock.
				// Never hold both: emit() checks spinner state before taking
				// the output lock, so nesting them in the other order here
				// would invert the lock order and could deadlock.
				s.mu.Lock()
				draw := s.running && s.pauseDepth == 0
				frame, text := spinFrames[s.frame], s.text
				if draw {
					s.frame = (s.frame + 1) % len(spinFrames)
				}
				s.mu.Unlock()

				if draw {
					outMu.Lock()
					fmt.Printf("\r\033[K  %s %s", spinColor(frame), text)
					outMu.Unlock()
				}
			}
		}
	}()
}

// Update changes the spinner text. Truncates to fit terminal width so long
// detail strings (e.g. apt package lists) don't wrap onto multiple rows — the
// per-tick redraw only clears one row with \r\033[K, so wrap remnants from the
// previous tick would otherwise pile up visibly.
func (s *Spinner) Update(msg string, args ...any) {
	text := fmt.Sprintf(msg, args...)

	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		// Prefix is "  X " (4 cols) + a 1-col safety margin for the cursor.
		avail := w - 5
		if avail < 20 {
			avail = 20
		}
		// Count and slice runes, not bytes: package names and paths are not
		// always ASCII, and cutting mid-rune renders as replacement junk.
		if r := []rune(text); len(r) > avail {
			text = string(r[:avail-1]) + "…"
		}
	}

	s.mu.Lock()
	s.text = text
	s.mu.Unlock()
}

// Pause suspends the animation and clears its line so an interactive prompt is
// visible. Pauses nest: two Pause calls need two Resumes before the animation
// comes back, which is what stops an inner Resume from redrawing over an outer
// caller's prompt.
func (s *Spinner) Pause() {
	s.mu.Lock()
	s.pauseDepth++
	first := s.pauseDepth == 1 && s.running && s.tty
	s.mu.Unlock()

	if first {
		clearLine()
	}
}

// Resume restarts the animation once every matching Pause has been released.
func (s *Spinner) Resume() {
	s.mu.Lock()
	if s.pauseDepth > 0 {
		s.pauseDepth--
	}
	s.mu.Unlock()
}

// Stop halts the spinner and clears its line. It is idempotent — a second call
// (an error path plus a defer, say) returns rather than panicking on a closed
// channel, and it waits for the render goroutine so the final clear can't race
// a half-written frame.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.running = false
	wasTTY := s.tty
	close(s.done)
	s.mu.Unlock()

	s.wg.Wait()

	if wasTTY {
		clearLine()
	}
	setActiveSpinner(nil)
}

// drawing reports whether this spinner is currently painting the line.
func (s *Spinner) drawing() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running && s.pauseDepth == 0 && s.tty
}

func clearLine() {
	outMu.Lock()
	fmt.Print("\r\033[K")
	outMu.Unlock()
}

// PauseSpinner temporarily suspends the active spinner so interactive prompts
// (like a sudo password) are visible. No-op if no spinner exists.
func PauseSpinner() {
	if s := getActiveSpinner(); s != nil {
		s.Pause()
	}
}

// ResumeSpinner releases one PauseSpinner. No-op if no spinner exists.
func ResumeSpinner() {
	if s := getActiveSpinner(); s != nil {
		s.Resume()
	}
}

// SpinnerDetail updates the active spinner's detail text.
// Use to show sub-step progress (e.g. which package is being installed).
func SpinnerDetail(msg string, args ...any) {
	if s := getActiveSpinner(); s != nil {
		s.Update(msg, args...)
	}
}

// PrintResult prints the final success/failure summary after Stop.
func PrintResult(total, failed int) {
	if failed == 0 {
		if total == 1 {
			emitRaw(fmt.Sprintf("  %s Done\n", doneColor("✓")))
		} else {
			emitRaw(fmt.Sprintf("  %s Done — %d modules installed\n", doneColor("✓"), total))
		}
		return
	}
	installed := total - failed
	emitRaw(fmt.Sprintf("  %s Done — %d/%d modules installed\n", failColor("⚠"), installed, total))
}

// PrintHint prints a dimmed hint message.
func PrintHint(msg string) {
	emitRaw(fmt.Sprintf("  %s\n", hintColor(msg)))
}
