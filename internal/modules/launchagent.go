package modules

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sresarehumantoo/dotfiles/internal/core"
)

// A LaunchAgent is a login item: macOS announces it and an MDM may forbid it,
// so a module only creates one when installed by name, and `install all` only
// keeps an existing one current.

func launchdDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

func launchAgentPath(label string) string {
	return core.HomeTarget("Library", "LaunchAgents", label+".plist")
}

// launchAgentPlist renders an agent that runs program at login. extra is
// raw plist XML for the keys that differ per agent (KeepAlive, StartInterval).
func launchAgentPlist(label, program, logPath, extra string) []byte {
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
	<string>` + esc(label) + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + esc(program) + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>` + extra + `
	<key>LimitLoadToSessionType</key>
	<string>Aqua</string>
	<key>StandardOutPath</key>
	<string>` + esc(logPath) + `</string>
	<key>StandardErrorPath</key>
	<string>` + esc(logPath) + `</string>
</dict>
</plist>
`)
}

func launchAgentLoaded(ctx context.Context, label string) bool {
	_, err := runProbe(ctx, "launchctl", "print", launchdDomain()+"/"+label)
	return err == nil
}

// loadLaunchAgent starts an agent that is not loaded, reloads one whose plist
// changed (kickstart keeps the plist launchd already read), and restarts one
// whose program was rebuilt.
func loadLaunchAgent(ctx context.Context, label string, plistChanged, restart bool) error {
	target := launchdDomain() + "/" + label
	loaded := launchAgentLoaded(ctx, label)
	if loaded && plistChanged {
		if err := runCmd(ctx, "launchctl", "bootout", target); err != nil {
			return fmt.Errorf("unloading %s: %w", label, err)
		}
		loaded = false
	}
	switch {
	case !loaded:
		if err := runCmd(ctx, "launchctl", "bootstrap", launchdDomain(), launchAgentPath(label)); err != nil {
			return fmt.Errorf("loading %s: %w", label, err)
		}
		core.Changed("started %s", label)
	case restart:
		if err := runCmd(ctx, "launchctl", "kickstart", "-k", target); err != nil {
			return fmt.Errorf("restarting %s: %w", label, err)
		}
		core.Changed("restarted %s", label)
	}
	return nil
}

// removeLaunchAgent unloads the agent and deletes its plist and the files it
// ran.
func removeLaunchAgent(ctx context.Context, label string, files ...string) error {
	if core.DryRun {
		core.Changed("would stop %s and remove its files", label)
		return nil
	}
	if launchAgentLoaded(ctx, label) {
		if err := runCmd(ctx, "launchctl", "bootout", launchdDomain()+"/"+label); err != nil {
			core.Warn("Could not stop %s: %v", label, err)
		}
	}
	for _, p := range append([]string{launchAgentPath(label)}, files...) {
		if err := core.CheckTarget(p); err != nil {
			return err
		}
		if err := os.Remove(p); err == nil {
			core.Changed("removed %s", p)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
