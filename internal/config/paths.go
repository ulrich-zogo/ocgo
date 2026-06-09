package config

import (
	"os"
	"path/filepath"
)

const CodexProfileName = "ocgo-launch"

func CodexConfigFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func CodexProfileConfigFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", CodexProfileName+".config.toml")
}

func CodexModelCatalogFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "ocgo-models.json")
}

func ModelCatalogCacheFile() string {
	return filepath.Join(ConfigDir(), "model-catalog-cache.json")
}

func CodexDesktopStateFile() string {
	return filepath.Join(ConfigDir(), "codex-desktop-state.json")
}

func CodexBackupDir() string {
	return filepath.Join(ConfigDir(), "codex-backups")
}

func ModelSelectionFile() string {
	return filepath.Join(ConfigDir(), "model-selection.json")
}

func DaemonStateFile() string {
	return filepath.Join(ConfigDir(), "daemon-state.json")
}
