package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// MackeysModule makes a Mac keyboard behave like a Windows laptop's without
// Accessibility, Input Monitoring or admin rights: everything here is a user
// preference or a key mapping any process may set.
type MackeysModule struct{}

func (MackeysModule) Name() string { return "mackeys" }

func (MackeysModule) Unsupported() string { return macOnly() }

func (MackeysModule) Links() core.LinkSet {
	if !core.IsMac() {
		return nil
	}
	return core.LinkSet{
		{Src: core.ConfigPath("macos", "DefaultKeyBinding.dict"), Dst: core.HomeTarget("Library", "KeyBindings", "DefaultKeyBinding.dict")},
	}
}

// HID usages as the modifier-mapping preference stores them: page << 32 | usage.
// Recent built-in keyboards report Globe on the Apple vendor page 0xFF01 as
// well as on 0xFF, and System Settings maps both.
const (
	hidGlobe       int64 = 0xFF00000003
	hidGlobeVendor int64 = 0xFF0100000003
	hidLeftCtrl    int64 = 0x7000000E0
	hidLeftOpt     int64 = 0x7000000E2
	hidLeftCmd     int64 = 0x7000000E3
	hidRightOpt    int64 = 0x7000000E6
	hidRightCmd    int64 = 0x7000000E7
)

// The built-in row fn/Globe, Control, Option, Command becomes Control, Globe,
// Command, Option: the Ctrl, Fn, Win, Alt order of a Windows laptop.
var modifierSwaps = [][2]int64{
	{hidGlobe, hidLeftCtrl}, {hidGlobeVendor, hidLeftCtrl}, {hidLeftCtrl, hidGlobe},
	{hidLeftOpt, hidLeftCmd}, {hidLeftCmd, hidLeftOpt},
	{hidRightOpt, hidRightCmd}, {hidRightCmd, hidRightOpt},
}

// The command layout, for commandLayoutApps, swaps the Control and Command
// the standard layout produces, so the Ctrl-position key sends Command and the
// Win-position key sends Control. It is a UserKeyMapping, which the keyboard
// filter applies after the modifier mapping, so it sits on top of the standard
// layout rather than replacing it: on macOS 26 a live
// HIDKeyboardModifierMappingPairs write never took effect, and UserKeyMapping
// did.
var commandLayoutSwaps = [][2]int64{{hidLeftCtrl, hidLeftCmd}, {hidLeftCmd, hidLeftCtrl}}

const builtinKeyboardMatch = `{"PrimaryUsagePage":1,"PrimaryUsage":6,"Built-In":true}`

// keyboardMatch takes in external keyboards too. The command layout goes on
// every keyboard: a browser's menus are back on Command, so a keyboard left
// without the swap sends a Ctrl+R nothing is bound to.
const keyboardMatch = `{"PrimaryUsagePage":1,"PrimaryUsage":6}`

type macDefault struct {
	domain, key string
	want        any // bool or int64
}

var macKeyDefaults = []macDefault{
	// Holding a key repeats it instead of offering accents, so held j/k scroll.
	{"NSGlobalDomain", "ApplePressAndHoldEnabled", false},
	{"NSGlobalDomain", "KeyRepeat", int64(2)},
	{"NSGlobalDomain", "InitialKeyRepeat", int64(15)},
	// Tapping Globe, now where Fn sits on a PC, does nothing.
	{"com.apple.HIToolbox", "AppleFnUsageType", int64(0)},
	// Scrolling moves the view the way the wheel turns, as on Windows. The
	// login helper sets this again, since it is easily reset.
	{"NSGlobalDomain", "com.apple.swipescrolldirection", false},
}

// appShortcut is a menu title bound to Ctrl in every app; to Ctrl+Shift or
// Command in terminals, where plain Ctrl belongs to the shell; and back to the
// macOS default in passthrough apps. Titles must match the menu item exactly,
// ellipsis character included.
type appShortcut struct {
	title, ctrl, terminal, mac string
}

// Modifier prefixes: ^ Control, $ Shift, @ Command, ~ Option.
var appShortcuts = []appShortcut{
	{"Copy", "^c", "^$c", "@c"},
	{"Paste", "^v", "^$v", "@v"},
	{"Paste and Match Style", "^$v", "@~$v", "@~$v"},
	{"Cut", "^x", "^$x", "@x"},
	{"Select All", "^a", "^$a", "@a"},
	{"Undo", "^z", "@z", "@z"},
	{"Redo", "^y", "@$z", "@$z"},
	{"Find…", "^f", "^$f", "@f"},
	{"Find...", "^f", "^$f", "@f"},
	{"Save", "^s", "@s", "@s"},
	{"Save As…", "^$s", "@$s", "@$s"},
	{"Open…", "^o", "@o", "@o"},
	{"Print…", "^p", "@p", "@p"},
	{"New Tab", "^t", "^$t", "@t"},
	{"Close Tab", "^w", "^$w", "@w"},
	{"New Window", "^n", "^$n", "@n"},
	{"Close Window", "^$w", "@$w", "@$w"},
	{"Reopen Closed Tab", "^$t", "@$t", "@$t"},
	{"New Incognito Window", "^$n", "@$n", "@$n"},
	{"New Private Window", "^$n", "@$n", "@$n"},
	{"Reload This Page", "^r", "@r", "@r"},
	{"Reload Page", "^r", "@r", "@r"},
	{"Open Location…", "^l", "@l", "@l"},
	{"Zoom In", "^=", "@=", "@="},
	{"Zoom Out", "^-", "@-", "@-"},
	{"Actual Size", "^0", "@0", "@0"},
	{"Bold", "^b", "@b", "@b"},
	{"Italic", "^i", "@i", "@i"},
	{"Underline", "^u", "@u", "@u"},
}

// An app's own NSUserKeyEquivalents is consulted before the global one, so
// every global title is restated here, not only the clipboard ones: a title
// left out would fall through to its Ctrl binding and take Ctrl+Z or Ctrl+W
// from the shell.
var terminalApps = []string{"com.mitchellh.ghostty", "com.apple.Terminal", "com.googlecode.iterm2"}

// Remote desktops and VMs hand keys to a guest that expects Ctrl itself, so a
// Ctrl menu shortcut would copy on the Mac instead of in the guest. They get
// the macOS defaults back.
var passthroughApps = []string{
	"com.microsoft.rdc.macos",       // Windows App / Microsoft Remote Desktop
	"com.citrix.receiver.nomas",     // Citrix Workspace
	"com.vmware.horizon",            // Omnissa/VMware Horizon Client
	"com.vmware.fusion",             // VMware Fusion
	"com.parallels.desktop.console", // Parallels Desktop
	"com.utmapp.UTM",                // UTM
}

// Web pages bind their shortcuts to Cmd on a Mac and ignore a Ctrl+Z they
// cannot see as one, and Mac Office uses the Windows letters on Cmd, so in
// these apps the mackeys-layout agent puts Command on the Ctrl key (and Ctrl
// on the Win key). Matched exactly: a
// Chrome PWA has its own bundle ID and keeps the standard layout. Safari is
// left out because its preferences cannot be written without Full Disk
// Access, and an app on this layout needs its menus moved back to Cmd.
var commandLayoutApps = []string{
	"com.google.Chrome", "com.google.Chrome.beta", "com.google.Chrome.canary",
	"com.microsoft.edgemac", "com.brave.Browser", "company.thebrowser.Browser",
	"com.vivaldi.Vivaldi", "org.mozilla.firefox",
	"com.tinyspeck.slackmacgap", "com.microsoft.teams2", "com.microsoft.VSCode",
	"com.hnc.Discord", "notion.id", "md.obsidian", "us.zoom.xos",
	"com.microsoft.Word", "com.microsoft.Excel", "com.microsoft.Powerpoint",
	"com.microsoft.Outlook", "com.microsoft.onenote.mac",
}

// Apps on the command layout get the macOS shortcuts back, which already put
// Windows' letters on the key that now sends Cmd, except where Windows differs.
var commandLayoutShortcuts = map[string]string{"Paste and Match Style": "@$v"}

// shortcutTable pairs a preference domain with the column it gets.
type shortcutTable struct {
	domain string
	key    func(appShortcut) string
}

func shortcutTables() []shortcutTable {
	tables := []shortcutTable{{"NSGlobalDomain", func(s appShortcut) string { return s.ctrl }}}
	for _, app := range terminalApps {
		tables = append(tables, shortcutTable{app, func(s appShortcut) string { return s.terminal }})
	}
	for _, app := range passthroughApps {
		tables = append(tables, shortcutTable{app, func(s appShortcut) string { return s.mac }})
	}
	for _, app := range commandLayoutApps {
		tables = append(tables, shortcutTable{app, func(s appShortcut) string {
			if k, ok := commandLayoutShortcuts[s.title]; ok {
				return k
			}
			return s.mac
		}})
	}
	return tables
}

// Symbolic hotkeys that take Ctrl+arrows (Mission Control, Spaces) and
// Ctrl+Space (input sources), which the shell, nvim and the key bindings
// above want. Parameters are the stock bindings, so re-enabling restores them.
var disabledHotkeys = []struct {
	id     int
	params [3]int64
}{
	{32, [3]int64{65535, 126, 8650752}}, // Mission Control: Ctrl+Up
	{33, [3]int64{65535, 125, 8650752}}, // Application windows: Ctrl+Down
	{79, [3]int64{65535, 123, 8650752}}, // Move left a space: Ctrl+Left
	{80, [3]int64{65535, 123, 8781824}},
	{81, [3]int64{65535, 124, 8650752}}, // Move right a space: Ctrl+Right
	{82, [3]int64{65535, 124, 8781824}},
	{60, [3]int64{32, 49, 262144}}, // Previous input source: Ctrl+Space
	{61, [3]int64{32, 49, 786432}}, // Next input source: Ctrl+Option+Space
}

const mackeysLabel = "io.github.sresarehumantoo.mackeys"

func mackeysHelper() string { return core.HomeTarget(".local", "bin", "mackeys-apply") }
func mackeysLog() string    { return core.HomeTarget("Library", "Logs", "mackeys.log") }

// launchd runs an interval job that fell due during sleep as soon as the Mac
// wakes, so this covers wake as well as login.
const mackeysAgentExtra = `
	<key>StartInterval</key>
	<integer>600</integer>`

// mackeysApplyScript is the login helper: it turns natural scrolling off again
// whenever something turns it back on. The modifier swap needs no such help,
// since macOS reapplies the preference itself, and could not be checked from
// here anyway: the keyboard filter never reports its live mapping, so it reads
// as null on every Mac.
func mackeysApplyScript() []byte {
	return []byte(fmt.Sprintf(`#!/bin/sh
# Written by dfinstall's mackeys module and run by the %s
# LaunchAgent at login, on wake and every 10 minutes. Silent unless it acts.
if [ "$(defaults read -g com.apple.swipescrolldirection 2>/dev/null)" != 0 ]; then
	defaults write -g com.apple.swipescrolldirection -bool false &&
		echo "$(date '+%%F %%T') turned natural scrolling off again"
fi
exit 0
`, mackeysLabel))
}

// installMackeysHelper adds the login helper on `install mackeys` and keeps
// an existing one current under `install all`; see launchagent.go.
func installMackeysHelper(ctx context.Context) error {
	agent := launchAgentPath(mackeysLabel)
	if core.InstallingAll && !fileExists(agent) {
		core.Debug("mackeys login helper not installed, skipping")
		return nil
	}
	scriptChanged, err := core.WriteFileIfChanged(mackeysHelper(), mackeysApplyScript(), 0o755)
	if err != nil {
		return err
	}
	agentChanged, err := core.WriteFileIfChanged(agent, launchAgentPlist(mackeysLabel, mackeysHelper(), mackeysLog(), mackeysAgentExtra), 0o644)
	if err != nil || core.DryRun {
		return err
	}
	return loadLaunchAgent(ctx, mackeysLabel, agentChanged, scriptChanged)
}

const layoutLabel = "io.github.sresarehumantoo.mackeys-layout"

func layoutSource() string { return core.ConfigPath("mackeys", "layout.swift") }
func layoutBinary() string { return core.HomeTarget(".local", "bin", "mackeys-layout") }
func layoutLog() string    { return core.HomeTarget("Library", "Logs", "mackeys-layout.log") }

// layoutConfigPath is read by mackeys-layout and by alttab, which moves its
// lock hotkey with the layout. It lives under Library rather than
// XDG_CONFIG_HOME, which a LaunchAgent does not inherit.
func layoutConfigPath() string {
	return core.HomeTarget("Library", "Application Support", core.AppDirName, "mackeys-layout.json")
}

// layoutConfig holds both layouts as the JSON hidutil --set takes, so the HID
// values live only in this file.
func layoutConfig(apps []string) []byte {
	b, _ := json.MarshalIndent(struct {
		Match    string   `json:"match"`
		Standard string   `json:"standard"`
		Command  string   `json:"command"`
		Apps     []string `json:"apps"`
	}{keyboardMatch, standardLayoutJSON, commandLayoutJSON(), apps}, "", "  ")
	return append(b, '\n')
}

// installLayoutAgent builds and starts mackeys-layout on `install mackeys`,
// and keeps an existing one current under `install all`. Without swiftc every
// app keeps the standard layout, which is a degraded setup, not a failed one.
// unwritable names the preference domains applyAppShortcuts could not write:
// such an app keeps the standard layout, since with the Ctrl key sending Cmd
// its menus would still be waiting for Ctrl.
func installLayoutAgent(ctx context.Context, unwritable []string) error {
	agent := launchAgentPath(layoutLabel)
	if core.InstallingAll && !fileExists(agent) {
		core.Debug("mackeys layout agent not installed, skipping")
		return nil
	}
	rebuilt := false
	if swiftStale(layoutSource(), layoutBinary()) {
		if _, err := exec.LookPath("swiftc"); err != nil && !core.DryRun {
			core.Warn("swiftc not found, so browsers keep the standard layout: run xcode-select --install, then install mackeys again")
			return nil
		}
		if err := buildSwift(ctx, "mackeys-layout", layoutSource(), layoutBinary()); err != nil {
			return err
		}
		rebuilt = true
	}
	var apps []string
	for _, app := range commandLayoutApps {
		if !slices.Contains(unwritable, app) {
			apps = append(apps, app)
		}
	}
	configChanged, err := core.WriteFileIfChanged(layoutConfigPath(), layoutConfig(apps), 0o644)
	if err != nil {
		return err
	}
	agentChanged, err := core.WriteFileIfChanged(agent, launchAgentPlist(layoutLabel, layoutBinary(), layoutLog(), alttabAgentExtra), 0o644)
	if err != nil || core.DryRun {
		return err
	}
	return loadLaunchAgent(ctx, layoutLabel, agentChanged, rebuilt || configChanged)
}

const activateSettingsPath = "/System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings"

// activateSettings is the tool that applies changed keyboard settings to the
// running session. One on PATH wins over the system's, which is never on PATH,
// so scripts/macos-harness can stand in for it like any other command.
func activateSettings() string {
	if p, err := exec.LookPath("activateSettings"); err == nil {
		return p
	}
	return activateSettingsPath
}

func (m MackeysModule) Install(ctx context.Context) error {
	if !core.IsMac() {
		core.Ok("mackeys is macOS only, skipping")
		return nil
	}
	if err := m.Links().Apply(); err != nil {
		return err
	}
	changed := false
	for _, step := range []func(context.Context) (bool, error){
		applyModifierSwap, applyKeyDefaults, applyHotkeys,
	} {
		c, err := step(ctx)
		if err != nil {
			return err
		}
		changed = changed || c
	}
	c, unwritable, err := applyAppShortcuts(ctx)
	if err != nil {
		return err
	}
	changed = changed || c
	if err := installMackeysHelper(ctx); err != nil {
		return err
	}
	if err := installLayoutAgent(ctx, unwritable); err != nil {
		return err
	}
	if changed && !core.DryRun {
		core.Notice("Quit and reopen apps to pick up the new shortcuts and key repeat; a logout covers everything")
	}
	core.Ok("mackeys done")
	return nil
}

func (m MackeysModule) Uninstall(ctx context.Context) error {
	if !core.IsMac() {
		return nil
	}
	if err := m.Links().Remove(); err != nil {
		return err
	}
	if err := removeLaunchAgent(ctx, mackeysLabel, mackeysHelper()); err != nil {
		return err
	}
	if err := removeLaunchAgent(ctx, layoutLabel, layoutBinary(), layoutConfigPath()); err != nil {
		return err
	}
	// The agent may have stopped with a browser in front.
	if !core.DryRun {
		if err := runCmd(ctx, "hidutil", "property", "--matching", keyboardMatch, "--set", standardLayoutJSON); err != nil {
			core.Warn("Could not clear the command layout: %v", err)
		}
	}
	if core.DryRun {
		core.Changed("would remove the modifier swap, key defaults, app shortcuts and hotkey changes")
		return nil
	}
	if key, ok := builtinKeyboardKey(ctx); ok {
		if _, err := runProbe(ctx, "defaults", "-currentHost", "delete", "NSGlobalDomain", key); err == nil {
			core.Changed("removed %s", key)
		}
		if err := runCmd(ctx, "hidutil", "property", "--matching", builtinKeyboardMatch,
			"--set", `{"HIDKeyboardModifierMappingPairs":[]}`); err != nil {
			core.Warn("Could not clear the live modifier mapping: %v", err)
		}
	}
	for _, d := range macKeyDefaults {
		if _, err := runProbe(ctx, "defaults", "delete", d.domain, d.key); err == nil {
			core.Changed("reset %s %s", d.domain, d.key)
		}
	}
	for _, t := range shortcutTables() {
		if err := removeShortcuts(ctx, t.domain, t.key); err != nil {
			if t.domain == "NSGlobalDomain" {
				return err
			}
			core.Warn("Could not remove the app shortcuts for %s: %v", t.domain, err)
		}
	}
	for _, h := range disabledHotkeys {
		if err := runCmd(ctx, "defaults", "write", "com.apple.symbolichotkeys", "AppleSymbolicHotKeys",
			"-dict-add", strconv.Itoa(h.id), hotkeyPlist(true, h.params)); err != nil {
			return err
		}
	}
	_ = runCmd(ctx, activateSettings(), "-u")
	core.Ok("mackeys uninstalled")
	return nil
}

func (m MackeysModule) Status() core.ModuleStatus {
	return m.Links().Status("mackeys")
}

// builtinKeyboardKey names the preference System Settings' Modifier Keys
// writes for the built-in keyboard: com.apple.keyboard.modifiermapping.
// <vendor>-<product>-0, in decimal. The IDs differ between models (some report
// 0-0), so they are read from the HID service rather than assumed.
func builtinKeyboardKey(ctx context.Context) (string, bool) {
	out, err := runProbe(ctx, "hidutil", "list", "--matching", builtinKeyboardMatch)
	if err != nil {
		core.Warn("hidutil list: %v", err)
		return "", false
	}
	ids := parseHidutilServices(string(out))
	if len(ids) != 1 {
		core.Warn("Expected one built-in keyboard from hidutil, found %d; set the modifier keys in System Settings > Keyboard > Keyboard Shortcuts > Modifier Keys instead", len(ids))
		return "", false
	}
	return modifierMappingKey(ids[0]), true
}

func modifierMappingKey(id [2]int64) string {
	return fmt.Sprintf("com.apple.keyboard.modifiermapping.%d-%d-0", id[0], id[1])
}

// modifierSwapSaved reports whether the built-in keyboard's preference holds
// the swap. That preference is the only part of it that can be read back.
func modifierSwapSaved(probe func(string, ...string) (string, bool)) bool {
	out, ok := probe("hidutil", "list", "--matching", builtinKeyboardMatch)
	if !ok {
		return false
	}
	ids := parseHidutilServices(out)
	if len(ids) != 1 {
		return false
	}
	prefs, ok := probe("defaults", "-currentHost", "export", "NSGlobalDomain", "-")
	if !ok {
		return false
	}
	v, err := parsePlist([]byte(prefs))
	m, _ := v.(map[string]any)
	return err == nil && modifierSwapSet(m[modifierMappingKey(ids[0])])
}

// parseHidutilServices returns the distinct VendorID/ProductID pairs in the
// Services table of `hidutil list`. Product names contain spaces, so only the
// two leading columns are trusted.
func parseHidutilServices(out string) [][2]int64 {
	var ids [][2]int64
	inServices, header := false, false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "Services:":
			inServices, header = true, false
			continue
		case strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " "):
			inServices = false
			continue
		}
		if !inServices || trimmed == "" {
			continue
		}
		f := strings.Fields(trimmed)
		if !header {
			header = len(f) > 1 && f[0] == "VendorID"
			continue
		}
		if len(f) < 2 {
			continue
		}
		v, err1 := strconv.ParseInt(f[0], 0, 64)
		p, err2 := strconv.ParseInt(f[1], 0, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		pair := [2]int64{v, p}
		dup := false
		for _, id := range ids {
			dup = dup || id == pair
		}
		if !dup {
			ids = append(ids, pair)
		}
	}
	return ids
}

func modifierPairsPlist() string {
	var b strings.Builder
	b.WriteString("<array>")
	for _, s := range modifierSwaps {
		fmt.Fprintf(&b, "<dict><key>HIDKeyboardModifierMappingSrc</key><integer>%d</integer>"+
			"<key>HIDKeyboardModifierMappingDst</key><integer>%d</integer></dict>", s[0], s[1])
	}
	b.WriteString("</array>")
	return b.String()
}

func modifierPairsJSON() string { return mappingJSON("HIDKeyboardModifierMappingPairs", modifierSwaps) }

func commandLayoutJSON() string { return mappingJSON("UserKeyMapping", commandLayoutSwaps) }

// standardLayoutJSON clears the command layout. It also clears any
// UserKeyMapping set by hand, which this module would not know to restore.
const standardLayoutJSON = `{"UserKeyMapping":[]}`

// mappingJSON renders swaps as the JSON `hidutil property --set` takes for
// property, which is either key the keyboard filter maps by.
func mappingJSON(property string, swaps [][2]int64) string {
	pairs := make([]string, 0, len(swaps))
	for _, s := range swaps {
		pairs = append(pairs, fmt.Sprintf(`{"HIDKeyboardModifierMappingSrc":%d,"HIDKeyboardModifierMappingDst":%d}`, s[0], s[1]))
	}
	return `{"` + property + `":[` + strings.Join(pairs, ",") + `]}`
}

// modifierSwapSet reports whether a modifiermapping preference value holds
// exactly modifierSwaps.
func modifierSwapSet(v any) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) != len(modifierSwaps) {
		return false
	}
	have := map[[2]int64]bool{}
	for _, e := range arr {
		d, ok := e.(map[string]any)
		if !ok {
			return false
		}
		src, ok1 := d["HIDKeyboardModifierMappingSrc"].(int64)
		dst, ok2 := d["HIDKeyboardModifierMappingDst"].(int64)
		if !ok1 || !ok2 {
			return false
		}
		have[[2]int64{src, dst}] = true
	}
	for _, s := range modifierSwaps {
		if !have[s] {
			return false
		}
	}
	return true
}

// applyModifierSwap writes the preference macOS reapplies at every login and
// keyboard attach, and sets the same property on the live service so it takes
// effect now. Both are the layer System Settings uses, so they replace each
// other rather than stacking into a double swap.
func applyModifierSwap(ctx context.Context) (bool, error) {
	key, ok := builtinKeyboardKey(ctx)
	if !ok {
		return false, nil
	}
	if modifierSwapSet(exportDomain(ctx, true, "NSGlobalDomain")[key]) {
		core.Ok("modifier swap already set (%s)", key)
		return false, nil
	}
	if core.DryRun {
		core.Changed("would swap Globe/Control and Option/Command on the built-in keyboard (%s)", key)
		return true, nil
	}
	if err := runCmd(ctx, "defaults", "-currentHost", "write", "NSGlobalDomain", key, modifierPairsPlist()); err != nil {
		return false, fmt.Errorf("writing %s: %w", key, err)
	}
	if err := runCmd(ctx, "hidutil", "property", "--matching", builtinKeyboardMatch, "--set", modifierPairsJSON()); err != nil {
		core.Warn("The swap is saved but not live until the next login: %v", err)
	}
	core.Changed("swapped Globe/Control and Option/Command on the built-in keyboard (%s)", key)
	return true, nil
}

func applyKeyDefaults(ctx context.Context) (bool, error) {
	changed := false
	domains := map[string]map[string]any{}
	for _, d := range macKeyDefaults {
		if _, ok := domains[d.domain]; !ok {
			domains[d.domain] = exportDomain(ctx, false, d.domain)
		}
		if domains[d.domain][d.key] == d.want {
			continue
		}
		changed = true
		if core.DryRun {
			core.Changed("would set %s %s to %v", d.domain, d.key, d.want)
			continue
		}
		typ, val := "-int", fmt.Sprint(d.want)
		if b, ok := d.want.(bool); ok {
			typ, val = "-bool", strconv.FormatBool(b)
		}
		if err := runCmd(ctx, "defaults", "write", d.domain, d.key, typ, val); err != nil {
			return changed, err
		}
		core.Changed("set %s %s to %v", d.domain, d.key, d.want)
	}
	return changed, nil
}

// applyAppShortcuts writes every shortcut table. Only the global one is
// required: a sandboxed app whose container macOS protects (Safari, anything
// under App Data protection) refuses the write without Full Disk Access, and
// that app is reported back rather than failing the module.
func applyAppShortcuts(ctx context.Context) (bool, []string, error) {
	changed := false
	var unwritable []string
	for _, t := range shortcutTables() {
		c, err := addShortcuts(ctx, t.domain, t.key)
		if err != nil {
			if t.domain == "NSGlobalDomain" {
				return changed, nil, err
			}
			unwritable = append(unwritable, t.domain)
			continue
		}
		changed = changed || c
	}
	if len(unwritable) > 0 {
		core.AlwaysWarn("Could not write the app shortcuts for %s. A sandboxed app's preferences can need Full Disk Access for this terminal; those apps keep the standard layout and the global shortcuts.",
			strings.Join(unwritable, ", "))
	}
	return changed, unwritable, nil
}

// addShortcuts merges the shortcuts into a domain's NSUserKeyEquivalents,
// leaving titles the user bound themselves alone.
func addShortcuts(ctx context.Context, domain string, key func(appShortcut) string) (bool, error) {
	current, _ := exportDomain(ctx, false, domain)["NSUserKeyEquivalents"].(map[string]any)
	args := []string{"write", domain, "NSUserKeyEquivalents", "-dict-add"}
	n := 0
	for _, s := range appShortcuts {
		if current[s.title] != key(s) {
			args = append(args, s.title, key(s))
			n++
		}
	}
	if n == 0 {
		core.Ok("app shortcuts already set for %s", domain)
		return false, nil
	}
	if core.DryRun {
		core.Changed("would set %d app shortcut(s) for %s", n, domain)
		return true, nil
	}
	if err := runCmd(ctx, "defaults", args...); err != nil {
		return false, err
	}
	core.Changed("set %d app shortcut(s) for %s", n, domain)
	return true, nil
}

// removeShortcuts drops only the entries this module wrote, and only while
// they still hold its value. `defaults` cannot delete one dictionary entry,
// so the remainder is written back whole.
func removeShortcuts(ctx context.Context, domain string, key func(appShortcut) string) error {
	current, ok := exportDomain(ctx, false, domain)["NSUserKeyEquivalents"].(map[string]any)
	if !ok {
		return nil
	}
	removed := 0
	for _, s := range appShortcuts {
		if current[s.title] == key(s) {
			delete(current, s.title)
			removed++
		}
	}
	if removed == 0 {
		return nil
	}
	var err error
	if len(current) == 0 {
		err = runCmd(ctx, "defaults", "delete", domain, "NSUserKeyEquivalents")
	} else {
		args := []string{"write", domain, "NSUserKeyEquivalents", "-dict"}
		for title, v := range current {
			args = append(args, title, fmt.Sprint(v))
		}
		err = runCmd(ctx, "defaults", args...)
	}
	if err == nil {
		core.Changed("removed %d app shortcut(s) from %s", removed, domain)
	}
	return err
}

func hotkeyPlist(enabled bool, p [3]int64) string {
	return fmt.Sprintf("<dict><key>enabled</key><%t/><key>value</key><dict><key>parameters</key>"+
		"<array><integer>%d</integer><integer>%d</integer><integer>%d</integer></array>"+
		"<key>type</key><string>standard</string></dict></dict>", enabled, p[0], p[1], p[2])
}

func hotkeyDisabled(hotkeys map[string]any, id int) bool {
	h, ok := hotkeys[strconv.Itoa(id)].(map[string]any)
	if !ok {
		return false
	}
	switch v := h["enabled"].(type) {
	case bool:
		return !v
	case int64:
		return v == 0
	}
	return false
}

func applyHotkeys(ctx context.Context) (bool, error) {
	hotkeys, _ := exportDomain(ctx, false, "com.apple.symbolichotkeys")["AppleSymbolicHotKeys"].(map[string]any)
	changed := false
	for _, h := range disabledHotkeys {
		if hotkeyDisabled(hotkeys, h.id) {
			continue
		}
		changed = true
		if core.DryRun {
			core.Changed("would disable system shortcut %d", h.id)
			continue
		}
		if err := runCmd(ctx, "defaults", "write", "com.apple.symbolichotkeys", "AppleSymbolicHotKeys",
			"-dict-add", strconv.Itoa(h.id), hotkeyPlist(false, h.params)); err != nil {
			return changed, err
		}
		core.Changed("disabled system shortcut %d", h.id)
	}
	if changed && !core.DryRun {
		// Applies the hotkey change without a logout. Private, so best-effort.
		if err := runCmd(ctx, activateSettings(), "-u"); err != nil {
			core.Debug("activateSettings: %v", err)
		}
	}
	return changed, nil
}

// exportDomain reads a preference domain through cfprefsd, which is current
// even when the plist on disk is not. A domain that does not exist yet reads
// as empty.
func exportDomain(ctx context.Context, currentHost bool, domain string) map[string]any {
	args := []string{"export", domain, "-"}
	if currentHost {
		args = append([]string{"-currentHost"}, args...)
	}
	out, err := runProbe(ctx, "defaults", args...)
	if err != nil {
		return map[string]any{}
	}
	v, err := parsePlist(out)
	if m, ok := v.(map[string]any); err == nil && ok {
		return m
	}
	return map[string]any{}
}

// parsePlist decodes an XML property list into map[string]any, []any, string,
// int64, float64 and bool. data and date elements come back as strings.
func parsePlist(data []byte) (any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local != "plist" {
			return plistElement(dec, se)
		}
	}
}

func plistElement(dec *xml.Decoder, se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "dict":
		m := map[string]any{}
		key := ""
		for {
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "key" {
					if err := dec.DecodeElement(&key, &t); err != nil {
						return nil, err
					}
					continue
				}
				v, err := plistElement(dec, t)
				if err != nil {
					return nil, err
				}
				m[key] = v
			case xml.EndElement:
				return m, nil
			}
		}
	case "array":
		a := []any{}
		for {
			tok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				v, err := plistElement(dec, t)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			case xml.EndElement:
				return a, nil
			}
		}
	case "true", "false":
		return se.Name.Local == "true", dec.Skip()
	}
	var s string
	if err := dec.DecodeElement(&s, &se); err != nil {
		return nil, err
	}
	switch se.Name.Local {
	case "integer":
		return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	case "real":
		return strconv.ParseFloat(strings.TrimSpace(s), 64)
	}
	return s, nil
}

// mackeysDoctorResult checks what is invisible when it goes missing: the live
// modifier swap, key repeat and the login helper.
func mackeysDoctorResult(probe func(string, ...string) (string, bool)) []DoctorResult {
	if s := (MackeysModule{}).Status(); s.Linked == 0 {
		return nil
	}
	var problems []string
	if !modifierSwapSaved(probe) {
		problems = append(problems, "modifier swap not saved for the built-in keyboard")
	}
	if v, _ := probe("defaults", "read", "NSGlobalDomain", "ApplePressAndHoldEnabled"); v != "0" {
		problems = append(problems, "press-and-hold is on, so held keys do not repeat")
	}
	if v, _ := probe("defaults", "read", "NSGlobalDomain", "com.apple.swipescrolldirection"); v != "0" {
		problems = append(problems, "natural scrolling is on")
	}
	helper := "login helper loaded"
	if !fileExists(launchAgentPath(mackeysLabel)) {
		helper = "no login helper (`dfinstall install mackeys` adds it)"
	} else if _, ok := probe("launchctl", "print", launchdDomain()+"/"+mackeysLabel); !ok {
		problems = append(problems, "login helper not loaded")
	}
	if fileExists(launchAgentPath(layoutLabel)) {
		if out, ok := probe("launchctl", "print", launchdDomain()+"/"+layoutLabel); !ok || !strings.Contains(out, "state = running") {
			problems = append(problems, "layout agent not running, so browsers get the standard layout")
		}
	}
	if len(problems) == 0 {
		return []DoctorResult{{Name: "mac keyboard", OK: true, Detail: "modifier swap saved, key repeat on, natural scrolling off, " + helper}}
	}
	return []DoctorResult{{Name: "mac keyboard", OK: false, Detail: strings.Join(problems, "; "),
		Extra: []string{"run `dfinstall install mackeys`", "the login helper logs to " + mackeysLog(), "the layout agent logs to " + layoutLog()}}}
}
