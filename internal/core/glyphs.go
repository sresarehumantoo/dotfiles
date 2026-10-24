package core

import (
	"os"
	"runtime"
	"strings"
)

// Glyphs are the symbols dfinstall prints. Every one is a single column wide in
// both sets, so swapping sets never moves a column.
type Glyphs struct {
	Ok, Fail, Warn, Info, Notice, Debug string
	Partial, Skip, Off                  string
	Dash, Ellipsis                      string
	Spin                                []string
}

var unicodeGlyphs = Glyphs{
	Ok: "✓", Fail: "✗", Warn: "⚠", Info: "▸", Notice: "ℹ", Debug: "◆",
	Partial: "!", Skip: "-", Off: "·",
	Dash: "—", Ellipsis: "…",
	Spin: []string{"⠹", "⢸", "⣰", "⣤", "⣆", "⡇", "⠏", "⠛"},
}

var asciiGlyphs = Glyphs{
	Ok: "+", Fail: "x", Warn: "!", Info: ">", Notice: "i", Debug: "*",
	Partial: "!", Skip: "-", Off: ".",
	Dash: "-", Ellipsis: "...",
	Spin: []string{"|", "/", "-", "\\"},
}

// G is the glyph set for this process, chosen once from the environment.
var G = pickGlyphs(os.Getenv, runtime.GOOS)

// UseASCII switches the glyph set, for tests and callers that know better
// than the environment.
func UseASCII(on bool) {
	if on {
		G = asciiGlyphs
	} else {
		G = unicodeGlyphs
	}
}

// pickGlyphs falls back to ASCII where Unicode symbols would render as junk:
// a dumb terminal, the Linux text console (its default font has no ✓), or a
// locale that is not UTF-8. DF_ASCII=1 or 0 overrides the guess, which is the
// only answer for a terminal whose font lacks the symbols, since no program
// can ask a terminal which glyphs its font has.
func pickGlyphs(getenv func(string) string, goos string) Glyphs {
	switch getenv("DF_ASCII") {
	case "1":
		return asciiGlyphs
	case "0":
		return unicodeGlyphs
	}
	if t := getenv("TERM"); t == "dumb" || t == "linux" {
		return asciiGlyphs
	}
	locale := ""
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := getenv(k); v != "" {
			locale = v
			break
		}
	}
	// macOS is UTF-8 throughout and its terminals often leave the locale
	// unset; elsewhere an unset locale is POSIX, which is ASCII.
	if locale == "" {
		if goos == "darwin" {
			return unicodeGlyphs
		}
		return asciiGlyphs
	}
	l := strings.ToLower(locale)
	if strings.Contains(l, "utf-8") || strings.Contains(l, "utf8") {
		return unicodeGlyphs
	}
	return asciiGlyphs
}

// DumbTerminal reports TERM=dumb, which cannot erase a line, so nothing may
// redraw in place there.
func DumbTerminal() bool { return os.Getenv("TERM") == "dumb" }
