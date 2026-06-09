package codex

import "ocgo/internal/config"

type Mode string

const (
	ModeCLI     Mode = "cli"
	ModeDesktop Mode = "desktop"
)

type DesktopState struct {
	Mode       Mode   `json:"mode"`
	UpdatedAt  string `json:"updated_at"`
	BaseURL    string `json:"base_url,omitempty"`
	Model      string `json:"model,omitempty"`
	BackupFile string `json:"backup_file,omitempty"`
}

func (m Manager) DesktopConfigFile() string {
	if m.Paths.DesktopConfigFile != "" {
		return m.Paths.DesktopConfigFile
	}
	return config.CodexConfigFile()
}

func (m Manager) DesktopStateFile() string {
	return config.CodexDesktopStateFile()
}

func (m Manager) BackupDir() string {
	if m.Paths.BackupDir != "" {
		return m.Paths.BackupDir
	}
	return config.CodexBackupDir()
}
