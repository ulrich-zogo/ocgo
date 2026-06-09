package codex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempDesktopPaths(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	return Paths{
		ConfigFile:        filepath.Join(root, ".codex", "config.toml"),
		ProfileFile:       filepath.Join(root, ".codex", "ocgo-launch.config.toml"),
		ModelCatalogFile:  filepath.Join(root, ".codex", "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(root, ".codex", "config.toml"),
		BackupDir:         filepath.Join(root, ".config", "ocgo", "codex-backups"),
	}
}

func TestBackupDesktopConfigCopiesFile(t *testing.T) {
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	original := "model = \"gpt-5\"\nmodel_provider = \"openai\"\n"
	if err := os.WriteFile(paths.DesktopConfigFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{Paths: paths}
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	bf, err := mgr.BackupDesktopConfig(now)
	if err != nil {
		t.Fatalf("BackupDesktopConfig: %v", err)
	}
	if bf == "" {
		t.Fatal("expected non-empty backup file path")
	}
	if _, err := os.Stat(bf); err != nil {
		t.Fatalf("backup file should exist: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(bf), "config-") {
		t.Errorf("backup file name = %q, want prefix config-", filepath.Base(bf))
	}
	got, err := os.ReadFile(bf)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("backup content = %q, want %q", got, original)
	}
}

func TestBackupDesktopConfigReturnsEmptyWhenMissing(t *testing.T) {
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	bf, err := mgr.BackupDesktopConfig(time.Now())
	if err != nil {
		t.Fatalf("BackupDesktopConfig: %v", err)
	}
	if bf != "" {
		t.Errorf("expected empty backup path when source missing, got %q", bf)
	}
}

func TestBackupDesktopConfigNeverOverwritesExisting(t *testing.T) {
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.BackupDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.DesktopConfigFile, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	stamp := now.UTC().Format("20060102T150405Z")
	preExisting := filepath.Join(paths.BackupDir, "config-"+stamp+".toml")
	if err := os.WriteFile(preExisting, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{Paths: paths}
	bf, err := mgr.BackupDesktopConfig(now)
	if err != nil {
		t.Fatalf("BackupDesktopConfig: %v", err)
	}
	if bf == preExisting {
		t.Fatalf("Backup should not overwrite pre-existing file %q", preExisting)
	}
	preContent, err := os.ReadFile(preExisting)
	if err != nil {
		t.Fatal(err)
	}
	if string(preContent) != "keep me" {
		t.Errorf("pre-existing backup should be preserved, got %q", preContent)
	}
}

func TestRestoreDesktopConfigRestoresContent(t *testing.T) {
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.BackupDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "model = \"gpt-5\"\n"
	bf := filepath.Join(paths.BackupDir, "config-test.toml")
	if err := os.WriteFile(bf, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.DesktopConfigFile, []byte("OCGO config"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{Paths: paths}
	if err := mgr.RestoreDesktopConfig(bf); err != nil {
		t.Fatalf("RestoreDesktopConfig: %v", err)
	}
	got, err := os.ReadFile(paths.DesktopConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("restored content = %q, want %q", got, original)
	}
}

func TestRestoreDesktopConfigRejectsEmptyBackup(t *testing.T) {
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	if err := mgr.RestoreDesktopConfig(""); err == nil {
		t.Fatal("expected error for empty backup path")
	}
}

func TestRestoreDesktopConfigRejectsMissingBackup(t *testing.T) {
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	err := mgr.RestoreDesktopConfig(filepath.Join(paths.BackupDir, "does-not-exist.toml"))
	if err == nil {
		t.Fatal("expected error for missing backup file")
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention missing: %v", err)
	}
}

func TestRestoreDesktopConfigCreatesCodexDir(t *testing.T) {
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(paths.BackupDir, 0755); err != nil {
		t.Fatal(err)
	}
	bf := filepath.Join(paths.BackupDir, "config-test.toml")
	if err := os.WriteFile(bf, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{Paths: paths}
	if err := mgr.RestoreDesktopConfig(bf); err != nil {
		t.Fatalf("RestoreDesktopConfig: %v", err)
	}
	if _, err := os.Stat(paths.DesktopConfigFile); err != nil {
		t.Fatalf("desktop config file should exist after restore: %v", err)
	}
}
