package modules

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// MackeysModule makes a Mac keyboard behave like a Windows laptop's without
// Accessibility, Input Monitoring or admin rights: everything here is a user
// preference or a key mapping any process may set.
type MackeysModule struct{}

func (MackeysModule) Name() string { return "mackeys" }

func (MackeysModule) Links() core.LinkSet {
	if !core.IsMac() {
		return nil
	}
	return core.LinkSet{
		{Src: core.ConfigPath("macos", "DefaultKeyBinding.dict"), Dst: core.HomeTarget("Library", "KeyBindings", "DefaultKeyBinding.dict")},
	}
}

// HID usages as the modifier-mapping preference stores them: page << 32 | usage.
const (
	hidGlobe    int64 = 0xFF00000003
	hidLeftCtrl int64 = 0x7000000E0
	hidLeftOpt  int64 = 0x7000000E2
	hidLeftCmd  int64 = 0x7000000E3
	hidRightOpt int64 = 0x7000000E6
	hidRightCmd int64 = 0x7000000E7
)

// The built-in row fn/Globe, Control, Option, Command becomes Control, Globe,
// Command, Option: the Ctrl, Fn, Win, Alt order of a Windows laptop.
var modifierSwaps = [][2]int64{
	{hidGlobe, hidLeftCtrl}, {hidLeftCtrl, hidGlobe},
	{hidLeftOpt, hidLeftCmd}, {hidLeftCmd, hidLeftOpt},
	{hidRightOpt, hidRightCmd}, {hidRightCmd, hidRightOpt},
}

const builtinKeyboardMatch = `{"PrimaryUsagePage":1,"PrimaryUsage":6,"Built-In":true}`

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

// mackeysApplyScript is the login helper. The preference is what macOS
// reapplies on its own; this covers the times it does not (a keyboard service
// that restarted, a preference key the system ignored) by setting the live
// mapping again whenever it has gone missing.
func mackeysApplyScript() []byte {
	return []byte(fmt.Sprintf(`#!/bin/sh
# Written by dfinstall's mackeys module and run by the %s
# LaunchAgent at login, on wake and every 10 minutes. Silent unless it acts.
match='%s'
live=$(hidutil property --matching "$match" --get HIDKeyboardModifierMappingPairs 2>/dev/null)
case $live in *%d*) case $live in *%d*) exit 0 ;; esac ;; esac
hidutil property --matching "$match" --set '%s' >/dev/null &&
	echo "$(date '+%%F %%T') reapplied the modifier swap"
`, mackeysLabel, builtinKeyboardMatch, hidGlobe, hidLeftCmd, modifierPairsJSON()))
}

// modifierSwapLive reads `hidutil property --get HIDKeyboardModifierMappingPairs`
// the way the login helper does.
func modifierSwapLive(out string) bool {
	return strings.Contains(out, strconv.FormatInt(hidGlobe, 10)) && strings.Contains(out, strconv.FormatInt(hidLeftCmd, 10))
}

// installMackeysHelper adds the login helper on `install mackeys` and keeps
// an existing one current under `install all`; see launchagent.go.
func installMackeysHelper(ctx context.Context) error {
	agent := launchAgentPath(mackeysLabel)
	if core.InstallingAll && !fileExists(agent) {
		core.Debug("mackeys login helper not installed — skipping")
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

const activateSettings = "/System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings"

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
		applyModifierSwap, applyKeyDefaults, applyAppShortcuts, applyHotkeys,
	} {
		c, err := step(ctx)
		if err != nil {
			return err
		}
		changed = changed || c
	}
	if err := installMackeysHelper(ctx); err != nil {
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
			return err
		}
	}
	for _, h := range disabledHotkeys {
		if err := runCmd(ctx, "defaults", "write", "com.apple.symbolichotkeys", "AppleSymbolicHotKeys",
			"-dict-add", strconv.Itoa(h.id), hotkeyPlist(true, h.params)); err != nil {
			return err
		}
	}
	_ = runCmd(ctx, activateSettings, "-u")
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
	return fmt.Sprintf("com.apple.keyboard.modifiermapping.%d-%d-0", ids[0][0], ids[0][1]), true
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

func modifierPairsJSON() string {
	pairs := make([]string, 0, len(modifierSwaps))
	for _, s := range modifierSwaps {
		pairs = append(pairs, fmt.Sprintf(`{"HIDKeyboardModifierMappingSrc":%d,"HIDKeyboardModifierMappingDst":%d}`, s[0], s[1]))
	}
	return `{"HIDKeyboardModifierMappingPairs":[` + strings.Join(pairs, ",") + `]}`
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

func applyAppShortcuts(ctx context.Context) (bool, error) {
	changed := false
	for _, t := range shortcutTables() {
		c, err := addShortcuts(ctx, t.domain, t.key)
		if err != nil {
			return changed, err
		}
		changed = changed || c
	}
	return changed, nil
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
		if err := runCmd(ctx, activateSettings, "-u"); err != nil {
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
	if out, ok := probe("hidutil", "property", "--matching", builtinKeyboardMatch, "--get", "HIDKeyboardModifierMappingPairs"); !ok || !modifierSwapLive(out) {
		problems = append(problems, "modifier swap not active on the built-in keyboard")
	}
	if v, _ := probe("defaults", "read", "NSGlobalDomain", "ApplePressAndHoldEnabled"); v != "0" {
		problems = append(problems, "press-and-hold is on, so held keys do not repeat")
	}
	helper := "login helper loaded"
	if !fileExists(launchAgentPath(mackeysLabel)) {
		helper = "no login helper (`dfinstall install mackeys` adds it)"
	} else if _, ok := probe("launchctl", "print", launchdDomain()+"/"+mackeysLabel); !ok {
		problems = append(problems, "login helper not loaded")
	}
	if len(problems) == 0 {
		return []DoctorResult{{Name: "mac keyboard", OK: true, Detail: "modifier swap active, key repeat on, " + helper}}
	}
	return []DoctorResult{{Name: "mac keyboard", OK: false, Detail: strings.Join(problems, "; "),
		Extra: []string{"run `dfinstall install mackeys`", "the login helper logs to " + mackeysLog()}}}
}
