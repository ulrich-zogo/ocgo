package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"ocgo/internal/app"
	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/doctor"
	"ocgo/internal/models"
)

func homeDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeOCGOConfig(t *testing.T, dir, host string, port int) {
	t.Helper()
	cfgPath := filepath.Join(dir, ".config", "ocgo", "config.json")
	body := `{"api_key":"sk-test-redacted","host":"` + host + `","port":` + itoa(port) + `}`
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixturePath(name string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "fixtures", name)
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func copyFixture(t *testing.T, name, dst string) {
	t.Helper()
	content := readFixture(t, name)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, content, 0644); err != nil {
		t.Fatal(err)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func setupModels(t *testing.T) {
	t.Helper()
	selPath := filepath.Join(t.TempDir(), "model-selection.json")
	restoreSel := models.SetModelSelectionFileForTest(selPath)
	t.Cleanup(restoreSel)
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	restoreCache := models.SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "deepseek-v4-pro", Object: "model", Created: 2, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })
}

func TestAllClientsUseSameDaemonPort(t *testing.T) {
	dir := homeDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	setupModels(t)
	writeOCGOConfig(t, dir, "127.0.0.1", 3456)

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + cfg.Host + ":" + itoa(cfg.Port)
	if base != "http://127.0.0.1:3456" {
		t.Fatalf("unexpected base URL: %s", base)
	}

	mgr := codex.Manager{Paths: codex.Paths{
		ConfigFile:        filepath.Join(dir, ".codex", "config.toml"),
		ProfileFile:       filepath.Join(dir, ".codex", "ocgo-launch.config.toml"),
		ModelCatalogFile:  filepath.Join(dir, ".codex", "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(dir, ".codex", "config.toml"),
		BackupDir:         filepath.Join(dir, ".config", "ocgo", "codex-backups"),
	}}

	if err := mgr.EnsureCLIConfig(base); err != nil {
		t.Fatal(err)
	}

	profileBody, err := os.ReadFile(mgr.Paths.ProfileFile)
	if err != nil {
		t.Fatal(err)
	}
	profile := string(profileBody)

	if !strings.Contains(profile, `openai_base_url = "http://127.0.0.1:3456/v1/"`) {
		t.Errorf("Codex CLI profile missing expected openai_base_url:\n%s", profile)
	}
	if !strings.Contains(profile, `base_url = "http://127.0.0.1:3456/v1/"`) {
		t.Errorf("Codex CLI profile provider missing expected base_url:\n%s", profile)
	}

	stateFile := filepath.Join(dir, ".config", "ocgo", "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)

	st, err := mgr.EnableDesktopOpenCode(base, "minimax-m3")
	if err != nil {
		t.Fatal(err)
	}
	if st.BaseURL != "http://127.0.0.1:3456/v1/" {
		t.Errorf("Desktop state BaseURL = %q, want %q", st.BaseURL, "http://127.0.0.1:3456/v1/")
	}

	env, ok, err := app.BuildClaudeModelEnv("minimax-m3")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok from BuildClaudeModelEnv")
	}

	claudeVars := []string{
		"ANTHROPIC_MODEL=minimax-m3",
		"ANTHROPIC_SMALL_FAST_MODEL=minimax-m3",
		"ANTHROPIC_CUSTOM_MODEL_OPTION=minimax-m3",
	}
	for _, want := range claudeVars {
		found := false
		for _, e := range env {
			if e == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Claude env missing %q in %v", want, env)
		}
	}

	wantURL := "http://127.0.0.1:3456"
	if base != wantURL {
		t.Errorf("Claude launch URL derived from config = %q, want %q", base, wantURL)
	}
	if st.BaseURL != wantURL+"/v1/" {
		t.Errorf("Desktop BaseURL = %q, want %q", st.BaseURL, wantURL+"/v1/")
	}

}

func TestClaudeLaunchDoesNotWriteCodexConfig(t *testing.T) {
	dir := homeDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	setupModels(t)
	writeOCGOConfig(t, dir, "127.0.0.1", 3456)

	codexConfig := filepath.Join(dir, ".codex", "config.toml")
	codexProfile := filepath.Join(dir, ".codex", "ocgo-launch.config.toml")
	codexCatalog := filepath.Join(dir, ".codex", "ocgo-models.json")

	env, _, err := app.BuildClaudeModelEnv("minimax-m3")
	if err != nil {
		t.Fatal(err)
	}

	if len(env) == 0 {
		t.Fatal("expected non-empty env from BuildClaudeModelEnv")
	}

	for _, p := range []string{codexConfig, codexProfile, codexCatalog} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("Claude launch should not create Codex config file: %s", p)
		}
	}
}

func TestDoctorJSONHasExpectedFields(t *testing.T) {
	checks := []doctor.Check{
		{ID: "core.config", Label: "Config", Status: doctor.StatusOK, Message: "loaded"},
		{ID: "core.model", Label: "Model", Status: doctor.StatusWarning, Message: "no default", Remediation: "set-default"},
		{ID: "core.catalog", Label: "Catalog", Status: doctor.StatusError, Message: "empty", Details: "no models"},
	}
	rep := doctor.NewReport(checks)

	if rep.Status != doctor.StatusError {
		t.Errorf("report status = %s, want error", rep.Status)
	}

	if len(rep.Checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(rep.Checks))
	}

	for _, c := range rep.Checks {
		if c.ID == "" {
			t.Error("check has empty ID")
		}
		if c.Label == "" {
			t.Error("check has empty Label")
		}
		if c.Status == "" {
			t.Error("check has empty Status")
		}
		if c.Message == "" {
			t.Error("check has empty Message")
		}
	}
}

func TestDoctorJSONRollupErrorDominates(t *testing.T) {
	checks := []doctor.Check{
		{ID: "a", Label: "A", Status: doctor.StatusOK, Message: "ok"},
		{ID: "b", Label: "B", Status: doctor.StatusWarning, Message: "warn"},
		{ID: "c", Label: "C", Status: doctor.StatusError, Message: "err"},
	}
	rep := doctor.NewReport(checks)
	if rep.Status != doctor.StatusError {
		t.Errorf("report with at least one error should be error, got %s", rep.Status)
	}
}

func TestDoctorJSONRollupWarning(t *testing.T) {
	checks := []doctor.Check{
		{ID: "a", Label: "A", Status: doctor.StatusOK, Message: "ok"},
		{ID: "b", Label: "B", Status: doctor.StatusWarning, Message: "warn"},
	}
	rep := doctor.NewReport(checks)
	if rep.Status != doctor.StatusWarning {
		t.Errorf("report with only warnings should be warning, got %s", rep.Status)
	}
}

func TestDoctorJSONAllOK(t *testing.T) {
	checks := []doctor.Check{
		{ID: "a", Label: "A", Status: doctor.StatusOK, Message: "ok"},
	}
	rep := doctor.NewReport(checks)
	if rep.Status != doctor.StatusOK {
		t.Errorf("all-ok report should be ok, got %s", rep.Status)
	}
}

func TestCodexCLIConfigPreservesExistingFixtureProfile(t *testing.T) {
	dir := homeDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	setupModels(t)
	writeOCGOConfig(t, dir, "127.0.0.1", 3456)

	codexDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, "codex_config.toml", filepath.Join(codexDir, "config.toml"))

	mgr := codex.Manager{Paths: codex.Paths{
		ConfigFile:        filepath.Join(codexDir, "config.toml"),
		ProfileFile:       filepath.Join(codexDir, "ocgo-launch.config.toml"),
		ModelCatalogFile:  filepath.Join(codexDir, "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(codexDir, "config.toml"),
		BackupDir:         filepath.Join(dir, ".config", "ocgo", "codex-backups"),
	}}

	base := "http://127.0.0.1:3456"
	if err := mgr.EnsureCLIConfig(base); err != nil {
		t.Fatal(err)
	}

	rootConfig, err := os.ReadFile(mgr.Paths.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	root := string(rootConfig)

	for _, want := range []string{
		`[profiles.default]`,
		`model = "gpt-5"`,
		`model_provider = "openai"`,
		`[model_providers.openai]`,
		`base_url = "https://api.openai.com/v1"`,
	} {
		if !strings.Contains(root, want) {
			t.Errorf("root config is missing expected pre-existing section: %q\n%s", want, root)
		}
	}

	profile, err := os.ReadFile(mgr.Paths.ProfileFile)
	if err != nil {
		t.Fatal(err)
	}
	pro := string(profile)

	for _, want := range []string{
		`openai_base_url = "http://127.0.0.1:3456/v1/"`,
		`model_provider = "ocgo-launch"`,
		`[model_providers.ocgo-launch]`,
		`wire_api = "responses"`,
		`base_url = "http://127.0.0.1:3456/v1/"`,
	} {
		if !strings.Contains(pro, want) {
			t.Errorf("profile is missing expected OCGO section: %q\n%s", want, pro)
		}
	}
}

func TestCodexDesktopBacksUpAndRestoresFixtureConfig(t *testing.T) {
	dir := homeDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	setupModels(t)
	writeOCGOConfig(t, dir, "127.0.0.1", 3456)

	paths := codex.Paths{
		ConfigFile:        filepath.Join(dir, ".codex", "config.toml"),
		ProfileFile:       filepath.Join(dir, ".codex", "ocgo-launch.config.toml"),
		ModelCatalogFile:  filepath.Join(dir, ".codex", "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(dir, ".codex", "config.toml"),
		BackupDir:         filepath.Join(dir, ".config", "ocgo", "codex-backups"),
	}

	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, "codex_desktop_config.toml", paths.DesktopConfigFile)
	fixtureContent := readFixture(t, "codex_desktop_config.toml")

	mgr := codex.Manager{Paths: paths}
	stateFile := filepath.Join(dir, ".config", "ocgo", "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)

	if _, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "minimax-m3"); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.EnableDesktopChatGPT(); err != nil {
		t.Fatal(err)
	}

	restored, err := os.ReadFile(paths.DesktopConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(fixtureContent) {
		t.Errorf("restored content differs from original fixture\n  got:  %q\n  want: %q", restored, fixtureContent)
	}
}

func TestReadOCGOConfigFixture(t *testing.T) {
	dir := homeDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)

	copyFixture(t, "ocgo_config.json", filepath.Join(dir, ".config", "ocgo", "config.json"))

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "sk-test-redacted" {
		t.Errorf("APIKey = %q, want sk-test-redacted", cfg.APIKey)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 3456 {
		t.Errorf("Port = %d, want 3456", cfg.Port)
	}
}

func TestCodexDesktopStatusIncludesProviderAndModel(t *testing.T) {
	dir := homeDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	setupModels(t)
	writeOCGOConfig(t, dir, "127.0.0.1", 3456)

	paths := codex.Paths{
		ConfigFile:        filepath.Join(dir, ".codex", "config.toml"),
		ProfileFile:       filepath.Join(dir, ".codex", "ocgo-launch.config.toml"),
		ModelCatalogFile:  filepath.Join(dir, ".codex", "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(dir, ".codex", "config.toml"),
		BackupDir:         filepath.Join(dir, ".config", "ocgo", "codex-backups"),
	}

	if err := os.MkdirAll(filepath.Dir(paths.DesktopConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	original := `model = "gpt-5"
model_provider = "openai"
`
	if err := os.WriteFile(paths.DesktopConfigFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := codex.Manager{Paths: paths}
	stateFile := filepath.Join(dir, ".config", "ocgo", "codex-desktop-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", stateFile)

	st, err := mgr.EnableDesktopOpenCode("http://127.0.0.1:3456", "deepseek-v4-pro")
	if err != nil {
		t.Fatal(err)
	}
	if st.Model != "deepseek-v4-pro" {
		t.Errorf("Model = %q, want deepseek-v4-pro", st.Model)
	}
	if st.Mode != codex.DesktopModeOpenCode {
		t.Errorf("Mode = %q, want %q", st.Mode, codex.DesktopModeOpenCode)
	}
	if st.BaseURL != "http://127.0.0.1:3456/v1/" {
		t.Errorf("BaseURL = %q, want %q", st.BaseURL, "http://127.0.0.1:3456/v1/")
	}

	status, err := mgr.DesktopStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Managed {
		t.Error("Managed = false, want true")
	}
	if status.Mode != codex.DesktopModeOpenCode {
		t.Errorf("Mode = %q, want %q", status.Mode, codex.DesktopModeOpenCode)
	}
	if status.Model != "deepseek-v4-pro" {
		t.Errorf("Model = %q, want deepseek-v4-pro", status.Model)
	}
	if status.BaseURL != "http://127.0.0.1:3456/v1/" {
		t.Errorf("BaseURL = %q, want %q", status.BaseURL, "http://127.0.0.1:3456/v1/")
	}
}
