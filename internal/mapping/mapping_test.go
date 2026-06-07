package mapping

import (
	"path/filepath"
	"testing"

	"ocgo/internal/config"
)

func TestModelMappingsLoadSaveAndResolve(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-mapping.json")
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return path }
	t.Cleanup(func() { config.ModelMappingFile = old })

	m, err := LoadModelMappings()
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveMappedModel("claude", "claude-sonnet-4-5", m); got != "claude-sonnet-4-5" {
		t.Fatalf("unconfigured claude model should pass through, got %q", got)
	}
	m["claude"]["claude-sonnet"] = "kimi-k2.6"
	m["claude"]["claude-sonnet-4-5"] = "qwen3.7-max"
	m["codex"]["gpt-5"] = "deepseek-v4-pro"
	if err := SaveModelMappings(m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadModelMappings()
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveMappedModel("claude", "claude-sonnet-4-5", reloaded); got != "qwen3.7-max" {
		t.Fatalf("custom claude mapping = %q", got)
	}
	if got := ResolveMappedModel("codex", "gpt-5", reloaded); got != "deepseek-v4-pro" {
		t.Fatalf("custom codex mapping = %q", got)
	}
}

func TestMappingUnsetCommandRemovesMapping(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })

	m := DefaultModelMappings()
	m["codex"]["gpt-5.5"] = "deepseek-v4-pro"
	if err := SaveModelMappings(m); err != nil {
		t.Fatal(err)
	}
	delete(m["codex"], "gpt-5.5")
	if err := SaveModelMappings(m); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadModelMappings()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded["codex"]["gpt-5.5"]; ok {
		t.Fatalf("mapping was not removed: %+v", reloaded["codex"])
	}
}
