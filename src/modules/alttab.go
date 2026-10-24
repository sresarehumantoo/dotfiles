package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

const alttabLabel = "io.github.sresarehumantoo.alttab"

// AlttabModule builds config/alttab/main.swift, an Option-Tab app switcher
// that needs no Accessibility permission, and runs it from a LaunchAgent.
type AlttabModule struct{}

func (AlttabModule) Name() string { return "alttab" }

func (AlttabModule) Unsupported() string { return macOnly() }

func alttabSource() string { return core.ConfigPath("alttab", "main.swift") }
func alttabBinary() string { return core.HomeTarget(".local", "bin", "alttab") }
func alttabAgent() string  { return launchAgentPath(alttabLabel) }
func alttabLog() string    { return core.HomeTarget("Library", "Logs", "alttab.log") }

func alttabStale() bool { return swiftStale(alttabSource(), alttabBinary()) }

// swiftStale reports whether a binary is missing or older than its source.
// mtime is enough: a pull or checkout that changes the source rewrites it.
func swiftStale(source, bin string) bool {
	src, err := os.Stat(source)
	if err != nil {
		return false
	}
	b, err := os.Stat(bin)
	return err != nil || b.ModTime().Before(src.ModTime())
}

// KeepAlive restarts the switcher only after a failed exit, so it can stop for
// good when no hotkey registers.
const alttabAgentExtra = `
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>ProcessType</key>
	<string>Interactive</string>`

func (m AlttabModule) Install(ctx context.Context) error {
	if !core.IsMac() {
		core.Ok("alttab is macOS only, skipping")
		return nil
	}
	agent := alttabAgent()
	// The LaunchAgent is a login item, which macOS announces and an MDM may
	// forbid, so only `install alttab` creates it. `install all` keeps an
	// existing one current.
	if core.InstallingAll && !fileExists(agent) {
		core.Debug("alttab not installed, skipping")
		return nil
	}
	bin := alttabBinary()
	for _, p := range []string{bin, agent, alttabLog()} {
		if err := core.CheckTarget(p); err != nil {
			return err
		}
	}

	rebuilt := false
	if alttabStale() {
		if err := buildSwift(ctx, "alttab", alttabSource(), bin); err != nil {
			return err
		}
		rebuilt = true
	}
	agentChanged, err := core.WriteFileIfChanged(agent, launchAgentPlist(alttabLabel, bin, alttabLog(), alttabAgentExtra), 0o644)
	if err != nil {
		return err
	}
	if core.DryRun {
		return nil
	}
	if err := loadLaunchAgent(ctx, alttabLabel, agentChanged, rebuilt); err != nil {
		return err
	}
	core.Ok("alttab done (log: %s)", alttabLog())
	return nil
}

// buildSwift compiles one Swift source file into bin.
func buildSwift(ctx context.Context, name, source, bin string) error {
	if core.DryRun {
		core.Changed("would build %s from %s", bin, source)
		return nil
	}
	if _, err := exec.LookPath("swiftc"); err != nil {
		return fmt.Errorf("swiftc not found: %s needs the Command Line Tools (xcode-select --install)", name)
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	// Built beside the target and renamed over it, so the running copy is
	// never a half-written file.
	tmp := bin + ".new"
	core.Info("Building %s...", name)
	if err := runCmd(ctx, "swiftc", "-O", "-swift-version", "5", "-o", tmp, source); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("building %s: %w", name, err)
	}
	if err := os.Rename(tmp, bin); err != nil {
		return err
	}
	core.Changed("built %s", bin)
	return nil
}

func (m AlttabModule) Uninstall(ctx context.Context) error {
	if !core.IsMac() {
		return nil
	}
	if err := removeLaunchAgent(ctx, alttabLabel, alttabBinary()); err != nil {
		return err
	}
	core.Ok("alttab uninstalled")
	return nil
}

// Status reads files only; whether the agent is actually running is a
// launchctl question, which the doctor asks.
func (m AlttabModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "alttab"}
	if !core.IsMac() || !fileExists(alttabAgent()) {
		return s
	}
	s.Linked++
	var notes []string
	if !fileExists(alttabBinary()) {
		s.Missing++
		notes = append(notes, "binary missing")
	} else {
		s.Linked++
		if alttabStale() {
			notes = append(notes, "binary older than source")
		}
	}
	s.Extra = strings.Join(notes, "; ")
	return s
}

// alttabDoctorResult reports whether an installed switcher is running, and
// nothing when it was never installed.
func alttabDoctorResult(probe func(string, ...string) (string, bool)) []DoctorResult {
	if !fileExists(alttabAgent()) {
		return nil
	}
	out, ok := probe("launchctl", "print", launchdDomain()+"/"+alttabLabel)
	switch {
	case !ok:
		return []DoctorResult{{Name: "alttab", OK: false, Detail: "LaunchAgent not loaded",
			Extra: []string{"run `dfinstall install alttab`"}}}
	case !strings.Contains(out, "state = running"):
		return []DoctorResult{{Name: "alttab", OK: false, Detail: "loaded but not running",
			Extra: []string{"see " + alttabLog()}}}
	case alttabStale():
		return []DoctorResult{{Name: "alttab", OK: true, Detail: "running an older build; `dfinstall install alttab` rebuilds it"}}
	}
	return []DoctorResult{{Name: "alttab", OK: true, Detail: "running"}}
}
