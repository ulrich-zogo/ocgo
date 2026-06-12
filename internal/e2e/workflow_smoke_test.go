package e2e

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ocgo/internal/codex"
	"ocgo/internal/config"
	"ocgo/internal/daemon"
)

func TestE2EFreshConfigDiagnosticsWorkflow(t *testing.T) {
	dir := newTempHome(t)

	out := runOCGOSuccess(t, "version", "--json")
	assertJSONValid(t, out)

	out = runOCGOSuccess(t, "config", "paths", "--json")
	assertJSONValid(t, out)

	before := listFiles(t, dir)
	out = runOCGOSuccess(t, "config", "inspect", "--json")
	assertJSONValid(t, out)
	after := listFiles(t, dir)
	if len(before) != len(after) {
		t.Errorf("config inspect created files: before=%v after=%v", before, after)
	}

	out, _, err := runOCGO(t, "doctor", "--json")
	assertJSONValid(t, out)
	_ = err

	zipPath := filepath.Join(dir, "support.zip")
	out = runOCGOSuccess(t, "support", "bundle", "--output", zipPath, "--force", "--json")
	assertJSONValid(t, out)
	assertFileExists(t, zipPath)

	var result struct {
		Path         string   `json:"path"`
		Files        []string `json:"files"`
		Redacted     bool     `json:"redacted"`
		LogsIncluded bool     `json:"logs_included"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal support bundle result: %v\noutput: %s", err, out)
	}
	if result.Path == "" {
		t.Fatal("support bundle path is empty")
	}
	if len(result.Files) == 0 {
		t.Fatal("support bundle files is empty")
	}
	if !result.Redacted {
		t.Fatal("support bundle not redacted")
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	entries := make(map[string]bool)
	for _, f := range r.File {
		entries[f.Name] = true
	}
	for _, want := range []string{"manifest.json", "version.json", "config-paths.json", "config-inspect.json"} {
		if !entries[want] {
			t.Errorf("support bundle missing entry: %s", want)
		}
	}
}

func TestE2EModelSelectionWorkflow(t *testing.T) {
	dir := newTempHome(t)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"), `{"api_key":"test","host":"127.0.0.1","port":3456}`)

	out := runOCGOSuccess(t, "opencode", "model", "current")
	if !strings.Contains(out, "minimax-m3") {
		t.Logf("model current output: %s", out)
	}

	runOCGOSuccess(t, "opencode", "model", "set-default", "minimax-m3")
	out = runOCGOSuccess(t, "opencode", "model", "current")
	if !strings.Contains(out, "minimax-m3") {
		t.Errorf("current model should be minimax-m3 after set-default, got: %s", out)
	}

	runOCGOSuccess(t, "opencode", "model", "set-default", "kimi-k2.6")
	out = runOCGOSuccess(t, "opencode", "model", "current")
	if !strings.Contains(out, "kimi-k2.6") {
		t.Errorf("current model should be kimi-k2.6 after set-default, got: %s", out)
	}

	_, _, err := runOCGO(t, "opencode", "model", "set-default", "invalid-model")
	if err == nil {
		t.Fatal("expected error for invalid model")
	}

	out = runOCGOSuccess(t, "opencode", "model", "current")
	if strings.Contains(out, "invalid-model") {
		t.Errorf("invalid model should not be persisted, got: %s", out)
	}

	selFile := filepath.Join(config.ModelSelectionFile())
	if _, err := os.Stat(selFile); err != nil {
		t.Errorf("model-selection.json should exist after valid set-default: %v", err)
	}

	runOCGOSuccess(t, "opencode", "model", "set-default", "minimax-m3")
}

func TestE2EDaemonWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping daemon workflow in short mode")
	}

	dir := newTempHome(t)
	port := freePort(t)

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"test-key","host":"127.0.0.1","port":`+itoa(port)+`}`)

	stateFile := filepath.Join(ocgoDir, "daemon-state.json")
	t.Setenv("OCGO_DAEMON_STATE_FILE", stateFile)

	healthy := atomic.Bool{}
	startCalls := atomic.Int32{}
	restore := daemon.SetRuntimeForTest(daemon.Runtime{
		Healthy: func(base string) bool { return healthy.Load() },
		ReadPID: func() (int, error) { return 0, nil },
		FindListenerPID: func(port int) (int, error) { return 0, nil },
		KillPID: func(pid int) error { return nil },
		StartBackground: func() error { startCalls.Add(1); healthy.Store(true); return nil },
		WaitHealthy: func(base string, timeout time.Duration) error { return nil },
	})
	t.Cleanup(restore)

	t.Cleanup(func() {
		healthy.Store(false)
		runOCGO(t, "daemon", "stop")
	})

	out := runOCGOSuccess(t, "daemon", "status", "--json")
	assertJSONValid(t, out)
	if strings.Contains(out, `"health": "ok"`) {
		t.Log("daemon status shows healthy before start")
	}

	runOCGOSuccess(t, "daemon", "start")

	if startCalls.Load() != 1 {
		t.Errorf("expected 1 start call, got %d", startCalls.Load())
	}

	out = runOCGOSuccess(t, "daemon", "status", "--json")
	assertJSONValid(t, out)
	if !strings.Contains(out, `"health": "ok"`) {
		t.Errorf("daemon status should be healthy after start, got: %s", out)
	}

	_, _, err := runOCGO(t, "daemon", "start")
	if err != nil {
		t.Logf("double start returned error (acceptable): %v", err)
	}
	if startCalls.Load() != 1 {
		t.Errorf("expected start calls to still be 1 after double start, got %d", startCalls.Load())
	}

	runOCGOSuccess(t, "daemon", "restart")
	out = runOCGOSuccess(t, "daemon", "status", "--json")
	assertJSONValid(t, out)

	runOCGOSuccess(t, "daemon", "stop")
	healthy.Store(false)

	out = runOCGOSuccess(t, "daemon", "status", "--json")
	assertJSONValid(t, out)

	out, _, _ = runOCGO(t, "daemon", "stop")
	t.Logf("double stop output: %s", out)
}

func TestE2EStalePIDAndLockWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stale PID workflow in short mode")
	}

	dir := newTempHome(t)
	port := freePort(t)

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"test-key","host":"127.0.0.1","port":`+itoa(port)+`}`)

	writeFile(t, filepath.Join(ocgoDir, "ocgo.pid"), "99999999\n")
	lock := `{"pid":99999999,"created_at":"2024-01-01T00:00:00Z"}` + "\n"
	writeFile(t, filepath.Join(ocgoDir, "daemon.lock"), lock)

	stateFile := filepath.Join(ocgoDir, "daemon-state.json")
	t.Setenv("OCGO_DAEMON_STATE_FILE", stateFile)

	healthy := atomic.Bool{}
	restore := daemon.SetRuntimeForTest(daemon.Runtime{
		Healthy: func(base string) bool { return healthy.Load() },
		ReadPID: func() (int, error) { return 0, nil },
		FindListenerPID: func(port int) (int, error) { return 0, nil },
		KillPID:         func(pid int) error { return nil },
		StartBackground: func() error { healthy.Store(true); return nil },
		WaitHealthy:     func(base string, timeout time.Duration) error { return nil },
	})
	t.Cleanup(restore)

	t.Cleanup(func() {
		healthy.Store(false)
		runOCGO(t, "daemon", "stop")
	})

	runOCGOSuccess(t, "daemon", "start")

	out := runOCGOSuccess(t, "daemon", "status", "--json")
	assertJSONValid(t, out)
}

func TestE2ECodexCLIConfigWorkflow(t *testing.T) {
	dir := newTempHome(t)
	port := freePort(t)

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"test-key","host":"127.0.0.1","port":`+itoa(port)+`}`)
	t.Setenv("OCGO_API_KEY", "test-key")

	runOCGOSuccess(t, "opencode", "model", "set-default", "minimax-m3")

	runOCGOSuccess(t, "launch", "codex", "--config")

	profileFile := filepath.Join(dir, ".codex", "ocgo-launch.config.toml")
	catalogFile := filepath.Join(dir, ".codex", "ocgo-models.json")
	assertFileExists(t, profileFile)
	assertFileExists(t, catalogFile)

	profile, err := os.ReadFile(profileFile)
	if err != nil {
		t.Fatal(err)
	}
	profileStr := string(profile)
	if !strings.Contains(profileStr, `"ocgo-launch"`) && !strings.Contains(profileStr, `ocgo-launch`) {
		t.Errorf("profile missing ocgo-launch reference:\n%s", profileStr)
	}
	if !strings.Contains(profileStr, itoa(port)) {
		t.Errorf("profile missing port %d:\n%s", port, profileStr)
	}

	catalog, err := os.ReadFile(catalogFile)
	if err != nil {
		t.Fatal(err)
	}
	var catalogData struct {
		Models []map[string]interface{} `json:"models"`
	}
	if err := json.Unmarshal(catalog, &catalogData); err != nil {
		t.Fatalf("catalog JSON invalid: %v\n%s", err, string(catalog))
	}
	if len(catalogData.Models) == 0 {
		t.Error("catalog has no models")
	}
}

func TestE2ECodexDesktopSwitchRestoreWorkflow(t *testing.T) {
	dir := newTempHome(t)
	port := freePort(t)

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"test-key","host":"127.0.0.1","port":`+itoa(port)+`}`)
	t.Setenv("OCGO_API_KEY", "test-key")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(ocgoDir, "codex-desktop-state.json"))

	codexDir := filepath.Join(dir, ".codex")
	os.MkdirAll(codexDir, 0755)
	writeFile(t, filepath.Join(codexDir, "config.toml"),
		`model = "gpt-5"`+"\n"+`model_provider = "openai"`+"\n")

	mgr := codex.Manager{Paths: codex.Paths{
		ConfigFile:       filepath.Join(codexDir, "config.toml"),
		ProfileFile:      filepath.Join(codexDir, "ocgo-launch.config.toml"),
		ModelCatalogFile: filepath.Join(codexDir, "ocgo-models.json"),
		DesktopConfigFile: filepath.Join(codexDir, "config.toml"),
		BackupDir:        filepath.Join(ocgoDir, "codex-backups"),
	}}

	originalContent, _ := os.ReadFile(filepath.Join(codexDir, "config.toml"))

	status, err := mgr.DesktopStatus()
	if err != nil {
		t.Fatal(err)
	}
	_ = status

	base := "http://127.0.0.1:" + itoa(port)
	st, err := mgr.EnableDesktopOpenCode(base, "minimax-m3")
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode != codex.DesktopModeOpenCode {
		t.Errorf("mode = %s, want opencode", st.Mode)
	}
	if st.Model != "minimax-m3" {
		t.Errorf("model = %s, want minimax-m3", st.Model)
	}
	if st.BaseURL != base+"/v1/" {
		t.Errorf("base_url = %s, want %s/v1/", st.BaseURL, base)
	}

	stateFile := filepath.Join(ocgoDir, "codex-desktop-state.json")
	assertFileExists(t, stateFile)

	desktopState, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	desktopStr := string(desktopState)
	if strings.Contains(desktopStr, "gpt-5") {
		t.Errorf("desktop config still contains original model after enable opencode:\n%s", desktopStr)
	}
	if !strings.Contains(desktopStr, "minimax-m3") && !strings.Contains(desktopStr, base) {
		t.Errorf("desktop config missing OCGO reference after enable opencode:\n%s", desktopStr)
	}

	backupDir := filepath.Join(ocgoDir, "codex-backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		t.Errorf("backup not created: %v, entries: %d", err, len(entries))
	}

	if _, err := mgr.EnableDesktopChatGPT(); err != nil {
		t.Fatal(err)
	}

	restored, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(originalContent) {
		t.Errorf("restored content differs from original\n  got:  %q\n  want: %q", restored, originalContent)
	}
}

func TestE2ESupportBundleAfterWorkflow(t *testing.T) {
	dir := newTempHome(t)
	secret := "SUPER_SECRET_TEST_KEY"

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"`+secret+`","host":"127.0.0.1","port":3456}`)
	t.Setenv("OCGO_API_KEY", secret)

	writeFile(t, filepath.Join(ocgoDir, "model-selection.json"),
		`{"model":"minimax-m3"}`+"\n")
	writeFile(t, filepath.Join(ocgoDir, "daemon-state.json"),
		`{"version":1,"pid":12345,"host":"127.0.0.1","port":3456}`+"\n")
	writeFile(t, filepath.Join(ocgoDir, "ocgo.log"),
		"Bearer sk-"+secret+"-token\nINFO request processed\n")

	codexDir := filepath.Join(dir, ".codex")
	os.MkdirAll(codexDir, 0755)
	writeFile(t, filepath.Join(codexDir, "config.toml"),
		`api_key = "`+secret+`"`+"\n")
	writeFile(t, filepath.Join(ocgoDir, "codex-desktop-state.json"),
		`{"provider":"opencode"}`+"\n")

	zipPath := filepath.Join(dir, "support.zip")
	out := runOCGOSuccess(t, "support", "bundle", "--output", zipPath, "--force", "--json")
	assertJSONValid(t, out)

	assertFileExists(t, zipPath)
	assertNoSecret(t, out, secret)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	foundRedacted := false
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b := new(bytes.Buffer)
		b.ReadFrom(rc)
		rc.Close()
		content := b.String()

		assertNoSecret(t, content, secret)

		if strings.Contains(content, "[REDACTED]") {
			foundRedacted = true
		}

		if f.Name == "manifest.json" {
			var m struct {
				CreatedAt    string `json:"created_at"`
				OCGOVersion string `json:"ocgo_version"`
				Redacted    bool   `json:"redacted"`
			}
			if err := json.Unmarshal(b.Bytes(), &m); err == nil {
				if !m.Redacted {
					t.Error("manifest redacted is false")
				}
			}
		}
	}
	if !foundRedacted {
		t.Error("no [REDACTED] found in any zip entry")
	}
}

func TestE2EConfigLifecycleDryRunWorkflow(t *testing.T) {
	dir := newTempHome(t)

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"test","host":"127.0.0.1","port":3456}`)
	t.Setenv("OCGO_API_KEY", "test")

	writeFile(t, filepath.Join(ocgoDir, "model-selection.json"),
		`{"model":"minimax-m3"}`+"\n")

	out := runOCGOSuccess(t, "config", "backup")
	if !strings.Contains(out, "Backup created") {
		t.Errorf("backup output missing success, got: %s", out)
	}
	assertFileExists(t, filepath.Join(ocgoDir, "backups"))

	backups, _ := os.ReadDir(filepath.Join(ocgoDir, "backups"))
	if len(backups) == 0 {
		t.Fatal("no backup files found")
	}

	codexDir := filepath.Join(dir, ".codex")
	os.MkdirAll(codexDir, 0755)
	writeFile(t, filepath.Join(codexDir, "config.toml"),
		`model = "gpt-5"`+"\n")

	out = runOCGOSuccess(t, "config", "backup", "--include-codex-config")
	if !strings.Contains(out, "Backup created") {
		t.Errorf("backup with codex output missing success, got: %s", out)
	}

	out = runOCGOSuccess(t, "config", "reset", "--scope", "ocgo", "--dry-run")
	if !strings.Contains(out, "Files to remove") {
		t.Errorf("dry-run ocgo missing 'Files to remove', got: %s", out)
	}

	out = runOCGOSuccess(t, "config", "reset", "--scope", "cache", "--dry-run")

	out = runOCGOSuccess(t, "config", "reset", "--scope", "codex-cli", "--dry-run")
	if !strings.Contains(out, "Nothing to reset") && !strings.Contains(out, "Files to remove") {
		t.Logf("codex-cli dry-run output: %s", out)
	}

	out = runOCGOSuccess(t, "config", "reset", "--scope", "all", "--dry-run")
	if !strings.Contains(out, "Files to remove") {
		t.Errorf("dry-run all missing 'Files to remove', got: %s", out)
	}

	assertFileExists(t, filepath.Join(ocgoDir, "config.json"))
	assertFileExists(t, filepath.Join(ocgoDir, "model-selection.json"))

	out = runOCGOSuccess(t, "config", "inspect", "--json")
	assertJSONValid(t, out)

	var parsed struct {
		Core struct {
			ConfigFile string `json:"config_file"`
		} `json:"core"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("inspect JSON invalid after dry-run: %v\n%s", err, out)
	}
	if parsed.Core.ConfigFile == "" {
		t.Error("inspect should show config after dry-run")
	}
}
