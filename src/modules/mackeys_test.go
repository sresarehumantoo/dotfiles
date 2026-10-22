package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Abbreviated `defaults export` output: the decoder must handle the doctype,
// nesting, every scalar type, and the symbolic hotkey shape.
const plistFixture = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>ApplePressAndHoldEnabled</key>
	<false/>
	<key>KeyRepeat</key>
	<integer>2</integer>
	<key>com.apple.mouse.scaling</key>
	<real>0.875</real>
	<key>NSUserKeyEquivalents</key>
	<dict>
		<key>Copy</key>
		<string>^c</string>
	</dict>
	<key>AppleSymbolicHotKeys</key>
	<dict>
		<key>60</key>
		<dict>
			<key>enabled</key>
			<false/>
			<key>value</key>
			<dict>
				<key>parameters</key>
				<array>
					<integer>32</integer>
					<integer>49</integer>
					<integer>262144</integer>
				</array>
				<key>type</key>
				<string>standard</string>
			</dict>
		</dict>
		<key>64</key>
		<dict>
			<key>enabled</key>
			<integer>1</integer>
		</dict>
	</dict>
	<key>Blob</key>
	<data>AAEC</data>
</dict>
</plist>
`

func TestParsePlist(t *testing.T) {
	v, err := parsePlist([]byte(plistFixture))
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if m["ApplePressAndHoldEnabled"] != false || m["KeyRepeat"] != int64(2) || m["com.apple.mouse.scaling"] != 0.875 {
		t.Errorf("scalars decoded as %#v %#v %#v", m["ApplePressAndHoldEnabled"], m["KeyRepeat"], m["com.apple.mouse.scaling"])
	}
	if keq := m["NSUserKeyEquivalents"].(map[string]any); keq["Copy"] != "^c" {
		t.Errorf("NSUserKeyEquivalents = %#v", keq)
	}
	hotkeys := m["AppleSymbolicHotKeys"].(map[string]any)
	if !hotkeyDisabled(hotkeys, 60) || hotkeyDisabled(hotkeys, 64) || hotkeyDisabled(hotkeys, 99) {
		t.Errorf("hotkeyDisabled wrong for %#v", hotkeys)
	}
}

// What the module writes must read back as set, or every run rewrites it.
func TestWrittenPlistsReadBack(t *testing.T) {
	v, err := parsePlist([]byte(modifierPairsPlist()))
	if err != nil {
		t.Fatal(err)
	}
	if !modifierSwapSet(v) {
		t.Errorf("modifierPairsPlist does not satisfy modifierSwapSet: %#v", v)
	}
	if modifierSwapSet(v.([]any)[1:]) {
		t.Error("a partial mapping counts as set")
	}
	for _, enabled := range []bool{true, false} {
		h, err := parsePlist([]byte(hotkeyPlist(enabled, [3]int64{32, 49, 262144})))
		if err != nil {
			t.Fatal(err)
		}
		if got := hotkeyDisabled(map[string]any{"60": h}, 60); got == enabled {
			t.Errorf("hotkeyPlist(%v) reads back as disabled=%v", enabled, got)
		}
	}
}

// A terminal entry with plain Ctrl would take that key from the shell, which
// is the whole reason terminals get their own table.
func TestTerminalShortcutsKeepCtrl(t *testing.T) {
	plainCtrl := regexp.MustCompile(`^\^[^$@~]$`)
	for _, s := range appShortcuts {
		if s.terminal == "" || plainCtrl.MatchString(s.terminal) {
			t.Errorf("%q: terminal binding %q leaves a plain Ctrl key to the menu", s.title, s.terminal)
		}
	}
}

// Layout assumed from macOS's `hidutil list`: product names contain spaces,
// and the Devices table that follows must not be read as services.
func TestParseHidutilServices(t *testing.T) {
	out := `Services:
VendorID ProductID LocationID UsagePage Usage RegistryID  Transport Class                         Product                            UserClass                           Built-In
0x5ac    0x342     0x0        0x1       0x6   0x1000009b1 SPI       AppleHIDKeyboardEventDriverV2 Apple Internal Keyboard / Trackpad IOHIDEventServiceFastPathUserClient 1
0x5ac    0x342     0x0        0xff00    0x3   0x1000009b4 SPI       AppleHIDKeyboardEventDriverV2 Apple Internal Keyboard / Trackpad IOHIDEventServiceFastPathUserClient 1
Devices:
VendorID ProductID Product
0x46d    0xc52b    USB Receiver
`
	ids := parseHidutilServices(out)
	if len(ids) != 1 || ids[0] != [2]int64{0x5ac, 0x342} {
		t.Errorf("got %v, want [[1452 834]]", ids)
	}
	if ids := parseHidutilServices("Services:\nVendorID ProductID\n"); len(ids) != 0 {
		t.Errorf("empty table gave %v", ids)
	}
}

func TestPassthroughShortcutsHaveNoCtrl(t *testing.T) {
	for _, s := range appShortcuts {
		if s.mac == "" || strings.Contains(s.mac, "^") {
			t.Errorf("%q: passthrough binding %q would keep Ctrl from the guest", s.title, s.mac)
		}
	}
}

// Runs the generated login helper against a fake hidutil: it must leave a
// live swap alone, and set exactly modifierPairsJSON when the swap is gone.
func TestMackeysApplyScript(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "mackeys-apply")
	if err := os.WriteFile(script, mackeysApplyScript(), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := `#!/bin/sh
case "$*" in
*--get*) cat "$FAKE_LIVE" ;;
*--set*) shift 3; printf '%s' "$2" > "$FAKE_SET" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "hidutil"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	live := fmt.Sprintf("RegistryID Key Value\n100 HIDKeyboardModifierMappingPairs (\n{ HIDKeyboardModifierMappingDst = %d; HIDKeyboardModifierMappingSrc = %d; },\n{ HIDKeyboardModifierMappingDst = %d; HIDKeyboardModifierMappingSrc = %d; }\n)\n",
		hidLeftCtrl, hidGlobe, hidLeftOpt, hidLeftCmd)
	for _, c := range []struct {
		name, live string
		wantSet    bool
	}{
		{"swap live", live, false},
		{"swap gone", "RegistryID Key Value\n100 HIDKeyboardModifierMappingPairs (null)\n", true},
	} {
		liveFile, setFile := filepath.Join(dir, "live"), filepath.Join(dir, "set")
		_ = os.Remove(setFile)
		if err := os.WriteFile(liveFile, []byte(c.live), 0o644); err != nil {
			t.Fatal(err)
		}
		if c.wantSet == modifierSwapLive(c.live) {
			t.Errorf("%s: modifierSwapLive disagrees with the fixture", c.name)
		}
		cmd := exec.Command("/bin/sh", script)
		cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin", "FAKE_LIVE="+liveFile, "FAKE_SET="+setFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", c.name, err, out)
		}
		set, err := os.ReadFile(setFile)
		switch {
		case !c.wantSet && err == nil:
			t.Errorf("%s: helper set the mapping again: %s", c.name, set)
		case c.wantSet && string(set) != modifierPairsJSON():
			t.Errorf("%s: helper set %q, want %q", c.name, set, modifierPairsJSON())
		}
	}
}
