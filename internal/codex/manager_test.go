package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocgo/internal/config"
	"ocgo/internal/models"
)

func tempCodexPaths(t *testing.T) Paths {
	t.Helper()
	return Paths{
		ConfigFile:        filepath.Join(t.TempDir(), ".codex", "config.toml"),
		ProfileFile:       filepath.Join(t.TempDir(), ".codex", config.CodexProfileName+".config.toml"),
		ModelCatalogFile:  filepath.Join(t.TempDir(), ".codex", "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(t.TempDir(), ".codex", "config.toml"),
		BackupDir:         filepath.Join(t.TempDir(), ".config", "ocgo", "codex-backups"),
	}
}

func TestNewManagerUsesConfigPaths(t *testing.T) {
	mgr := NewManager()
	if mgr.Paths.ConfigFile != config.CodexConfigFile() {
		t.Fatalf("ConfigFile = %q, want %q", mgr.Paths.ConfigFile, config.CodexConfigFile())
	}
	if mgr.Paths.ProfileFile != config.CodexProfileConfigFile() {
		t.Fatalf("ProfileFile = %q, want %q", mgr.Paths.ProfileFile, config.CodexProfileConfigFile())
	}
	if mgr.Paths.ModelCatalogFile != config.CodexModelCatalogFile() {
		t.Fatalf("ModelCatalogFile = %q, want %q", mgr.Paths.ModelCatalogFile, config.CodexModelCatalogFile())
	}
	if mgr.Paths.DesktopConfigFile == "" {
		t.Fatal("DesktopConfigFile should be non-empty (prepared for future Desktop support)")
	}
	if mgr.Paths.BackupDir == "" {
		t.Fatal("BackupDir should be non-empty (prepared for future Desktop support)")
	}
}

func TestManagerDesktopPaths(t *testing.T) {
	mgr := NewManager()
	if !strings.HasSuffix(mgr.DesktopStateFile(), "codex-desktop-state.json") {
		t.Fatalf("DesktopStateFile = %q, want it to end with codex-desktop-state.json", mgr.DesktopStateFile())
	}
	if !strings.HasSuffix(mgr.BackupDir(), "codex-backups") {
		t.Fatalf("BackupDir = %q, want it to end with codex-backups", mgr.BackupDir())
	}
}

func TestManagerDesktopStateFileIsUnchanged(t *testing.T) {
	mgr := Manager{}
	if mgr.DesktopStateFile() != config.CodexDesktopStateFile() {
		t.Fatalf("DesktopStateFile = %q, want %q (production default)", mgr.DesktopStateFile(), config.CodexDesktopStateFile())
	}
}

func TestManagerEnsureCLIConfigWritesProfileAndCatalog(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)

	mgr := Manager{Paths: tempCodexPaths(t)}
	if err := mgr.EnsureCLIConfig("http://127.0.0.1:3456"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(mgr.Paths.ProfileFile); err != nil {
		t.Fatalf("profile file should exist: %v", err)
	}
	if _, err := os.Stat(mgr.Paths.ModelCatalogFile); err != nil {
		t.Fatalf("model catalog file should exist: %v", err)
	}

	profile, err := os.ReadFile(mgr.Paths.ProfileFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(profile)
	expectedCatalog := strings.ReplaceAll(mgr.Paths.ModelCatalogFile, `\`, `\\`)
	for _, want := range []string{
		`openai_base_url = "http://127.0.0.1:3456/v1/"`,
		`forced_login_method = "api"`,
		`model_provider = "ocgo-launch"`,
		`model_catalog_json = "` + expectedCatalog + `"`,
		`model_reasoning_effort = "minimal"`,
		`model_reasoning_summary = "none"`,
		"[model_providers.ocgo-launch]",
		`name = "OpenCode Go"`,
		`base_url = "http://127.0.0.1:3456/v1/"`,
		`wire_api = "responses"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("profile missing %q in:\n%s", want, content)
		}
	}
	if strings.Contains(content, "[profiles.ocgo-launch]") {
		t.Fatalf("profile file must not contain legacy [profiles.ocgo-launch] table:\n%s", content)
	}

	catalog, err := os.ReadFile(mgr.Paths.ModelCatalogFile)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(catalog, &parsed); err != nil {
		t.Fatalf("catalog not valid JSON: %v", err)
	}
	if len(parsed.Models) == 0 {
		t.Fatalf("catalog should have at least one model, got 0")
	}
}

func TestManagerEnsureCLIConfigPreservesOtherProfiles(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)

	paths := tempCodexPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	existing := `profile = "default"

[profiles.default]
model = "gpt-5"

[model_providers.openai]
name = "OpenAI"
base_url = "https://api.openai.com/v1"
`
	if err := os.WriteFile(paths.ConfigFile, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := Manager{Paths: paths}
	if err := mgr.EnsureCLIConfig("http://127.0.0.1:3456"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		`profile = "default"`,
		`[profiles.default]`,
		`model = "gpt-5"`,
		`[model_providers.openai]`,
		`base_url = "https://api.openai.com/v1"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q to be preserved, full text:\n%s", want, text)
		}
	}
}

func TestManagerCleanLegacyMainConfig(t *testing.T) {
	existing := `profile = "ocgo-launch"

[profiles.ocgo-launch]
model = "old"

[profiles.default]
model = "gpt-5"

[model_providers.ocgo-launch]
base_url = "old"

[model_providers.openai]
base_url = "https://api.openai.com/v1"
`
	cleaned := StripLegacyProfile(existing)
	for _, gone := range []string{
		`profile = "ocgo-launch"`,
		`[profiles.ocgo-launch]`,
		`[model_providers.ocgo-launch]`,
		`base_url = "old"`,
	} {
		if strings.Contains(cleaned, gone) {
			t.Fatalf("expected %q to be removed, full text:\n%s", gone, cleaned)
		}
	}
	for _, kept := range []string{
		`[profiles.default]`,
		`model = "gpt-5"`,
		`[model_providers.openai]`,
		`base_url = "https://api.openai.com/v1"`,
	} {
		if !strings.Contains(cleaned, kept) {
			t.Fatalf("expected %q to be preserved, full text:\n%s", kept, cleaned)
		}
	}
}

func TestManagerWriteModelCatalogUsesManagerPath(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)

	paths := tempCodexPaths(t)
	mgr := Manager{Paths: paths}
	if err := mgr.WriteModelCatalog(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.ModelCatalogFile); err != nil {
		t.Fatalf("manager should write to its own ModelCatalogFile path: %v", err)
	}

	if _, err := os.Stat(config.CodexModelCatalogFile()); err == nil {
		t.Fatal("manager should not write to the production config path when given a custom path")
	}
}

func TestWriteModelCatalogWrapperUsesProvidedPath(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)

	path := filepath.Join(t.TempDir(), "ocgo-models.json")
	if err := WriteModelCatalog(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("wrapper should write to the provided path: %v", err)
	}
}

func TestManagerCheckVersionErrorWhenCodexMissing(t *testing.T) {
	old := execLookPath
	execLookPath = func(file string) (string, error) {
		return "", errors.New("not in path")
	}
	t.Cleanup(func() { execLookPath = old })

	mgr := NewManager()
	if err := mgr.CheckVersion(); err == nil {
		t.Fatal("expected error when codex is missing")
	}
}

func TestManagerDesktopStateShape(t *testing.T) {
	state := DesktopState{
		Version:   DesktopStateVersion,
		Mode:      DesktopModeOpenCode,
		UpdatedAt: time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BaseURL:   "http://127.0.0.1:3456/v1/",
		Model:     "minimax-m3",
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"version":1`,
		`"mode":"opencode"`,
		`"updated_at":"2026-06-09T00:00:00Z"`,
		`"base_url":"http://127.0.0.1:3456/v1/"`,
		`"model":"minimax-m3"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("DesktopState JSON missing %q in %s", want, string(data))
		}
	}
}

func TestEnsureConfigWrapperStillWorks(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(tmpHome, ".config", "ocgo", "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })
	restoreCache := models.SetCacheFileForTest(filepath.Join(tmpHome, ".config", "ocgo", "model-catalog-cache.json"))
	t.Cleanup(restoreCache)

	if err := EnsureConfig("http://127.0.0.1:3456"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmpHome, ".codex", "ocgo-launch.config.toml")); err != nil {
		t.Fatalf("profile file should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpHome, ".codex", "ocgo-models.json")); err != nil {
		t.Fatalf("catalog file should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpHome, ".codex", "config.toml")); err != nil {
		t.Fatalf("config file should exist: %v", err)
	}
}
