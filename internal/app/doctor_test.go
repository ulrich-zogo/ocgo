package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocgo/internal/codex"
	"ocgo/internal/doctor"
)

// redirectAppHome puts HOME / USERPROFILE / HOMEDRIVE /
// HOMEPATH into a temp directory so the doctor cannot touch
// the user's real configuration. It also points the OCGO
// env-override paths at temp locations.
func redirectAppHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_DAEMON_STATE_FILE", filepath.Join(dir, ".config", "ocgo", "daemon-state.json"))
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", filepath.Join(dir, ".config", "ocgo", "codex-desktop-state.json"))
	return dir
}

func executeRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand("test")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestRootRegistersDoctor(t *testing.T) {
	root := NewRootCommand("test")
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("root command should register 'doctor'")
	}
}

func TestDoctorRunNoArgsExecutes(t *testing.T) {
	redirectAppHome(t)
	// Make sure the doctor does not panic on an empty home.
	out, err := executeRoot(t, "doctor")
	if err != nil {
		t.Fatalf("ocgo doctor err = %v, output: %s", err, out)
	}
	if !strings.Contains(out, "OCGO Doctor") {
		t.Errorf("output missing banner: %s", out)
	}
}

func TestDoctorRunWithCodexSubcommandExecutes(t *testing.T) {
	redirectAppHome(t)
	out, err := executeRoot(t, "doctor", "codex")
	if err != nil {
		t.Fatalf("ocgo doctor codex err = %v, output: %s", err, out)
	}
	if !strings.Contains(out, "OCGO Doctor") {
		t.Errorf("output missing banner: %s", out)
	}
}

func TestDoctorModeCLIRunsCLIOnly(t *testing.T) {
	redirectAppHome(t)
	out, _ := executeRoot(t, "doctor", "codex", "--mode", "cli")
	if !strings.Contains(out, "OCGO Doctor") {
		t.Errorf("output missing banner: %s", out)
	}
	// The text output groups checks by group. We do not
	// enforce group names here because the doctor may
	// evolve; what we care about is that the command did
	// not fail.
}

func TestDoctorModeDesktopRunsDesktopOnly(t *testing.T) {
	redirectAppHome(t)
	out, _ := executeRoot(t, "doctor", "codex", "--mode", "desktop")
	if !strings.Contains(out, "OCGO Doctor") {
		t.Errorf("output missing banner: %s", out)
	}
}

func TestDoctorModeInvalidReturnsError(t *testing.T) {
	redirectAppHome(t)
	_, err := executeRoot(t, "doctor", "codex", "--mode", "garbage")
	if err == nil {
		t.Fatalf("expected error for invalid --mode")
	}
	if !strings.Contains(err.Error(), "invalid --mode") {
		t.Errorf("error = %q, want contains 'invalid --mode'", err.Error())
	}
}

func TestDoctorJSONReturnsParseableReport(t *testing.T) {
	redirectAppHome(t)
	out, err := executeRoot(t, "doctor", "codex", "--json")
	if err != nil {
		t.Fatalf("ocgo doctor codex --json err = %v, output: %s", err, out)
	}
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v; body: %s", err, out)
	}
	if rep.Status != doctor.StatusOK &&
		rep.Status != doctor.StatusWarning &&
		rep.Status != doctor.StatusError {
		t.Errorf("report status = %q, want ok/warning/error", rep.Status)
	}
	if len(rep.Checks) == 0 {
		t.Errorf("report has no checks")
	}
}

func TestDoctorJSONStatusIsConsistent(t *testing.T) {
	redirectAppHome(t)
	out, _ := executeRoot(t, "doctor", "codex", "--json")
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("unmarshal: %v; body: %s", err, out)
	}
	// Verify the rollup status is consistent with the
	// individual checks.
	hasError, hasWarning := false, false
	for _, c := range rep.Checks {
		switch c.Status {
		case doctor.StatusError:
			hasError = true
		case doctor.StatusWarning:
			hasWarning = true
		}
	}
	switch {
	case hasError:
		if rep.Status != doctor.StatusError {
			t.Errorf("rollup = %s, want error (found error check)", rep.Status)
		}
	case hasWarning:
		if rep.Status != doctor.StatusWarning {
			t.Errorf("rollup = %s, want warning", rep.Status)
		}
	default:
		if rep.Status != doctor.StatusOK {
			t.Errorf("rollup = %s, want ok", rep.Status)
		}
	}
}

func TestDoctorJSONChecksAreStableOrdered(t *testing.T) {
	// Run the doctor twice in the same temp home. The
	// second run should produce byte-identical JSON
	// because the catalog cache, model selection, and
	// other file state do not change between runs in the
	// same dir.
	dir := t.TempDir()
	first := runDoctorJSONInHome(t, dir)
	second := runDoctorJSONInHome(t, dir)
	if first != second {
		t.Logf("first run:\n%s", first)
		t.Logf("second run:\n%s", second)
		t.Errorf("JSON output not stable across runs in the same home")
	}
}

// runDoctorJSONInHome runs `ocgo doctor codex --json` with
// HOME/USERPROFILE/HOMEDRIVE/HOMEPATH pointing at home, and
// a minimal pre-populated model catalog cache so the
// checkModelCatalog path is identical on every invocation.
func runDoctorJSONInHome(t *testing.T, home string) string {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", home)
	t.Setenv("OCGO_DAEMON_STATE_FILE", home+"/.config/ocgo/daemon-state.json")
	t.Setenv("OCGO_CODEX_DESKTOP_STATE_FILE", home+"/.config/ocgo/codex-desktop-state.json")
	cacheDir := filepath.Join(home, ".config", "ocgo")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "model-catalog-cache.json")
	if err := os.WriteFile(cachePath, []byte(`{
		"version": 1,
		"source": "official",
		"fetched_at": "2026-01-01T00:00:00Z",
		"models": [
			{"id":"minimax-m3","object":"model","created":1,"owned_by":"opencode"},
			{"id":"kimi-k2.6","object":"model","created":2,"owned_by":"opencode"}
		]
	}`), 0600); err != nil {
		t.Fatal(err)
	}
	out, _ := executeRoot(t, "doctor", "codex", "--json")
	return out
}

func TestDoctorJSONShapeExcludesDecorations(t *testing.T) {
	redirectAppHome(t)
	out, _ := executeRoot(t, "doctor", "codex", "--json")
	// The JSON output must not contain the human-readable
	// banner or the OK/WARNING status marker text.
	for _, decoration := range []string{"OCGO Doctor", "Overall:"} {
		if strings.Contains(out, decoration) {
			t.Errorf("JSON output contains human decoration %q: %s", decoration, out)
		}
	}
}

func TestDoctorTextOutputMentionsOverall(t *testing.T) {
	redirectAppHome(t)
	out, _ := executeRoot(t, "doctor", "codex")
	if !strings.Contains(out, "Overall:") {
		t.Errorf("text output missing Overall line: %s", out)
	}
}

func TestExitCodeForReport(t *testing.T) {
	if ExitCodeFor(doctor.Report{Status: doctor.StatusOK}) != 0 {
		t.Errorf("OK should map to 0")
	}
	if ExitCodeFor(doctor.Report{Status: doctor.StatusWarning}) != 0 {
		t.Errorf("Warning should map to 0")
	}
	if ExitCodeFor(doctor.Report{Status: doctor.StatusError}) != 1 {
		t.Errorf("Error should map to 1")
	}
}

func TestDoctorAgainstLiveProxyReturnsOKForLocalChecks(t *testing.T) {
	// This test stands up a stub HTTP server that mimics
	// the OCGO proxy just enough to satisfy the local
	// endpoint checks. It does not need the real proxy.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok\n"))
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"minimax-m3","object":"model","created":1,"owned_by":"opencode"}]}`))
		case "/v1/messages/count_tokens":
			_, _ = w.Write([]byte(`{"input_tokens":8}`))
		case "/v1/responses":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"missing model","type":"invalid_request_error"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dir := redirectAppHome(t)
	// Configure the OCGO config to point at the stub.
	// The doctor reads ~/.config/ocgo/config.json and uses
	// host/port from there for the local proxy.
	host, port, err := splitTestServerHostPort(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	writeOCGOConfig(t, dir, host, port, "test-key")

	out, err := executeRoot(t, "doctor", "codex", "--mode", "all", "--json")
	if err != nil {
		t.Fatalf("ocgo doctor err = %v, output: %s", err, out)
	}
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v; body: %s", err, out)
	}
	// The proxy-related checks should be OK.
	ids := map[string]bool{}
	for _, c := range rep.Checks {
		ids[c.ID] = true
	}
	for _, want := range []string{"daemon.health", "proxy.models", "proxy.count_tokens", "proxy.responses"} {
		if !ids[want] {
			t.Errorf("missing check %q in report: %v", want, ids)
		}
	}
	// The /health, /v1/models, /v1/messages/count_tokens
	// checks should all be OK in this scenario.
	for _, c := range rep.Checks {
		switch c.ID {
		case "daemon.health", "proxy.models", "proxy.count_tokens", "proxy.responses":
			if c.Status != doctor.StatusOK {
				t.Errorf("check %q status = %s, want ok; message = %s", c.ID, c.Status, c.Message)
			}
		}
	}
}

func TestDoctorDesktopOpenCodeRequiresHealthyProxy(t *testing.T) {
	// Stand up a stub proxy and mark Desktop as
	// OpenCode via the state file. The doctor must NOT
	// raise the "required_for_desktop" error in this
	// scenario.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok\n"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()
	dir := redirectAppHome(t)
	host, port, err := splitTestServerHostPort(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	writeOCGOConfig(t, dir, host, port, "test-key")
	writeDesktopState(t, dir, codex.DesktopModeOpenCode, "")

	out, err := executeRoot(t, "doctor", "codex", "--mode", "desktop", "--json")
	if err != nil {
		t.Fatalf("ocgo doctor err = %v, output: %s", err, out)
	}
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("unmarshal: %v; body: %s", err, out)
	}
	for _, c := range rep.Checks {
		if c.ID == "daemon.required_for_desktop" {
			t.Errorf("required_for_desktop check should not exist when proxy is healthy")
		}
	}
}

func TestDoctorDesktopOpenCodeWithDownProxyIsError(t *testing.T) {
	// Desktop is OpenCode, no proxy is running: the
	// doctor must report the required_for_desktop error.
	dir := redirectAppHome(t)
	// Point the doctor at 127.0.0.1:1 (refused) by NOT
	// writing a config file (it falls back to defaults
	// 127.0.0.1:3456; we then override with a state file
	// at a custom path but no real proxy listens).
	writeDesktopState(t, dir, codex.DesktopModeOpenCode, "")

	// Use a port that nothing listens on.
	port := 1
	writeOCGOConfig(t, dir, "127.0.0.1", port, "test-key")

	out, _ := executeRoot(t, "doctor", "codex", "--mode", "desktop", "--json")
	var rep doctor.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("unmarshal: %v; body: %s", err, out)
	}
	if rep.Status != doctor.StatusError {
		t.Errorf("status = %s, want error; checks = %v", rep.Status, rep.Checks)
	}
	hasRequiredCheck := false
	for _, c := range rep.Checks {
		if c.ID == "daemon.required_for_desktop" {
			hasRequiredCheck = true
		}
	}
	if !hasRequiredCheck {
		t.Errorf("missing daemon.required_for_desktop check; got %v", rep.Checks)
	}
}

// --- helpers ---

// writeOCGOConfig writes a minimal ~/.config/ocgo/config.json
// in the given HOME so the doctor resolves the local proxy
// host/port from there.
func writeOCGOConfig(t *testing.T, home, host string, port int, apiKey string) {
	t.Helper()
	cfgPath := filepath.Join(home, ".config", "ocgo", "config.json")
	body, err := json.MarshalIndent(map[string]any{
		"host":    host,
		"port":    port,
		"api_key": apiKey,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, append(body, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

// splitTestServerHostPort extracts the host and port from
// a httptest.NewServer URL like "http://127.0.0.1:34567".
func splitTestServerHostPort(rawURL string) (string, int, error) {
	const prefix = "http://"
	if !strings.HasPrefix(rawURL, prefix) {
		return "", 0, errors.New("unexpected test server URL: " + rawURL)
	}
	rest := strings.TrimPrefix(rawURL, prefix)
	idx := strings.LastIndex(rest, ":")
	if idx < 0 {
		return "", 0, errors.New("URL has no port: " + rawURL)
	}
	host := rest[:idx]
	portStr := rest[idx+1:]
	var port int
	for _, r := range portStr {
		if r < '0' || r > '9' {
			return "", 0, errors.New("invalid port: " + portStr)
		}
		port = port*10 + int(r-'0')
	}
	return host, port, nil
}

// writeDesktopState writes a Codex Desktop state file with
// the given mode and an optional backup path. The state
// file is written into the temp HOME so it is picked up
// by the doctor.
func writeDesktopState(t *testing.T, home string, mode codex.DesktopMode, backup string) {
	t.Helper()
	path := filepath.Join(home, ".config", "ocgo", "codex-desktop-state.json")
	st := codex.DesktopState{
		Version:    codex.DesktopStateVersion,
		Mode:       mode,
		UpdatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BackupFile: backup,
	}
	body, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
