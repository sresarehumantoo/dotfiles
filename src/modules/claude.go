package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// ClaudeModule links the global Claude Code instructions and merges a tracked
// base into ~/.claude/settings.json.
//
// settings.json is merged rather than linked because Claude Code rewrites it
// itself (/config, /model, plugin toggles): a symlink would either be written
// through into this repo or replaced by a plain file. Only the top-level keys
// in config/claude/settings.json are managed. Everything else in the live
// file, notably the machine-specific autoMode environment and anything in
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
	if err := mergeClaudeSettings(); err != nil {
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
	_, _, stale, err := claudeSettingsState()
	switch {
	case err != nil:
		s.Extra = "settings.json unreadable"
	case len(stale) > 0:
		s.Extra = "settings.json stale: " + strings.Join(stale, ", ")
	}
	return s
}

// ContentDrift reports managed settings keys that something else changed,
// typically Claude Code's own /config.
func (ClaudeModule) ContentDrift() []DiffProblem {
	if _, err := os.Stat(claudeSettingsPath()); errors.Is(err, fs.ErrNotExist) {
		return nil // never installed here; the missing link already says so
	}
	_, _, stale, err := claudeSettingsState()
	switch {
	case err != nil:
		return []DiffProblem{{Dst: claudeSettingsPath(), State: "content", Detail: "unreadable"}}
	case len(stale) > 0:
		return []DiffProblem{{Dst: claudeSettingsPath(), State: "content", Detail: "managed keys differ: " + strings.Join(stale, ", ")}}
	}
	return nil
}

func claudeSettingsPath() string { return core.HomeTarget(".claude", "settings.json") }

// claudeSettingsState returns the tracked base, the live settings (empty when
// the file does not exist yet) and the managed keys whose live value differs.
func claudeSettingsState() (base, live map[string]any, stale []string, err error) {
	base, err = readJSONObject(core.ConfigPath("claude", "settings.json"))
	if err != nil {
		return nil, nil, nil, err
	}
	live, err = readJSONObject(claudeSettingsPath())
	if errors.Is(err, fs.ErrNotExist) {
		live, err = map[string]any{}, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	for k, v := range base {
		if lv, ok := live[k]; !ok || !reflect.DeepEqual(lv, v) {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	return base, live, stale, nil
}

func mergeClaudeSettings() error {
	path := claudeSettingsPath()
	if err := core.CheckTarget(path); err != nil {
		return err
	}
	base, live, stale, err := claudeSettingsState()
	if err != nil {
		// Never replace a file we could not parse: it holds settings we don't own.
		return fmt.Errorf("not merging Claude settings: %w", err)
	}
	if len(stale) == 0 {
		core.Ok("Claude settings already up to date")
		return nil
	}
	if core.DryRun {
		core.Info("would set %s in %s", strings.Join(stale, ", "), path)
		return nil
	}

	for _, k := range stale {
		live[k] = base[k]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// The live file holds prose with <, > and &; keep them readable.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(live); err != nil {
		return err
	}

	if err := core.BackupFile(path); err != nil {
		core.Warn("backup failed for %s: %v", path, err)
	}
	perm := fs.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := core.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings.json-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	core.Ok("Claude settings: set %s", strings.Join(stale, ", "))
	return nil
}

// readJSONObject decodes a JSON object with UseNumber so integers round-trip
// exactly through a rewrite.
func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m == nil {
		return nil, fmt.Errorf("%s: not a JSON object", path)
	}
	return m, nil
}
