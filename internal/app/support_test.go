package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportBundleCommandCreatesZip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	outPath := filepath.Join(dir, "out.zip")
	root := NewRootCommand("test")
	root.SetArgs([]string{"support", "bundle", "--output", outPath, "--force"})
	if err := root.Execute(); err != nil {
		t.Fatalf("support bundle: %v", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("bundle zip not created: %v", err)
	}
	r, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}
	for _, required := range []string{"manifest.json", "version.json", "config-paths.json", "config-inspect.json", "environment.json"} {
		if !names[required] {
			t.Errorf("missing required entry: %s", required)
		}
	}
}

func TestSupportBundleCommandJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	outPath := filepath.Join(dir, "out.zip")
	buf := new(bytes.Buffer)
	root := NewRootCommand("test")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"support", "bundle", "--output", outPath, "--force", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("support bundle: %v", err)
	}

	var result struct {
		Path         string   `json:"path"`
		Files        []string `json:"files"`
		Redacted     bool     `json:"redacted"`
		LogsIncluded bool     `json:"logs_included"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &result); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	if result.Path == "" {
		t.Fatal("expected non-empty path in JSON output")
	}
	if !result.Redacted {
		t.Fatal("expected redacted=true")
	}
}

func TestSupportBundleCommandNoLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	outPath := filepath.Join(dir, "out.zip")
	root := NewRootCommand("test")
	root.SetArgs([]string{"support", "bundle", "--output", outPath, "--force", "--no-logs"})
	if err := root.Execute(); err != nil {
		t.Fatalf("support bundle: %v", err)
	}

	r, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "logs/") {
			t.Fatalf("found log entry %s with --no-logs", f.Name)
		}
	}
}

func TestSupportBundleCommandRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	outPath := filepath.Join(dir, "out.zip")
	os.WriteFile(outPath, []byte("existing"), 0644)

	root := NewRootCommand("test")
	root.SetArgs([]string{"support", "bundle", "--output", outPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for existing file without --force")
	}
}

func TestSupportBundleCommandForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	outPath := filepath.Join(dir, "out.zip")
	os.WriteFile(outPath, []byte("existing"), 0644)

	root := NewRootCommand("test")
	root.SetArgs([]string{"support", "bundle", "--output", outPath, "--force"})
	if err := root.Execute(); err != nil {
		t.Fatalf("support bundle with --force: %v", err)
	}
}

func TestSupportHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	root := NewRootCommand("test")
	root.SetOut(buf)
	root.SetArgs([]string{"support", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("support --help: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "bundle") {
		t.Fatal("help should mention bundle subcommand")
	}
}

func TestSupportBundleHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	root := NewRootCommand("test")
	root.SetOut(buf)
	root.SetArgs([]string{"support", "bundle", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("support bundle --help: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "--output") {
		t.Fatal("help should mention --output flag")
	}
	if !strings.Contains(output, "--force") {
		t.Fatal("help should mention --force flag")
	}
	if !strings.Contains(output, "--no-logs") {
		t.Fatal("help should mention --no-logs flag")
	}
	if !strings.Contains(output, "--json") {
		t.Fatal("help should mention --json flag")
	}
}
