package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ocgo/internal/config"
	"ocgo/internal/process"
)

type Lock struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

func LockPath() string {
	return filepath.Join(config.ConfigDir(), "daemon.lock")
}

func AcquireLock() (func(), error) {
	path := LockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	lock := Lock{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}

	b, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, err
	}
	data := append(b, '\n')

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			existing, readErr := readLock(path)
			if readErr == nil && existing.PID > 0 {
				ps := process.StatusForPID(existing.PID)
				if ps == process.StatusPresent || ps == process.StatusUnknown {
					return nil, errors.New("another OCGO daemon operation is already running")
				}
			}
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("failed to remove stale lock: %w", err)
			}
			f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to acquire lock after stale cleanup: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to write lock: %w", err)
	}

	released := false
	return func() {
		if released {
			return
		}
		released = true
		os.Remove(path)
	}, nil
}

func readLock(path string) (Lock, error) {
	var l Lock
	b, err := os.ReadFile(path)
	if err != nil {
		return l, err
	}
	err = json.Unmarshal(b, &l)
	return l, err
}

func writeLock(path string, l Lock) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(b, '\n'), 0644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}


