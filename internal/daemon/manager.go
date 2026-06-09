package daemon

import (
	"errors"
	"fmt"
	"os"
	"time"

	"ocgo/internal/config"
	"ocgo/internal/process"
)

type Status struct {
	Running  bool
	Healthy  bool
	PID      int
	State    State
	HasState bool
	BaseURL  string
	Source   string
}

const (
	SourceNone       = "none"
	SourceState      = "state"
	SourcePIDFile    = "pid-file"
	SourceListener   = "listener"
	SourceHealthOnly = "health-only"
)

type Manager struct {
	StateFile   string
	WaitTimeout time.Duration
}

func NewManager() Manager {
	return Manager{
		StateFile:   DaemonStateFile(),
		WaitTimeout: 10 * time.Second,
	}
}

func DaemonStateFile() string {
	if v := os.Getenv("OCGO_DAEMON_STATE_FILE"); v != "" {
		return v
	}
	return config.DaemonStateFile()
}

func (m Manager) Start(cfg config.Config) (State, bool, error) {
	base := process.BaseURL(cfg)
	if healthyFn(base) {
		pid, _ := m.discoverPID(cfg)
		if pid <= 0 {
			pid, _ = findListenerPIDFn(cfg.Port)
		}
		st := State{
			Version:   StateVersion,
			PID:       pid,
			Host:      cfg.Host,
			Port:      cfg.Port,
			BaseURL:   base,
			StartedAt: time.Now().UTC(),
			Mode:      ModeDaemon,
		}
		if err := WriteState(m.StateFile, st); err != nil {
			return st, true, err
		}
		return st, true, nil
	}

	if err := startBackgroundFn(); err != nil {
		return State{}, false, fmt.Errorf("start background server: %w", err)
	}
	if err := waitHealthyFn(base, m.WaitTimeout); err != nil {
		return State{}, false, err
	}

	pid := 0
	if p, perr := readPIDFn(); perr == nil {
		pid = p
	}
	if pid <= 0 {
		if p, perr := findListenerPIDFn(cfg.Port); perr == nil {
			pid = p
		}
	}

	st := State{
		Version:   StateVersion,
		PID:       pid,
		Host:      cfg.Host,
		Port:      cfg.Port,
		BaseURL:   base,
		StartedAt: time.Now().UTC(),
		Mode:      ModeDaemon,
	}
	if err := WriteState(m.StateFile, st); err != nil {
		return st, false, err
	}
	return st, false, nil
}

func (m Manager) Stop(cfg config.Config) error {
	base := process.BaseURL(cfg)
	pid, _, _ := m.discoverPIDWithSource(cfg)

	if !healthyFn(base) && pid <= 0 {
		_ = RemoveState(m.StateFile)
		_ = osRemoveFile(config.PIDFile())
		return ErrNotRunning
	}

	if pid > 0 {
		if err := killPIDFn(pid); err != nil {
			return fmt.Errorf("kill pid %d: %w", pid, err)
		}
	}

	_ = RemoveState(m.StateFile)
	_ = osRemoveFile(config.PIDFile())
	return nil
}

func (m Manager) Restart(cfg config.Config) (State, bool, error) {
	if err := m.Stop(cfg); err != nil && !errors.Is(err, ErrNotRunning) {
		return State{}, false, err
	}
	return m.Start(cfg)
}

func (m Manager) Status(cfg config.Config) (Status, error) {
	base := process.BaseURL(cfg)
	healthy := healthyFn(base)
	st, stateErr := ReadState(m.StateFile)
	hasState := stateErr == nil

	pid, source := m.resolvePID(cfg, st, stateErr, healthy)

	running := healthy

	st2 := Status{
		Running:  running,
		Healthy:  healthy,
		PID:      pid,
		State:    st,
		HasState: hasState,
		BaseURL:  base,
		Source:   source,
	}
	if !running {
		st2.Source = SourceNone
	}
	return st2, nil
}

func (m Manager) resolvePID(cfg config.Config, st State, stateErr error, healthy bool) (int, string) {
	if stateErr == nil && st.PID > 0 {
		return st.PID, SourceState
	}
	if pid, err := readPIDFn(); err == nil && pid > 0 {
		return pid, SourcePIDFile
	}
	if pid, err := findListenerPIDFn(cfg.Port); err == nil && pid > 0 {
		return pid, SourceListener
	}
	if healthy {
		return 0, SourceHealthOnly
	}
	return 0, SourceNone
}

func (m Manager) discoverPID(cfg config.Config) (int, error) {
	pid, _, err := m.discoverPIDWithSource(cfg)
	return pid, err
}

func (m Manager) discoverPIDWithSource(cfg config.Config) (int, string, error) {
	if st, err := ReadState(m.StateFile); err == nil && st.PID > 0 {
		return st.PID, SourceState, nil
	}
	if pid, err := readPIDFn(); err == nil && pid > 0 {
		return pid, SourcePIDFile, nil
	}
	pid, err := findListenerPIDFn(cfg.Port)
	if err == nil && pid > 0 {
		return pid, SourceListener, nil
	}
	return 0, SourceNone, err
}

var ErrNotRunning = errors.New("ocgo daemon is not running")

func osRemoveFile(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
