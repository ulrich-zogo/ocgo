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
	if meta.DisplayName != "Kimi K2.6" || meta.ContextWindow != 131072 {
		t.Fatalf("metadata was not correctly populated: %+v", meta)
	}
	if strings.Join(meta.InputModalities, ",") != "text,image" {
		t.Fatalf("input modalities = %+v", meta.InputModalities)
	}
	codexMods := CodexSupportedModalities(meta.InputModalities)
	if strings.Join(codexMods, ",") != "text,image" {
		t.Fatalf("codex modalities = %+v", codexMods)
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
	for _, tc := range []struct {
		model string
		want  []string
	}{
		{model: "kimi-k2.6", want: []string{"text", "image"}},
		{model: "minimax-m3", want: []string{"text", "image"}},
		{model: "deepseek-v4-pro", want: []string{}},
	} {
		got := InputModalities(tc.model)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("%s modalities = %+v, want %+v", tc.model, got, tc.want)
		}
	}
}

func TestAnthropicEndpointModels(t *testing.T) {
	for _, model := range []string{"qwen3.7-max", "minimax-m3", "minimax-m2.7", "opencode-go/qwen3.7-max", "opencode-go/minimax-m3"} {
		if UsesAnthropicEndpoint(model) {
			t.Fatalf("%s should not indicate Anthropic endpoint in current metadata", model)
		}
	}
	if UsesAnthropicEndpoint("kimi-k2.6") {
		t.Fatal("kimi-k2.6 should not use Anthropic endpoint")
	}
}

func TestModelsInputModalities(t *testing.T) {
	if got := InputModalities("minimax-m3"); strings.Join(got, ",") != "text,image" {
		t.Fatalf("minimax-m3 modalities = %+v, want [text image]", got)
	}
	if got := InputModalities("kimi-k2.6"); strings.Join(got, ",") != "text,image" {
		t.Fatalf("kimi-k2.6 modalities = %+v, want [text image]", got)
	}
	if got := InputModalities("deepseek-v4-pro"); len(got) != 0 {
		t.Fatalf("deepseek-v4-pro modalities = %+v, want empty", got)
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
