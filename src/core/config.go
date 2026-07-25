package core

import (
	"errors"
	"fmt"
	"io/fs"
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

// cfgUnreadable is set when the config file is present but could not be read or
// parsed. SaveConfig refuses to write while it is set: overwriting a file we
// failed to understand would silently discard the user's settings.
var cfgUnreadable bool

// LoadConfig reads the config file into Cfg.
//
// Only a genuinely missing file counts as a first run. Any other failure —
// permissions, EISDIR, an I/O error, malformed YAML — leaves Cfg at defaults
// but marks the config unreadable, because the alternative is data loss: a
// missing file makes shouldBackup report a first run, and InstallSession then
// writes defaults straight over the config it couldn't read.
func LoadConfig() error {
	path := ConfigFilePath()

	Cfg = Config{SkipBackup: false}
	CfgFileExists = false
	cfgUnreadable = false

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // genuine first run
	}
	if err != nil {
		CfgFileExists = true
		cfgUnreadable = true
		return fmt.Errorf("read config %s: %w", path, err)
	}

	CfgFileExists = true
	if err := yaml.Unmarshal(data, &Cfg); err != nil {
		Cfg = Config{SkipBackup: false}
		cfgUnreadable = true
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}

// SaveConfig writes the current Cfg to disk with a comment header.
//
// It refuses to run when LoadConfig couldn't read the existing file: Cfg is
// only defaults in that case, so writing would replace the user's real
// settings with an empty config.
func SaveConfig() error {
	path := ConfigFilePath()

	if cfgUnreadable {
		return fmt.Errorf("refusing to overwrite %s: it exists but could not be read; fix or remove it first", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(&Cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	header := "# dfinstall configuration\n# Auto-generated after first install run.\n\n"
	content := header + string(data)

	// Atomic write: unique temp file in the target dir + rename. A fixed
	// "<path>.tmp" name meant two concurrent dfinstall runs wrote the same
	// scratch file and one renamed the other's half-written content into place.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.yaml.tmp*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write config: %w", err)
	}
	// Flush to disk before the rename, so a crash can't leave the new name
	// pointing at empty content — which is what "atomic" is supposed to buy.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename config: %w", err)
	}

	Debug("config: saved to %s", path)
	return nil
}
