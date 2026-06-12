package support

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ocgo/internal/buildinfo"
	"ocgo/internal/config"
	"ocgo/internal/configlifecycle"
	"ocgo/internal/daemon"
	"ocgo/internal/doctor"
)

const maxLogBytes = 1 * 1024 * 1024

type BundleOptions struct {
	OutputPath  string
	Force       bool
	IncludeLogs bool
}

type BundleManifestFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	Error     string `json:"error,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	MaxBytes  int    `json:"max_bytes,omitempty"`
}

type BundleManifest struct {
	CreatedAt    string              `json:"created_at"`
	OCGOVersion  string              `json:"ocgo_version"`
	Redacted     bool                `json:"redacted"`
	LogsIncluded bool                `json:"logs_included"`
	Files        []BundleManifestFile `json:"files"`
}

type BundleResult struct {
	Path         string   `json:"path"`
	Files        []string `json:"files"`
	Redacted     bool     `json:"redacted"`
	LogsIncluded bool     `json:"logs_included"`
}

func CreateBundle(opts BundleOptions) (BundleResult, error) {
	redacted := true
	includeLogs := opts.IncludeLogs
	outputPath := opts.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(config.ConfigDir(), "support-bundles",
			fmt.Sprintf("ocgo-support-bundle-%s.zip", time.Now().UTC().Format("20060102-150405")))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return BundleResult{}, fmt.Errorf("cannot create output directory: %w", err)
	}

	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return BundleResult{}, fmt.Errorf("output file exists: %s (use --force to overwrite)", outputPath)
		}
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	var files []BundleManifestFile

	addBytes := func(name, source string, data []byte) {
		if redacted {
			data = RedactJSONBytes(data)
		}
		h, err := zw.Create(filepath.ToSlash(name))
		if err != nil {
			files = append(files, BundleManifestFile{Path: name, Status: "error", Source: source, Error: err.Error()})
			return
		}
		if _, err := h.Write(data); err != nil {
			files = append(files, BundleManifestFile{Path: name, Status: "error", Source: source, Error: err.Error()})
			return
		}
		files = append(files, BundleManifestFile{Path: name, Status: "included", Source: source})
	}

	addJSON := func(name string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			files = append(files, BundleManifestFile{Path: name, Status: "error", Source: "generated", Error: err.Error()})
			return
		}
		addBytes(name, "generated", append(b, '\n'))
	}

	addExistingFile := func(relPath, absPath string) {
		if absPath == "" {
			files = append(files, BundleManifestFile{Path: relPath, Status: "skipped", Source: "empty path"})
			return
		}
		b, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				files = append(files, BundleManifestFile{Path: relPath, Status: "missing", Source: absPath})
				return
			}
			files = append(files, BundleManifestFile{Path: relPath, Status: "error", Source: absPath, Error: err.Error()})
			return
		}
		if redacted {
			b = RedactJSONBytes(b)
		}
		h, err := zw.Create(filepath.ToSlash(relPath))
		if err != nil {
			files = append(files, BundleManifestFile{Path: relPath, Status: "error", Source: absPath, Error: err.Error()})
			return
		}
		if _, err := h.Write(b); err != nil {
			files = append(files, BundleManifestFile{Path: relPath, Status: "error", Source: absPath, Error: err.Error()})
			return
		}
		files = append(files, BundleManifestFile{Path: relPath, Status: "included", Source: absPath})
	}

	addVersion := func() {
		addJSON("version.json", buildinfo.Current())
	}

	addDoctor := func() {
		rep := doctor.NewRunner().RunCodex(context.Background(), doctor.ModeAll)
		addJSON("doctor.json", rep)
	}

	addDaemonStatus := func() {
		cfg, err := config.LoadConfig()
		if err != nil {
			files = append(files, BundleManifestFile{Path: "daemon-status.json", Status: "error", Source: "generated", Error: err.Error()})
			return
		}
		mgr := daemon.NewManager()
		ds := mgr.DetailedStatus(cfg)
		addJSON("daemon-status.json", ds)
	}

	addConfigPaths := func() {
		addJSON("config-paths.json", configlifecycle.AllPaths())
	}

	addConfigInspect := func() {
		addJSON("config-inspect.json", configlifecycle.Inspect())
	}

	addEnvironment := func() {
		env := map[string]any{
			"goos":             runtime.GOOS,
			"goarch":           runtime.GOARCH,
			"go_version":       runtime.Version(),
			"user_config_dir":  config.ConfigDir(),
			"home_detected":    true,
			"shell":            detectShell(),
			"timestamp":        time.Now().UTC().Format(time.RFC3339),
		}
		addJSON("environment.json", env)
	}

	addLogs := func() {
		if !includeLogs {
			files = append(files, BundleManifestFile{Path: "logs/ocgo.log", Status: "skipped", Source: "--no-logs"})
			return
		}
		logPath := config.LogFile()
		b, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				files = append(files, BundleManifestFile{Path: "logs/ocgo.log", Status: "missing", Source: logPath})
				return
			}
			files = append(files, BundleManifestFile{Path: "logs/ocgo.log", Status: "error", Source: logPath, Error: err.Error()})
			return
		}
		truncated := false
		if len(b) > maxLogBytes {
			b = b[len(b)-maxLogBytes:]
			truncated = true
		}
		content := string(b)
		if redacted {
			content = redactText(content)
		}
		h, err := zw.Create("logs/ocgo.log")
		if err != nil {
			files = append(files, BundleManifestFile{Path: "logs/ocgo.log", Status: "error", Source: logPath, Error: err.Error()})
			return
		}
		if _, err := h.Write([]byte(content)); err != nil {
			files = append(files, BundleManifestFile{Path: "logs/ocgo.log", Status: "error", Source: logPath, Error: err.Error()})
			return
		}
		mf := BundleManifestFile{Path: "logs/ocgo.log", Status: "included", Source: logPath}
		if truncated {
			mf.Truncated = true
			mf.MaxBytes = maxLogBytes
		}
		files = append(files, mf)
	}

	addStateFiles := func() {
		p := configlifecycle.AllPaths()
		stateFiles := map[string]string{
			"state/daemon-state.json":            p.DaemonStateFile,
			"state/model-selection.json":         p.ModelSelectionFile,
			"state/model-mapping.json":           p.ModelMappingFile,
			"state/codex-desktop-state.json":     p.DesktopStateFile,
			"state/codex-ocgo-profile.config.toml": p.CodexProfileFile,
			"state/codex-models.json":            p.CodexCatalogFile,
		}
		for rel, abs := range stateFiles {
			addExistingFile(rel, abs)
		}
	}

	addVersion()
	addDoctor()
	addDaemonStatus()
	addConfigPaths()
	addConfigInspect()
	addEnvironment()
	addLogs()
	addStateFiles()

	manifest := BundleManifest{
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		OCGOVersion:  buildinfo.Current().Version,
		Redacted:     redacted,
		LogsIncluded: includeLogs,
		Files:        files,
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	mfh, err := zw.Create("manifest.json")
	if err != nil {
		return BundleResult{}, fmt.Errorf("cannot create manifest.json in zip: %w", err)
	}
	if _, err := mfh.Write(append(mb, '\n')); err != nil {
		return BundleResult{}, fmt.Errorf("cannot write manifest.json: %w", err)
	}

	if err := zw.Close(); err != nil {
		return BundleResult{}, fmt.Errorf("cannot finalise zip: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return BundleResult{}, fmt.Errorf("cannot write bundle: %w", err)
	}

	fileNames := make([]string, 0, len(files)+1)
	fileNames = append(fileNames, "manifest.json")
	for _, f := range files {
		fileNames = append(fileNames, f.Path)
	}

	return BundleResult{
		Path:         outputPath,
		Files:        fileNames,
		Redacted:     redacted,
		LogsIncluded: includeLogs,
	}, nil
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("PSModulePath"); v != "" {
			return "powershell"
		}
		return "cmd"
	}
	if v := os.Getenv("SHELL"); v != "" {
		return v
	}
	return "unknown"
}

func SafeZipPath(path string) (string, error) {
	slash := filepath.ToSlash(path)
	if strings.HasPrefix(slash, "/") {
		return "", fmt.Errorf("absolute path not allowed: %s", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute path not allowed: %s", path)
	}
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}
	if strings.HasPrefix(slash, "..") || strings.Contains(slash, "/../") || strings.HasSuffix(slash, "/..") {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}
	if len(clean) >= 2 && clean[1] == ':' {
		return "", fmt.Errorf("windows drive path not allowed: %s", path)
	}
	return filepath.ToSlash(clean), nil
}
