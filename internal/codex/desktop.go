package codex

import "ocgo/internal/config"

type Mode string

const (
	ModeCLI     Mode = "cli"
	ModeDesktop Mode = "desktop"
)

func (m Manager) DesktopConfigFile() string {
	if m.Paths.DesktopConfigFile != "" {
		return m.Paths.DesktopConfigFile
	}
	return config.CodexConfigFile()
}

func (m Manager) DesktopStateFile() string {
	return DesktopStateFile()
}

func (m Manager) BackupDir() string {
	if m.Paths.BackupDir != "" {
		return m.Paths.BackupDir
	}
	return config.CodexBackupDir()
}
