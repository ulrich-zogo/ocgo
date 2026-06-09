package models

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func withSelectionFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model-selection.json")
	restore := SetModelSelectionFileForTest(path)
	t.Cleanup(restore)
	return path
}

func TestModelSelectionFilePath(t *testing.T) {
	if got := configModelSelectionFile(); !strings.HasSuffix(got, "model-selection.json") {
		t.Fatalf("ModelSelectionFile() = %q, want suffix model-selection.json", got)
	}
}

func configModelSelectionFile() string {
	return ModelSelectionFile()
}

func TestReadModelSelectionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-selection.json")
	when := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	sel := ModelSelection{
		Version:      modelSelectionVersion,
		DefaultModel: "minimax-m3",
		UpdatedAt:    when,
	}
	if err := WriteModelSelection(path, sel); err != nil {
		t.Fatal(err)
	}
	got, err := ReadModelSelection(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != modelSelectionVersion {
		t.Fatalf("Version = %d, want %d", got.Version, modelSelectionVersion)
	}
	if got.DefaultModel != "minimax-m3" {
		t.Fatalf("DefaultModel = %q, want minimax-m3", got.DefaultModel)
	}
	if !got.UpdatedAt.Equal(when) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, when)
	}
}

func TestReadModelSelectionRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-selection.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadModelSelection(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReadModelSelectionRejectsIncompatibleVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-selection.json")
	body := `{"version":999,"default_model":"minimax-m3","updated_at":"2026-06-09T12:00:00Z"}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadModelSelection(path); err == nil {
		t.Fatal("expected error for incompatible version")
	}
}

func TestReadModelSelectionRejectsEmptyPath(t *testing.T) {
	if _, err := ReadModelSelection(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestWriteModelSelectionCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "model-selection.json")
	if err := WriteModelSelection(path, ModelSelection{DefaultModel: "minimax-m3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestSetDefaultModelNormalizesOpencodeGoPrefix(t *testing.T) {
	path := withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("qwen3.7-max"), nil)
	if err := SetDefaultModel("opencode-go/qwen3.7-max"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sel ModelSelection
	if err := json.Unmarshal(data, &sel); err != nil {
		t.Fatal(err)
	}
	if sel.DefaultModel != "qwen3.7-max" {
		t.Fatalf("DefaultModel = %q, want qwen3.7-max (no opencode-go/ prefix)", sel.DefaultModel)
	}
}

func TestSetDefaultModelTrimsWhitespace(t *testing.T) {
	path := withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	if err := SetDefaultModel("  minimax-m3  "); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var sel ModelSelection
	if err := json.Unmarshal(data, &sel); err != nil {
		t.Fatal(err)
	}
	if sel.DefaultModel != "minimax-m3" {
		t.Fatalf("DefaultModel = %q, want minimax-m3", sel.DefaultModel)
	}
}

func TestSetDefaultModelRejectsUnknown(t *testing.T) {
	path := withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	err := SetDefaultModel("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	if !strings.Contains(err.Error(), "ocgo models") {
		t.Fatalf("error should mention `ocgo models`: %v", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("file should not be written when model is invalid")
	}
}

func TestSetDefaultModelRejectsEmpty(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	if err := SetDefaultModel(""); err == nil {
		t.Fatal("expected error for empty model")
	}
	if err := SetDefaultModel("   "); err == nil {
		t.Fatal("expected error for whitespace-only model")
	}
}

func TestGetDefaultModelReadsFromFile(t *testing.T) {
	path := withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	if err := SetDefaultModel("minimax-m3"); err != nil {
		t.Fatal(err)
	}
	got, err := GetDefaultModel()
	if err != nil {
		t.Fatal(err)
	}
	if got != "minimax-m3" {
		t.Fatalf("GetDefaultModel = %q, want minimax-m3", got)
	}
	_ = path
}

func TestGetDefaultModelReturnsErrorOnMissing(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	if _, err := GetDefaultModel(); err == nil {
		t.Fatal("expected error when no selection file exists")
	}
}

func TestGetDefaultModelRejectsUnknownConfigured(t *testing.T) {
	path := withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	if err := WriteModelSelection(path, ModelSelection{
		Version:      modelSelectionVersion,
		DefaultModel: "not-a-real-model",
		UpdatedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := GetDefaultModel(); err == nil {
		t.Fatal("expected error for unknown configured model")
	}
}

func TestResolveEffectiveModelPrioritizesExplicit(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3", "qwen3.7-max"), nil)
	if err := SetDefaultModel("minimax-m3"); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveEffectiveModel("qwen3.7-max")
	if err != nil {
		t.Fatal(err)
	}
	if got != "qwen3.7-max" {
		t.Fatalf("ResolveEffectiveModel(\"qwen3.7-max\") = %q, want qwen3.7-max (explicit wins)", got)
	}
}

func TestResolveEffectiveModelNormalizesExplicit(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3", "qwen3.7-max"), nil)
	got, err := ResolveEffectiveModel("opencode-go/qwen3.7-max")
	if err != nil {
		t.Fatal(err)
	}
	if got != "qwen3.7-max" {
		t.Fatalf("ResolveEffectiveModel = %q, want qwen3.7-max", got)
	}
}

func TestResolveEffectiveModelUsesConfiguredDefault(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3", "kimi-k2.6", "qwen3.7-max"), nil)
	if err := SetDefaultModel("minimax-m3"); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveEffectiveModel("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "minimax-m3" {
		t.Fatalf("ResolveEffectiveModel(\"\") = %q, want minimax-m3 (configured default)", got)
	}
}

func TestResolveEffectiveModelFallsBackToFirstKnown(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, nil, errors.New("no official"))
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		return map[string]remoteModelInfo{
			"alpha-only": {},
			"minimax-m3": {},
			"kimi-k2.6":  {},
		}, nil
	})
	got, err := ResolveEffectiveModel("")
	if err != nil {
		t.Fatal(err)
	}
	// Remote models are sorted alphabetically (PR 4 contract), so the
	// first KnownID here is "alpha-only".
	if got != "alpha-only" {
		t.Fatalf("ResolveEffectiveModel(\"\") = %q, want alpha-only (first sorted KnownID)", got)
	}
}

func TestResolveEffectiveModelRejectsUnknownExplicit(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3"), nil)
	_, err := ResolveEffectiveModel("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown explicit model")
	}
}

func TestResolveEffectiveModelFallsBackToStaticFallback(t *testing.T) {
	withSelectionFile(t)
	ResetFetchersForTest()
	SetFetchersForTest(nil, nil, errors.New("no official"), errors.New("no remote"))
	restoreCache := SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	defer restoreCache()
	got, err := ResolveEffectiveModel("")
	if err != nil {
		t.Fatal(err)
	}
	// With no official, no remote, and no cache, the 18-model
	// fallback list is used; first entry is minimax-m3.
	if got != "minimax-m3" {
		t.Fatalf("ResolveEffectiveModel(\"\") = %q, want minimax-m3 (first fallback)", got)
	}
}


func TestGetDefaultModelStatusWithConfiguredDefault(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3", "qwen3.7-max"), nil)
	if err := SetDefaultModel("qwen3.7-max"); err != nil {
		t.Fatal(err)
	}
	id, configured, err := GetDefaultModelStatus()
	if err != nil {
		t.Fatal(err)
	}
	if id != "qwen3.7-max" {
		t.Fatalf("model = %q, want qwen3.7-max", id)
	}
	if !configured {
		t.Fatal("configured = false, want true")
	}
}

func TestGetDefaultModelStatusFallsBackToFirstKnown(t *testing.T) {
	withSelectionFile(t)
	withModelFetchers(t, nil, officialFromIDs("minimax-m3", "kimi-k2.6"), nil)
	id, configured, err := GetDefaultModelStatus()
	if err != nil {
		t.Fatal(err)
	}
	if id != "minimax-m3" {
		t.Fatalf("model = %q, want minimax-m3 (first known)", id)
	}
	if configured {
		t.Fatal("configured = true, want false (no selection file)")
	}
}

func TestGetDefaultModelStatusFallsBackToStaticList(t *testing.T) {
	withSelectionFile(t)
	ResetFetchersForTest()
	SetFetchersForTest(nil, nil, errors.New("no oficial"), errors.New("no remote"))
	restoreCache := SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	defer restoreCache()
	id, configured, err := GetDefaultModelStatus()
	if err != nil {
		t.Fatal(err)
	}
	// With no oficial, no remote, and no cache, the 18-model
	// fallback list is used; first entry is minimax-m3.
	if id != "minimax-m3" {
		t.Fatalf("model = %q, want minimax-m3 (first fallback)", id)
	}
	if configured {
		t.Fatal("configured = true, want false (no selection file)")
	}
}