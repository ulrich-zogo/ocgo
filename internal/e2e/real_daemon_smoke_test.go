package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealDaemonProcessSmoke(t *testing.T) {
	if os.Getenv("OCGO_E2E_REAL_DAEMON") != "1" {
		t.Skip("set OCGO_E2E_REAL_DAEMON=1 to run the real daemon smoke test")
	}

	home := newTempHome(t)
	port := freePort(t)
	bin := buildOCGOBinary(t)

	t.Logf("built binary: %s", bin)
	t.Logf("temp home: %s", home)
	t.Logf("daemon port: %d", port)

	ocgoDir := filepath.Join(home, ".config", "ocgo")
	writeFile(t, filepath.Join(ocgoDir, "config.json"),
		`{"api_key":"test-key","host":"127.0.0.1","port":`+itoa(port)+`}`)

	pidPath := filepath.Join(ocgoDir, "ocgo.pid")
	statePath := filepath.Join(ocgoDir, "daemon-state.json")
	logPath := filepath.Join(ocgoDir, "ocgo.log")
	t.Setenv("OCGO_DAEMON_STATE_FILE", statePath)

	env := append(os.Environ(),
		"HOME="+home,
		"USERPROFILE="+home,
		"HOMEDRIVE=",
		"HOMEPATH="+home,
		"OCGO_API_KEY=test-key",
		"OCGO_DAEMON_STATE_FILE="+statePath,
	)

	t.Cleanup(func() {
		runBinary(t, bin, env, "daemon", "stop")
	})

	t.Run("status before start", func(t *testing.T) {
		out := runBinarySuccess(t, bin, env, "daemon", "status", "--json")
		assertJSONValid(t, out)
		if strings.Contains(out, `"health": "ok"`) {
			t.Log("daemon status already healthy before start (unexpected but acceptable)")
		}
	})

	t.Run("start", func(t *testing.T) {
		out := runBinarySuccess(t, bin, env, "daemon", "start")
		if !strings.Contains(out, "started") && !strings.Contains(out, "already") {
			t.Logf("start output: %s", out)
		}
		assertFileExists(t, pidPath)
		assertFileExists(t, statePath)
	})

	t.Run("health endpoint", func(t *testing.T) {
		waitHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), 200, 10*time.Second)
	})

	t.Run("status after start", func(t *testing.T) {
		out := runBinarySuccess(t, bin, env, "daemon", "status", "--json")
		assertJSONValid(t, out)
		if !strings.Contains(out, `"health": "ok"`) {
			t.Errorf("daemon status should be healthy after start, got: %s", out)
		}
		if !strings.Contains(out, itoa(port)) {
			t.Logf("status output (port check): %s", out)
		}
	})

	t.Run("/v1/models", func(t *testing.T) {
		body := getJSON(t, fmt.Sprintf("http://127.0.0.1:%d/v1/models", port))
		var parsed struct {
			Object string                   `json:"object"`
			Data   []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("/v1/models JSON unmarshal: %v\nbody: %s", err, string(body))
		}
		if parsed.Object != "list" {
			t.Errorf("/v1/models object = %q, want list", parsed.Object)
		}
		if len(parsed.Data) < 18 {
			t.Errorf("/v1/models has %d models, want at least 18", len(parsed.Data))
		}
		ids := make(map[string]bool)
		for _, m := range parsed.Data {
			id, _ := m["id"].(string)
			if id != "" {
				ids[id] = true
			}
		}
		for _, model := range expectedOfficialModels {
			if !ids[model] {
				t.Errorf("/v1/models missing %q", model)
			}
		}
	})

	t.Run("/v1/messages/count_tokens", func(t *testing.T) {
		body := postJSON(t, fmt.Sprintf("http://127.0.0.1:%d/v1/messages/count_tokens", port),
			`{"model":"minimax-m3","messages":[{"role":"user","content":"hello world"}]}`)
		var parsed struct {
			InputTokens int `json:"input_tokens"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("count_tokens JSON unmarshal: %v\nbody: %s", err, string(body))
		}
		if parsed.InputTokens <= 0 {
			t.Errorf("count_tokens input_tokens = %d, want > 0", parsed.InputTokens)
		}
	})

	t.Run("double start", func(t *testing.T) {
		pidBefore, err := os.ReadFile(pidPath)
		if err != nil {
			t.Fatalf("read pid before double start: %v", err)
		}
		stdout, _, err := runBinary(t, bin, env, "daemon", "start")
		if err != nil {
			t.Logf("double start error (acceptable): %v", err)
		}
		t.Logf("double start stdout: %s", stdout)
		pidAfter, err := os.ReadFile(pidPath)
		if err != nil {
			t.Fatalf("read pid after double start: %v", err)
		}
		if strings.TrimSpace(string(pidBefore)) != strings.TrimSpace(string(pidAfter)) {
			t.Fatalf("double start changed pid: before=%q after=%q", pidBefore, pidAfter)
		}
		out := runBinarySuccess(t, bin, env, "daemon", "status", "--json")
		if !strings.Contains(out, `"health": "ok"`) {
			t.Fatalf("daemon should still be healthy after double start, got: %s", out)
		}
	})

	t.Run("restart", func(t *testing.T) {
		runBinarySuccess(t, bin, env, "daemon", "restart")
		waitHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), 200, 10*time.Second)
		out := runBinarySuccess(t, bin, env, "daemon", "status", "--json")
		assertJSONValid(t, out)
	})

	t.Run("stop", func(t *testing.T) {
		runBinarySuccess(t, bin, env, "daemon", "stop")
		waitHTTPUnavailable(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), 10*time.Second)
		out := runBinarySuccess(t, bin, env, "daemon", "status", "--json")
		assertJSONValid(t, out)
		if strings.Contains(out, `"health": "ok"`) {
			t.Fatalf("daemon status still reports healthy after stop: %s", out)
		}
	})

	t.Run("double stop", func(t *testing.T) {
		stdout, _, err := runBinary(t, bin, env, "daemon", "stop")
		if err != nil {
			t.Logf("double stop error (acceptable): %v", err)
		}
		t.Logf("double stop output: %s", stdout)
	})

	if _, err := os.Stat(logPath); err != nil {
		t.Logf("daemon log not created: %v", err)
	}
}
