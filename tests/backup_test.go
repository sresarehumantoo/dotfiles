package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestFlattenPath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/home/owen/.zshrc", "home--owen--.zshrc"},
		{"/home/owen/.config/nvim/init.lua", "home--owen--.config--nvim--init.lua"},
		{"/tmp/file.txt", "tmp--file.txt"},
	}
	for _, tt := range tests {
		got := core.FlattenPath(tt.input)
		if got != tt.want {
			t.Errorf("FlattenPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsSystemPath(t *testing.T) {
	if !core.IsSystemPath("/etc/hosts") {
		t.Error("expected /etc/hosts to be a system path")
	}
	if !core.IsSystemPath("/etc/wsl.conf") {
		t.Error("expected /etc/wsl.conf to be a system path")
	}
	if core.IsSystemPath("/home/owen/.zshrc") {
		t.Error("expected /home/owen/.zshrc NOT to be a system path")
	}
	if core.IsSystemPath("/tmp/file") {
		t.Error("expected /tmp/file NOT to be a system path")
	}
}

func TestBackupFile_NoSession(t *testing.T) {
	// BackupFile should be a no-op when no session is active
	err := core.BackupFile("/tmp/nonexistent-test-path")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestBackupFile_Missing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := core.StartBackup(); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(tmp, "does-not-exist")
	if err := core.BackupFile(target); err != nil {
		t.Fatalf("BackupFile: %v", err)
	}

	if err := core.FinishBackup(); err != nil {
		t.Fatal(err)
	}

	backups, err := core.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].Count != 1 {
		t.Errorf("expected 1 entry, got %d", backups[0].Count)
	}
}

func TestBackupFile_Symlink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a symlink to back up
	linkPath := filepath.Join(tmp, "mylink")
	os.Symlink("/some/target", linkPath)

	if err := core.StartBackup(); err != nil {
		t.Fatal(err)
	}

	if err := core.BackupFile(linkPath); err != nil {
		t.Fatalf("BackupFile: %v", err)
	}

	if err := core.FinishBackup(); err != nil {
		t.Fatal(err)
	}

	backups, _ := core.ListBackups()
	if len(backups) != 1 || backups[0].Count != 1 {
		t.Fatalf("expected 1 backup with 1 entry, got %v", backups)
	}
}

func TestBackupFile_RegularFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a regular file to back up
	filePath := filepath.Join(tmp, "myfile.conf")
	os.WriteFile(filePath, []byte("original content"), 0644)

	if err := core.StartBackup(); err != nil {
		t.Fatal(err)
	}

	if err := core.BackupFile(filePath); err != nil {
		t.Fatalf("BackupFile: %v", err)
	}

	if err := core.FinishBackup(); err != nil {
		t.Fatal(err)
	}

	backups, _ := core.ListBackups()
	if len(backups) != 1 || backups[0].Count != 1 {
		t.Fatalf("expected 1 backup with 1 entry, got %v", backups)
	}

	// Verify the backup file was actually copied
	backupDir := filepath.Join(core.BackupDir(), backups[0].Timestamp, "files")
	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in backup, got %d", len(entries))
	}
}

func TestBackupFile_Dedup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	filePath := filepath.Join(tmp, "dupfile")
	os.WriteFile(filePath, []byte("data"), 0644)

	if err := core.StartBackup(); err != nil {
		t.Fatal(err)
	}

	// Back up same path twice
	core.BackupFile(filePath)
	core.BackupFile(filePath)

	if err := core.FinishBackup(); err != nil {
		t.Fatal(err)
	}

	backups, _ := core.ListBackups()
	if backups[0].Count != 1 {
		t.Errorf("expected 1 entry after dedup, got %d", backups[0].Count)
	}
}

func TestFinishBackup_EmptyCleanup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := core.StartBackup(); err != nil {
		t.Fatal(err)
	}

	// Don't record anything — FinishBackup should clean up
	if err := core.FinishBackup(); err != nil {
		t.Fatal(err)
	}

	backups, _ := core.ListBackups()
	if len(backups) != 0 {
		t.Errorf("expected 0 backups after empty cleanup, got %d", len(backups))
	}
}

func TestRestoreRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Enable verbose so Ok/Info calls don't suppress
	core.Level = core.LogVerbose
	defer func() { core.Level = 0 }()

	// 1. Create original files
	origFile := filepath.Join(tmp, "myconfig")
	os.WriteFile(origFile, []byte("original"), 0644)

	origLink := filepath.Join(tmp, "mylink")
	os.Symlink("/original/target", origLink)

	missingPath := filepath.Join(tmp, "willbecreated")

	// 2. Start backup and record states
	if err := core.StartBackup(); err != nil {
		t.Fatal(err)
	}

	core.BackupFile(origFile)
	core.BackupFile(origLink)
	core.BackupFile(missingPath)

	if err := core.FinishBackup(); err != nil {
		t.Fatal(err)
	}

	// 3. Simulate dfinstall modifying things
	os.Remove(origFile)
	os.Symlink("/dotfiles/config/myconfig", origFile)

	os.Remove(origLink)
	os.Symlink("/dotfiles/config/mylink", origLink)

	os.Symlink("/dotfiles/config/newfile", missingPath)

	// 4. Restore
	backups, _ := core.ListBackups()
	if len(backups) == 0 {
		t.Fatal("no backups found")
	}

	if err := core.RestoreBackup(backups[0].Timestamp); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}

	// 5. Verify origFile is restored as regular file
	fi, err := os.Lstat(origFile)
	if err != nil {
		t.Fatalf("origFile missing after restore: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("origFile should be a regular file, got symlink")
	}
	data, _ := os.ReadFile(origFile)
	if string(data) != "original" {
		t.Errorf("origFile content = %q, want %q", data, "original")
	}

	// 6. Verify origLink is restored as symlink to original target
	target, err := os.Readlink(origLink)
	if err != nil {
		t.Fatalf("origLink not a symlink after restore: %v", err)
	}
	if target != "/original/target" {
		t.Errorf("origLink target = %q, want %q", target, "/original/target")
	}

	// 7. Verify missingPath was removed
	if _, err := os.Lstat(missingPath); !os.IsNotExist(err) {
		t.Errorf("missingPath should not exist after restore, got err=%v", err)
	}
}
