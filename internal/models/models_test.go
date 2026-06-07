package models

import (
	"errors"
	"strings"
	"testing"
)

func withModelFetchers(t *testing.T, remote map[string]remoteModelInfo, official []string, officialErr error) {
	t.Helper()
	oldRemoteModels := remoteModels
	oldOfficialModels := officialModels
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		if remote == nil {
			return nil, errors.New("remote unavailable")
		}
		return remote, nil
	})
	officialModels = newLazyFetcher(func() ([]string, error) {
		if officialErr != nil {
			return nil, officialErr
		}
		if official == nil {
			return nil, errors.New("official unavailable")
		}
		return official, nil
	})
	t.Cleanup(func() {
		remoteModels = oldRemoteModels
		officialModels = oldOfficialModels
	})
}

func TestKnownIDsReturnsOfficialSorted(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-b": {}, "remote-a": {}},
		[]string{"official-b", "official-a"},
		nil,
	)
	got := KnownIDs()
	want := []string{"official-a", "official-b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v", got, want)
	}
}

func TestKnownIDsReturnsRemoteWhenOfficialFails(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-b": {}, "remote-a": {}},
		nil,
		errors.New("official API down"),
	)
	got := KnownIDs()
	want := []string{"remote-a", "remote-b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v", got, want)
	}
}

func TestKnownIDsReturnsFallbackWhenBothFail(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("official down"))
	got := KnownIDs()
	want := append([]string(nil), fallbackModelIDs...)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v", got, want)
	}
}

func TestKnownIDsReturnsRemoteWhenOfficialEmpty(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-a": {}},
		[]string{},
		nil,
	)
	got := KnownIDs()
	want := []string{"remote-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v", got, want)
	}
}

func TestKnownIDsReturnsEmptyOfficialThenRemote(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-a": {}, "remote-b": {}},
		[]string{"official-a"},
		nil,
	)
	got := KnownIDs()
	want := []string{"official-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v (official should take priority over remote)", got, want)
	}
}

func TestModelMetadataMinimaxM3DefaultModalities(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	meta := Metadata("minimax-m3")
	if strings.Join(meta.InputModalities, ",") != "text,image,video" {
		t.Fatalf("minimax-m3 InputModalities = %v, want [text image video]", meta.InputModalities)
	}
	if strings.Join(meta.CodexInputModalities, ",") != "text,image" {
		t.Fatalf("minimax-m3 CodexInputModalities = %v, want [text image]", meta.CodexInputModalities)
	}
	if meta.ContextWindow != 512000 {
		t.Fatalf("minimax-m3 ContextWindow = %d, want 512000", meta.ContextWindow)
	}
	if meta.MaxContextWindow != 512000 {
		t.Fatalf("minimax-m3 MaxContextWindow = %d, want 512000", meta.MaxContextWindow)
	}
	if meta.DisplayName != "MiniMax M3" {
		t.Fatalf("minimax-m3 DisplayName = %q, want MiniMax M3", meta.DisplayName)
	}
	if !meta.UsesAnthropicEndpoint {
		t.Fatal("minimax-m3 should use Anthropic endpoint")
	}
}

func TestModelMetadataKimiDefaultModalities(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	for _, id := range []string{"kimi-k2.6", "kimi-k2.5", "mimo-v2-omni"} {
		meta := Metadata(id)
		if strings.Join(meta.InputModalities, ",") != "text,image" {
			t.Fatalf("%s InputModalities = %v, want [text image]", id, meta.InputModalities)
		}
		if strings.Join(meta.CodexInputModalities, ",") != "text,image" {
			t.Fatalf("%s CodexInputModalities = %v, want [text image]", id, meta.CodexInputModalities)
		}
		if !meta.SupportsImageOriginal {
			t.Fatalf("%s should support images", id)
		}
	}
}

func TestModelMetadataQwenUsesAnthropic(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	if !UsesAnthropicEndpoint("qwen3.7-max") {
		t.Fatal("qwen3.7-max should use Anthropic endpoint")
	}
	if !Metadata("qwen3.7-max").SupportsSearchTool {
		t.Fatal("qwen3.7-max should support search tool")
	}
}

func TestModelMetadataUsesRemoteEnrichment(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{
			"kimi-k2.6": {
				Name: "Kimi K2.6",
				Modalities: struct {
					Input  []string `json:"input"`
					Output []string `json:"output"`
				}{Input: []string{"text", "image", "video"}, Output: []string{"text"}},
				Limit: struct {
					Context int `json:"context"`
					Output  int `json:"output"`
				}{Context: 262144, Output: 8192},
			},
		},
		nil,
		errors.New("no official"),
	)
	meta := Metadata("kimi-k2.6")
	if meta.DisplayName != "Kimi K2.6" {
		t.Fatalf("DisplayName = %q, want Kimi K2.6", meta.DisplayName)
	}
	if meta.Description != "Kimi K2.6 via OpenCode Go" {
		t.Fatalf("Description = %q, want Kimi K2.6 via OpenCode Go", meta.Description)
	}
	if meta.ContextWindow != 262144 {
		t.Fatalf("ContextWindow = %d, want 262144", meta.ContextWindow)
	}
}

func TestCodexModelCatalogAllowsImagesForKnownVisionModels(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	if !SupportsImages("kimi-k2.6") {
		t.Fatal("kimi-k2.6 should support image inputs")
	}
	if !SupportsImages("minimax-m3") {
		t.Fatal("minimax-m3 should support image inputs")
	}
}

func TestAnthropicEndpointModels(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	for _, model := range []string{"qwen3.7-max", "minimax-m3", "minimax-m2.7", "minimax-m2.5", "opencode-go/qwen3.7-max", "opencode-go/minimax-m3"} {
		if !UsesAnthropicEndpoint(model) {
			t.Fatalf("%s should use Anthropic endpoint", model)
		}
	}
	if UsesAnthropicEndpoint("kimi-k2.6") {
		t.Fatal("kimi-k2.6 should not use Anthropic endpoint")
	}
}

func TestModelsInputModalities(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
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
