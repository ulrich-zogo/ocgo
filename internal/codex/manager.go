package codex

import (
	"os"
	"path/filepath"
	"strings"

	"ocgo/internal/config"
)

type Paths struct {
	ConfigFile       string
	ProfileFile      string
	ModelCatalogFile string

	DesktopConfigFile string
	BackupDir         string
}

type Manager struct {
	Paths Paths
}

func NewManager() Manager {
	return Manager{
		Paths: Paths{
			ConfigFile:       config.CodexConfigFile(),
			ProfileFile:      config.CodexProfileConfigFile(),
			ModelCatalogFile: config.CodexModelCatalogFile(),
			DesktopConfigFile: config.CodexConfigFile(),
			BackupDir:         config.CodexBackupDir(),
		},
	}
}

func (m Manager) EnsureCLIConfig(base string) error {
	if err := os.MkdirAll(filepath.Dir(m.Paths.ConfigFile), 0755); err != nil {
		return err
	}
	if err := m.WriteModelCatalog(); err != nil {
		return err
	}
	return m.WriteCLIProfile(strings.TrimRight(base, "/") + "/v1/")
}
