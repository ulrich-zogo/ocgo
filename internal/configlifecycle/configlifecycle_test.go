package configlifecycle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBackupIncludesManifest(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test"}`), 0644)

	bp := filepath.Join(t.TempDir(), "backup.zip")
	result, err := Backup(bp, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != bp {
		t.Errorf("Backup path = %q, want %q", result.Path, bp)
	}

	zr, err := zip.OpenReader(bp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	hasManifest := false
	for _, f := range zr.File {
		if f.Name == "backup-manifest.json" {
			hasManifest = true
			rc, _ := f.Open()
			var buf bytes.Buffer
			buf.ReadFrom(rc)
			rc.Close()
			var m BackupManifest
			if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
				t.Fatalf("invalid manifest JSON: %v", err)
			}
			if m.CreatedAt == "" {
				t.Error("manifest missing created_at")
			}
			if m.OCGOVersion == "" {
				t.Error("manifest missing ocgo_version")
			}
			break
		}
	}
	if !hasManifest {
		t.Error("backup missing backup-manifest.json")
	}
}

func TestBackupSkipsCodexConfigByDefault(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test"}`), 0644)

	codexDir := filepath.Join(tmpHome, ".codex")
	os.MkdirAll(codexDir, 0755)
	os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(`[profiles]`), 0644)

	bp := filepath.Join(t.TempDir(), "backup.zip")
	_, err := Backup(bp, false)
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(bp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == ".codex/config.toml" {
			t.Error("backup included .codex/config.toml without --include-codex-config")
		}
	}
}

func TestBackupIncludesCodexConfigWhenRequested(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test"}`), 0644)

	codexDir := filepath.Join(tmpHome, ".codex")
	os.MkdirAll(codexDir, 0755)
	os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(`[profiles]`), 0644)

	bp := filepath.Join(t.TempDir(), "backup.zip")
	_, err := Backup(bp, true)
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(bp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	found := false
	for _, f := range zr.File {
		if f.Name == ".codex/config.toml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("backup missing .codex/config.toml with --include-codex-config")
	}
}

func TestRestoreRejectsZipSlipPaths(t *testing.T) {
	tmpHome := t.TempDir()
	homeDir, _ := os.UserHomeDir()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	fh, _ := zw.Create("backup-manifest.json")
	fh.Write([]byte(`{"created_at":"","ocgo_version":"test","files":[],"include_codex_config":false}`))
	fh, _ = zw.Create("../../.ssh/id_rsa")
	fh.Write([]byte("evil"))
	zw.Close()

	bp := filepath.Join(t.TempDir(), "evil.zip")
	os.WriteFile(bp, buf.Bytes(), 0644)

	_, err := Restore(bp, RestoreOptions{DryRun: true, Yes: true})
	if err == nil {
		t.Error("expected error for zip-slip path, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "parent directory reference") {
		t.Errorf("expected parent directory reference error, got: %v", err)
	}

	// Restore HOME to not pollute the real one
	_ = homeDir
	_ = tmpHome
}

func TestRestoreRejectsAbsolutePaths(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	fh, _ := zw.Create("backup-manifest.json")
	fh.Write([]byte(`{"created_at":"","ocgo_version":"test","files":[],"include_codex_config":false}`))
	fh, _ = zw.Create("/etc/passwd")
	fh.Write([]byte("evil"))
	zw.Close()

	bp := filepath.Join(t.TempDir(), "abs.zip")
	os.WriteFile(bp, buf.Bytes(), 0644)

	_, err := Restore(bp, RestoreOptions{DryRun: true, Yes: true})
	if err == nil {
		t.Error("expected error for absolute path, got nil")
	}
}

func TestResetOcgoScopeDeletesOnlyManagedFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)

	managedFile := filepath.Join(ocgoDir, "config.json")
	os.WriteFile(managedFile, []byte(`{"api_key":"test"}`), 0644)

	unmanagedFile := filepath.Join(ocgoDir, "user-data.txt")
	os.WriteFile(unmanagedFile, []byte("hello"), 0644)

	result, err := Reset(ResetOptions{
		Scope: ResetScopeOcgo,
		Yes:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(managedFile); err == nil {
		t.Error("managed file was not deleted")
	}
	if _, err := os.Stat(unmanagedFile); err != nil {
		t.Error("unmanaged file was incorrectly deleted")
	}

	foundManaged := false
	for _, f := range result.Removed {
		if f == managedFile {
			foundManaged = true
		}
		if f == unmanagedFile {
			t.Error("unmanaged file in removed list")
		}
	}
	if !foundManaged {
		t.Error("managed file not in removed list")
	}
}

func TestResetCodexCLIScopeDoesNotDeleteCodexConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	codexDir := filepath.Join(tmpHome, ".codex")
	os.MkdirAll(codexDir, 0755)

	codexConfig := filepath.Join(codexDir, "config.toml")
	os.WriteFile(codexConfig, []byte(`[profiles]`), 0644)

	ocgoProfile := filepath.Join(codexDir, "ocgo-launch.config.toml")
	os.WriteFile(ocgoProfile, []byte(`[ocgo]`), 0644)

	_, err := Reset(ResetOptions{
		Scope: ResetScopeCodexCLI,
		Yes:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(codexConfig); err != nil {
		t.Error("codex config.toml was incorrectly deleted")
	}
	if _, err := os.Stat(ocgoProfile); err == nil {
		t.Error("ocgo profile was not deleted")
	}
}

func TestResetNoBackupRequiresYes(t *testing.T) {
	_, err := Reset(ResetOptions{
		Scope:   ResetScopeOcgo,
		NoBackup: true,
		Yes:     false,
	})
	if err == nil {
		t.Error("expected error for --no-backup without --yes")
	}
	if err != nil && !strings.Contains(err.Error(), "--no-backup requires --yes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResetAllScopePreservesBackupsByDefault(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test"}`), 0644)

	backupsDir := filepath.Join(ocgoDir, "backups")
	os.MkdirAll(backupsDir, 0755)
	backupFile := filepath.Join(backupsDir, "my-backup.zip")
	os.WriteFile(backupFile, []byte("data"), 0644)

	_, err := Reset(ResetOptions{
		Scope: ResetScopeAll,
		Yes:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(backupFile); err != nil {
		t.Error("backup file was deleted without --include-backups")
	}
}

func TestIsWithin(t *testing.T) {
	if !isWithin("/home/user/.config/ocgo", "/home/user/.config/ocgo/config.json") {
		t.Error("isWithin should accept files inside base")
	}
	if !isWithin("/home/user/.config/ocgo", "/home/user/.config/ocgo") {
		t.Error("isWithin should accept base itself")
	}
	if isWithin("/home/user/.config/ocgo", "/home/user/.config/ocgo-malicious/file") {
		t.Error("isWithin should reject files with similar prefix")
	}
	if isWithin("/home/user/.config/ocgo", "/home/user/.config/ocgo2/file") {
		t.Error("isWithin should reject files with similar prefix")
	}
	if isWithin("/home/user/.config/ocgo", "/etc/passwd") {
		t.Error("isWithin should reject files outside base")
	}
}

func TestProcessStatusForInvalidPID(t *testing.T) {
	status := processStatus(-1)
	if status != StatusStale {
		t.Errorf("processStatus(-1) = %q, want %q", status, StatusStale)
	}
}

func TestProcessStatusForSelfPID(t *testing.T) {
	pid := os.Getpid()
	status := processStatus(pid)
	if runtime.GOOS == "windows" {
		if status != StatusUnknown {
			t.Errorf("processStatus(self) on windows = %q, want %q", status, StatusUnknown)
		}
	} else {
		if status != StatusPresent {
			t.Errorf("processStatus(self) = %q, want %q", status, StatusPresent)
		}
	}
}

func TestProcessStatusForNonExistentPID(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessExitImmediately")
	cmd.Env = append(os.Environ(), "OCGO_TEST_HELPER_PROCESS=1")

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}

	status := processStatus(pid)
	if runtime.GOOS == "windows" {
		if status != StatusUnknown {
			t.Errorf("processStatus(terminated child) on windows = %q, want %q", status, StatusUnknown)
		}
	} else {
		if status != StatusStale {
			t.Errorf("processStatus(terminated child) = %q, want %q", status, StatusStale)
		}
	}
}

func TestProcessStatusHasNoSideEffects(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessSleep")
	cmd.Env = append(os.Environ(), "OCGO_TEST_HELPER_SLEEP=1")

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid

	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	status := processStatus(pid)

	if runtime.GOOS == "windows" {
		if status != StatusUnknown {
			t.Fatalf("processStatus(live child) on windows = %q, want %q", status, StatusUnknown)
		}
	} else {
		if status != StatusPresent {
			t.Fatalf("processStatus(live child) = %q, want %q", status, StatusPresent)
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		t.Fatalf("child process exited after processStatus: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHelperProcessExitImmediately(t *testing.T) {
	if os.Getenv("OCGO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestHelperProcessSleep(t *testing.T) {
	if os.Getenv("OCGO_TEST_HELPER_SLEEP") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

