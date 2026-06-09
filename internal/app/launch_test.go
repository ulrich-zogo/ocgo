package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ocgo/internal/models"
)

func TestLaunchCodexConfigValidatesExplicitModel(t *testing.T) {
	// Redirect HOME so EnsureCLIConfig doesn't touch the real user dir.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)
	restoreModel := models.SetModelSelectionFileForTest(filepath.Join(t.TempDir(), "model-selection.json"))
	t.Cleanup(restoreModel)
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	// Provide a config so the command proceeds past LoadConfig.
	cfgPath := filepath.Join(tmpHome, ".config", "ocgo", "config.json")
	cfgBody := `{"api_key":"test-key","host":"127.0.0.1","port":3456}`
	if err := writeFile(cfgPath, cfgBody); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"launch", "codex", "--config", "--model", "does-not-exist"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown --model even with --config")
	}
	if !strings.Contains(err.Error(), "unknown OpenCode Go model") {
		t.Fatalf("error should mention `unknown OpenCode Go model`, got: %v", err)
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("error should mention the bad model, got: %v", err)
	}
}

func TestLaunchCodexConfigPrintsEffectiveModel(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", tmpHome)
	restoreModel := models.SetModelSelectionFileForTest(filepath.Join(t.TempDir(), "model-selection.json"))
	t.Cleanup(restoreModel)
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	cfgPath := filepath.Join(tmpHome, ".config", "ocgo", "config.json")
	cfgBody := `{"api_key":"test-key","host":"127.0.0.1","port":3456}`
	if err := writeFile(cfgPath, cfgBody); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"launch", "codex", "--config", "--model", "minimax-m3"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Configured Codex profile") {
		t.Fatalf("output missing profile line: %s", got)
	}
	if !strings.Contains(got, "Effective OpenCode Go model: minimax-m3") {
		t.Fatalf("output missing effective model line: %s", got)
	}
}

func writeFile(path string, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0644)
}
