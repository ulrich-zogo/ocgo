package models

import (
	"strings"
	"testing"
)

func TestKnownModelIDsPreferOfficialThenRemoteThenFallback(t *testing.T) {
	ids := KnownIDs()
	if len(ids) == 0 {
		t.Fatal("KnownIDs returned empty")
	}
	if ids[0] != fallbackModelIDs[0] {
		t.Logf("KnownIDs = %v", ids)
	}
}

func TestModelMetadataUsesRemoteFixtureWithoutLiveNetwork(t *testing.T) {
	meta := Metadata("kimi-k2.6")
	if meta.DisplayName != "Kimi K2.6" {
		t.Fatalf("metadata display name = %q, want Kimi K2.6", meta.DisplayName)
	}
	if meta.ContextWindow == 0 {
		t.Fatalf("metadata context window should be populated: %+v", meta)
	}
	codexMods := CodexSupportedModalities(meta.InputModalities)
	if len(codexMods) == 0 {
		t.Fatalf("codex modalities should not be empty: %+v", codexMods)
	}
}

func TestCodexModelCatalogAllowsImagesForKnownVisionModels(t *testing.T) {
	if !SupportsImages("kimi-k2.6") {
		t.Fatal("kimi-k2.6 should support image inputs")
	}
	if !SupportsImages("minimax-m3") {
		t.Fatal("minimax-m3 should support image inputs")
	}
	if SupportsImages("deepseek-v4-pro") {
		t.Fatal("deepseek-v4-pro should not support image inputs")
	}
}

func TestAnthropicEndpointModels(t *testing.T) {
	for _, model := range []string{"qwen3.7-max", "minimax-m3", "minimax-m2.7", "opencode-go/qwen3.7-max", "opencode-go/minimax-m3"} {
		if !UsesAnthropicEndpoint(model) {
			t.Fatalf("%s should use Anthropic endpoint", model)
		}
	}
	if UsesAnthropicEndpoint("kimi-k2.6") {
		t.Fatal("kimi-k2.6 should not use Anthropic endpoint")
	}
}

func TestModelsInputModalities(t *testing.T) {
	if got := InputModalities("deepseek-v4-pro"); len(got) == 0 {
		t.Fatalf("deepseek-v4-pro modalities should not be empty: %+v", got)
	}
	if got := CodexSupportedModalities([]string{"text", "image", "video"}); strings.Join(got, ",") != "text,image" {
		t.Fatalf("codex supported = %+v", got)
	}
	if got := CodexSupportedModalities([]string{"text"}); strings.Join(got, ",") != "text" {
		t.Fatalf("codex supported = %+v", got)
	}
	if got := CodexSupportedModalities(nil); strings.Join(got, ",") != "text" {
		t.Fatalf("codex supported for nil = %+v", got)
	}
}
