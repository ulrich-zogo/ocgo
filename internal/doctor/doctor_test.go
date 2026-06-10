package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/daemon"
	"ocgo/internal/models"
)

// nowUTC returns a stable UTC time used in test fixtures.
func nowUTC() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

// setupRunner builds a Runner whose paths all live in a
// temp dir. Tests should never touch the real HOME.
func setupRunner(t *testing.T) (Runner, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_DAEMON_STATE_FILE", filepath.Join(dir, ".config", "ocgo", "daemon-state.json"))
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(dir, ".config", "ocgo", "codex-desktop-state.json"))
	paths := Paths{
		ConfigDir:          filepath.Join(dir, ".config", "ocgo"),
		ConfigFile:         filepath.Join(dir, ".config", "ocgo", "config.json"),
		ModelSelectionFile: filepath.Join(dir, ".config", "ocgo", "model-selection.json"),
		CatalogCacheFile:   filepath.Join(dir, ".config", "ocgo", "model-catalog-cache.json"),
		DaemonStateFile:    filepath.Join(dir, ".config", "ocgo", "daemon-state.json"),
		CodexConfigFile:    filepath.Join(dir, ".codex", "config.toml"),
		CodexProfileFile:   filepath.Join(dir, ".codex", "ocgo-launch.config.toml"),
		CodexCatalogFile:   filepath.Join(dir, ".codex", "ocgo-models.json"),
		DesktopStateFile:   filepath.Join(dir, ".config", "ocgo", "codex-desktop-state.json"),
		BackupDir:          filepath.Join(dir, ".config", "ocgo", "codex-backups"),
	}
	// Reset the models package fetchers so tests are
	// deterministic: use the static fallback (18 models).
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, nil, errors.New("no official"), errors.New("no remote"))
	restoreCache := models.SetCacheFileForTest(paths.CatalogCacheFile)
	t.Cleanup(restoreCache)
	t.Cleanup(func() { models.ResetFetchersForTest() })
	runner := NewRunnerWithPaths(paths)
	runner.HostPort = func() (string, int, error) { return "127.0.0.1", 1, nil }
	// The HTTP client is wired lazily by client(); we
	// replace it per-test if we need a real local server.
	return runner, dir
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// --- Report status rollup ---

func TestReportGlobalStatusOKWhenAllOK(t *testing.T) {
	rep := NewReport([]Check{
		OK("a", "A", "ok"),
		OK("b", "B", "ok"),
	})
	if rep.Status != StatusOK {
		t.Errorf("status = %s, want ok", rep.Status)
	}
}

func TestReportGlobalStatusWarningWhenOnlyWarnings(t *testing.T) {
	rep := NewReport([]Check{
		OK("a", "A", "ok"),
		Warning("b", "B", "warn", ""),
	})
	if rep.Status != StatusWarning {
		t.Errorf("status = %s, want warning", rep.Status)
	}
}

func TestReportGlobalStatusErrorWhenAtLeastOneError(t *testing.T) {
	rep := NewReport([]Check{
		OK("a", "A", "ok"),
		Warning("b", "B", "warn", ""),
		Error("c", "C", "boom", ""),
	})
	if rep.Status != StatusError {
		t.Errorf("status = %s, want error", rep.Status)
	}
}

func TestReportEmptyStatusIsOK(t *testing.T) {
	rep := NewReport(nil)
	if rep.Status != StatusOK {
		t.Errorf("empty report status = %s, want ok", rep.Status)
	}
}

func TestReportAppendReDerivesStatus(t *testing.T) {
	rep := NewReport([]Check{OK("a", "A", "ok")})
	rep = rep.Append(Error("b", "B", "boom", ""))
	if rep.Status != StatusError {
		t.Errorf("appended status = %s, want error", rep.Status)
	}
}

func TestCheckWithDetailsTrimsWhitespace(t *testing.T) {
	c := Error("id", "L", "msg", "").WithDetails("  hello \n")
	if c.Details != "hello" {
		t.Errorf("details = %q, want %q", c.Details, "hello")
	}
}

func TestCheckWithDetailsEmpty(t *testing.T) {
	c := OK("id", "L", "msg").WithDetails("   ")
	if c.Details != "" {
		t.Errorf("details = %q, want empty", c.Details)
	}
}

// --- Mode validation ---

func TestModeIsValid(t *testing.T) {
	for _, m := range []Mode{ModeAll, ModeCLI, ModeDesktop} {
		if !m.IsValid() {
			t.Errorf("mode %q should be valid", m)
		}
	}
	if Mode("garbage").IsValid() {
		t.Errorf("mode garbage should not be valid")
	}
}

func TestRunCodexInvalidModeReturnsError(t *testing.T) {
	runner, _ := setupRunner(t)
	rep := runner.RunCodex(context.Background(), Mode("garbage"))
	if rep.Status != StatusError {
		t.Errorf("status = %s, want error", rep.Status)
	}
	if len(rep.Checks) != 1 {
		t.Errorf("checks = %d, want 1", len(rep.Checks))
	}
}

// --- Config check ---

func TestCheckOCGOConfigMissing(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkOCGOConfig()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
	if !strings.Contains(c.Remediation, "ocgo setup") {
		t.Errorf("remediation = %q, want contains 'ocgo setup'", c.Remediation)
	}
}

func TestCheckOCGOConfigInvalidJSON(t *testing.T) {
	runner, _ := setupRunner(t)
	writeFile(t, runner.Paths.ConfigFile, "not json")
	c := runner.checkOCGOConfig()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error", c.Status)
	}
}

func TestCheckOCGOConfigEmptyAPIKey(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.ConfigFile, config.Config{Host: "127.0.0.1", Port: 3456})
	c := runner.checkOCGOConfig()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error", c.Status)
	}
	if !strings.Contains(c.Message, "api_key") {
		t.Errorf("message = %q, want mentions api_key", c.Message)
	}
}

func TestCheckOCGOConfigOK(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.ConfigFile, config.Config{
		Host: "127.0.0.1", Port: 3456, APIKey: "test-key",
	})
	c := runner.checkOCGOConfig()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

// --- Model / catalog ---

func TestCheckEffectiveModelFallsBackToFirstKnown(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkEffectiveModel()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning; message = %s", c.Status, c.Message)
	}
}

func TestCheckEffectiveModelConfiguredInvalid(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.ModelSelectionFile, models.ModelSelection{
		Version: 1, DefaultModel: "does-not-exist", UpdatedAt: nowUTC(),
	})
	c := runner.checkEffectiveModel()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckEffectiveModelConfiguredOK(t *testing.T) {
	runner, _ := setupRunner(t)
	// Pick a real known ID.
	known := models.KnownIDs()
	if len(known) == 0 {
		t.Skip("no known models in fallback")
	}
	writeJSONFile(t, runner.Paths.ModelSelectionFile, models.ModelSelection{
		Version: 1, DefaultModel: known[0], UpdatedAt: nowUTC(),
	})
	c := runner.checkEffectiveModel()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckModelCatalogOK(t *testing.T) {
	runner, _ := setupRunner(t)
	// Create a minimal cache file.
	writeJSONFile(t, runner.Paths.CatalogCacheFile, map[string]any{
		"version":    1,
		"source":     "official",
		"fetched_at": "2026-01-01T00:00:00Z",
		"models":     []map[string]any{{"id": "minimax-m3", "object": "model", "created": 1, "owned_by": "opencode"}},
	})
	c := runner.checkModelCatalog()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckModelCatalogCacheMissingButFallbackAvailable(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkModelCatalog()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning; message = %s", c.Status, c.Message)
	}
}

// --- Daemon checks (with a stubbed healthyFn via the
// real doctor; we simulate "no daemon" by leaving HOME
// empty and using the proxy stub below) ---

func TestCheckDaemonStateFileMissingIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkDaemonStateFile()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
}

func TestCheckDaemonStateFileInvalidJSONIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	writeFile(t, runner.Paths.DaemonStateFile, "not json")
	c := runner.checkDaemonStateFile()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error", c.Status)
	}
}

func TestCheckDaemonStateFileOK(t *testing.T) {
	runner, _ := setupRunner(t)
	st := daemon.State{
		Version: daemon.StateVersion, Host: "127.0.0.1", Port: 3456,
		BaseURL: "http://127.0.0.1:3456", Mode: daemon.ModeDaemon,
		StartedAt: nowUTC(),
	}
	writeJSONFile(t, runner.Paths.DaemonStateFile, st)
	c := runner.checkDaemonStateFile()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok", c.Status)
	}
}

func TestCheckHealthEndpointUnreachableIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	// runner.HostPort points to 127.0.0.1:1 which is closed.
	c := runner.checkHealthEndpoint(context.Background())
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckHealthEndpointOK(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))
	defer srv.Close()
	// Parse the test server URL and rewire HostPort.
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	if len(parts) != 2 {
		t.Fatalf("unexpected test server URL: %s", srv.URL)
	}
	port := 0
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
		t.Fatal(err)
	}
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkHealthEndpoint(context.Background())
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckModelsEndpointOK(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"minimax-m3","object":"model","created":1,"owned_by":"opencode"}]}`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkModelsEndpoint(context.Background())
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckModelsEndpointWrongObjectIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"object":"other","data":[]}`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkModelsEndpoint(context.Background())
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckCountTokensOK(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"input_tokens":8}`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkCountTokensEndpoint(context.Background())
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckCountTokensZeroIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"input_tokens":0}`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkCountTokensEndpoint(context.Background())
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckResponsesValidationOK(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"missing model","type":"invalid_request_error"}}`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkResponsesValidation(context.Background())
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckResponsesValidation5xxIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`bad gateway`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	parts := strings.Split(host, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	runner.HostPort = func() (string, int, error) { return parts[0], port, nil }
	c := runner.checkResponsesValidation(context.Background())
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

// --- Codex CLI checks ---

func TestCheckCodexBinaryMissing(t *testing.T) {
	runner, _ := setupRunner(t)
	restore := codex.SetExecForTest(
		func(string) (string, error) { return "", errors.New("not on path") },
		func(string, ...string) ([]byte, error) { return nil, errors.New("not invoked") },
	)
	defer restore()
	c := runner.checkCodexBinary()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckCodexBinaryTooOldIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	restore := codex.SetExecForTest(
		func(string) (string, error) { return "/usr/bin/codex", nil },
		func(string, ...string) ([]byte, error) { return []byte("codex 0.10.0\n"), nil },
	)
	defer restore()
	c := runner.checkCodexBinary()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning; message = %s", c.Status, c.Message)
	}
}

func TestCheckCodexBinaryOK(t *testing.T) {
	runner, _ := setupRunner(t)
	restore := codex.SetExecForTest(
		func(string) (string, error) { return "/usr/bin/codex", nil },
		func(string, ...string) ([]byte, error) { return []byte("codex 0.81.0\n"), nil },
	)
	defer restore()
	c := runner.checkCodexBinary()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckCLIProfileMissingIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkCLIProfile()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning; message = %s", c.Status, c.Message)
	}
}

func TestCheckCLIProfileIncoherentIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	writeFile(t, runner.Paths.CodexProfileFile, "# empty\n")
	c := runner.checkCLIProfile()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckCLIProfileOK(t *testing.T) {
	runner, _ := setupRunner(t)
	body := strings.Join([]string{
		`openai_base_url = "http://127.0.0.1:3456/v1/"`,
		`model_provider = "ocgo-launch"`,
		`model_catalog_json = "/tmp/ocgo-models.json"`,
		``,
		`[model_providers.ocgo-launch]`,
		`name = "OpenCode Go"`,
		`base_url = "http://127.0.0.1:3456/v1/"`,
		`wire_api = "responses"`,
		``,
	}, "\n")
	writeFile(t, runner.Paths.CodexProfileFile, body)
	c := runner.checkCLIProfile()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckCLIModelCatalogMissing(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkCLIModelCatalog()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
}

func TestCheckCLIModelCatalogInvalidJSON(t *testing.T) {
	runner, _ := setupRunner(t)
	writeFile(t, runner.Paths.CodexCatalogFile, "not json")
	c := runner.checkCLIModelCatalog()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error", c.Status)
	}
}

func TestCheckCLIModelCatalogOK(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.CodexCatalogFile, []map[string]any{
		{"id": "minimax-m3"}, {"id": "kimi-k2.6"},
	})
	c := runner.checkCLIModelCatalog()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckCLIModelCatalogMissingEffectiveModel(t *testing.T) {
	runner, _ := setupRunner(t)
	known := models.KnownIDs()
	if len(known) == 0 {
		t.Skip("no known models")
	}
	writeJSONFile(t, runner.Paths.ModelSelectionFile, models.ModelSelection{
		Version: 1, DefaultModel: known[0], UpdatedAt: nowUTC(),
	})
	writeJSONFile(t, runner.Paths.CodexCatalogFile, []map[string]any{
		{"id": "other-model"},
	})
	c := runner.checkCLIModelCatalog()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning; message = %s", c.Status, c.Message)
	}
}

// --- Codex Desktop checks ---

func TestCheckDesktopConfigMissingIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkDesktopConfig()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
}

func TestCheckDesktopConfigChatGPTDetectedIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	writeFile(t, runner.Paths.CodexConfigFile, `model_provider = "chatgpt"`+"\n")
	c := runner.checkDesktopConfig()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
	if !strings.Contains(c.Message, "ChatGPT") {
		t.Errorf("message = %q, want mentions ChatGPT", c.Message)
	}
}

func TestCheckDesktopConfigOK(t *testing.T) {
	runner, _ := setupRunner(t)
	body := strings.Join([]string{
		`model_provider = "ocgo-desktop"`,
		``,
		`[model_providers.ocgo-desktop]`,
		`name = "OpenCode Go"`,
		`base_url = "http://127.0.0.1:3456/v1/"`,
		`env_key = "OPENAI_API_KEY"`,
		`wire_api = "responses"`,
		``,
	}, "\n")
	writeFile(t, runner.Paths.CodexConfigFile, body)
	c := runner.checkDesktopConfig()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckDesktopStateMissingIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkDesktopState()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
}

func TestCheckDesktopStateInvalidJSONIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	writeFile(t, runner.Paths.DesktopStateFile, "{not json")
	c := runner.checkDesktopState()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error", c.Status)
	}
}

func TestCheckDesktopStateOpenCode(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: codex.DesktopModeOpenCode,
		UpdatedAt: nowUTC(), BaseURL: "http://127.0.0.1:3456/v1/", Model: "minimax-m3",
	})
	c := runner.checkDesktopState()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckDesktopStateChatGPT(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: codex.DesktopModeChatGPT,
		UpdatedAt: nowUTC(),
	})
	c := runner.checkDesktopState()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckDesktopStateUnknownMode(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: "weird", UpdatedAt: nowUTC(),
	})
	c := runner.checkDesktopState()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckDesktopBackupReferencedMissingIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: codex.DesktopModeOpenCode,
		UpdatedAt: nowUTC(), BackupFile: "/no/such/file.toml",
	})
	c := runner.checkDesktopBackup()
	if c.Status != StatusError {
		t.Errorf("status = %s, want error; message = %s", c.Status, c.Message)
	}
}

func TestCheckDesktopBackupPresentOK(t *testing.T) {
	runner, dir := setupRunner(t)
	backupPath := filepath.Join(dir, "backup.toml")
	writeFile(t, backupPath, "old config")
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: codex.DesktopModeOpenCode,
		UpdatedAt: nowUTC(), BackupFile: backupPath,
	})
	c := runner.checkDesktopBackup()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

func TestCheckDesktopBackupNoStateFileIsWarning(t *testing.T) {
	runner, _ := setupRunner(t)
	c := runner.checkDesktopBackup()
	if c.Status != StatusWarning {
		t.Errorf("status = %s, want warning", c.Status)
	}
}

func TestCheckDesktopBackupChatGPTNoBackupIsOK(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: codex.DesktopModeChatGPT,
		UpdatedAt: nowUTC(),
	})
	c := runner.checkDesktopBackup()
	if c.Status != StatusOK {
		t.Errorf("status = %s, want ok; message = %s", c.Status, c.Message)
	}
}

// --- End-to-end: RunCodex with various states ---

func TestRunCodexEmptyHomeMostlyWarnings(t *testing.T) {
	runner, _ := setupRunner(t)
	rep := runner.RunCodex(context.Background(), ModeAll)
	if rep.Status == StatusOK {
		t.Errorf("expected non-OK with empty home; got ok; checks = %v", rep.Checks)
	}
}

func TestRunCodexDesktopOpenCodeRequiresHealthyProxy(t *testing.T) {
	runner, _ := setupRunner(t)
	// No config, no state, no daemon: just an empty home.
	// With no state file, we cannot tell that the user is
	// in OpenCode mode, so the doctor must not raise the
	// dedicated "daemon.required_for_desktop" error. The
	// proxy-down checks will still surface as errors
	// (because that is the truth), but the status rollup
	// should not include the OpenCode-specific check.
	rep := runner.RunCodex(context.Background(), ModeDesktop)
	for _, c := range rep.Checks {
		if c.ID == "daemon.required_for_desktop" {
			t.Fatalf("required_for_desktop check should not exist when state is absent; got %+v", rep.Checks)
		}
	}
}

func TestRunCodexDesktopOpenCodeProxyDownIsError(t *testing.T) {
	runner, _ := setupRunner(t)
	writeJSONFile(t, runner.Paths.DesktopStateFile, codex.DesktopState{
		Version: codex.DesktopStateVersion, Mode: codex.DesktopModeOpenCode,
		UpdatedAt: nowUTC(),
	})
	rep := runner.RunCodex(context.Background(), ModeDesktop)
	// No proxy is running (HostPort points to 127.0.0.1:1).
	if rep.Status != StatusError {
		t.Errorf("status = %s, want error; checks = %v", rep.Status, rep.Checks)
	}
}

func TestRunCodexModeOnlyCLI(t *testing.T) {
	runner, _ := setupRunner(t)
	rep := runner.RunCodex(context.Background(), ModeCLI)
	// The CLI mode should include only core checks and codex
	// CLI checks. It must NOT include daemon.*, proxy.*, or
	// codex.desktop.* checks.
	for _, c := range rep.Checks {
		if strings.HasPrefix(c.ID, "daemon.") || strings.HasPrefix(c.ID, "proxy.") || strings.HasPrefix(c.ID, "codex.desktop.") {
			t.Errorf("CLI mode should not include check %q", c.ID)
		}
	}
	// Verify that core.* and codex.cli.* checks ARE present.
	var hasCoreConfig, hasCoreModel, hasCoreCatalog bool
	var hasCliBinary, hasCliProfile, hasCliCatalog bool
	for _, c := range rep.Checks {
		switch c.ID {
		case "core.config":
			hasCoreConfig = true
		case "core.model":
			hasCoreModel = true
		case "core.catalog":
			hasCoreCatalog = true
		case "codex.cli.binary":
			hasCliBinary = true
		case "codex.cli.profile":
			hasCliProfile = true
		case "codex.cli.catalog":
			hasCliCatalog = true
		}
	}
	if !hasCoreConfig || !hasCoreModel || !hasCoreCatalog {
		t.Errorf("CLI mode missing core checks (config=%v, model=%v, catalog=%v)", hasCoreConfig, hasCoreModel, hasCoreCatalog)
	}
	if !hasCliBinary || !hasCliProfile || !hasCliCatalog {
		t.Errorf("CLI mode missing codex CLI checks (binary=%v, profile=%v, catalog=%v)", hasCliBinary, hasCliProfile, hasCliCatalog)
	}
}

func TestRunCodexModeOnlyDesktop(t *testing.T) {
	runner, _ := setupRunner(t)
	rep := runner.RunCodex(context.Background(), ModeDesktop)
	for _, c := range rep.Checks {
		if strings.HasPrefix(c.ID, "codex.cli.") {
			t.Errorf("Desktop mode should not include CLI check %q", c.ID)
		}
	}
}

func TestRunCodexAllIncludesBoth(t *testing.T) {
	runner, _ := setupRunner(t)
	rep := runner.RunCodex(context.Background(), ModeAll)
	var hasCLI, hasDesktop bool
	for _, c := range rep.Checks {
		if strings.HasPrefix(c.ID, "codex.cli.") {
			hasCLI = true
		}
		if strings.HasPrefix(c.ID, "codex.desktop.") {
			hasDesktop = true
		}
	}
	if !hasCLI {
		t.Errorf("All mode missing CLI checks")
	}
	if !hasDesktop {
		t.Errorf("All mode missing Desktop checks")
	}
}

// --- helpers: test hooks for codex package vars ---

// The codex package's exec hooks are overridden via
// codex.SetExecForTest (defined in internal/codex/testing.go).
// The doctor tests use it to simulate the codex binary being
// present, missing, or too old without touching the real
// filesystem.
