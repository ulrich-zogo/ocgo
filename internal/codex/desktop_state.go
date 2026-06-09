package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ocgo/internal/config"
)

const DesktopStateVersion = 1

type DesktopMode string

const (
	DesktopModeOpenCode DesktopMode = "opencode"
	DesktopModeChatGPT  DesktopMode = "chatgpt"
)

type DesktopState struct {
	Version    int         `json:"version"`
	Mode       DesktopMode `json:"mode"`
	UpdatedAt  time.Time   `json:"updated_at"`
	BaseURL    string      `json:"base_url,omitempty"`
	Model      string      `json:"model,omitempty"`
	BackupFile string      `json:"backup_file,omitempty"`
}

func DesktopStateFile() string {
	if v := os.Getenv("OCGO_CODEX_DESKTOP_STATE_FILE"); v != "" {
		return v
	}
	return config.CodexDesktopStateFile()
}

func ReadDesktopState(path string) (DesktopState, error) {
	var s DesktopState
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse codex desktop state: %w", err)
	}
	if s.Version != DesktopStateVersion {
		return s, fmt.Errorf("unsupported codex desktop state version %d (expected %d)", s.Version, DesktopStateVersion)
	}
	return s, nil
}

func WriteDesktopState(path string, state DesktopState) error {
	if state.Version == 0 {
		state.Version = DesktopStateVersion
	}
	if state.Mode == "" {
		return errors.New("codex desktop state: mode is required")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create codex desktop state dir: %w", err)
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codex desktop state: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		return fmt.Errorf("write codex desktop state: %w", err)
	}
	return nil
}

func RemoveDesktopState(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
