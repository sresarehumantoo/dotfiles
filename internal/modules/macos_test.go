package modules

import "testing"

func pem(body string) string {
	return "-----BEGIN CERTIFICATE-----\n" + body + "\n-----END CERTIFICATE-----\n"
}

// The doctor's stale-bundle check: a proxy root added to the System keychain
// after the bundle was built must be counted, and certificates the bundle has
// must not be.
func TestMissingPEMs(t *testing.T) {
	bundle := pem("PUBLIC1") + pem("PUBLIC2") + pem("CORP-OLD")
	cases := []struct {
		name string
		have string
		want int
	}{
		{"all present", pem("CORP-OLD"), 0},
		{"rotated root", pem("CORP-OLD") + pem("CORP-NEW"), 1},
		{"empty keychain", "", 0},
		{"trailing noise", pem("CORP-OLD") + "\n\n", 0},
	}
	for _, c := range cases {
		if got := missingPEMs(c.have, bundle); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
