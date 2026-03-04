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

// Cfg is the active configuration, loaded at startup.
var Cfg Config

// ExtendedMode is set by the --extended CLI flag.
var ExtendedMode bool

// ToolkitMode is set by the --toolkit CLI flag.
var ToolkitMode bool

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

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	Debug("config: saved to %s", path)
	return nil
}
