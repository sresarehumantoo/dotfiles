package modules

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
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

func alttabSource() string { return core.ConfigPath("alttab", "main.swift") }
func alttabBinary() string { return core.HomeTarget(".local", "bin", "alttab") }
func alttabAgent() string {
	return core.HomeTarget("Library", "LaunchAgents", alttabLabel+".plist")
}
func alttabLog() string { return core.HomeTarget("Library", "Logs", "alttab.log") }

func launchdDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// alttabStale reports whether the binary is missing or older than its source.
// mtime is enough: a pull or checkout that changes main.swift rewrites it.
func alttabStale() bool {
	src, err := os.Stat(alttabSource())
	if err != nil {
		return false
	}
	bin, err := os.Stat(alttabBinary())
	return err != nil || bin.ModTime().Before(src.ModTime())
}

// alttabPlist renders the LaunchAgent. KeepAlive restarts it only after a
// failed exit, so the switcher can stop for good when no hotkey registers.
func alttabPlist(bin, logPath string) []byte {
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + alttabLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + esc(bin) + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>LimitLoadToSessionType</key>
	<string>Aqua</string>
	<key>ProcessType</key>
	<string>Interactive</string>
	<key>StandardErrorPath</key>
	<string>` + esc(logPath) + `</string>
</dict>
</plist>
`)
}

func alttabLoaded(ctx context.Context) bool {
	_, err := runProbe(ctx, "launchctl", "print", launchdDomain()+"/"+alttabLabel)
	return err == nil
}

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
		core.Debug("alttab not installed — skipping")
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
		if err := buildAlttab(ctx, bin); err != nil {
			return err
		}
		rebuilt = true
	}
	agentChanged, err := core.WriteFileIfChanged(agent, alttabPlist(bin, alttabLog()), 0o644)
	if err != nil {
		return err
	}
	if core.DryRun {
		return nil
	}

	domain := launchdDomain()
	loaded := alttabLoaded(ctx)
	// kickstart reruns the binary but keeps the plist launchd already read,
	// so a changed plist needs a fresh bootstrap.
	if loaded && agentChanged {
		if err := runCmd(ctx, "launchctl", "bootout", domain+"/"+alttabLabel); err != nil {
			return fmt.Errorf("unloading the old alttab agent: %w", err)
		}
		loaded = false
	}
	switch {
	case !loaded:
		if err := runCmd(ctx, "launchctl", "bootstrap", domain, agent); err != nil {
			return fmt.Errorf("loading the alttab agent: %w", err)
		}
		core.Changed("started alttab")
	case rebuilt:
		if err := runCmd(ctx, "launchctl", "kickstart", "-k", domain+"/"+alttabLabel); err != nil {
			return fmt.Errorf("restarting alttab: %w", err)
		}
		core.Changed("restarted alttab")
	}
	core.Ok("alttab done (log: %s)", alttabLog())
	return nil
}

func buildAlttab(ctx context.Context, bin string) error {
	if core.DryRun {
		core.Changed("would build %s from %s", bin, alttabSource())
		return nil
	}
	if _, err := exec.LookPath("swiftc"); err != nil {
		return errors.New("swiftc not found: alttab needs the Command Line Tools (xcode-select --install)")
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	// Built beside the target and renamed over it, so the running copy is
	// never a half-written file.
	tmp := bin + ".new"
	core.Info("Building alttab...")
	if err := runCmd(ctx, "swiftc", "-O", "-swift-version", "5", "-o", tmp, alttabSource()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("building alttab: %w", err)
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
	if core.DryRun {
		core.Changed("would stop alttab and remove %s and %s", alttabAgent(), alttabBinary())
		return nil
	}
	if alttabLoaded(ctx) {
		if err := runCmd(ctx, "launchctl", "bootout", launchdDomain()+"/"+alttabLabel); err != nil {
			core.Warn("Could not stop alttab: %v", err)
		}
	}
	for _, p := range []string{alttabAgent(), alttabBinary()} {
		if err := core.CheckTarget(p); err != nil {
			return err
		}
		if err := os.Remove(p); err == nil {
			core.Changed("removed %s", p)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
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
	var notes []string
	if !fileExists(alttabBinary()) {
		notes = append(notes, "binary missing")
	} else if alttabStale() {
		notes = append(notes, "binary older than source")
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
