package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// withConfigDir points DotfilesDir at a temp dir and restores global config
// state afterwards, so these tests can't leak into their neighbours.
func withConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOTFILES", dir)
	core.ResetDotfilesDir()

	origCfg, origExists := core.Cfg, core.CfgFileExists
	t.Cleanup(func() {
		core.Cfg, core.CfgFileExists = origCfg, origExists
		core.ResetDotfilesDir()
	})
	return dir
}

const realUserConfig = `skip_backup: true
extended_plugins:
    - pip
    - sudo
toolkit_tools:
    - nmap
`

// A config that exists but can't be read must not be mistaken for a first run.
// It used to be: LoadConfig returned defaults with CfgFileExists=false, which
// made shouldBackup report a first run, which made InstallSession write
// defaults straight over the file — silently destroying the user's settings.
func TestLoadConfig_UnreadableIsNotAFirstRun(t *testing.T) {
	withConfigDir(t)
	path := core.ConfigFilePath()

	// A directory where the config file should be: readable stat, failing read.
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}

	err := core.LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig accepted an unreadable config without error")
	}
	if !core.CfgFileExists {
		t.Error("CfgFileExists = false — an unreadable config would be treated as a first run")
	}

	// And the destructive follow-up must be blocked.
	if err := core.SaveConfig(); err == nil {
		t.Error("SaveConfig overwrote a config it could not read")
	} else if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("SaveConfig error = %v, want a refusal", err)
	}
}

// Malformed YAML must not be silently replaced with defaults either.
func TestLoadConfig_MalformedIsPreserved(t *testing.T) {
	withConfigDir(t)
	path := core.ConfigFilePath()

	const malformed = "skip_backup: true\n  extended_plugins: [oops\n"
	if err := os.WriteFile(path, []byte(malformed), 0644); err != nil {
		t.Fatal(err)
	}

	if err := core.LoadConfig(); err == nil {
		t.Fatal("LoadConfig accepted malformed YAML without error")
	}

	if err := core.SaveConfig(); err == nil {
		t.Error("SaveConfig overwrote a config it could not parse")
	}

	// The user's file must still be on disk, untouched.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file gone after failed load: %v", err)
	}
	if string(data) != malformed {
		t.Errorf("config file was modified:\n%s", data)
	}
}

// A genuinely absent file is still a first run, and saving still works.
func TestLoadConfig_MissingFileIsAFirstRun(t *testing.T) {
	withConfigDir(t)

	if err := core.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig on a missing file: %v", err)
	}
	if core.CfgFileExists {
		t.Error("CfgFileExists = true for a missing config")
	}

	core.Cfg.SkipBackup = true
	if err := core.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig after a clean first-run load: %v", err)
	}
	if _, err := os.Stat(core.ConfigFilePath()); err != nil {
		t.Errorf("config not written: %v", err)
	}
}

// Round-trip: a readable config loads, and saving preserves its contents.
func TestLoadConfig_RoundTripPreservesSettings(t *testing.T) {
	withConfigDir(t)
	path := core.ConfigFilePath()

	if err := os.WriteFile(path, []byte(realUserConfig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := core.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !core.Cfg.SkipBackup || len(core.Cfg.ExtendedPlugins) != 2 || len(core.Cfg.ToolkitTools) != 1 {
		t.Fatalf("settings not loaded: %+v", core.Cfg)
	}

	if err := core.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if err := core.LoadConfig(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !core.Cfg.SkipBackup || len(core.Cfg.ExtendedPlugins) != 2 || len(core.Cfg.ToolkitTools) != 1 {
		t.Errorf("settings lost across a save/load round trip: %+v", core.Cfg)
	}
}

// SaveConfig must not leave scratch files behind next to the config.
func TestSaveConfig_LeavesNoTempFiles(t *testing.T) {
	dir := withConfigDir(t)

	if err := core.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := core.SaveConfig(); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", filepath.Join(dir, e.Name()))
		}
	}
}
