package modules

import (
	"context"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// ClaudeModule links the global Claude Code instructions and merges a tracked
// base into ~/.claude/settings.json, which Claude Code rewrites itself
// (/config, /model, plugin toggles). Only the top-level keys in
// config/claude/settings.json are managed. Everything else in the live file,
// notably the machine-specific autoMode environment and anything in
// settings.local.json, is never touched.
type ClaudeModule struct{}

func (ClaudeModule) Name() string { return "claude" }

func (ClaudeModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("claude", "CLAUDE.md"), Dst: core.HomeTarget(".claude", "CLAUDE.md")},
	}
}

func (m ClaudeModule) Install(ctx context.Context) error {
	core.Info("Linking Claude Code config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	if err := claudeSettings().merge(); err != nil {
		return err
	}
	core.Ok("claude config done")
	return nil
}

// Uninstall removes the CLAUDE.md link but leaves the merged settings: taking
// the keys back out would silently re-enable commit trailers.
func (m ClaudeModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("claude config uninstalled (settings.json left as is)")
	return nil
}

// Status counts only the link, so it agrees with Links(); settings drift is
// reported in Extra.
func (m ClaudeModule) Status() core.ModuleStatus {
	s := m.Links().Status("claude")
	s.Extra = claudeSettings().statusExtra()
	return s
}

// ContentDrift reports managed settings keys that something else changed,
// typically Claude Code's own /config.
func (ClaudeModule) ContentDrift() []DiffProblem { return claudeSettings().drift() }

func claudeSettings() managedJSON {
	return managedJSON{
		label: "Claude settings",
		base:  core.ConfigPath("claude", "settings.json"),
		live:  core.HomeTarget(".claude", "settings.json"),
	}
}
