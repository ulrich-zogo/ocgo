package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ocgo/internal/config"
	"ocgo/internal/process"
)

type stubRuntime struct {
	healthy         atomic.Bool
	healthyCalls    atomic.Int32
	startCalls      atomic.Int32
	killCalls       atomic.Int32
	lastKilledPID   atomic.Int32
	readPIDFn       func() (int, error)
	findListenerFn  func(port int) (int, error)
	killFn          func(pid int) error
	startFn         func() error
	waitHealthyFn   func(base string, timeout time.Duration) error
}

func newStubRuntime() *stubRuntime {
	return &stubRuntime{
		readPIDFn:     func() (int, error) { return 0, errors.New("no pid file") },
		findListenerFn: func(port int) (int, error) { return 0, errors.New("no listener") },
		killFn:        func(pid int) error { return nil },
		startFn:       func() error { return nil },
		waitHealthyFn: func(base string, timeout time.Duration) error { return nil },
	}
}

func (s *stubRuntime) install(t *testing.T) {
	t.Helper()
	restore := SetRuntimeForTest(Runtime{
		Healthy: func(base string) bool { s.healthyCalls.Add(1); return s.healthy.Load() },
		ReadPID: s.readPIDFn,
		FindListenerPID: s.findListenerFn,
		KillPID: func(pid int) error { s.killCalls.Add(1); s.lastKilledPID.Store(int32(pid)); return s.killFn(pid) },
		StartBackground: func() error { s.startCalls.Add(1); return s.startFn() },
		WaitHealthy: s.waitHealthyFn,
	})
	t.Cleanup(restore)
}

func redirectHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	return dir
}

func TestManagerStartAlreadyHealthy(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.findListenerFn = func(port int) (int, error) { return 123, nil }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	st, alreadyRunning, err := mgr.Start(cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if stub.startCalls.Load() != 0 {
		t.Errorf("StartBackground should not be called when already healthy; got %d calls", stub.startCalls.Load())
	}
	if !alreadyRunning {
		t.Errorf("alreadyRunning = false, want true when /health is OK")
	}
	if st.PID != 123 {
		t.Errorf("PID = %d, want 123", st.PID)
	}
	if st.Mode != ModeDaemon {
		t.Errorf("Mode = %q, want %q", st.Mode, ModeDaemon)
	}
	if st.BaseURL != "http://127.0.0.1:3456" {
		t.Errorf("BaseURL = %q, want http://127.0.0.1:3456", st.BaseURL)
	}

	got, err := ReadState(mgr.StateFile)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if got.PID != 123 {
		t.Errorf("written state PID = %d, want 123", got.PID)
	}
}

func TestManagerStartUnhealthyThenHealthy(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.findListenerFn = func(port int) (int, error) { return 777, nil }
	stub.startFn = func() error {
		stub.healthy.Store(true)
		return nil
	}
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 2 * time.Second}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	st, alreadyRunning, err := mgr.Start(cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if stub.startCalls.Load() != 1 {
		t.Errorf("StartBackground should be called once; got %d", stub.startCalls.Load())
	}
	if alreadyRunning {
		t.Errorf("alreadyRunning = true, want false when starting fresh")
	}
	if st.PID != 777 {
		t.Errorf("PID = %d, want 777 (from listener stub)", st.PID)
	}
	if st.BaseURL != "http://127.0.0.1:3456" {
		t.Errorf("BaseURL = %q, want http://127.0.0.1:3456", st.BaseURL)
	}
}

func TestManagerStartStartBackgroundFails(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.startFn = func() error { return errors.New("boom") }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	_, alreadyRunning, err := mgr.Start(cfg)
	if err == nil {
		t.Fatal("expected error when StartBackground fails")
	}
	if alreadyRunning {
		t.Errorf("alreadyRunning = true, want false on failure")
	}
	if !strings.Contains(err.Error(), "start background server") {
		t.Errorf("error should mention start background: %v", err)
	}
}

func TestManagerStartHealthTimeout(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.waitHealthyFn = func(base string, timeout time.Duration) error { return errors.New("never healthy") }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 200 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	_, alreadyRunning, err := mgr.Start(cfg)
	if err == nil {
		t.Fatal("expected error when health never OK")
	}
	if alreadyRunning {
		t.Errorf("alreadyRunning = true, want false on health timeout")
	}
}

func TestManagerStopWithStatePID(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.install(t)
	dir := redirectHome(t)

	statePath := filepath.Join(dir, "daemon-state.json")
	pidPath := config.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("999"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := WriteState(statePath, State{Version: StateVersion, PID: 555, Host: "127.0.0.1", Port: 3456, BaseURL: "http://127.0.0.1:3456", StartedAt: now, Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{StateFile: statePath, WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	if err := mgr.Stop(cfg); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stub.killCalls.Load() != 1 {
		t.Errorf("kill calls = %d, want 1", stub.killCalls.Load())
	}
	if int(stub.lastKilledPID.Load()) != 555 {
		t.Errorf("killed PID = %d, want 555 (from state)", stub.lastKilledPID.Load())
	}
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("daemon-state.json should be removed; err=%v", err)
	}
	if _, err := os.Stat(pidPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ocgo.pid should be removed; err=%v", err)
	}
}

func TestManagerStopNotRunning(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	err := mgr.Stop(cfg)
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop should return ErrNotRunning, got: %v", err)
	}
	if stub.killCalls.Load() != 0 {
		t.Errorf("kill should not be called when not running; got %d", stub.killCalls.Load())
	}
}

func TestManagerRestart(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.findListenerFn = func(port int) (int, error) { return 42, nil }
	stub.startFn = func() error { stub.healthy.Store(true); return nil }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 2 * time.Second}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	st, alreadyRunning, err := mgr.Restart(cfg)
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if stub.startCalls.Load() != 1 {
		t.Errorf("Start should be called once during Restart; got %d", stub.startCalls.Load())
	}
	if alreadyRunning {
		t.Errorf("alreadyRunning = true, want false after Restart")
	}
	if st.PID != 42 {
		t.Errorf("PID = %d, want 42", st.PID)
	}
}

func TestManagerRestartWhenNotRunning(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.findListenerFn = func(port int) (int, error) { return 11, nil }
	stub.startFn = func() error { stub.healthy.Store(true); return nil }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 2 * time.Second}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	st, _, err := mgr.Restart(cfg)
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if stub.startCalls.Load() != 1 {
		t.Errorf("Start should still be called when not running; got %d", stub.startCalls.Load())
	}
	if st.PID != 11 {
		t.Errorf("PID = %d, want 11", st.PID)
	}
}

func TestManagerStatusHealthyWithState(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.install(t)
	dir := redirectHome(t)

	statePath := filepath.Join(dir, "daemon-state.json")
	now := time.Now().UTC()
	if err := WriteState(statePath, State{Version: StateVersion, PID: 100, Host: "127.0.0.1", Port: 3456, BaseURL: "http://127.0.0.1:3456", StartedAt: now, Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{StateFile: statePath, WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !s.Running {
		t.Errorf("Running = false, want true")
	}
	if !s.Healthy {
		t.Errorf("Healthy = false, want true")
	}
	if !s.HasState {
		t.Errorf("HasState = false, want true (state file present)")
	}
	if s.PID != 100 {
		t.Errorf("PID = %d, want 100", s.PID)
	}
	if s.Source != SourceState {
		t.Errorf("Source = %q, want %q", s.Source, SourceState)
	}
}

func TestManagerStatusHealthyFromListenerNoState(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.findListenerFn = func(port int) (int, error) { return 222, nil }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !s.Running {
		t.Errorf("Running = false, want true")
	}
	if s.HasState {
		t.Errorf("HasState = true, want false (state file missing)")
	}
	if s.PID != 222 {
		t.Errorf("PID = %d, want 222", s.PID)
	}
	if s.Source != SourceListener {
		t.Errorf("Source = %q, want %q", s.Source, SourceListener)
	}
}

func TestManagerStatusHealthyOnlyNoStateNoListener(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !s.Running {
		t.Errorf("Running = false, want true (healthy)")
	}
	if s.HasState {
		t.Errorf("HasState = true, want false (state file missing)")
	}
	if s.Source != SourceHealthOnly {
		t.Errorf("Source = %q, want %q", s.Source, SourceHealthOnly)
	}
}

func TestManagerStatusNotRunning(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if s.Running {
		t.Errorf("Running = true, want false")
	}
	if s.Source != SourceNone {
		t.Errorf("Source = %q, want %q", s.Source, SourceNone)
	}
}

func TestManagerStatusUnhealthyWithStaleState(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.install(t)
	dir := redirectHome(t)

	statePath := filepath.Join(dir, "daemon-state.json")
	if err := WriteState(statePath, State{Version: StateVersion, PID: 9999, Host: "127.0.0.1", Port: 3456, BaseURL: "http://127.0.0.1:3456", StartedAt: time.Now().UTC(), Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{StateFile: statePath, WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file missing before Status: %v", err)
	}
	if s.Running {
		t.Errorf("Running = true, want false (health is down even with stale state PID)")
	}
	if !s.HasState {
		t.Errorf("HasState = false, want true (state file IS on disk; just stale); stateErr=%v", err)
	}
	if s.Source != SourceNone {
		t.Errorf("Source = %q, want %q when not running", s.Source, SourceNone)
	}
}

func TestManagerStatusFromPIDFile(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.readPIDFn = func() (int, error) { return 333, nil }
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if s.PID != 333 {
		t.Errorf("PID = %d, want 333", s.PID)
	}
	if s.HasState {
		t.Errorf("HasState = true, want false (state file missing, only pid-file present)")
	}
	if s.Source != SourcePIDFile {
		t.Errorf("Source = %q, want %q", s.Source, SourcePIDFile)
	}
}

func TestManagerStatusFromStateTakesPriority(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(true)
	stub.readPIDFn = func() (int, error) { return 333, nil }
	stub.findListenerFn = func(port int) (int, error) { return 444, nil }
	stub.install(t)
	dir := redirectHome(t)

	statePath := filepath.Join(dir, "daemon-state.json")
	now := time.Now().UTC()
	if err := WriteState(statePath, State{Version: StateVersion, PID: 111, Host: "127.0.0.1", Port: 3456, BaseURL: "http://127.0.0.1:3456", StartedAt: now, Mode: ModeDaemon}); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{StateFile: statePath, WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "127.0.0.1", Port: 3456, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if s.PID != 111 {
		t.Errorf("PID = %d, want 111 (state priority)", s.PID)
	}
	if s.Source != SourceState {
		t.Errorf("Source = %q, want %q", s.Source, SourceState)
	}
}

func TestProcessBaseURLAppliedToManagerStatus(t *testing.T) {
	stub := newStubRuntime()
	stub.healthy.Store(false)
	stub.install(t)
	dir := redirectHome(t)

	mgr := Manager{StateFile: filepath.Join(dir, "daemon-state.json"), WaitTimeout: 100 * time.Millisecond}
	cfg := config.Config{Host: "", Port: 0, APIKey: "test-key"}

	s, err := mgr.Status(cfg)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := process.BaseURL(config.Config{Host: "127.0.0.1", Port: 3456})
	if s.BaseURL != want {
		t.Errorf("BaseURL = %q, want %q", s.BaseURL, want)
	}
}
