package support

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateBundleIncludesManifest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	assertZipContains(t, result.Path, "manifest.json")
}

func TestCreateBundleIncludesGeneratedReports(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	for _, name := range []string{"version.json", "config-paths.json", "config-inspect.json", "environment.json"} {
		assertZipContains(t, result.Path, name)
	}
}

func TestCreateBundleRedactsSecretsInLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{"api_key":"secret123"}`)
	logPath := filepath.Join(dir, ".config", "ocgo", "ocgo.log")
	os.MkdirAll(filepath.Dir(logPath), 0755)
	os.WriteFile(logPath, []byte("Bearer sk-my-secret-key\n"), 0644)

	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true, IncludeLogs: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	data := readZipEntry(t, result.Path, "logs/ocgo.log")
	if strings.Contains(string(data), "sk-my-secret-key") {
		t.Fatal("secret key leaked in redacted log")
	}
	if !strings.Contains(string(data), "[REDACTED]") {
		t.Fatal("expected [REDACTED] in redacted log")
	}
}

func TestCreateBundleNoLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true, IncludeLogs: false})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	_, err = readZipEntryRaw(result.Path, "logs/ocgo.log")
	if err == nil {
		t.Fatal("logs should not be included when IncludeLogs=false")
	}

	manifest := readManifest(t, result.Path)
	if manifest.LogsIncluded {
		t.Fatal("manifest.LogsIncluded should be false")
	}
}

func TestCreateBundleRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	outPath := filepath.Join(dir, "bundle.zip")
	os.WriteFile(outPath, []byte("existing"), 0644)

	_, err := CreateBundle(BundleOptions{OutputPath: outPath})
	if err == nil {
		t.Fatal("expected error for existing file without --force")
	}
}

func TestCreateBundleForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	outPath := filepath.Join(dir, "bundle.zip")
	os.WriteFile(outPath, []byte("existing"), 0644)

	_, err := CreateBundle(BundleOptions{OutputPath: outPath, Force: true})
	if err != nil {
		t.Fatalf("CreateBundle with Force: %v", err)
	}
}

func TestSafeZipPathRejectsTraversal(t *testing.T) {
	cases := []struct {
		path string
		ok   bool
	}{
		{"version.json", true},
		{"logs/ocgo.log", true},
		{"state/daemon-state.json", true},
		{"../evil", false},
		{"/abs/path", false},
		{"C:\\evil", false},
		{"logs/../evil", false},
		{"..\\traversal", false},
	}
	for _, c := range cases {
		_, err := SafeZipPath(c.path)
		if c.ok && err != nil {
			t.Errorf("SafeZipPath(%q) should be ok, got: %v", c.path, err)
		}
		if !c.ok && err == nil {
			t.Errorf("SafeZipPath(%q) should be rejected", c.path)
		}
	}
}

func TestCreateBundleSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	manifest := readManifest(t, result.Path)
	for _, f := range manifest.Files {
		if f.Path == "state/model-mapping.json" && f.Status != "missing" {
			t.Errorf("model-mapping.json status: got %q, want missing (file does not exist)", f.Status)
		}
	}
}

func TestLargeLogIsTruncated(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	logPath := filepath.Join(dir, ".config", "ocgo", "ocgo.log")
	os.MkdirAll(filepath.Dir(logPath), 0755)
	large := make([]byte, maxLogBytes+100)
	for i := range large {
		large[i] = 'A'
	}
	os.WriteFile(logPath, large, 0644)

	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true, IncludeLogs: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	data, err := readZipEntryRaw(result.Path, "logs/ocgo.log")
	if err != nil {
		t.Fatalf("read log from zip: %v", err)
	}
	if len(data) > maxLogBytes {
		t.Fatalf("log was truncated to %d bytes, want <= %d", len(data), maxLogBytes)
	}
}

func TestCreateBundleIncludesExistingStateFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	os.MkdirAll(ocgoDir, 0755)

	stateData := `{"version":1,"pid":12345}`
	os.WriteFile(filepath.Join(ocgoDir, "daemon-state.json"), []byte(stateData), 0644)

	result, err := CreateBundle(BundleOptions{OutputPath: filepath.Join(dir, "bundle.zip"), Force: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	data, err := readZipEntryRaw(result.Path, "state/daemon-state.json")
	if err != nil {
		t.Fatalf("state/daemon-state.json missing from bundle: %v", err)
	}
	if !strings.Contains(string(data), "12345") {
		t.Errorf("expected PID in state file, got: %s", string(data))
	}
}

func TestDefaultBundlePathInSupportBundlesDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	result, err := CreateBundle(BundleOptions{Force: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	if !strings.Contains(result.Path, "support-bundles") {
		t.Errorf("expected path to contain support-bundles, got: %s", result.Path)
	}
}

func TestBundleRedactedByDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	writeConfig(t, dir, `{}`)
	result, err := CreateBundle(BundleOptions{Force: true})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	manifest := readManifest(t, result.Path)
	if !manifest.Redacted {
		t.Error("bundle should be redacted by default")
	}
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	ocgoDir := filepath.Join(dir, ".config", "ocgo")
	if err := os.MkdirAll(ocgoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ocgoDir, "config.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertZipContains(t *testing.T, zipPath, name string) {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			return
		}
	}
	t.Fatalf("zip %s does not contain %s", zipPath, name)
}

func readZipEntry(t *testing.T, zipPath, name string) string {
	t.Helper()
	data, err := readZipEntryRaw(zipPath, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readZipEntryRaw(zipPath, name string) ([]byte, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			b := new(bytes.Buffer)
			if _, err := b.ReadFrom(rc); err != nil {
				return nil, err
			}
			return b.Bytes(), nil
		}
	}
	return nil, zip.ErrFormat
}

func TestCreateSafeZipEntryRejectsUnsafe(t *testing.T) {
	_, _, err := createSafeZipEntry(nil, "../evil")
	if err == nil {
		t.Fatal("expected error for traversal path")
	}
	_, _, err = createSafeZipEntry(nil, "C:\\evil")
	if err == nil {
		t.Fatal("expected error for Windows drive path")
	}
}

func readManifest(t *testing.T, zipPath string) BundleManifest {
	t.Helper()
	data := readZipEntry(t, zipPath, "manifest.json")
	var m BundleManifest
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return m
}


