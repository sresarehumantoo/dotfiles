package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func realWslconfigTemplate(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "wsl", "wslconfig"))
	if err != nil {
		t.Fatalf("reading template: %v", err)
	}
	return string(data)
}

// The whole point of comparing by key: a file this repo rendered itself must
// never report drift against the template it came from. A byte comparison
// cannot say that — the template holds @HOST_SIZING@ — and that is exactly the
// failure mode /etc/wsl.conf's check had before checkWslConf existed.
func TestWslconfigDrift_OurOwnRenderIsClean(t *testing.T) {
	tmpl := realWslconfigTemplate(t)

	for _, specs := range []hostSpecs{
		{logicalCPUs: 16, memBytes: 32 * gib},
		{logicalCPUs: 4, memBytes: 8 * gib},
		{}, // both probes failed: sizing keys omitted entirely
	} {
		rendered := renderWslconfig(tmpl, specs)
		// The other half of the claim: a byte comparison, which is what this
		// check would have been written as by default, DOES differ here. That
		// is the whole reason the comparison is by key.
		if rendered == tmpl {
			t.Fatalf("specs=%+v: render produced the template verbatim, so this proves nothing", specs)
		}
		if drift := wslconfigDrift(tmpl, rendered); len(drift) > 0 {
			t.Errorf("specs=%+v: rendering our own template reports drift: %v", specs, drift)
		}
	}
}

// Sizing is host-derived and deliberately not ours to police. A hand-tuned
// memory= is the owner's call and must not show up as drift, or every WSL box
// with a tweaked allocation nags forever.
func TestWslconfigDrift_IgnoresHostSizing(t *testing.T) {
	tmpl := realWslconfigTemplate(t)
	rendered := renderWslconfig(tmpl, hostSpecs{logicalCPUs: 16, memBytes: 32 * gib})

	edited := strings.NewReplacer(
		"memory=16GB", "memory=4GB",
		"swap=4GB", "swap=1GB",
		"processors=16", "processors=2",
	).Replace(rendered)
	if edited == rendered {
		t.Fatal("fixture did not change; renderHostSizing's output format moved")
	}

	if drift := wslconfigDrift(tmpl, edited); len(drift) > 0 {
		t.Errorf("hand-tuned sizing reported as drift: %v", drift)
	}
}

func TestWslconfigDrift_ReportsChangedAndMissingKeys(t *testing.T) {
	tmpl := `[wsl2]
@HOST_SIZING@
networkingMode=mirrored

[experimental]
sparseVhd=true
autoMemoryReclaim=gradual
`
	// sparseVhd flipped off (the case this check was built for), and
	// autoMemoryReclaim dropped entirely.
	installed := `[wsl2]
memory=8GB
networkingMode=mirrored

[experimental]
sparseVhd=false
`
	got := wslconfigDrift(tmpl, installed)
	want := []string{
		"experimental.autoMemoryReclaim is missing (want gradual)",
		"experimental.sparseVhd=false (want true)",
	}
	// Keys are lowercased for comparison, so the report reads lowercase too.
	for i := range want {
		want[i] = strings.ToLower(strings.SplitN(want[i], " ", 2)[0]) + " " + strings.SplitN(want[i], " ", 2)[1]
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if !strings.EqualFold(got[i], want[i]) {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Sorted, so a report does not reshuffle between runs.
func TestWslconfigDrift_IsOrdered(t *testing.T) {
	tmpl := "[wsl2]\nzeta=1\nalpha=1\nmiddle=1\n"
	got := wslconfigDrift(tmpl, "[wsl2]\n")
	if len(got) != 3 {
		t.Fatalf("want 3 drift lines, got %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("not sorted: %v", got)
		}
	}
}

// A key the file has and the template does not is the user's business — the
// host sizing arrives that way, and so does anything they added themselves.
func TestWslconfigDrift_IgnoresForeignKeys(t *testing.T) {
	tmpl := "[wsl2]\nnetworkingMode=mirrored\n"
	installed := "[wsl2]\nnetworkingMode=mirrored\nkernelCommandLine=quiet\n\n[experimental]\nhostAddressLoopback=true\n"
	if drift := wslconfigDrift(tmpl, installed); len(drift) > 0 {
		t.Errorf("foreign keys reported as drift: %v", drift)
	}
}

// Another tool rewriting the file need not preserve our camelCase or our value
// casing, and neither difference means the setting changed.
func TestWslconfigDrift_IsCaseInsensitive(t *testing.T) {
	tmpl := "[wsl2]\nnetworkingMode=mirrored\n\n[experimental]\nsparseVhd=true\n"
	installed := "[WSL2]\nNETWORKINGMODE=Mirrored\n\n[Experimental]\nsparsevhd=TRUE\n"
	if drift := wslconfigDrift(tmpl, installed); len(drift) > 0 {
		t.Errorf("case difference reported as drift: %v", drift)
	}
}

func TestParseWslconfigKeys_SkipsCommentsAndJunk(t *testing.T) {
	keys := parseWslconfigKeys(`
# a hash comment with an = sign in it
; a semicolon comment
[wsl2]
  vmIdleTimeout=28800000
not a key-value line
=novalue
[experimental]
sparseVhd = true
`)
	if got, want := keys["wsl2.vmidletimeout"], "28800000"; got != want {
		t.Errorf("vmIdleTimeout: got %q want %q", got, want)
	}
	if got, want := keys["experimental.sparsevhd"], "true"; got != want {
		t.Errorf("sparseVhd: got %q want %q (whitespace around = must be trimmed)", got, want)
	}
	if len(keys) != 2 {
		t.Errorf("comments or junk lines leaked in: %v", keys)
	}
}

func TestRenderWslconfigResult(t *testing.T) {
	ok := renderWslconfigResult(wslconfigReport{State: "ok"})
	if !ok.OK || ok.Detail == "" {
		t.Errorf("ok state: want a passing row with a detail, got %+v", ok)
	}

	missing := renderWslconfigResult(wslconfigReport{State: "missing"})
	if missing.OK || len(missing.Extra) == 0 {
		t.Errorf("missing state: want a failing row that says what to run, got %+v", missing)
	}

	drift := renderWslconfigResult(wslconfigReport{
		State: "outdated",
		Drift: []string{"experimental.sparsevhd=false (want true)"},
	})
	if drift.OK {
		t.Error("outdated state must fail the check: this is the one the whole thing exists for")
	}
	if !strings.Contains(strings.Join(drift.Extra, "\n"), "sparsevhd") {
		t.Errorf("the drifted keys must reach the user, got %+v", drift.Extra)
	}
	// A .wslconfig change does nothing until the VM restarts, and forgetting
	// that reads as "the fix did not work".
	if !strings.Contains(strings.Join(drift.Extra, "\n"), "wsl --shutdown") {
		t.Errorf("want the restart step, got %+v", drift.Extra)
	}

	unknown := renderWslconfigResult(wslconfigReport{State: "unknown", Why: "could not resolve the Windows home"})
	if unknown.OK || unknown.Detail != "could not resolve the Windows home" {
		t.Errorf("unknown state must surface its reason, got %+v", unknown)
	}
}
