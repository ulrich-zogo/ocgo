package codex

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m Manager) BackupDesktopConfig(now time.Time) (string, error) {
	src := m.DesktopConfigFile()
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat codex desktop config: %w", err)
	}
	dir := m.BackupDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create codex backup dir: %w", err)
	}
	stamp := now.UTC().Format("20060102T150405Z")
	base := fmt.Sprintf("config-%s.toml", stamp)
	target := filepath.Join(dir, base)
	if _, err := os.Stat(target); err == nil {
		target = filepath.Join(dir, fmt.Sprintf("config-%s-%09d.toml", stamp, now.UTC().Nanosecond()))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat existing backup: %w", err)
	}
	if err := copyFileAtomic(src, target); err != nil {
		return "", fmt.Errorf("copy codex desktop config to backup: %w", err)
	}
	return target, nil
}

func (m Manager) RestoreDesktopConfig(backupFile string) error {
	if strings.TrimSpace(backupFile) == "" {
		return errors.New("no backup file provided; cannot restore Codex Desktop config")
	}
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file %s is missing: %w", backupFile, err)
	}
	dst := m.DesktopConfigFile()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create codex config dir: %w", err)
	}
	if err := copyFileAtomic(backupFile, dst); err != nil {
		return fmt.Errorf("restore codex desktop config: %w", err)
	}
	return nil
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		cleanup()
		return err
	}
	return nil
}
