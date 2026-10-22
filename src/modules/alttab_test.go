package modules

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

// The agent is the only thing that starts the switcher, and launchd rejects
// a malformed plist without saying much, so it must parse and must escape the
// paths it is given.
func TestAlttabPlist(t *testing.T) {
	data := alttabPlist("/Users/a&b/.local/bin/alttab", "/Users/a&b/Library/Logs/alttab.log")
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	dec.Strict = true
	var strs []string
	inString := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("plist does not parse: %v", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			inString = tok.Name.Local == "string"
		case xml.EndElement:
			inString = false
		case xml.CharData:
			if inString {
				strs = append(strs, string(tok))
			}
		}
	}
	for _, want := range []string{alttabLabel, "/Users/a&b/.local/bin/alttab", "/Users/a&b/Library/Logs/alttab.log"} {
		if !slices.Contains(strs, want) {
			t.Errorf("plist has no <string>%s</string>; strings: %q", want, strs)
		}
	}
}
