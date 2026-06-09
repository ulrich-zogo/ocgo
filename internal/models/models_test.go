package models

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func withModelFetchers(t *testing.T, remote map[string]remoteModelInfo, official []OfficialModel, officialErr error) {
	t.Helper()
	oldRemoteModels := remoteModels
	oldOfficialModels := officialModels
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		if remote == nil {
			return nil, errors.New("remote unavailable")
		}
		return remote, nil
	})
	officialModels = newLazyFetcher(func() ([]OfficialModel, error) {
		if officialErr != nil {
			return nil, officialErr
		}
		if official == nil {
			return nil, errors.New("official unavailable")
		}
		return official, nil
	})
	restoreCache := SetCacheFileForTest(filepath.Join(t.TempDir(), "model-catalog-cache.json"))
	t.Cleanup(func() {
		remoteModels = oldRemoteModels
		officialModels = oldOfficialModels
		restoreCache()
	})
}

func officialFromIDs(ids ...string) []OfficialModel {
	out := make([]OfficialModel, len(ids))
	for i, id := range ids {
		out[i] = OfficialModel{
			ID:      id,
			Object:  "model",
			Created: int64(1780792361 + i),
			OwnedBy: "opencode",
		}
	}
	return out
}

func TestKnownIDsPreservesOfficialOrder(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-b": {}, "remote-a": {}},
		officialFromIDs("minimax-m3", "kimi-k2.6", "glm-5.1"),
		nil,
	)
	got := KnownIDs()
	want := []string{"minimax-m3", "kimi-k2.6", "glm-5.1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v (official order must be preserved)", got, want)
	}
}

func TestKnownIDsReturnsRemoteSortedWhenOfficialFails(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-b": {}, "remote-a": {}},
		nil,
		errors.New("official API down"),
	)
	got := KnownIDs()
	want := []string{"remote-a", "remote-b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v (remote must be sorted)", got, want)
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

func TestKnownIDsFallbackContains18Models(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no data"))
	ids := KnownIDs()
	if len(ids) != 18 {
		t.Fatalf("fallback should contain 18 models, got %d: %v", len(ids), ids)
	}
	for _, want := range []string{"minimax-m3", "qwen3.7-plus", "hy3-preview"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in fallback, got %v", want, ids)
		}
	}
}

func TestKnownIDsReturnsRemoteWhenOfficialEmpty(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-a": {}},
		[]OfficialModel{},
		nil,
	)
	got := KnownIDs()
	want := []string{"remote-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v", got, want)
	}
}

func TestKnownIDsOfficialTakesPriorityOverRemote(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-a": {}, "remote-b": {}},
		officialFromIDs("official-a"),
		nil,
	)
	got := KnownIDs()
	want := []string{"official-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("KnownIDs = %v, want %v (official should take priority over remote)", got, want)
	}
}

func TestKnownModelsPreservesOfficialOrder(t *testing.T) {
	withModelFetchers(t,
		nil,
		[]OfficialModel{
			{ID: "minimax-m3", Object: "model", Created: 1780792361, OwnedBy: "opencode"},
			{ID: "kimi-k2.6", Object: "model", Created: 1780792362, OwnedBy: "opencode"},
			{ID: "glm-5.1", Object: "model", Created: 1780792363, OwnedBy: "opencode"},
		},
		nil,
	)
	known := KnownModels()
	if len(known) != 3 {
		t.Fatalf("len(KnownModels) = %d, want 3", len(known))
	}
	if known[0].ID != "minimax-m3" || known[0].Created != 1780792361 || known[0].OwnedBy != "opencode" {
		t.Fatalf("known[0] = %+v, want ID=minimax-m3 Created=1780792361 OwnedBy=opencode", known[0])
	}
	if known[1].ID != "kimi-k2.6" {
		t.Fatalf("known[1].ID = %q, want kimi-k2.6", known[1].ID)
	}
	if known[2].ID != "glm-5.1" {
		t.Fatalf("known[2].ID = %q, want glm-5.1", known[2].ID)
	}
}

func TestKnownModelsFallbackIsStructured(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no data"))
	known := KnownModels()
	if len(known) != 18 {
		t.Fatalf("len(KnownModels) = %d, want 18", len(known))
	}
	if known[0].ID != "minimax-m3" {
		t.Fatalf("known[0].ID = %q, want minimax-m3", known[0].ID)
	}
	if known[0].Object != "model" {
		t.Fatalf("known[0].Object = %q, want model", known[0].Object)
	}
	if known[0].OwnedBy != "opencode" {
		t.Fatalf("known[0].OwnedBy = %q, want opencode", known[0].OwnedBy)
	}
	if known[0].Created != 0 {
		t.Fatalf("known[0].Created = %d, want 0", known[0].Created)
	}
}

func TestKnownModelsRemoteSorted(t *testing.T) {
	withModelFetchers(t,
		map[string]remoteModelInfo{"remote-b": {}, "remote-a": {}},
		nil,
		errors.New("official down"),
	)
	known := KnownModels()
	if len(known) != 2 {
		t.Fatalf("len = %d, want 2", len(known))
	}
	if known[0].ID != "remote-a" || known[1].ID != "remote-b" {
		t.Fatalf("known = %v, want sorted", known)
	}
	for _, m := range known {
		if m.Object != "model" || m.OwnedBy != "opencode" || m.Created != 0 {
			t.Fatalf("remote known model should have defaults: %+v", m)
		}
	}
}

func TestOfficialModelsReturnsCopy(t *testing.T) {
	official := []OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
	}
	withModelFetchers(t, nil, official, nil)
	got, err := OfficialModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "minimax-m3" {
		t.Fatalf("OfficialModels = %+v, want one model", got)
	}
	got[0].ID = "modified"
	got2, _ := OfficialModels()
	if got2[0].ID != "minimax-m3" {
		t.Fatal("OfficialModels should return a defensive copy")
	}
}

func TestOfficialModelIDs(t *testing.T) {
	official := officialFromIDs("a", "b", "c")
	withModelFetchers(t, nil, official, nil)
	ids, err := OfficialModelIDs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("OfficialModelIDs = %v, want %v", ids, want)
	}
}

func TestFetchOfficialModelsParsesAllFields(t *testing.T) {
	body := `{
		"object": "list",
		"data": [
			{"id": "minimax-m3", "object": "model", "created": 1780792361, "owned_by": "opencode"},
			{"id": "kimi-k2.6", "object": "model", "created": 1780792361, "owned_by": "opencode"}
		]
	}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "minimax-m3" || got[0].Object != "model" || got[0].Created != 1780792361 || got[0].OwnedBy != "opencode" {
		t.Fatalf("got[0] = %+v", got[0])
	}
	if got[1].ID != "kimi-k2.6" {
		t.Fatalf("got[1].ID = %q", got[1].ID)
	}
}

func TestFetchOfficialModelsPreservesOrder(t *testing.T) {
	body := `{
		"object": "list",
		"data": [
			{"id":"minimax-m3","object":"model","created":1,"owned_by":"opencode"},
			{"id":"kimi-k2.6","object":"model","created":1,"owned_by":"opencode"},
			{"id":"glm-5.1","object":"model","created":1,"owned_by":"opencode"}
		]
	}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"minimax-m3", "kimi-k2.6", "glm-5.1"}
	for i, m := range got {
		if m.ID != want[i] {
			t.Fatalf("got[%d].ID = %q, want %q", i, m.ID, want[i])
		}
	}
}

func TestFetchOfficialModelsDeduplicatesPreservingFirst(t *testing.T) {
	body := `{
		"object": "list",
		"data": [
			{"id":"minimax-m3","object":"model","created":1,"owned_by":"opencode"},
			{"id":"kimi-k2.6","object":"model","created":1,"owned_by":"opencode"},
			{"id":"minimax-m3","object":"model","created":2,"owned_by":"duplicate"}
		]
	}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (duplicates removed): %+v", len(got), got)
	}
	if got[0].ID != "minimax-m3" || got[0].Created != 1 {
		t.Fatalf("first occurrence should be preserved, got %+v", got[0])
	}
	if got[1].ID != "kimi-k2.6" {
		t.Fatalf("got[1].ID = %q, want kimi-k2.6", got[1].ID)
	}
}

func TestFetchOfficialModelsNormalizesOpencodeGoPrefix(t *testing.T) {
	body := `{"object":"list","data":[{"id":"opencode-go/minimax-m3","object":"model","created":1,"owned_by":"opencode"}]}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "minimax-m3" {
		t.Fatalf("ID = %q, want minimax-m3", got[0].ID)
	}
}

func TestFetchOfficialModelsSkipsEmptyIDs(t *testing.T) {
	body := `{"object":"list","data":[{"id":"   ","object":"model","created":1,"owned_by":"opencode"},{"id":"kimi-k2.6","object":"model","created":1,"owned_by":"opencode"}]}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "kimi-k2.6" {
		t.Fatalf("got = %+v, want only kimi-k2.6", got)
	}
}

func TestFetchOfficialModelsDefaultsObjectAndOwnedBy(t *testing.T) {
	body := `{"object":"list","data":[{"id":"minimax-m3","created":1}]}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Object != "model" {
		t.Fatalf("Object = %q, want model", got[0].Object)
	}
	if got[0].OwnedBy != "opencode" {
		t.Fatalf("OwnedBy = %q, want opencode", got[0].OwnedBy)
	}
}

func TestFetchOfficialModelsEmptyData(t *testing.T) {
	body := `{"object":"list","data":[]}`
	got, err := parseOfficialModelsBody([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
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

func TestModelMetadataNewFallbackModels(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	for _, id := range []string{"qwen3.7-plus", "hy3-preview"} {
		meta := Metadata(id)
		if len(meta.InputModalities) == 0 {
			t.Fatalf("%s should have input modalities", id)
		}
		if meta.ContextWindow == 0 {
			t.Fatalf("%s should have a context window", id)
		}
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

func TestIsKnownWithNormalization(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no remote"))
	if !IsKnown("opencode-go/minimax-m3") {
		t.Fatal("opencode-go/ prefix should be normalized for IsKnown")
	}
}

func TestKnownIDsFallbackOrderIsStable(t *testing.T) {
	withModelFetchers(t, nil, nil, errors.New("no data"))
	got := KnownIDs()
	want := []string{
		"minimax-m3", "minimax-m2.7", "minimax-m2.5",
		"kimi-k2.6", "kimi-k2.5",
		"glm-5.1", "glm-5",
		"deepseek-v4-pro", "deepseek-v4-flash",
		"qwen3.7-max", "qwen3.7-plus", "qwen3.6-plus", "qwen3.5-plus",
		"mimo-v2-pro", "mimo-v2-omni", "mimo-v2.5-pro", "mimo-v2.5",
		"hy3-preview",
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("got[%d] = %q, want %q (full: %v)", i, got[i], id, got)
		}
	}
	sortedCopy := append([]string(nil), want...)
	sort.Strings(sortedCopy)
	if sort.StringsAreSorted(got) {
		t.Fatal("fallback should NOT be sorted, order must be preserved as defined")
	}
	_ = sortedCopy
}

func TestWriteCatalogCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	when := time.Date(2026, 6, 8, 15, 30, 0, 0, time.UTC)
	in := []OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1780792361, OwnedBy: "opencode"},
	}
	if err := WriteCatalogCache(path, in, sourceOfficial, when); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cache CatalogCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cache.Version != catalogCacheVersion {
		t.Fatalf("version = %d, want %d", cache.Version, catalogCacheVersion)
	}
	if cache.Source != sourceOfficial {
		t.Fatalf("source = %q, want %q", cache.Source, sourceOfficial)
	}
	if !cache.FetchedAt.Equal(when) {
		t.Fatalf("fetched_at = %v, want %v", cache.FetchedAt, when)
	}
	if len(cache.Models) != 1 || cache.Models[0].ID != "minimax-m3" {
		t.Fatalf("models = %+v", cache.Models)
	}
	if cache.Models[0].Created != 1780792361 {
		t.Fatalf("created = %d, want 1780792361", cache.Models[0].Created)
	}
}

func TestReadCatalogCachePreservesOrderAndFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{
		"version": 1,
		"source": "official",
		"fetched_at": "2026-06-08T15:30:00Z",
		"models": [
			{"id":"minimax-m3","object":"model","created":1780792361,"owned_by":"opencode"},
			{"id":"kimi-k2.6","object":"model","created":1780792361,"owned_by":"opencode"}
		]
	}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCatalogCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "minimax-m3" || got[1].ID != "kimi-k2.6" {
		t.Fatalf("order = %+v", got)
	}
	if got[0].Created != 1780792361 || got[0].OwnedBy != "opencode" {
		t.Fatalf("got[0] = %+v", got[0])
	}
}

func TestReadCatalogCacheReturnsErrorWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	if _, err := ReadCatalogCache(path); err == nil {
		t.Fatal("expected error for missing cache file")
	}
}

func TestReadCatalogCacheReturnsErrorForCorruptedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCatalogCache(path); err == nil {
		t.Fatal("expected error for corrupted cache")
	}
}

func TestReadCatalogCacheIgnoresIncompatibleVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{"version":999,"source":"official","fetched_at":"2026-06-08T15:30:00Z","models":[{"id":"minimax-m3"}]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCatalogCache(path); err == nil {
		t.Fatal("expected error for incompatible version")
	}
}

func TestReadCatalogCacheIgnoresEmptyModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{"version":1,"source":"official","fetched_at":"2026-06-08T15:30:00Z","models":[]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCatalogCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0 (empty cache returns empty slice, not used)", len(got))
	}
}

func TestReadCatalogCacheNormalizesAndDedups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{
		"version": 1,
		"source": "official",
		"fetched_at": "2026-06-08T15:30:00Z",
		"models": [
			{"id":"opencode-go/minimax-m3"},
			{"id":"minimax-m3"},
			{"id":" "},
			{"id":"kimi-k2.6","owned_by":""}
		]
	}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCatalogCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (full=%+v)", len(got), got)
	}
	if got[0].ID != "minimax-m3" || got[1].ID != "kimi-k2.6" {
		t.Fatalf("order = %+v", got)
	}
	for _, m := range got {
		if m.Object != "model" {
			t.Fatalf("%s object = %q, want model", m.ID, m.Object)
		}
		if m.OwnedBy != "opencode" {
			t.Fatalf("%s owned_by = %q, want opencode", m.ID, m.OwnedBy)
		}
	}
}

func TestKnownModelsWithSourceReturnsOfficialAndWritesCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	oldRemoteModels := remoteModels
	oldOfficialModels := officialModels
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		return nil, errors.New("no remote")
	})
	officialModels = newLazyFetcher(func() ([]OfficialModel, error) {
		return []OfficialModel{
			{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		}, nil
	})
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(func() {
		remoteModels = oldRemoteModels
		officialModels = oldOfficialModels
		restoreCache()
	})

	models, source := KnownModelsWithSource()
	if source != sourceOfficial {
		t.Fatalf("source = %q, want %q", source, sourceOfficial)
	}
	if len(models) != 1 || models[0].ID != "minimax-m3" {
		t.Fatalf("models = %+v", models)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file should exist after official success: %v", err)
	}
	data, _ := os.ReadFile(cachePath)
	var cache CatalogCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatal(err)
	}
	if len(cache.Models) != 1 || cache.Models[0].ID != "minimax-m3" {
		t.Fatalf("cache models = %+v", cache.Models)
	}
	if cache.Source != sourceOfficial {
		t.Fatalf("cache source = %q, want %q", cache.Source, sourceOfficial)
	}
}

func TestKnownModelsWithSourceUsesCacheWhenOfficialFails(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{
		"version": 1,
		"source": "official",
		"fetched_at": "2026-06-08T15:30:00Z",
		"models": [
			{"id":"minimax-m3","object":"model","created":1,"owned_by":"opencode"},
			{"id":"kimi-k2.6","object":"model","created":2,"owned_by":"opencode"}
		]
	}`
	if err := os.WriteFile(cachePath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	oldRemoteModels := remoteModels
	oldOfficialModels := officialModels
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		return map[string]remoteModelInfo{"remote-a": {}}, nil
	})
	officialModels = newLazyFetcher(func() ([]OfficialModel, error) {
		return nil, errors.New("no official")
	})
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(func() {
		remoteModels = oldRemoteModels
		officialModels = oldOfficialModels
		restoreCache()
	})

	got, source := KnownModelsWithSource()
	if source != sourceCache {
		t.Fatalf("source = %q, want %q", source, sourceCache)
	}
	if len(got) != 2 || got[0].ID != "minimax-m3" || got[1].ID != "kimi-k2.6" {
		t.Fatalf("got = %+v, want cached order preserved", got)
	}
}

func TestKnownModelsWithSourceUsesRemoteWhenOfficialFailsAndCacheAbsent(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	withModelFetchers(t, map[string]remoteModelInfo{"remote-b": {}, "remote-a": {}}, nil, errors.New("no official"))
	got, source := KnownModelsWithSource()
	if source != sourceRemote {
		t.Fatalf("source = %q, want %q", source, sourceRemote)
	}
	if len(got) != 2 || got[0].ID != "remote-a" || got[1].ID != "remote-b" {
		t.Fatalf("got = %+v, want remote sorted", got)
	}
}

func TestKnownModelsWithSourceFallsBackWhenAllFail(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	withModelFetchers(t, nil, nil, errors.New("no official"))
	got, source := KnownModelsWithSource()
	if source != sourceFallback {
		t.Fatalf("source = %q, want %q", source, sourceFallback)
	}
	if len(got) != 18 {
		t.Fatalf("len = %d, want 18", len(got))
	}
	if got[0].ID != "minimax-m3" {
		t.Fatalf("got[0].ID = %q, want minimax-m3", got[0].ID)
	}
	found := false
	for _, m := range got {
		if m.ID == "hy3-preview" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("hy3-preview missing in fallback")
	}
}

func TestKnownModelsWithSourceIgnoresCorruptedCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	if err := os.WriteFile(cachePath, []byte("{not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	withModelFetchers(t, map[string]remoteModelInfo{"remote-a": {}}, nil, errors.New("no official"))
	got, source := KnownModelsWithSource()
	if source != sourceRemote {
		t.Fatalf("source = %q, want %q (corrupted cache must be ignored)", source, sourceRemote)
	}
	if len(got) != 1 || got[0].ID != "remote-a" {
		t.Fatalf("got = %+v, want remote-a", got)
	}
}

func TestKnownModelsWithSourceIgnoresEmptyCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{"version":1,"source":"official","fetched_at":"2026-06-08T15:30:00Z","models":[]}`
	if err := os.WriteFile(cachePath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	withModelFetchers(t, map[string]remoteModelInfo{"remote-a": {}}, nil, errors.New("no official"))
	_, source := KnownModelsWithSource()
	if source != sourceRemote {
		t.Fatalf("source = %q, want %q (empty cache must be ignored)", source, sourceRemote)
	}
}

func TestKnownModelsWithSourceIgnoresIncompatibleVersion(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	body := `{"version":999,"source":"official","fetched_at":"2026-06-08T15:30:00Z","models":[{"id":"minimax-m3"}]}`
	if err := os.WriteFile(cachePath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	withModelFetchers(t, map[string]remoteModelInfo{"remote-a": {}}, nil, errors.New("no official"))
	_, source := KnownModelsWithSource()
	if source != sourceRemote {
		t.Fatalf("source = %q, want %q (incompatible version must be ignored)", source, sourceRemote)
	}
}

func TestRefreshOfficialModelsWritesCacheOnSuccess(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "model-catalog-cache.json")
	restoreCache := SetCacheFileForTest(cachePath)
	t.Cleanup(restoreCache)
	oldRemote := remoteModels
	oldOfficial := officialModels
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		return nil, errors.New("no remote")
	})
	officialModels = newLazyFetcher(func() ([]OfficialModel, error) {
		return []OfficialModel{
			{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		}, nil
	})
	t.Cleanup(func() {
		remoteModels = oldRemote
		officialModels = oldOfficial
	})

	if err := RefreshOfficialModels(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file should exist after RefreshOfficialModels success: %v", err)
	}
}
