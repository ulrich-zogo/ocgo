package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"ocgo/internal/models"
)

func withTempOpencodeState(t *testing.T) {
	t.Helper()
	selectionPath := filepath.Join(t.TempDir(), "model-selection.json")
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	restoreSel := models.SetModelSelectionFileForTest(selectionPath)
	t.Cleanup(restoreSel)
	restoreCache := models.SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
}

func TestOpencodeModelCurrentWithConfiguredDefault(t *testing.T) {
	withTempOpencodeState(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "qwen3.7-max", Object: "model", Created: 2, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })
	if err := models.SetDefaultModel("qwen3.7-max"); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"opencode", "model", "current"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Default OpenCode Go model: qwen3.7-max") {
		t.Fatalf("output missing default line: %s", got)
	}
	if !strings.Contains(got, "Source: configured") {
		t.Fatalf("output missing configured source: %s", got)
	}
}

func TestOpencodeModelCurrentWithoutDefault(t *testing.T) {
	withTempOpencodeState(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "kimi-k2.6", Object: "model", Created: 2, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	root := NewRootCommand("test")
	root.SetArgs([]string{"opencode", "model", "current"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Default OpenCode Go model: minimax-m3") {
		t.Fatalf("output missing default line: %s", got)
	}
	if !strings.Contains(got, "Source: first-known-model") {
		t.Fatalf("output missing first-known-model source: %s", got)
	}
}

func TestOpencodeModelSetDefault(t *testing.T) {
	withTempOpencodeState(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "qwen3.7-max", Object: "model", Created: 2, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	root := NewRootCommand("test")
	root.SetArgs([]string{"opencode", "model", "set-default", "qwen3.7-max"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Default OpenCode Go model set to qwen3.7-max") {
		t.Fatalf("output missing confirmation: %s", got)
	}
	got2, err := models.GetDefaultModel()
	if err != nil {
		t.Fatal(err)
	}
	if got2 != "qwen3.7-max" {
		t.Fatalf("GetDefaultModel = %q, want qwen3.7-max", got2)
	}
}

func TestOpencodeModelSetDefaultUnknownErrors(t *testing.T) {
	withTempOpencodeState(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	root := NewRootCommand("test")
	root.SetArgs([]string{"opencode", "model", "set-default", "does-not-exist"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	if !strings.Contains(err.Error(), "ocgo models") {
		t.Fatalf("error should mention `ocgo models`: %v", err)
	}
}

func TestOpencodeModelSetDefaultNormalizesPrefix(t *testing.T) {
	withTempOpencodeState(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "qwen3.7-max", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	root := NewRootCommand("test")
	root.SetArgs([]string{"opencode", "model", "set-default", "opencode-go/qwen3.7-max"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got, _ := models.GetDefaultModel()
	if got != "qwen3.7-max" {
		t.Fatalf("GetDefaultModel = %q, want qwen3.7-max (no opencode-go/ prefix)", got)
	}
}
