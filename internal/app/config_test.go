package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPathsCommand(t *testing.T) {
	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"paths"})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ocgo config dir") {
		t.Errorf("paths missing 'ocgo config dir', got: %s", out)
	}
	if !strings.Contains(out, "codex config") {
		t.Errorf("paths missing 'codex config', got: %s", out)
	}
}

func TestConfigPathsJSON(t *testing.T) {
	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"paths", "--json"})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	for _, key := range []string{"ocgo_config_dir", "ocgo_config_file", "codex_config_file"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("paths JSON missing key %q", key)
		}
	}
}

func TestConfigPathsDoesNotCreateFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"paths"})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(tmpHome, ".config", "ocgo")
	if _, err := os.Stat(configDir); err == nil {
		t.Errorf("paths created config directory: %s", configDir)
	}
}

func TestConfigInspectJSONCommand(t *testing.T) {
	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"inspect", "--json"})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	for _, key := range []string{"core", "model", "daemon", "codex_cli", "codex_desktop"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("inspect JSON missing key %q", key)
		}
	}
}

func TestConfigPathsJSONStrict(t *testing.T) {
	cmd := ConfigCmd()
	var buf bytes.Buffer
	cmd.SetArgs([]string{"paths", "--json"})
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !json.Valid([]byte(output)) {
		t.Fatalf("config paths --json is not valid JSON:\n%s", output)
	}
	for _, key := range []string{"ocgo_config_dir", "ocgo_config_file", "model_mapping_file", "model_selection_file", "model_cache_file", "daemon_state_file", "daemon_pid_file", "daemon_log_file", "desktop_state_file", "codex_config_file", "codex_ocgo_profile_file", "codex_model_catalog_file", "codex_backups_dir"} {
		var parsed map[string]any
		json.Unmarshal([]byte(output), &parsed)
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
}

func TestConfigInspectJSONStrict(t *testing.T) {
	cmd := ConfigCmd()
	var buf bytes.Buffer
	cmd.SetArgs([]string{"inspect", "--json"})
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !json.Valid([]byte(output)) {
		t.Fatalf("config inspect --json is not valid JSON:\n%s", output)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}
	for _, key := range []string{"core", "model", "daemon", "codex_cli", "codex_desktop"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
}

func TestConfigInspectJSONNoSecretLeak(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"SUPER_SECRET_TEST_KEY","host":"127.0.0.1","port":3456}`), 0644)

	cmd := ConfigCmd()
	var buf bytes.Buffer
	cmd.SetArgs([]string{"inspect", "--json"})
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if strings.Contains(output, "SUPER_SECRET_TEST_KEY") {
		t.Fatal("config inspect --json leaked secret API key")
	}
	var parsed struct {
		Core struct {
			OpenCodeAPIKey string `json:"opencode_api_key"`
		} `json:"core"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Core.OpenCodeAPIKey == "SUPER_SECRET_TEST_KEY" {
		t.Fatal("opencode_api_key value is the raw secret")
	}
	if parsed.Core.OpenCodeAPIKey != "present" && parsed.Core.OpenCodeAPIKey != "redacted" {
		t.Logf("opencode_api_key = %q (expected present or redacted)", parsed.Core.OpenCodeAPIKey)
	}
}

func TestConfigInspectDoesNotCreateFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"inspect"})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(tmpHome, ".config", "ocgo")
	if _, err := os.Stat(configDir); err == nil {
		t.Errorf("inspect created config directory: %s", configDir)
	}
}

func TestConfigBackupCommand(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test"}`), 0644)

	bp := filepath.Join(t.TempDir(), "test-backup.zip")
	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"backup", "--output", bp})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Backup created") {
		t.Errorf("backup output missing success, got: %s", buf.String())
	}
	if _, err := os.Stat(bp); err != nil {
		t.Errorf("backup file not created: %v", err)
	}
}

func TestConfigResetDryRunCommand(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	ocgoDir := filepath.Join(tmpHome, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	tf := filepath.Join(ocgoDir, "config.json")
	os.WriteFile(tf, []byte(`{"api_key":"test"}`), 0644)

	cmd := ConfigCmd()
	buf := new(bytes.Buffer)
	cmd.SetArgs([]string{"reset", "--scope", "ocgo", "--dry-run"})
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tf); err != nil {
		t.Errorf("dry-run deleted file: %s", tf)
	}
}
