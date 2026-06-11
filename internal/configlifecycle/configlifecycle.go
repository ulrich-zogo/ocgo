package configlifecycle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ocgo/internal/buildinfo"
	"ocgo/internal/config"
	"ocgo/internal/process"
)

type Paths struct {
	ConfigDir          string `json:"ocgo_config_dir"`
	ConfigFile         string `json:"ocgo_config_file"`
	ModelMappingFile   string `json:"model_mapping_file"`
	ModelSelectionFile string `json:"model_selection_file"`
	ModelCacheFile     string `json:"model_cache_file"`
	DaemonStateFile    string `json:"daemon_state_file"`
	DaemonPIDFile      string `json:"daemon_pid_file"`
	DaemonLogFile      string `json:"daemon_log_file"`
	DesktopStateFile   string `json:"desktop_state_file"`
	CodexConfigFile    string `json:"codex_config_file"`
	CodexProfileFile   string `json:"codex_ocgo_profile_file"`
	CodexCatalogFile   string `json:"codex_model_catalog_file"`
	CodexBackupsDir    string `json:"codex_backups_dir"`
}

func AllPaths() Paths {
	return Paths{
		ConfigDir:          config.ConfigDir(),
		ConfigFile:         config.ConfigFile(),
		ModelMappingFile:   config.ModelMappingFile(),
		ModelSelectionFile: config.ModelSelectionFile(),
		ModelCacheFile:     config.ModelCatalogCacheFile(),
		DaemonStateFile:    config.DaemonStateFile(),
		DaemonPIDFile:      config.PIDFile(),
		DaemonLogFile:      config.LogFile(),
		DesktopStateFile:   config.CodexDesktopStateFile(),
		CodexConfigFile:    config.CodexConfigFile(),
		CodexProfileFile:   config.CodexProfileConfigFile(),
		CodexCatalogFile:   config.CodexModelCatalogFile(),
		CodexBackupsDir:    config.CodexBackupDir(),
	}
}

func (p Paths) AllFiles() []string {
	return []string{
		p.ConfigFile, p.ModelMappingFile, p.ModelSelectionFile,
		p.ModelCacheFile, p.DaemonStateFile, p.DaemonPIDFile,
		p.DaemonLogFile, p.DesktopStateFile, p.CodexProfileFile,
		p.CodexCatalogFile,
	}
}

type Status string

const (
	StatusPresent  Status = "present"
	StatusMissing  Status = "missing"
	StatusRedacted Status = "redacted"
	StatusStale    Status = "stale"
	StatusUnknown  Status = "unknown"
)

func (s Status) String() string { return string(s) }

func fileStatus(path string) Status {
	if _, err := os.Stat(path); err == nil {
		return StatusPresent
	}
	return StatusMissing
}

type CoreSection struct {
	ConfigFile     Status `json:"config_file"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	OpenCodeAPIKey Status `json:"opencode_api_key"`
}

type ModelSection struct {
	SelectedModel string `json:"selected_model"`
	MappingFile   Status `json:"mapping_file"`
	ModelCache    Status `json:"model_cache"`
}

type DaemonSection struct {
	StateFile Status `json:"state_file"`
	PIDFile   Status `json:"pid_file"`
	PIDStatus Status `json:"pid_status"`
	LogFile   Status `json:"log_file"`
}

type CodexCLISection struct {
	ConfigFile   Status `json:"config_file"`
	OcgoProfile  Status `json:"ocgo_profile"`
	ModelCatalog Status `json:"model_catalog"`
}

type CodexDesktopSection struct {
	StateFile      Status `json:"state_file"`
	ActiveProvider string `json:"active_provider"`
	BackupFile     Status `json:"backup_file"`
}

type Inspection struct {
	Core         CoreSection         `json:"core"`
	Model        ModelSection        `json:"model"`
	Daemon       DaemonSection       `json:"daemon"`
	CodexCLI     CodexCLISection     `json:"codex_cli"`
	CodexDesktop CodexDesktopSection `json:"codex_desktop"`
}

func Inspect() Inspection {
	p := AllPaths()
	ins := Inspection{
		Core: CoreSection{
			ConfigFile:     fileStatus(p.ConfigFile),
			Host:           config.DefaultHost,
			Port:           config.DefaultPort,
			OpenCodeAPIKey: StatusMissing,
		},
		Model: ModelSection{
			MappingFile: fileStatus(p.ModelMappingFile),
			ModelCache:  fileStatus(p.ModelCacheFile),
		},
		Daemon: DaemonSection{
			StateFile: fileStatus(p.DaemonStateFile),
			PIDFile:   fileStatus(p.DaemonPIDFile),
			PIDStatus: StatusUnknown,
			LogFile:   fileStatus(p.DaemonLogFile),
		},
		CodexCLI: CodexCLISection{
			ConfigFile:   fileStatus(p.CodexConfigFile),
			OcgoProfile:  fileStatus(p.CodexProfileFile),
			ModelCatalog: fileStatus(p.CodexCatalogFile),
		},
		CodexDesktop: CodexDesktopSection{
			StateFile:      fileStatus(p.DesktopStateFile),
			ActiveProvider: StatusUnknown.String(),
			BackupFile:     StatusMissing,
		},
	}
	if b, err := os.ReadFile(p.ConfigFile); err == nil {
		var cfg struct {
			APIKey string `json:"api_key"`
			Host   string `json:"host"`
			Port   int    `json:"port"`
		}
		if json.Unmarshal(b, &cfg) == nil {
			if cfg.APIKey != "" {
				ins.Core.OpenCodeAPIKey = StatusPresent
			}
			if cfg.Host != "" {
				ins.Core.Host = cfg.Host
			}
			if cfg.Port != 0 {
				ins.Core.Port = cfg.Port
			}
		}
	}
	if b, err := os.ReadFile(p.ModelSelectionFile); err == nil {
		var sel struct{ Model string `json:"model"` }
		if json.Unmarshal(b, &sel) == nil {
			ins.Model.SelectedModel = sel.Model
		}
	}
	if _, err := os.Stat(p.DaemonStateFile); err == nil {
		ins.Daemon.PIDStatus = StatusUnknown
		if pid, err := config.ReadPID(); err == nil {
			ins.Daemon.PIDStatus = Status(process.StatusForPID(pid))
		}
	}
	if b, err := os.ReadFile(p.DesktopStateFile); err == nil {
		var st struct{ Provider string `json:"provider"` }
		if json.Unmarshal(b, &st) == nil {
			ins.CodexDesktop.ActiveProvider = st.Provider
		}
	}
	if entries, err := os.ReadDir(config.CodexBackupDir()); err == nil && len(entries) > 0 {
		ins.CodexDesktop.BackupFile = StatusPresent
	}
	return ins
}

type BackupResult struct {
	Path      string `json:"path"`
	FileCount int    `json:"file_count"`
}

type BackupManifest struct {
	CreatedAt          string   `json:"created_at"`
	OCGOVersion        string   `json:"ocgo_version"`
	Files              []string `json:"files"`
	IncludeCodexConfig bool     `json:"include_codex_config"`
}

func Backup(dest string, includeCodexConfig bool) (BackupResult, error) {
	p := AllPaths()
	ocgoDir := config.ConfigDir()
	codexDir := filepath.Dir(config.CodexConfigFile())

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return BackupResult{}, fmt.Errorf("failed to create backup directory: %w", err)
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	manifest := BackupManifest{
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		OCGOVersion:        buildinfo.Current().Version,
		Files:              []string{},
		IncludeCodexConfig: includeCodexConfig,
	}

	added := 0
	for _, path := range p.AllFiles() {
		if !fileExists(path) {
			continue
		}
		rel, err := makeRelative(path, ocgoDir, codexDir)
		if err != nil {
			continue
		}
		if err := addFileToZip(zw, path, rel); err != nil {
			return BackupResult{}, err
		}
		manifest.Files = append(manifest.Files, rel)
		added++
	}
	if includeCodexConfig {
		cc := config.CodexConfigFile()
		if fileExists(cc) {
			if err := addFileToZip(zw, cc, ".codex/config.toml"); err != nil {
				return BackupResult{}, err
			}
			manifest.Files = append(manifest.Files, ".codex/config.toml")
			added++
		}
	}
	if entries, err := os.ReadDir(config.CodexBackupDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ep := filepath.Join(config.CodexBackupDir(), e.Name())
			rel := filepath.Join(".config/ocgo/codex-backups", e.Name())
			if err := addFileToZip(zw, ep, rel); err != nil {
				return BackupResult{}, err
			}
			manifest.Files = append(manifest.Files, rel)
			added++
		}
	}
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BackupResult{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	mfh, err := zw.Create("backup-manifest.json")
	if err != nil {
		return BackupResult{}, fmt.Errorf("failed to create manifest in zip: %w", err)
	}
	if _, err := mfh.Write(append(mb, '\n')); err != nil {
		return BackupResult{}, fmt.Errorf("failed to write manifest: %w", err)
	}
	if err := zw.Close(); err != nil {
		return BackupResult{}, fmt.Errorf("failed to close zip: %w", err)
	}
	if err := os.WriteFile(dest, buf.Bytes(), 0644); err != nil {
		return BackupResult{}, fmt.Errorf("failed to write backup file: %w", err)
	}
	return BackupResult{Path: dest, FileCount: added + 1}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func makeRelative(path, ocgoDir, codexDir string) (string, error) {
	if isWithin(ocgoDir, path) {
		rel, err := filepath.Rel(ocgoDir, path)
		if err != nil {
			return "", err
		}
		return filepath.Join(".config/ocgo", rel), nil
	}
	if isWithin(codexDir, path) {
		rel, err := filepath.Rel(codexDir, path)
		if err != nil {
			return "", err
		}
		return filepath.Join(".codex", rel), nil
	}
	return "", fmt.Errorf("path %s is outside managed directories", path)
}

func addFileToZip(zw *zip.Writer, src, name string) error {
	fh, err := zw.Create(filepath.ToSlash(name))
	if err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_, err = fh.Write(b)
	return err
}

type RestoreOptions struct {
	DryRun             bool
	Yes                bool
	IncludeCodexConfig bool
}

type RestoreResult struct {
	Files []string `json:"files"`
}

func Restore(backupPath string, opts RestoreOptions) (RestoreResult, error) {
	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("cannot open backup: %w", err)
	}
	defer zr.Close()

	var manifest BackupManifest
	hasManifest := false
	for _, f := range zr.File {
		if f.Name == "backup-manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return RestoreResult{}, fmt.Errorf("failed to open manifest: %w", err)
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return RestoreResult{}, fmt.Errorf("failed to read manifest: %w", err)
			}
			if json.Unmarshal(b, &manifest) == nil {
				hasManifest = true
			}
			break
		}
	}
	if !hasManifest {
		return RestoreResult{}, errors.New("backup does not contain backup-manifest.json")
	}

	ocgoDir := config.ConfigDir()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return RestoreResult{}, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var toRestore []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || f.Name == "backup-manifest.json" {
			continue
		}
		name := filepath.ToSlash(f.Name)
		clean := filepath.Clean(filepath.FromSlash(name))
		if strings.HasPrefix(clean, "..") || strings.Contains(clean, ".."+string(filepath.Separator)) {
			return RestoreResult{}, fmt.Errorf("invalid path in backup %q: contains parent directory reference", name)
		}
		if filepath.IsAbs(clean) {
			return RestoreResult{}, fmt.Errorf("invalid path in backup %q: absolute path not allowed", name)
		}
		abs := filepath.Join(homeDir, clean)
		if !isWithin(ocgoDir, abs) && !isWithin(filepath.Dir(config.CodexConfigFile()), abs) {
			return RestoreResult{}, fmt.Errorf("invalid path in backup %q: outside allowed directories", name)
		}
		if clean == ".codex/config.toml" && !opts.IncludeCodexConfig {
			return RestoreResult{}, fmt.Errorf("backup contains .codex/config.toml; pass --include-codex-config to restore it")
		}
		toRestore = append(toRestore, name)
	}

	if opts.DryRun {
		return RestoreResult{Files: toRestore}, nil
	}
	if !opts.Yes {
		return RestoreResult{}, errors.New("refusing to continue without --yes")
	}

	prePath := filepath.Join(ocgoDir, "backups", fmt.Sprintf("pre-restore-%s.zip", time.Now().UTC().Format("20060102-150405")))
	if _, err := Backup(prePath, false); err != nil {
		return RestoreResult{}, fmt.Errorf("pre-restore backup failed: %w", err)
	}

	for _, name := range toRestore {
		abs := filepath.Join(homeDir, filepath.Clean(filepath.FromSlash(name)))
		dir := filepath.Dir(abs)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return RestoreResult{}, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		for _, zf := range zr.File {
			if filepath.ToSlash(zf.Name) != name {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return RestoreResult{}, fmt.Errorf("failed to open %s in backup: %w", name, err)
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return RestoreResult{}, fmt.Errorf("failed to read %s from backup: %w", name, err)
			}
			if err := os.WriteFile(abs, b, 0644); err != nil {
				return RestoreResult{}, fmt.Errorf("failed to write %s: %w", abs, err)
			}
			break
		}
	}
	return RestoreResult{Files: toRestore}, nil
}

type ResetScope string

const (
	ResetScopeOcgo         ResetScope = "ocgo"
	ResetScopeCache        ResetScope = "cache"
	ResetScopeCodexCLI     ResetScope = "codex-cli"
	ResetScopeCodexDesktop ResetScope = "codex-desktop"
	ResetScopeAll          ResetScope = "all"
)

type ResetOptions struct {
	Scope          ResetScope
	DryRun         bool
	Yes            bool
	IncludeBackups bool
	NoBackup       bool
}

type ResetResult struct {
	Scope    ResetScope `json:"scope"`
	Removed  []string   `json:"removed"`
	Backup   string     `json:"backup,omitempty"`
}

func Reset(opts ResetOptions) (ResetResult, error) {
	if opts.NoBackup && !opts.Yes {
		return ResetResult{}, errors.New("--no-backup requires --yes")
	}

	p := AllPaths()
	files := resolveScopeFiles(opts, p)

	if len(files) == 0 && opts.Scope != ResetScopeCodexDesktop {
		return ResetResult{Scope: opts.Scope}, nil
	}

	if opts.DryRun {
		return ResetResult{Scope: opts.Scope, Removed: files}, nil
	}
	if !opts.Yes {
		return ResetResult{}, errors.New("refusing to continue without --yes")
	}

	var backupPath string
	if !opts.NoBackup {
		bDir := filepath.Join(config.ConfigDir(), "backups")
		if err := os.MkdirAll(bDir, 0755); err != nil {
			return ResetResult{}, fmt.Errorf("failed to create backup directory: %w", err)
		}
		bPath := filepath.Join(bDir, fmt.Sprintf("pre-reset-%s.zip", time.Now().UTC().Format("20060102-150405")))
		result, err := Backup(bPath, false)
		if err != nil {
			return ResetResult{}, fmt.Errorf("pre-reset backup failed: %w", err)
		}
		backupPath = result.Path
	}

	removed := make([]string, 0, len(files))
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return ResetResult{}, fmt.Errorf("failed to remove %s: %w", f, err)
		}
		removed = append(removed, f)
	}

	return ResetResult{Scope: opts.Scope, Removed: removed, Backup: backupPath}, nil
}

func resolveScopeFiles(opts ResetOptions, p Paths) []string {
	scope := opts.Scope
	switch scope {
	case ResetScopeOcgo:
		return filterExisting([]string{
			p.ConfigFile, p.ModelMappingFile, p.ModelSelectionFile,
			p.DaemonStateFile, p.DesktopStateFile, p.DaemonPIDFile,
		})
	case ResetScopeCache:
		return filterExisting([]string{p.ModelCacheFile})
	case ResetScopeCodexCLI:
		return filterExisting([]string{p.CodexProfileFile, p.CodexCatalogFile})
	case ResetScopeCodexDesktop:
		return filterExisting([]string{p.DesktopStateFile})
	case ResetScopeAll:
		files := resolveScopeFiles(ResetOptions{Scope: ResetScopeOcgo}, p)
		files = append(files, resolveScopeFiles(ResetOptions{Scope: ResetScopeCache}, p)...)
		files = append(files, resolveScopeFiles(ResetOptions{Scope: ResetScopeCodexCLI}, p)...)
		if opts.IncludeBackups {
			files = appendBackupFiles(files, p)
		}
		return files
	}
	return nil
}

func appendBackupFiles(files []string, p Paths) []string {
	backupsDir := filepath.Join(config.ConfigDir(), "backups")
	if entries, err := os.ReadDir(backupsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(backupsDir, e.Name()))
			}
		}
	}
	if entries, err := os.ReadDir(p.CodexBackupsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(p.CodexBackupsDir, e.Name()))
			}
		}
	}
	return files
}

func filterExisting(paths []string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
