package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// Facts about a Mac that decide whether an install step can run without
// admin rights. A managed Mac usually means a standard account: /Applications
// and other admin-owned paths are not writable, and Homebrew may have been
// installed by someone else.

const wOK = 0x2 // access(2) W_OK

func writable(path string) bool {
	return path != "" && syscall.Access(path, wOK) == nil
}

// brewPrefix returns Homebrew's prefix, or "" when brew is not installed.
func brewPrefix() string {
	if p := os.Getenv("HOMEBREW_PREFIX"); p != "" {
		return p
	}
	brew, err := exec.LookPath("brew")
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(brew))
}

// brewUsable reports whether this user can install into the Homebrew prefix.
// Homebrew supports one owning account; anyone else gets permission errors
// from every install.
func brewUsable() bool {
	prefix := brewPrefix()
	if prefix == "" {
		return false
	}
	if cellar := filepath.Join(prefix, "Cellar"); writable(cellar) {
		return true
	}
	return writable(prefix)
}

// caskAppDir is where a cask's .app should go: /Applications when this user
// can write it, otherwise ~/Applications, which needs no admin.
func caskAppDir() string {
	if writable("/Applications") {
		return "/Applications"
	}
	return core.HomeTarget("Applications")
}

// caskArgs builds `brew install --cask` arguments, adding --appdir when the
// app has to go to ~/Applications.
func caskArgs(cask string) []string {
	args := []string{"install", "--cask"}
	if dir := caskAppDir(); dir != "/Applications" {
		args = append(args, "--appdir="+dir)
	}
	return append(args, cask)
}

// macDoctorResults reports the facts that decide how a Mac install goes on a
// managed machine, so a first run that breaks says why. They only fail on
// what actually stops an install: a Homebrew this user cannot write to, and
// Santa in Lockdown, which blocks every locally built binary. The rest are
// reported, not judged.
func macDoctorResults() []DoctorResult {
	// context.Background: doctor has no context to inherit, and each probe is
	// bounded by runProbe's own timeout.
	ctx := context.Background()
	probe := func(name string, args ...string) (string, bool) {
		out, err := runProbe(ctx, name, args...)
		return strings.TrimSpace(string(out)), err == nil
	}
	var results []DoctorResult

	admin := "no — standard account; nothing here needs admin once Homebrew exists"
	if groups, ok := probe("id", "-Gn"); ok && slices.Contains(strings.Fields(groups), "admin") {
		admin = "yes"
	}
	results = append(results, DoctorResult{Name: "macOS admin", OK: true, Detail: admin})

	switch prefix := brewPrefix(); {
	case prefix == "":
		results = append(results, DoctorResult{Name: "homebrew", OK: false,
			Detail: "not installed", Extra: []string{"run bootstrap/macos-setup.sh, or have IT install Homebrew for this account"}})
	case !brewUsable():
		results = append(results, DoctorResult{Name: "homebrew", OK: false,
			Detail: prefix + " is not writable by this user",
			Extra:  []string{"Homebrew supports one owning account; every brew install will fail with permission errors", "ask IT to chown the prefix to you, or use a Homebrew installed for your account"}})
	default:
		results = append(results, DoctorResult{Name: "homebrew", OK: true, Detail: prefix})
	}

	results = append(results, DoctorResult{Name: "app folder", OK: true, Detail: "casks install to " + caskAppDir()})

	clt := "installed"
	if _, ok := probe("xcode-select", "-p"); !ok {
		clt = "missing — needed for git, make and cc; on a managed Mac IT or Self Service usually deploys it"
	}
	results = append(results, DoctorResult{Name: "command line tools", OK: clt == "installed", Detail: clt})

	mdm := "not enrolled"
	if out, ok := probe("profiles", "status", "-type", "enrollment"); ok {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "MDM enrollment:") {
				mdm = strings.TrimSpace(strings.TrimPrefix(line, "MDM enrollment:"))
			}
		}
	}
	results = append(results, DoctorResult{Name: "mdm", OK: true, Detail: mdm})

	if out, ok := probe("santactl", "status"); ok {
		mode := "unknown"
		for _, line := range strings.Split(out, "\n") {
			if k, v, found := strings.Cut(line, "|"); found && strings.TrimSpace(k) == "Mode" {
				mode = strings.TrimSpace(v)
			}
		}
		if strings.EqualFold(mode, "Lockdown") {
			results = append(results, DoctorResult{Name: "santa", OK: false, Detail: "Lockdown",
				Extra: []string{"unsigned local binaries are blocked: bin/dfinstall, tree-sitter, rustup toolchains, Mason servers, ~/.local/bin tools", "they need allowlist rules from IT"}})
		} else {
			results = append(results, DoctorResult{Name: "santa", OK: true, Detail: mode})
		}
	}

	gk := "unknown"
	if out, _ := probe("spctl", "--status"); out != "" {
		gk = out
	}
	results = append(results, DoctorResult{Name: "gatekeeper", OK: true, Detail: gk})

	return results
}
