package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	StateVersion = 1
	ModeDaemon   = "daemon"
)

type State struct {
	Version   int       `json:"version"`
	PID       int       `json:"pid"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	BaseURL   string    `json:"base_url"`
	StartedAt time.Time `json:"started_at"`
	Mode      string    `json:"mode"`
}

func ReadState(path string) (State, error) {
	var s State
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse daemon state: %w", err)
	}
	if s.Version != StateVersion {
		return s, fmt.Errorf("unsupported daemon state version %d (expected %d)", s.Version, StateVersion)
	}
	return s, nil
}

func WriteState(path string, state State) error {
	if state.Version == 0 {
		state.Version = StateVersion
	}
	if state.Mode == "" {
		state.Mode = ModeDaemon
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create daemon state dir: %w", err)
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal daemon state: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		return fmt.Errorf("write daemon state: %w", err)
	}
	return nil
}

func RemoveState(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
