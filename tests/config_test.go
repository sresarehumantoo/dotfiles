package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOTFILES", tmp)

	// Reset cached dotfiles dir so DotfilesDir() picks up the new env
	core.ResetDotfilesDir()
	defer core.ResetDotfilesDir()

	core.LoadConfig()

	if core.CfgFileExists {
		t.Error("expected CfgFileExists to be false for missing file")
	}
	if core.Cfg.SkipBackup {
		t.Error("expected SkipBackup default to be false")
	}
	if core.Cfg.BackupDirP != "" {
		t.Error("expected BackupDirP default to be empty")
	}
}

func TestLoadConfig_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOTFILES", tmp)

	core.ResetDotfilesDir()
	defer core.ResetDotfilesDir()

	content := "skip_backup: true\nbackup_dir: /tmp/mybackups\n"
	os.WriteFile(filepath.Join(tmp, ".config.yaml"), []byte(content), 0644)

	core.LoadConfig()

	if !core.CfgFileExists {
		t.Error("expected CfgFileExists to be true")
	}
	if !core.Cfg.SkipBackup {
		t.Error("expected SkipBackup to be true")
	}
	if core.Cfg.BackupDirP != "/tmp/mybackups" {
		t.Errorf("expected BackupDirP = /tmp/mybackups, got %q", core.Cfg.BackupDirP)
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOTFILES", tmp)

	core.ResetDotfilesDir()
	defer core.ResetDotfilesDir()

	core.Cfg = core.Config{SkipBackup: true, BackupDirP: "/custom/path"}
	if err := core.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(filepath.Join(tmp, ".config.yaml"))
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if !strings.Contains(string(data), "skip_backup: true") {
		t.Error("expected skip_backup: true in saved config")
	}
	if !strings.Contains(string(data), "backup_dir: /custom/path") {
		t.Error("expected backup_dir in saved config")
	}

	// Reload and verify
	core.LoadConfig()
	if !core.CfgFileExists {
		t.Error("expected CfgFileExists to be true after save")
	}
	if !core.Cfg.SkipBackup {
		t.Error("expected SkipBackup to be true after reload")
	}
	if core.Cfg.BackupDirP != "/custom/path" {
		t.Errorf("expected BackupDirP = /custom/path, got %q", core.Cfg.BackupDirP)
	}
}

func TestBackupDir_ConfigOverride(t *testing.T) {
	core.Cfg.BackupDirP = "/custom/backup/dir"
	defer func() { core.Cfg.BackupDirP = "" }()

	got := core.BackupDir()
	if got != "/custom/backup/dir" {
		t.Errorf("BackupDir() = %q, want /custom/backup/dir", got)
	}
}

func TestBackupDir_DefaultFallback(t *testing.T) {
	core.Cfg.BackupDirP = ""

	got := core.BackupDir()
	if !strings.HasSuffix(got, filepath.Join(".local", "share", "dfinstall", "backups")) {
		t.Errorf("BackupDir() = %q, expected default path", got)
	}
}
