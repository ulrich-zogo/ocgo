package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCommandText(t *testing.T) {
	cmd := VersionCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "ocgo version") {
		t.Errorf("version text missing 'ocgo version', got: %s", output)
	}
	if !strings.Contains(output, "commit:") {
		t.Errorf("version text missing 'commit:', got: %s", output)
	}
	if !strings.Contains(output, "built:") {
		t.Errorf("version text missing 'built:', got: %s", output)
	}
	if !strings.Contains(output, "platform:") {
		t.Errorf("version text missing 'platform:', got: %s", output)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	cmd := VersionCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("version --json output not valid JSON: %v\noutput: %s", err, output)
	}

	expectedFields := []string{"version", "commit", "date", "go_version", "os", "arch"}
	for _, key := range expectedFields {
		if _, ok := parsed[key]; !ok {
			t.Errorf("version --json missing field %q in %v", key, parsed)
		}
	}
}

func TestVersionCommandDoesNotCreateConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)

	cmd := VersionCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Verify no config or codex directory was created
	configDir := filepath.Join(tmpHome, ".config", "ocgo")
	if _, err := os.Stat(configDir); err == nil {
		t.Errorf("version command created config directory: %s", configDir)
	}
	codexDir := filepath.Join(tmpHome, ".codex")
	if _, err := os.Stat(codexDir); err == nil {
		t.Errorf("version command created codex directory: %s", codexDir)
	}
}

func TestVersionCommandJSONHasStableFields(t *testing.T) {
	cmd := VersionCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var first, second map[string]interface{}
	json.Unmarshal(buf.Bytes(), &first)

	buf.Reset()
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(buf.Bytes(), &second)

	for key := range first {
		if _, ok := second[key]; !ok {
			t.Errorf("second call missing field %q present in first call", key)
		}
	}
}
