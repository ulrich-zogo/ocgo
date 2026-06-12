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

func TestAllJSONCommandsProduceValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-command JSON test in short mode")
	}

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	tests := []struct {
		name string
		args []string
	}{
		{"version --json", []string{"version", "--json"}},
		{"config paths --json", []string{"config", "paths", "--json"}},
		{"config inspect --json", []string{"config", "inspect", "--json"}},
		{"daemon status --json", []string{"daemon", "status", "--json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCommand("test")
			var out bytes.Buffer
			var errBuf bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errBuf)
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v, stdout: %s, stderr: %s", err, out.String(), errBuf.String())
			}
			output := strings.TrimSpace(out.String())
			if output == "" {
				t.Fatal("stdout is empty")
			}
			if !json.Valid([]byte(output)) {
				t.Fatalf("output is not valid JSON:\n%s", output)
			}
			if output[0] != '{' && output[0] != '[' {
				t.Fatalf("output does not start with JSON object/array:\n%s", output)
			}
			if output[len(output)-1] != '}' && output[len(output)-1] != ']' {
				t.Fatalf("output does not end with JSON object/array:\n%s", output)
			}
		})
	}
}

func TestAllJSONCommandsTextStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-command text test in short mode")
	}

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "test")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"test","host":"127.0.0.1","port":3456}`), 0644)

	tests := []struct {
		name string
		args []string
	}{
		{"version", []string{"version"}},
		{"config paths", []string{"config", "paths"}},
		{"daemon status", []string{"daemon", "status"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCommand("test")
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v, stdout: %s", err, out.String())
			}
			output := out.String()
			if output == "" {
				t.Fatal("text output is empty")
			}
			if json.Valid([]byte(strings.TrimSpace(output))) {
				t.Log("text mode output happens to be valid JSON; not an error")
			}
		})
	}
}

func TestDiagnosticJSONCommandsDoNotLeakSecrets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping secret leak test in short mode")
	}

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", dir)
	t.Setenv("OCGO_API_KEY", "SUPER_SECRET_TEST_KEY")

	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)
	os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(`{"api_key":"SUPER_SECRET_TEST_KEY","host":"127.0.0.1","port":3456}`), 0644)

	t.Run("config inspect --json", func(t *testing.T) {
		root := NewRootCommand("test")
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"config", "inspect", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "SUPER_SECRET_TEST_KEY") {
			t.Fatal("config inspect --json leaked secret")
		}
	})

	t.Run("doctor --json", func(t *testing.T) {
		root := NewRootCommand("test")
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"doctor", "--json"})
		_ = root.Execute()
		if strings.Contains(out.String(), "SUPER_SECRET_TEST_KEY") {
			t.Fatal("doctor --json leaked secret")
		}
	})

	t.Run("support bundle --json", func(t *testing.T) {
		outPath := filepath.Join(dir, "support.zip")
		root := NewRootCommand("test")
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"support", "bundle", "--output", outPath, "--force", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "SUPER_SECRET_TEST_KEY") {
			t.Fatal("support bundle --json stdout leaked secret")
		}

		r, err := zip.OpenReader(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		for _, f := range r.File {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b := new(bytes.Buffer)
			b.ReadFrom(rc)
			rc.Close()
			if strings.Contains(b.String(), "SUPER_SECRET_TEST_KEY") {
				t.Errorf("secret leaked in zip entry: %s", f.Name)
			}
		}
	})
}
