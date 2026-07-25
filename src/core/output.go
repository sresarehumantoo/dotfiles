package core

import (
	"fmt"
	"sync"

	"github.com/fatih/color"
)

// LogLevel controls output verbosity.
type LogLevel int

const (
	LogQuiet   LogLevel = iota // default: spinner only
	LogVerbose                 // -v: detailed output
	LogDebug                   // --debug: verbose + debug
)

// Level is the current output verbosity.
var Level LogLevel

// outMu serializes every write to stdout. The spinner's render goroutine and
// the main goroutine's log lines both target the same row, so without this
// they interleave mid-escape-sequence and garble the terminal.
var outMu sync.Mutex

var (
	infoSymbol  = color.New(color.FgBlue, color.Bold).SprintFunc()
	okSymbol    = color.New(color.FgGreen, color.Bold).SprintFunc()
	warnSymbol  = color.New(color.FgYellow, color.Bold).SprintFunc()
	errSymbol   = color.New(color.FgRed, color.Bold).SprintFunc()
	debugSymbol = color.New(color.FgMagenta, color.Bold).SprintFunc()
)

var (
	bufMu            sync.Mutex
	bufferedWarnings []string
	bufferedNotices  []string
)

// emit writes one finished line, first clearing the spinner's row if one is
// drawing so the message doesn't land on top of a frame.
//
// The spinner check happens before outMu is taken: the render goroutine holds
// s.mu then outMu, so acquiring them in the other order here would invert the
// lock order and risk a deadlock.
func emit(symbol, msg string) {
	clear := getActiveSpinner().drawing()

	outMu.Lock()
	defer outMu.Unlock()
	if clear {
		fmt.Print("\r\033[K")
	}
	fmt.Printf("  %s %s\n", symbol, msg)
}

// emitRaw writes a pre-formatted line under the same lock.
func emitRaw(line string) {
	clear := getActiveSpinner().drawing()

	outMu.Lock()
	defer outMu.Unlock()
	if clear {
		fmt.Print("\r\033[K")
	}
	fmt.Print(line)
}

// Info prints an informational message. Suppressed in quiet mode.
func Info(msg string, args ...any) {
	if Level < LogVerbose {
		return
	}
	emit(infoSymbol("▸"), fmt.Sprintf(msg, args...))
}

// Ok prints a success message. Suppressed in quiet mode.
func Ok(msg string, args ...any) {
	if Level < LogVerbose {
		return
	}
	emit(okSymbol("✓"), fmt.Sprintf(msg, args...))
}

// Notice prints an informational notice. Always visible (buffered in quiet mode).
// Use for expected operational messages (e.g. backups) that aren't warnings.
func Notice(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if Level < LogVerbose {
		bufMu.Lock()
		bufferedNotices = append(bufferedNotices, formatted)
		bufMu.Unlock()
		return
	}
	emit(infoSymbol("ℹ"), formatted)
}

// Warn prints a warning. Buffered in quiet mode, printed immediately otherwise.
func Warn(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if Level < LogVerbose {
		bufMu.Lock()
		bufferedWarnings = append(bufferedWarnings, formatted)
		bufMu.Unlock()
		return
	}
	emit(warnSymbol("⚠"), formatted)
}

// AlwaysWarn prints a warning that's always visible regardless of log level.
// Use when the user needs to see it right now (e.g. during a decision point
// where buffered output would arrive too late to act on).
func AlwaysWarn(msg string, args ...any) {
	emit(warnSymbol("⚠"), fmt.Sprintf(msg, args...))
}

// Err prints an error message. Always printed regardless of log level.
func Err(msg string, args ...any) {
	emit(errSymbol("✗"), fmt.Sprintf(msg, args...))
}

// Status prints a success message. Always visible regardless of log level.
// Use for direct user-facing feedback (e.g. after interactive prompts).
func Status(msg string, args ...any) {
	emit(okSymbol("✓"), fmt.Sprintf(msg, args...))
}

// Debug prints a debug message. Only shown in debug mode.
func Debug(msg string, args ...any) {
	if Level < LogDebug {
		return
	}
	emit(debugSymbol("◆"), fmt.Sprintf(msg, args...))
}

// FlushWarnings prints all buffered notices and warnings, then clears both buffers.
func FlushWarnings() {
	bufMu.Lock()
	notices, warnings := bufferedNotices, bufferedWarnings
	bufferedNotices, bufferedWarnings = nil, nil
	bufMu.Unlock()

	for _, n := range notices {
		emit(infoSymbol("ℹ"), n)
	}
	for _, w := range warnings {
		emit(warnSymbol("⚠"), w)
	}
}
