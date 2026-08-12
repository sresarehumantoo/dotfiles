package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The template must never carry sizing of its own. It used to hardcode
// memory=10GB / processors=8, one developer's desktop, copied verbatim onto
// whatever host ran the installer. This test fails against that old file.
func TestWslconfigTemplateHasNoHardcodedSizing(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "wsl", "wslconfig"))
	if err != nil {
		t.Fatalf("reading template: %v", err)
	}
	tmpl := string(data)

	if !strings.Contains(tmpl, "@HOST_SIZING@") {
		t.Fatal("template lost its @HOST_SIZING@ placeholder; sizing would never be written")
	}

	for _, line := range strings.Split(tmpl, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue // prose may legitimately mention these
		}
		for _, key := range []string{"memory=", "processors=", "swap="} {
			if strings.HasPrefix(trimmed, key) {
				t.Errorf("template hardcodes %q (%q); sizing must come from the host", key, trimmed)
			}
		}
	}
}

// Render the REAL template and assert the result is well-formed INI.
//
// ⚠ This is the test that matters, and its absence let a real bug ship. The
// header prose mentioned the @HOST_SIZING@ token by name; substitution is a
// plain ReplaceAll, so the multi-line, non-comment-prefixed sizing block was
// injected into the middle of the top comment as well, emitting bare
// `memory=`/`swap=`/`processors=` keys ABOVE the [wsl2] header, in no section,
// plus a mangled `processors=16 line`. The earlier tests missed it because one
// skips comment lines and the other used a toy inline template.
func TestRenderedWslconfigIsWellFormed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "wsl", "wslconfig"))
	if err != nil {
		t.Fatalf("reading template: %v", err)
	}
	tmpl := string(data)

	if n := strings.Count(tmpl, "@HOST_SIZING@"); n != 1 {
		t.Fatalf("expected exactly 1 @HOST_SIZING@ placeholder, found %d", n)
	}

	for _, specs := range []hostSpecs{
		{logicalCPUs: 16, memBytes: 32 * gib},
		{}, // detection failed
	} {
		rendered := renderWslconfig(tmpl, specs)

		if strings.Contains(rendered, "@HOST_SIZING@") {
			t.Error("placeholder survived substitution")
		}

		section := ""
		seen := map[string]int{}
		for i, line := range strings.Split(rendered, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				section = trimmed
				continue
			}
			if section == "" {
				t.Errorf("specs=%+v: key outside any section at line %d: %q", specs, i+1, trimmed)
				continue
			}
			key, _, ok := strings.Cut(trimmed, "=")
			if !ok {
				t.Errorf("specs=%+v: line %d is neither a section, comment, nor key=value: %q", specs, i+1, trimmed)
				continue
			}
			key = strings.TrimSpace(key)
			if strings.ContainsAny(key, " \t") {
				t.Errorf("specs=%+v: malformed key at line %d: %q", specs, i+1, trimmed)
			}
			seen[section+"/"+key]++
		}

		for k, n := range seen {
			if n > 1 {
				t.Errorf("specs=%+v: %s declared %d times", specs, k, n)
			}
		}
	}
}

// localhostForwarding is documented as ignored under networkingMode=mirrored,
// which this config sets. Keeping it implied a setting that does nothing.
func TestWslconfigTemplateOmitsIgnoredKeys(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "wsl", "wslconfig"))
	if err != nil {
		t.Fatalf("reading template: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "localhostforwarding") {
			t.Errorf("localhostForwarding is ignored under mirrored networking: %q", trimmed)
		}
	}
}

func TestRenderWslconfigSubstitutesPlaceholder(t *testing.T) {
	const tmpl = "[wsl2]\n@HOST_SIZING@\nnetworkingMode=mirrored\n"

	tests := []struct {
		name         string
		specs        hostSpecs
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "both detected",
			specs:        hostSpecs{logicalCPUs: 16, memBytes: 32 * gib},
			wantContains: []string{"memory=16GB", "swap=4GB", "processors=16"},
		},
		{
			// The whole point: a failed probe must omit the key, never guess.
			name:         "nothing detected",
			specs:        hostSpecs{},
			wantContains: []string{"omitted"},
			wantAbsent:   []string{"memory=", "processors=", "swap="},
		},
		{
			name:         "cpu only",
			specs:        hostSpecs{logicalCPUs: 4},
			wantContains: []string{"processors=4"},
			wantAbsent:   []string{"memory=", "swap="},
		},
		{
			// 8GB host -> 4GB, well inside the clamp.
			name:         "small host",
			specs:        hostSpecs{logicalCPUs: 2, memBytes: 8 * gib},
			wantContains: []string{"memory=4GB", "swap=1GB", "processors=2"},
		},
		{
			// 128GB host would want 64GB; clamped to 24.
			name:         "huge host is clamped",
			specs:        hostSpecs{logicalCPUs: 64, memBytes: 128 * gib},
			wantContains: []string{"memory=24GB", "swap=6GB"},
		},
		{
			// 2GB host would want 1GB; floored to 2.
			name:         "tiny host is floored",
			specs:        hostSpecs{logicalCPUs: 1, memBytes: 2 * gib},
			wantContains: []string{"memory=2GB", "swap=1GB"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderWslconfig(tmpl, tc.specs)

			if strings.Contains(got, "@HOST_SIZING@") {
				t.Error("placeholder survived substitution")
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
			for _, absent := range tc.wantAbsent {
				for _, line := range strings.Split(got, "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "#") {
						continue
					}
					if strings.HasPrefix(trimmed, absent) {
						t.Errorf("expected no %q directive, got %q", absent, trimmed)
					}
				}
			}
		})
	}
}

// /etc/wsl.conf's `[user] default=` decides which account WSL logs into, so a
// literal username shipped to another host names a user that may not exist
// there. This test fails against the old file, which had `default=owen`.
func TestWslConfTemplateHasNoHardcodedUser(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "wsl", "wsl.conf"))
	if err != nil {
		t.Fatalf("reading template: %v", err)
	}
	tmpl := string(data)

	// Exactly once. Substitution is a plain ReplaceAll, so a second mention
	// (e.g. naming the token in a nearby comment) is rewritten too, leaving the
	// rendered file with a prose line that contradicts itself. Comment lines are
	// skipped by the hardcoded-username loop below, so only this catches it.
	if n := strings.Count(tmpl, "@DEFAULT_USER@"); n != 1 {
		t.Fatalf("expected exactly 1 @DEFAULT_USER@ placeholder, found %d", n)
	}
	for _, line := range strings.Split(tmpl, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "default=") {
			t.Errorf("template hardcodes a username: %q", trimmed)
		}
	}
}

func TestRenderWslConfContent(t *testing.T) {
	const tmpl = "[user]\n@DEFAULT_USER@\n\n[time]\nuseWindowsTimezone=true\n"

	got := renderWslConfContent(tmpl, "alice")
	if !strings.Contains(got, "default=alice") {
		t.Errorf("username not substituted:\n%s", got)
	}
	if strings.Contains(got, "@DEFAULT_USER@") {
		t.Error("placeholder survived substitution")
	}

	// Unknown user must OMIT the key, not guess one. WSL then falls back to the
	// distro's initial user, which is correct; a wrong name is not recoverable
	// without editing /etc by hand.
	got = renderWslConfContent(tmpl, "")
	if strings.Contains(got, "@DEFAULT_USER@") {
		t.Error("placeholder survived substitution for the empty case")
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "default=") {
			t.Errorf("emitted a default= key with no known user: %q", line)
		}
	}
}

// The rendered file is what lands on disk, so Status and doctor must compare
// against the rendered form. Comparing the raw template would never match and
// would report permanent, unfixable drift on a healthy machine.
func TestRenderedWslConfLeavesNoPlaceholders(t *testing.T) {
	rendered, err := renderedWslConf()
	if err != nil {
		t.Fatalf("renderedWslConf: %v", err)
	}
	if len(rendered) == 0 {
		t.Fatal("rendered wsl.conf is empty")
	}
	if strings.Contains(string(rendered), "@") &&
		strings.Contains(string(rendered), "@DEFAULT_USER@") {
		t.Errorf("rendered output still contains a placeholder:\n%s", rendered)
	}
}

func TestCurrentUsernameNeverReturnsRoot(t *testing.T) {
	// dfinstall refuses to run as root, so a "root" answer here would mean the
	// probe picked up a sudo artifact and would write the wrong login user.
	t.Setenv("USER", "root")
	if got := currentUsername(); got == "root" {
		t.Error("currentUsername() returned root")
	}
}

// A Windows path can contain spaces ("Program Files"); an unquoted shim would
// split into two arguments and exec the wrong thing.
func TestShellQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{`/mnt/c/Windows/System32/cmd.exe`, `'/mnt/c/Windows/System32/cmd.exe'`},
		{`/mnt/c/Program Files/PowerShell/7/pwsh.exe`, `'/mnt/c/Program Files/PowerShell/7/pwsh.exe'`},
		{`/mnt/c/we'ird/x.exe`, `'/mnt/c/we'\''ird/x.exe'`},
	}
	for _, tc := range tests {
		if got := shellQuote(tc.in); got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestShimBody(t *testing.T) {
	body := shimBody(`/mnt/c/Program Files/PowerShell/7/pwsh.exe`)

	if !strings.HasPrefix(body, "#!/bin/sh\n") {
		t.Error("shim must start with a shebang, or binfmt would try to interpret it")
	}
	// removeWinShims and installWinShims both key off this marker to avoid
	// clobbering or deleting a wrapper the user wrote.
	if !strings.Contains(body, shimHeader) {
		t.Error("shim must carry the managed header")
	}
	if !strings.Contains(body, `exec '/mnt/c/Program Files/PowerShell/7/pwsh.exe' "$@"`) {
		t.Errorf("shim does not exec the quoted target:\n%s", body)
	}
}

// The load-bearing five are the ones this repo calls by bare name; losing any
// of them silently breaks demorec, wsl-restart or the module's own interop.
func TestWinShimNamesCoverRepoDependencies(t *testing.T) {
	required := []string{"cmd.exe", "powershell.exe", "tasklist.exe", "taskkill.exe", "wsl.exe"}
	have := make(map[string]bool, len(winShimNames))
	for _, n := range winShimNames {
		have[n] = true
	}
	for _, r := range required {
		if !have[r] {
			t.Errorf("%s is called by name in this repo but has no shim", r)
		}
	}
}
