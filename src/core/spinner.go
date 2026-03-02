package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/fatih/color"
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

// NewSpinner creates a new Spinner (call Start to begin).
func NewSpinner() *Spinner {
	return &Spinner{
		done: make(chan struct{}),
	}
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

// Update changes the spinner text.
func (s *Spinner) Update(msg string, args ...any) {
	s.mu.Lock()
	s.text = fmt.Sprintf(msg, args...)
	s.mu.Unlock()
}

// Stop halts the spinner and clears its line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
	spinnerRunning.Store(false)
	close(s.done)
	fmt.Print("\r\033[K")
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
