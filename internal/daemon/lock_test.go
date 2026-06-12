package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"ocgo/internal/config"
	"ocgo/internal/process"
)

func TestAcquireLockIsExclusive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)

	release1, err := AcquireLock()
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer release1()

	_, err = AcquireLock()
	if err == nil {
		t.Fatal("second AcquireLock should fail when first lock is held")
	}
}

func TestAcquireLockReleasesExclusively(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)

	release1, err := AcquireLock()
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	release1()

	release2, err := AcquireLock()
	if err != nil {
		t.Fatalf("second AcquireLock after release: %v", err)
	}
	release2()
}

func TestCleanStalePIDDoesNotCleanUnknown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)

	pidPath := config.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("42"), 0644); err != nil {
		t.Fatal(err)
	}

	statePath := DaemonStateFile()
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(statePath, State{Version: StateVersion, PID: 42, Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	restore := SetRuntimeForTest(Runtime{
		ReadPID:     func() (int, error) { return 42, nil },
		StatusForPID: func(pid int) process.ProcessStatus { return process.StatusUnknown },
	})
	defer restore()

	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test"}
	cleanStalePID(cfg)

	if _, err := os.Stat(pidPath); err != nil {
		t.Error("PID file was removed despite StatusUnknown")
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Error("state file was removed despite StatusUnknown")
	}
}

func TestCleanStalePIDCleansStale(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)

	pidPath := config.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("42"), 0644); err != nil {
		t.Fatal(err)
	}

	statePath := DaemonStateFile()
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(statePath, State{Version: StateVersion, PID: 42, Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	restore := SetRuntimeForTest(Runtime{
		ReadPID:     func() (int, error) { return 42, nil },
		StatusForPID: func(pid int) process.ProcessStatus { return process.StatusStale },
	})
	defer restore()

	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test"}
	cleanStalePID(cfg)

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should be removed for StatusStale")
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("state file should be removed for StatusStale")
	}
}

func TestStopDoesNotCleanUnknownPID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)

	statePath := filepath.Join(dir, "daemon-state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(statePath, State{Version: StateVersion, PID: 42, Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	pidPath := config.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("42"), 0644); err != nil {
		t.Fatal(err)
	}

	restore := SetRuntimeForTest(Runtime{
		Healthy:    func(base string) bool { return false },
		ReadPID:    func() (int, error) { return 42, nil },
		StatusForPID: func(pid int) process.ProcessStatus { return process.StatusUnknown },
	})
	defer restore()

	mgr := Manager{StateFile: statePath}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test"}

	err := mgr.Stop(cfg)
	if err == nil {
		t.Fatal("expected error when StatusUnknown, got nil")
	}

	if _, statErr := os.Stat(statePath); statErr != nil {
		t.Error("state file was deleted despite StatusUnknown")
	}
	if _, statErr := os.Stat(pidPath); statErr != nil {
		t.Error("PID file was deleted despite StatusUnknown")
	}
}
