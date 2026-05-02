package core

import (
	"fmt"
	"sync/atomic"

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

// spinnerRunning tracks whether a spinner is active (for safe Err output).
var spinnerRunning atomic.Bool

var (
	infoSymbol  = color.New(color.FgBlue, color.Bold).SprintFunc()
	okSymbol    = color.New(color.FgGreen, color.Bold).SprintFunc()
	warnSymbol  = color.New(color.FgYellow, color.Bold).SprintFunc()
	errSymbol   = color.New(color.FgRed, color.Bold).SprintFunc()
	debugSymbol = color.New(color.FgMagenta, color.Bold).SprintFunc()
)

var bufferedWarnings []string
var bufferedNotices []string

// Info prints an informational message. Suppressed in quiet mode.
func Info(msg string, args ...any) {
	if Level < LogVerbose {
		return
	}
	fmt.Printf("  %s %s\n", infoSymbol("▸"), fmt.Sprintf(msg, args...))
}

// Ok prints a success message. Suppressed in quiet mode.
func Ok(msg string, args ...any) {
	if Level < LogVerbose {
		return
	}
	fmt.Printf("  %s %s\n", okSymbol("✓"), fmt.Sprintf(msg, args...))
}

// Notice prints an informational notice. Always visible (buffered in quiet mode).
// Use for expected operational messages (e.g. backups) that aren't warnings.
func Notice(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if Level < LogVerbose {
		bufferedNotices = append(bufferedNotices, formatted)
		return
	}
	fmt.Printf("  %s %s\n", infoSymbol("ℹ"), formatted)
}

// Warn prints a warning. Buffered in quiet mode, printed immediately otherwise.
func Warn(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if Level < LogVerbose {
		bufferedWarnings = append(bufferedWarnings, formatted)
		return
	}
	fmt.Printf("  %s %s\n", warnSymbol("⚠"), formatted)
}

// AlwaysWarn prints a warning that's always visible regardless of log level.
// Clears the spinner line first so the message isn't overwritten by the next
// tick. Use when the user needs to see a warning right now (e.g. during a
// decision point where buffered output would arrive too late to act on).
func AlwaysWarn(msg string, args ...any) {
	if spinnerRunning.Load() {
		fmt.Print("\r\033[K")
	}
	fmt.Printf("  %s %s\n", warnSymbol("⚠"), fmt.Sprintf(msg, args...))
}

// Err prints an error message. Always printed regardless of log level.
func Err(msg string, args ...any) {
	if spinnerRunning.Load() {
		fmt.Print("\r\033[K")
	}
	fmt.Printf("  %s %s\n", errSymbol("✗"), fmt.Sprintf(msg, args...))
}

// Status prints a success message. Always visible regardless of log level.
// Use for direct user-facing feedback (e.g. after interactive prompts).
func Status(msg string, args ...any) {
	fmt.Printf("  %s %s\n", okSymbol("✓"), fmt.Sprintf(msg, args...))
}

// Debug prints a debug message. Only shown in debug mode.
func Debug(msg string, args ...any) {
	if Level < LogDebug {
		return
	}
	fmt.Printf("  %s %s\n", debugSymbol("◆"), fmt.Sprintf(msg, args...))
}

// FlushWarnings prints all buffered notices and warnings, then clears both buffers.
func FlushWarnings() {
	for _, n := range bufferedNotices {
		fmt.Printf("  %s %s\n", infoSymbol("ℹ"), n)
	}
	bufferedNotices = nil
	for _, w := range bufferedWarnings {
		fmt.Printf("  %s %s\n", warnSymbol("⚠"), w)
	}
	bufferedWarnings = nil
}
