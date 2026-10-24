package core

import (
	"reflect"
	"testing"
)

func TestPickGlyphs(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		goos string
		want Glyphs
	}{
		{"utf-8 locale", map[string]string{"LANG": "en_US.UTF-8", "TERM": "xterm-256color"}, "linux", unicodeGlyphs},
		{"utf8 spelling", map[string]string{"LANG": "C.utf8"}, "linux", unicodeGlyphs},
		{"C locale", map[string]string{"LANG": "C"}, "linux", asciiGlyphs},
		{"LC_ALL wins over LANG", map[string]string{"LC_ALL": "POSIX", "LANG": "en_US.UTF-8"}, "linux", asciiGlyphs},
		{"unset locale on Linux", map[string]string{}, "linux", asciiGlyphs},
		{"unset locale on macOS", map[string]string{}, "darwin", unicodeGlyphs},
		{"dumb terminal", map[string]string{"TERM": "dumb", "LANG": "en_US.UTF-8"}, "linux", asciiGlyphs},
		{"Linux console", map[string]string{"TERM": "linux", "LANG": "en_US.UTF-8"}, "linux", asciiGlyphs},
		{"DF_ASCII=1 forces ASCII", map[string]string{"DF_ASCII": "1", "LANG": "en_US.UTF-8"}, "linux", asciiGlyphs},
		{"DF_ASCII=0 forces Unicode", map[string]string{"DF_ASCII": "0", "TERM": "linux"}, "linux", unicodeGlyphs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickGlyphs(func(k string) string { return tt.env[k] }, tt.goos)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Every glyph must be one column in both sets, or switching sets shifts the
// status columns. Ellipsis is exempt: it only ends a truncated line.
func TestGlyphsAreOneColumn(t *testing.T) {
	for name, g := range map[string]Glyphs{"unicode": unicodeGlyphs, "ascii": asciiGlyphs} {
		syms := append([]string{g.Ok, g.Fail, g.Warn, g.Info, g.Notice, g.Debug, g.Partial, g.Skip, g.Off, g.Dash}, g.Spin...)
		for _, s := range syms {
			if n := len([]rune(s)); n != 1 {
				t.Errorf("%s glyph %q is %d runes", name, s, n)
			}
		}
	}
}
