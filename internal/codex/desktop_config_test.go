package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocgo/internal/config"
	"ocgo/internal/models"
)

func redirectHomeForCodex(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(dir, ".config", "ocgo", "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })
	restoreCache := models.SetCacheFileForTest(filepath.Join(dir, ".config", "ocgo", "model-catalog-cache.json"))
	t.Cleanup(restoreCache)
}

func TestNormalizeBaseURLWithV1(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http://127.0.0.1:3456", "http://127.0.0.1:3456/v1/"},
		{"http://127.0.0.1:3456/", "http://127.0.0.1:3456/v1/"},
		{"http://127.0.0.1:3456/v1", "http://127.0.0.1:3456/v1/"},
		{"http://127.0.0.1:3456/v1/", "http://127.0.0.1:3456/v1/"},
		{"http://127.0.0.1:3456/v1/extra/", "http://127.0.0.1:3456/v1/extra/v1/"},
	}
	for _, tc := range cases {
		if got := NormalizeBaseURLWithV1(tc.in); got != tc.want {
			t.Errorf("NormalizeBaseURLWithV1(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEnableDesktopOpenCodeBacksUpExistingConfig(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	original := "model = \"gpt-5\"\nmodel_provider = \"openai\"\n"
	if err := os.WriteFile(paths.DesktopConfigFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "codex-desktop-state.json"))

	st, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m3")
	if err != nil {
		t.Fatalf("EnableDesktopOpenCode: %v", err)
	}
	if st.Mode != DesktopModeOpenCode {
		t.Errorf("Mode = %q, want %q", st.Mode, DesktopModeOpenCode)
	}
	if !strings.HasSuffix(st.BaseURL, "/v1/") {
		t.Errorf("BaseURL = %q, want suffix /v1/", st.BaseURL)
	}
	if st.Model != "minimax-m3" {
		t.Errorf("Model = %q, want minimax-m3", st.Model)
	}
	if st.BackupFile == "" {
		t.Fatal("BackupFile should be set when prior config exists")
	}
	backupContent, err := os.ReadFile(st.BackupFile)
	if err != nil {
		t.Fatalf("backup file should exist: %v", err)
	}
	if string(backupContent) != original {
		t.Errorf("backup content = %q, want %q", backupContent, original)
	}
	got, err := os.ReadFile(paths.DesktopConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		`model = "minimax-m3"`,
		`model_provider = "ocgo-desktop"`,
		`[model_providers.ocgo-desktop]`,
		`name = "OCGO OpenCode Go"`,
		`base_url = "http://127.0.0.1:3456/v1/"`,
		`env_key = "OPENAI_API_KEY"`,
		`wire_api = "responses"`,
		`model_catalog_json = "`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("desktop config missing %q in:\n%s", want, text)
		}
	}
	if _, err := os.Stat(paths.ModelCatalogFile); err != nil {
		t.Errorf("model catalog should be written: %v", err)
	}
}

func TestEnableDesktopOpenCodeWithoutExistingConfig(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "codex-desktop-state.json"))

	st, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m3")
	if err != nil {
		t.Fatalf("EnableDesktopOpenCode: %v", err)
	}
	if st.BackupFile != "" {
		t.Errorf("BackupFile should be empty when no prior config; got %q", st.BackupFile)
	}
	if st.Mode != DesktopModeOpenCode {
		t.Errorf("Mode = %q, want %q", st.Mode, DesktopModeOpenCode)
	}
	if _, err := os.Stat(paths.DesktopConfigFile); err != nil {
		t.Errorf("desktop config should be written: %v", err)
	}
}

func TestEnableDesktopOpenCodeTwicePreservesOriginalBackup(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	original := "model = \"gpt-5\"\n"
	if err := os.WriteFile(paths.DesktopConfigFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "codex-desktop-state.json"))

	st1, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m3")
	if err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if st1.BackupFile == "" {
		t.Fatal("first enable should produce a backup")
	}

	time.Sleep(1100 * time.Millisecond)

	st2, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m2.7")
	if err != nil {
		t.Fatalf("second enable: %v", err)
	}
	if st2.BackupFile != st1.BackupFile {
		t.Errorf("second enable should reuse original backup %q, got %q", st1.BackupFile, st2.BackupFile)
	}
	backupContent, err := os.ReadFile(st2.BackupFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupContent) != original {
		t.Errorf("original backup must not be overwritten; got %q, want %q", backupContent, original)
	}
}

func TestEnableDesktopChatGPTRestoresBackup(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	original := "model = \"gpt-5\"\nmodel_provider = \"openai\"\n"
	if err := os.WriteFile(paths.DesktopConfigFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "codex-desktop-state.json"))

	if _, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m3"); err != nil {
		t.Fatalf("enable opencode: %v", err)
	}
	st, err := mgr.EnableDesktopChatGPT()
	if err != nil {
		t.Fatalf("EnableDesktopChatGPT: %v", err)
	}
	if st.Mode != DesktopModeChatGPT {
		t.Errorf("Mode = %q, want %q", st.Mode, DesktopModeChatGPT)
	}
	got, err := os.ReadFile(paths.DesktopConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("restored content = %q, want %q", got, original)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func TestEnableDesktopChatGPTNoBackupReturnsError(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "codex-desktop-state.json"))

	if _, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m3"); err != nil {
		t.Fatalf("enable opencode: %v", err)
	}
	st, err := mgr.EnableDesktopChatGPT()
	if err == nil {
		t.Fatal("expected error for missing backup")
	}
	if st.Mode != "" {
		t.Errorf("Mode = %q, want empty on failure", st.Mode)
	}
}

func TestEnableDesktopChatGPTNotInOpenCodeModeFails(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "codex-desktop-state.json"))

	if err := WriteDesktopState(mgr.DesktopStateFile(), DesktopState{
		Version:   DesktopStateVersion,
		Mode:      DesktopModeChatGPT,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := mgr.EnableDesktopChatGPT()
	if err == nil {
		t.Fatal("expected error when mode is not opencode")
	}
}

func TestDesktopStatusNotManaged(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(t.TempDir(), "missing-state.json"))

	rep, err := mgr.DesktopStatus()
	if err != nil {
		t.Fatalf("DesktopStatus: %v", err)
	}
	if rep.Managed {
		t.Error("Managed = true, want false when no state file")
	}
}

func TestDesktopStatusChatGPT(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	if err := WriteDesktopState(stateFile, DesktopState{
		Version:   DesktopStateVersion,
		Mode:      DesktopModeChatGPT,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := mgr.DesktopStatus()
	if err != nil {
		t.Fatalf("DesktopStatus: %v", err)
	}
	if !rep.Managed {
		t.Error("Managed = false, want true")
	}
	if rep.Mode != DesktopModeChatGPT {
		t.Errorf("Mode = %q, want %q", rep.Mode, DesktopModeChatGPT)
	}
}

func TestDesktopStatusOpenCode(t *testing.T) {
	redirectHomeForCodex(t)
	paths := tempDesktopPaths(t)
	mgr := Manager{Paths: paths}
	stateFile := filepath.Join(t.TempDir(), "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)
	if err := WriteDesktopState(stateFile, DesktopState{
		Version:    DesktopStateVersion,
		Mode:       DesktopModeOpenCode,
		UpdatedAt:  time.Now().UTC(),
		BaseURL:    "http://127.0.0.1:3456/v1/",
		Model:      "minimax-m3",
		BackupFile: filepath.Join(paths.BackupDir, "config-test.toml"),
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := mgr.DesktopStatus()
	if err != nil {
		t.Fatalf("DesktopStatus: %v", err)
	}
	if !rep.Managed {
		t.Error("Managed = false, want true")
	}
	if rep.Mode != DesktopModeOpenCode {
		t.Errorf("Mode = %q, want %q", rep.Mode, DesktopModeOpenCode)
	}
	if rep.BaseURL != "http://127.0.0.1:3456/v1/" {
		t.Errorf("BaseURL = %q, want %q", rep.BaseURL, "http://127.0.0.1:3456/v1/")
	}
	if rep.Model != "minimax-m3" {
		t.Errorf("Model = %q, want minimax-m3", rep.Model)
	}
	if rep.BackupFile == "" {
		t.Error("BackupFile = empty, want non-empty")
	}
}
