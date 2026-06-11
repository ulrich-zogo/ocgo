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
