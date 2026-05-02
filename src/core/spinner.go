package core

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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
type Spinner struct {
	mu     sync.Mutex
	text   string
	frame  int
	active bool
	done   chan struct{}
}

// activeSpinner holds the current spinner so modules can pause it for
// interactive prompts (e.g. sudo password).
var activeSpinner *Spinner

// NewSpinner creates a new Spinner (call Start to begin).
func NewSpinner() *Spinner {
	s := &Spinner{
		done: make(chan struct{}),
	}
	activeSpinner = s
	return s
}

// Start begins the spinner animation in a background goroutine.
func (s *Spinner) Start() {
	s.mu.Lock()
	s.active = true
	s.mu.Unlock()
	spinnerRunning.Store(true)

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				s.mu.Lock()
				if s.active {
					fmt.Printf("\r\033[K  %s %s", spinColor(spinFrames[s.frame]), s.text)
					s.frame = (s.frame + 1) % len(spinFrames)
				}
				s.mu.Unlock()
			}
		}
	}()
}

// Update changes the spinner text. Truncates to fit terminal width so long
// detail strings (e.g. apt package lists) don't wrap onto multiple rows —
// the per-tick redraw only clears one row with \r\033[K, so wrap remnants
// from the previous tick would otherwise pile up visibly.
func (s *Spinner) Update(msg string, args ...any) {
	text := fmt.Sprintf(msg, args...)
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		// Prefix is "  X " (4 cols) + a 1-col safety margin for the cursor.
		avail := w - 5
		if avail < 20 {
			avail = 20
		}
		if len(text) > avail {
			text = text[:avail-1] + "…"
		}
	}
	s.mu.Lock()
	s.text = text
	s.mu.Unlock()
}

// Pause temporarily suspends the spinner animation and clears its line.
// The spinner can be resumed with Resume. This is used to allow interactive
// prompts (like sudo password) to be visible.
func (s *Spinner) Pause() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
	spinnerRunning.Store(false)
	fmt.Print("\r\033[K")
}

// Resume restarts the spinner animation after a Pause.
func (s *Spinner) Resume() {
	s.mu.Lock()
	s.active = true
	s.mu.Unlock()
	spinnerRunning.Store(true)
}

// Stop halts the spinner and clears its line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
	spinnerRunning.Store(false)
	activeSpinner = nil
	close(s.done)
	fmt.Print("\r\033[K")
}

// spinnerPaused tracks whether PauseSpinner actually paused a running spinner.
var spinnerPaused atomic.Bool

// PauseSpinner temporarily suspends the active spinner so interactive
// prompts (like sudo password) are visible. No-op if no spinner is running.
func PauseSpinner() {
	if activeSpinner != nil && spinnerRunning.Load() {
		activeSpinner.Pause()
		spinnerPaused.Store(true)
	}
}

// ResumeSpinner restarts the active spinner after a PauseSpinner call.
// No-op if no spinner was paused.
func ResumeSpinner() {
	if spinnerPaused.CompareAndSwap(true, false) && activeSpinner != nil {
		activeSpinner.Resume()
	}
}

// SpinnerDetail updates the active spinner's detail text.
// Use to show sub-step progress (e.g. which package is being installed).
// No-op if no spinner is running.
func SpinnerDetail(msg string, args ...any) {
	if activeSpinner != nil && spinnerRunning.Load() {
		activeSpinner.Update(msg, args...)
	}
}

// PrintResult prints the final success/failure summary after Stop.
func PrintResult(total, failed int) {
	if failed == 0 {
		if total == 1 {
			fmt.Printf("  %s Done\n", doneColor("✓"))
		} else {
			fmt.Printf("  %s Done — %d modules installed\n", doneColor("✓"), total)
		}
	} else {
		installed := total - failed
		fmt.Printf("  %s Done — %d/%d modules installed\n", failColor("⚠"), installed, total)
	}
}

// PrintHint prints a dimmed hint message.
func PrintHint(msg string) {
	fmt.Printf("  %s\n", hintColor(msg))
}
