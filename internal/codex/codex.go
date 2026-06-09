package codex

import (
	"path/filepath"

	"ocgo/internal/config"
)

func EnsureConfig(base string) error {
	return NewManager().EnsureCLIConfig(base)
}

func WriteProfile(path, baseURL string) error {
	mgr := Manager{
		Paths: Paths{
			ConfigFile:        path,
			ProfileFile:       filepath.Join(filepath.Dir(path), config.CodexProfileName+".config.toml"),
			ModelCatalogFile:  config.CodexModelCatalogFile(),
			DesktopConfigFile: path,
			BackupDir:         config.CodexBackupDir(),
		},
	}
	return mgr.WriteCLIProfile(baseURL)
}

func WriteModelCatalog(path string) error {
	mgr := NewManager()
	mgr.Paths.ModelCatalogFile = path
	return mgr.WriteModelCatalog()
}

func CheckVersion() error {
	return NewManager().CheckVersion()
}
