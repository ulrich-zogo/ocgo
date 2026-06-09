package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadWriteDesktopState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-desktop-state.json")
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	st := DesktopState{
		Version:    DesktopStateVersion,
		Mode:       DesktopModeOpenCode,
		UpdatedAt:  now,
		BaseURL:    "http://127.0.0.1:3456/v1/",
		Model:      "minimax-m3",
		BackupFile: filepath.Join(dir, "config-backup.toml"),
	}
	if err := WriteDesktopState(path, st); err != nil {
		t.Fatalf("WriteDesktopState: %v", err)
	}
	got, err := ReadDesktopState(path)
	if err != nil {
		t.Fatalf("ReadDesktopState: %v", err)
	}
	if got.Version != DesktopStateVersion {
		t.Errorf("Version = %d, want %d", got.Version, DesktopStateVersion)
	}
	if got.Mode != DesktopModeOpenCode {
		t.Errorf("Mode = %q, want %q", got.Mode, DesktopModeOpenCode)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, now)
	}
	if got.BaseURL != "http://127.0.0.1:3456/v1/" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://127.0.0.1:3456/v1/")
	}
	if got.Model != "minimax-m3" {
		t.Errorf("Model = %q, want minimax-m3", got.Model)
	}
	if got.BackupFile == "" {
		t.Errorf("BackupFile = empty, want %q", st.BackupFile)
	}
}

func TestWriteDesktopStateDefaultsVersionAndUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-desktop-state.json")
	st := DesktopState{Mode: DesktopModeChatGPT}
	if err := WriteDesktopState(path, st); err != nil {
		t.Fatalf("WriteDesktopState: %v", err)
	}
	got, err := ReadDesktopState(path)
	if err != nil {
		t.Fatalf("ReadDesktopState: %v", err)
	}
	if got.Version != DesktopStateVersion {
		t.Errorf("Version = %d, want %d", got.Version, DesktopStateVersion)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be auto-stamped")
	}
}

func TestWriteDesktopStateRejectsEmptyMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-desktop-state.json")
	if err := WriteDesktopState(path, DesktopState{}); err == nil {
		t.Fatal("expected error for empty mode")
	}
}

func TestReadDesktopStateRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-desktop-state.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadDesktopState(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReadDesktopStateRejectsVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-desktop-state.json")
	body, _ := json.Marshal(DesktopState{Version: 99, Mode: DesktopModeOpenCode})
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadDesktopState(path)
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}
}

func TestReadDesktopStateMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	_, err := ReadDesktopState(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got: %v", err)
	}
}

func TestRemoveDesktopStateMissingIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	if err := RemoveDesktopState(path); err != nil {
		t.Fatalf("RemoveDesktopState on missing file should be no-op, got: %v", err)
	}
}

func TestRemoveDesktopStateRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-desktop-state.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveDesktopState(path); err != nil {
		t.Fatalf("RemoveDesktopState: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file removed, err=%v", err)
	}
}

func TestDesktopStateFileEnvOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", override)
	if got := DesktopStateFile(); got != override {
		t.Fatalf("DesktopStateFile = %q, want %q", got, override)
	}
}
