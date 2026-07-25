package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds user-tunable dfinstall settings.
type Config struct {
	SkipBackup         bool     `yaml:"skip_backup"`
	BackupDirP         string   `yaml:"backup_dir,omitempty"`
	ExtendedPlugins    []string `yaml:"extended_plugins,omitempty"`
	PreservedFiles     []string `yaml:"preserved_files,omitempty"`
	DismissedFiles     []string `yaml:"dismissed_files,omitempty"`
	SkipModules        []string `yaml:"skip_modules,omitempty"`
	ToolkitTools       []string `yaml:"toolkit_tools,omitempty"`
	ToolkitRegistryURL string   `yaml:"toolkit_registry_url,omitempty"`
	WindevEnabled      bool     `yaml:"windev_enabled,omitempty"`
}

// IsModuleSkipped returns true if the named module is in the SkipModules list.
func IsModuleSkipped(name string) bool {
	for _, s := range Cfg.SkipModules {
		if s == name {
			return true
		}
	}
	return false
}

// SkipInAll reports whether a module should be omitted from `install all`.
// Combines user SkipModules with opt-in modules that haven't been enabled yet
// (currently just windev — explicit `install windev` flips Cfg.WindevEnabled
// on, after which it's included like any other module).
//
// Every caller that iterates AllModules for an "install all" must use this, not
// IsModuleSkipped alone; the MCP server previously used the latter and so
// installed windev on machines that had opted out.
func SkipInAll(name string) bool {
	if IsModuleSkipped(name) {
		return true
	}
	if name == "windev" && !Cfg.WindevEnabled {
		return true
	}
	return false
}

// SetWindevOptIn records an explicit `install windev` so later `install all`
// runs keep it current. The flag is persisted by InstallSession.Finish via
// WindevMode. No-op under --dry-run so a preview never mutates config state.
func SetWindevOptIn() {
	if DryRun {
		return
	}
	WindevMode = true
	Cfg.WindevEnabled = true
}

// ClearWindevOptIn drops the opt-in on an explicit `uninstall windev` so
// `install all` stops re-applying it. Best-effort: warns but doesn't fail.
func ClearWindevOptIn() {
	if DryRun || !Cfg.WindevEnabled {
		return
	}
	Cfg.WindevEnabled = false
	if err := SaveConfig(); err != nil {
		Warn("failed to save config: %v", err)
	}
}

// Cfg is the active configuration, loaded at startup.
var Cfg Config

// ExtendedMode is set by the --extended CLI flag.
var ExtendedMode bool

// ToolkitMode is set by the --toolkit CLI flag.
var ToolkitMode bool

// WindevMode is set when the windev module is the explicit install target.
// Like ExtendedMode/ToolkitMode, it gates persisting the windev opt-in to config.
var WindevMode bool

// CfgFileExists is true when the config file was present at load time.
// Used to distinguish "first run" from "user explicitly set skip_backup: false".
var CfgFileExists bool

// ConfigFilePath returns the path to the dfinstall config file.
func ConfigFilePath() string {
	return filepath.Join(DotfilesDir(), ".config.yaml")
}

// LoadConfig reads the config file into Cfg.
// If the file does not exist, Cfg gets sensible defaults and CfgFileExists is false.
func LoadConfig() {
	path := ConfigFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		// File missing or unreadable — first run defaults
		CfgFileExists = false
		Cfg = Config{SkipBackup: false}
		return
	}

	CfgFileExists = true
	Cfg = Config{SkipBackup: false}
	if err := yaml.Unmarshal(data, &Cfg); err != nil {
		Warn("config: failed to parse %s: %v (using defaults)", path, err)
		Cfg = Config{SkipBackup: false}
	}
}

// SaveConfig writes the current Cfg to disk with a comment header.
func SaveConfig() error {
	path := ConfigFilePath()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(&Cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	header := "# dfinstall configuration\n# Auto-generated after first install run.\n\n"
	content := header + string(data)

	// Atomic write: temp file + rename to avoid corruption on crash.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename config: %w", err)
	}

	Debug("config: saved to %s", path)
	return nil
}
