package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon-state.json")
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	st := State{
		Version:   StateVersion,
		PID:       12345,
		Host:      "127.0.0.1",
		Port:      3456,
		BaseURL:   "http://127.0.0.1:3456",
		StartedAt: now,
		Mode:      ModeDaemon,
	}
	if err := WriteState(path, st); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if got.Version != StateVersion {
		t.Errorf("Version = %d, want %d", got.Version, StateVersion)
	}
	if got.PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.PID)
	}
	if got.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", got.Host)
	}
	if got.Port != 3456 {
		t.Errorf("Port = %d, want 3456", got.Port)
	}
	if got.BaseURL != "http://127.0.0.1:3456" {
		t.Errorf("BaseURL = %q, want http://127.0.0.1:3456", got.BaseURL)
	}
	if !got.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, now)
	}
	if got.Mode != ModeDaemon {
		t.Errorf("Mode = %q, want %q", got.Mode, ModeDaemon)
	}
}

func TestWriteStateDefaultsVersionAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon-state.json")
	st := State{PID: 1, Host: "127.0.0.1", Port: 3456, BaseURL: "http://127.0.0.1:3456"}
	if err := WriteState(path, st); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if got.Version != StateVersion {
		t.Errorf("Version = %d, want %d", got.Version, StateVersion)
	}
	if got.Mode != ModeDaemon {
		t.Errorf("Mode = %q, want %q", got.Mode, ModeDaemon)
	}
}

func TestWriteStateCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "subdir", "daemon-state.json")
	st := State{PID: 1, Host: "127.0.0.1", Port: 3456, BaseURL: "http://127.0.0.1:3456"}
	if err := WriteState(path, st); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}
}

func TestReadStateMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	_, err := ReadState(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestReadStateInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon-state.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadState(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestReadStateUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon-state.json")
	body := `{"version":99,"pid":1,"host":"127.0.0.1","port":3456,"base_url":"http://127.0.0.1:3456","started_at":"2026-06-09T12:00:00Z","mode":"daemon"}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadState(path)
	if err == nil {
		t.Fatal("expected error for unsupported version, got nil")
	}
}

func TestRemoveStateMissingIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	if err := RemoveState(path); err != nil {
		t.Fatalf("RemoveState on missing file should be nil, got: %v", err)
	}
}

func TestRemoveStateRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon-state.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveState(path); err != nil {
		t.Fatalf("RemoveState: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be removed, got err=%v", err)
	}
}
