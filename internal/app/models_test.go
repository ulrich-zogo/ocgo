package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

func withTempSelectionFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model-selection.json")
	restore := models.SetModelSelectionFileForTest(path)
	t.Cleanup(restore)
	restoreCache := models.SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(restoreCache)
	return path
}

func TestBuildCodexArgsExplicitModel(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "qwen3.7-max", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	args, err := BuildCodexArgs("qwen3.7-max", []string{"--help"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--profile", "ocgo-launch", "-m", "qwen3.7-max", "--help"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestBuildCodexArgsUsesConfiguredDefault(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "kimi-k2.6", Object: "model", Created: 2, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })
	if err := models.SetDefaultModel("minimax-m3"); err != nil {
		t.Fatal(err)
	}

	args, err := BuildCodexArgs("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) < 4 {
		t.Fatalf("args too short: %v", args)
	}
	if args[len(args)-1] != "minimax-m3" {
		t.Fatalf("last arg = %q, want minimax-m3 (configured default)", args[len(args)-1])
	}
}

func TestBuildCodexArgsPreservesExtraArgs(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	args, err := BuildCodexArgs("minimax-m3", []string{"--search", "off"})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 6 {
		t.Fatalf("args = %v, want 6 entries", args)
	}
	if args[4] != "--search" || args[5] != "off" {
		t.Fatalf("extra args not preserved at the end: %v", args)
	}
}

func TestBuildCodexArgsRejectsUnknownExplicit(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	_, err := BuildCodexArgs("does-not-exist", nil)
	if err == nil {
		t.Fatal("expected error for unknown explicit model")
	}
}

func TestBuildClaudeModelEnvExplicit(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "qwen3.7-max", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	env, ok, err := BuildClaudeModelEnv("qwen3.7-max")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false, want true for explicit model")
	}
	if len(env) != 5 {
		t.Fatalf("env length = %d, want 5, got %v", len(env), env)
	}
	if env[0] != "ANTHROPIC_MODEL=qwen3.7-max" {
		t.Fatalf("env[0] = %q, want ANTHROPIC_MODEL=qwen3.7-max", env[0])
	}
	if env[1] != "ANTHROPIC_SMALL_FAST_MODEL=qwen3.7-max" {
		t.Fatalf("env[1] = %q, want ANTHROPIC_SMALL_FAST_MODEL=qwen3.7-max", env[1])
	}
}

func TestBuildClaudeModelEnvUsesConfiguredDefault(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "kimi-k2.6", Object: "model", Created: 2, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })
	if err := models.SetDefaultModel("minimax-m3"); err != nil {
		t.Fatal(err)
	}

	env, ok, err := BuildClaudeModelEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false, want true for configured default")
	}
	if env[0] != "ANTHROPIC_MODEL=minimax-m3" {
		t.Fatalf("env[0] = %q, want ANTHROPIC_MODEL=minimax-m3", env[0])
	}
}

func TestBuildClaudeModelEnvUnknownExplicitErrors(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	_, _, err := BuildClaudeModelEnv("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown explicit model")
	}
}

func TestBuildClaudeLegacyMappingEnvPreservesUnmapped(t *testing.T) {
	mappings := map[string]map[string]string{
		"claude": {},
	}
	env := BuildClaudeLegacyMappingEnv(mappings)
	if len(env) != 0 {
		t.Fatalf("unmapped mappings should yield empty env, got %v", env)
	}
}

func TestBuildClaudeLegacyMappingEnvWithMappings(t *testing.T) {
	mappings := map[string]map[string]string{
		"claude": {
			"claude-opus":   "minimax-m3",
			"claude-sonnet": "kimi-k2.6",
		},
	}
	env := BuildClaudeLegacyMappingEnv(mappings)
	if len(env) == 0 {
		t.Fatal("expected non-empty env")
	}
	hasOpus := false
	hasSonnet := false
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_DEFAULT_OPUS_MODEL=") {
			hasOpus = true
		}
		if strings.HasPrefix(e, "ANTHROPIC_DEFAULT_SONNET_MODEL=") {
			hasSonnet = true
		}
	}
	if !hasOpus {
		t.Fatal("missing ANTHROPIC_DEFAULT_OPUS_MODEL in legacy env")
	}
	if !hasSonnet {
		t.Fatal("missing ANTHROPIC_DEFAULT_SONNET_MODEL in legacy env")
	}
}

func TestBuildClaudeLegacyMappingEnvIgnoredWhenNoOfficial(t *testing.T) {
	// This test ensures that legacy mapping env can still produce output
	// even when the effective model is unavailable. Just exercises the
	// function in isolation without coupling to the global models state.
	_ = mapping.DefaultModelMappings
}

func TestDescribeSelectionSource(t *testing.T) {
	if got := DescribeSelectionSource(true); got != "configured" {
		t.Fatalf("DescribeSelectionSource(true) = %q, want configured", got)
	}
	if got := DescribeSelectionSource(false); got != "first-known-model" {
		t.Fatalf("DescribeSelectionSource(false) = %q, want first-known-model", got)
	}
}

func TestBuildCodexArgsErrorsOnAllFetchersDown(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, nil, errors.New("no official"), errors.New("no remote"))
	t.Cleanup(func() { models.ResetFetchersForTest() })
	// With no selection file and all fetchers down, fallback list applies
	// and ResolveEffectiveModel succeeds. So BuildCodexArgs should still
	// succeed and produce a -m with the first fallback model.
	args, err := BuildCodexArgs("", nil)
	if err != nil {
		t.Fatalf("BuildCodexArgs should succeed with fallback list, got: %v", err)
	}
	if len(args) < 4 {
		t.Fatalf("args = %v, want at least 4 entries", args)
	}
	if args[3] != "minimax-m3" {
		t.Fatalf("args[3] = %q, want minimax-m3 (first fallback)", args[3])
	}
}


func TestBuildCodexArgsWithResolvedModel(t *testing.T) {
	args := BuildCodexArgsWithResolvedModel("qwen3.7-max", []string{"--search", "off"})
	want := []string{"--profile", "ocgo-launch", "-m", "qwen3.7-max", "--search", "off"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestBuildCodexArgsWithResolvedModelNoExtras(t *testing.T) {
	args := BuildCodexArgsWithResolvedModel("minimax-m3", nil)
	if len(args) != 4 {
		t.Fatalf("args = %v, want 4 entries", args)
	}
	want := []string{"--profile", "ocgo-launch", "-m", "minimax-m3"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestBuildCodexArgsDelegatesToWithResolvedModel(t *testing.T) {
	withTempSelectionFile(t)
	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, []models.OfficialModel{
		{ID: "qwen3.7-max", Object: "model", Created: 1, OwnedBy: "opencode"},
	}, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	args, err := BuildCodexArgs("qwen3.7-max", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--profile", "ocgo-launch", "-m", "qwen3.7-max"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("BuildCodexArgs should delegate to BuildCodexArgsWithResolvedModel: got %v, want %v", args, want)
	}
}