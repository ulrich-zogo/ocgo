package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ocgo/internal/daemon"
)

func setupDaemonTestHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	cfgPath := filepath.Join(dir, ".config", "ocgo", "config.json")
	if err := writeFile(cfgPath, `{"api_key":"test-key","host":"127.0.0.1","port":3456}`); err != nil {
		t.Fatal(err)
	}
}

type daemonStub struct {
	healthy      atomic.Bool
	startCalls   atomic.Int32
	stopCalls    atomic.Int32
	restartCalls atomic.Int32
}

func (s *daemonStub) install(t *testing.T) {
	t.Helper()
	restore := daemon.SetRuntimeForTest(daemon.Runtime{
		Healthy: func(base string) bool { return s.healthy.Load() },
		ReadPID: func() (int, error) { return 0, nil },
		FindListenerPID: func(port int) (int, error) { return 0, nil },
		KillPID: func(pid int) error { return nil },
		StartBackground: func() error { s.startCalls.Add(1); s.healthy.Store(true); return nil },
		WaitHealthy: func(base string, timeout time.Duration) error { return nil },
	})
	t.Cleanup(restore)
}

func TestDaemonStatusNotRunning(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon is not running") {
		t.Fatalf("output missing 'OCGO daemon is not running':\n%s", got)
	}
	if !strings.Contains(got, "Base URL: http://127.0.0.1:3456") {
		t.Fatalf("output missing base URL:\n%s", got)
	}
}

func TestDaemonStatusJSONIsStrict(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status", "--json"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !json.Valid([]byte(output)) {
		t.Fatalf("daemon status --json is not valid JSON:\n%s", output)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}
	for _, key := range []string{"state_file", "pid_file", "pid", "process", "health", "base_url", "log_file", "started_at"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing key %q in JSON output", key)
		}
	}
}

func TestDaemonStatusJSONDaemonMissingExitZero(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status", "--json"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("should exit 0 when daemon absent, got: %v, output: %s", err, out.String())
	}
}

func TestDaemonStatusJSONNoSideEffects(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	cfgPath := filepath.Join(dir, ".config", "ocgo", "config.json")
	if err := writeFile(cfgPath, `{"api_key":"test-key","host":"127.0.0.1","port":3456}`); err != nil {
		t.Fatal(err)
	}

	var before []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			before = append(before, rel)
		}
		return nil
	})

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status", "--json"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var after []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			after = append(after, rel)
		}
		return nil
	})
	if len(before) != len(after) {
		t.Fatalf("daemon status --json created files: before=%v after=%v", before, after)
	}
}

func TestDaemonStartWhenStopped(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "start"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon started") {
		t.Fatalf("output missing 'OCGO daemon started':\n%s", got)
	}
	if !strings.Contains(got, "Base URL: http://127.0.0.1:3456") {
		t.Fatalf("output missing base URL:\n%s", got)
	}
	if !strings.Contains(got, "Log:") {
		t.Fatalf("output missing Log line:\n%s", got)
	}
	if stub.startCalls.Load() != 1 {
		t.Errorf("Start should call background exactly once; got %d", stub.startCalls.Load())
	}
}

func TestDaemonStartWhenAlreadyHealthy(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.healthy.Store(true)
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "start"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon already running") {
		t.Fatalf("output missing 'OCGO daemon already running':\n%s", got)
	}
	if strings.Contains(got, "OCGO daemon started\n") || strings.HasPrefix(got, "OCGO daemon started") {
		t.Fatalf("output should not say 'OCGO daemon started' when already running:\n%s", got)
	}
	if stub.startCalls.Load() != 0 {
		t.Errorf("Start should not spawn a new background when already healthy; got %d calls", stub.startCalls.Load())
	}
}

func TestDaemonStatusHealthyWithStateMissing(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.healthy.Store(true)
	stub.install(t)

	dir := t.TempDir()
	t.Setenv("OCGO_DAEMON_STATE_FILE", filepath.Join(dir, "missing-daemon-state.json"))

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "health:         ok") {
		t.Fatalf("output missing health status:\n%s", got)
	}
	if !strings.Contains(got, "state file:     missing") {
		t.Fatalf("output missing state file status:\n%s", got)
	}
}

func TestDaemonStatusUnhealthyWithStaleState(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.healthy.Store(false)
	stub.install(t)

	dir := t.TempDir()
	statePath := filepath.Join(dir, "daemon-state.json")
	if err := daemon.WriteState(statePath, daemon.State{
		Version:   daemon.StateVersion,
		PID:       9999,
		Host:      "127.0.0.1",
		Port:      3456,
		BaseURL:   "http://127.0.0.1:3456",
		StartedAt: time.Now().UTC(),
		Mode:      daemon.ModeDaemon,
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCGO_DAEMON_STATE_FILE", statePath)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon is not running") {
		t.Fatalf("output should report 'not running' when /health fails even with stale state:\n%s", got)
	}
	if strings.Contains(got, "OCGO daemon is running\n") || strings.HasPrefix(got, "OCGO daemon is running") {
		t.Fatalf("output should not say 'OCGO daemon is running' when /health fails:\n%s", got)
	}
}

func TestDaemonStatusRunningFromState(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.healthy.Store(true)
	stub.install(t)

	dir := t.TempDir()
	statePath := filepath.Join(dir, "daemon-state.json")
	if err := daemon.WriteState(statePath, daemon.State{
		Version:   daemon.StateVersion,
		PID:       1234,
		Host:      "127.0.0.1",
		Port:      3456,
		BaseURL:   "http://127.0.0.1:3456",
		StartedAt: time.Now().UTC(),
		Mode:      daemon.ModeDaemon,
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCGO_DAEMON_STATE_FILE", statePath)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "status"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "pid:            1234") {
		t.Fatalf("output missing 'pid:            1234':\n%s", got)
	}
	if !strings.Contains(got, "state file:     present") {
		t.Fatalf("output missing state file status:\n%s", got)
	}
}

func TestDaemonStopRemovesState(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.healthy.Store(true)
	stub.install(t)

	dir := t.TempDir()
	statePath := filepath.Join(dir, "daemon-state.json")
	if err := daemon.WriteState(statePath, daemon.State{
		Version:   daemon.StateVersion,
		PID:       1234,
		Host:      "127.0.0.1",
		Port:      3456,
		BaseURL:   "http://127.0.0.1:3456",
		StartedAt: time.Now().UTC(),
		Mode:      daemon.ModeDaemon,
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCGO_DAEMON_STATE_FILE", statePath)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "stop"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon stopped") {
		t.Fatalf("output missing 'OCGO daemon stopped':\n%s", got)
	}
	if _, err := readStateForTest(statePath); err == nil {
		t.Errorf("daemon-state.json should be removed")
	}
}

func TestDaemonStopWhenNotRunning(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "stop"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon is not running") {
		t.Fatalf("output missing 'OCGO daemon is not running':\n%s", got)
	}
}

func TestDaemonRestart(t *testing.T) {
	setupDaemonTestHome(t)
	stub := &daemonStub{}
	stub.install(t)

	root := NewRootCommand("test")
	root.SetArgs([]string{"daemon", "restart"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "OCGO daemon restarted") {
		t.Fatalf("output missing 'OCGO daemon restarted':\n%s", got)
	}
	if stub.startCalls.Load() != 1 {
		t.Errorf("Restart should call start once; got %d", stub.startCalls.Load())
	}
}

func TestRootRegistersDaemon(t *testing.T) {
	root := NewRootCommand("test")
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "daemon" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("root command should register 'daemon'")
	}
}

func TestRootPreservesLegacyCommands(t *testing.T) {
	root := NewRootCommand("test")
	want := map[string]bool{
		"serve": false, "status": false, "stop": false, "launch": false, "opencode": false, "daemon": false,
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("root command missing %q", name)
		}
	}
}

func readStateForTest(path string) (daemon.State, error) {
	return daemon.ReadState(path)
}
