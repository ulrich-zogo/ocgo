package codex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ocgo/internal/config"
	"ocgo/internal/models"
)

func TestWriteCodexProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := WriteProfile(path, "http://127.0.0.1:3456/v1/"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "ocgo-launch.config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	for _, want := range []string{
		`openai_base_url = "http://127.0.0.1:3456/v1/"`,
		`forced_login_method = "api"`,
		`model_provider = "ocgo-launch"`,
		`model_reasoning_effort = "minimal"`,
		`model_reasoning_summary = "none"`,
		"[model_providers.ocgo-launch]",
		`name = "OpenCode Go"`,
		`base_url = "http://127.0.0.1:3456/v1/"`,
		`wire_api = "responses"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in:\n%s", want, content)
		}
	}
	if strings.Contains(content, "[profiles.ocgo-launch]") {
		t.Fatalf("new Codex profile file must not contain legacy [profiles] table:\n%s", content)
	}
	b, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != "" {
		t.Fatalf("root Codex config should not contain ocgo legacy profile entries:\n%s", string(b))
	}
}

func TestWriteCodexProfileMigratesLegacySections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	existing := "profile = \"ocgo-launch\"\nkeep = \"top\"\n\n[profiles.ocgo-launch]\nopenai_base_url = \"http://old/v1/\"\n\n[profiles.ocgo-launch.features]\nmemories = false\n\n[other]\nkey = \"value\"\n\n[model_providers.ocgo-launch]\nbase_url = \"http://old/v1/\"\n\n[model_providers.ocgo-launch.headers]\nfoo = \"bar\"\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteProfile(path, "http://new/v1/"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	content := string(b)
	for _, gone := range []string{"http://old", `profile = "ocgo-launch"`} {
		if strings.Contains(content, gone) {
			t.Fatalf("legacy Codex profile config %q was not removed:\n%s", gone, content)
		}
	}
	for _, gone := range []string{"[profiles.ocgo-launch]", "[profiles.ocgo-launch.features]", "[model_providers.ocgo-launch]", "[model_providers.ocgo-launch.headers]", `openai_base_url = "http://new/v1/"`} {
		if strings.Contains(content, gone) {
			t.Fatalf("legacy Codex profile config %q was re-added:\n%s", gone, content)
		}
	}
	if !strings.Contains(content, `keep = "top"`) || !strings.Contains(content, "[other]") || !strings.Contains(content, `key = "value"`) {
		t.Fatalf("unrelated section was not preserved:\n%s", content)
	}
	profile, _ := os.ReadFile(filepath.Join(dir, "ocgo-launch.config.toml"))
	if !strings.Contains(string(profile), `openai_base_url = "http://new/v1/"`) || !strings.Contains(string(profile), "[model_providers.ocgo-launch]") {
		t.Fatalf("new profile file was not written correctly:\n%s", string(profile))
	}
}

func TestWriteCodexModelCatalog(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })

	path := filepath.Join(t.TempDir(), "ocgo-models.json")
	if err := WriteModelCatalog(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	for _, want := range []string{`"models"`, `"slug"`, `"truncation_policy"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in:\n%s", want, content)
		}
	}
	var catalog struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(b, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) == 0 {
		t.Fatal("catalog has no models")
	}
}

func TestCompareVersions(t *testing.T) {
	if CompareVersions("0.80.9", "0.81.0") >= 0 {
		t.Fatal("0.80.9 should be older")
	}
	if CompareVersions("0.81.0", "0.81.0") != 0 {
		t.Fatal("same versions should compare equal")
	}
	if CompareVersions("codex-cli", "0.81.0") >= 0 {
		t.Fatal("invalid version should compare as old")
	}
	if CompareVersions("0.87.0", "0.81.0") <= 0 {
		t.Fatal("0.87.0 should be newer")
	}
}

func TestWriteCodexModelCatalogRespectsOfficialOrder(t *testing.T) {
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return filepath.Join(t.TempDir(), "model-mapping.json") }
	t.Cleanup(func() { config.ModelMappingFile = old })

	models.ResetFetchersForTest()
	official := []models.OfficialModel{
		{ID: "minimax-m3", Object: "model", Created: 1, OwnedBy: "opencode"},
		{ID: "kimi-k2.6", Object: "model", Created: 2, OwnedBy: "opencode"},
		{ID: "glm-5.1", Object: "model", Created: 3, OwnedBy: "opencode"},
	}
	models.SetFetchersForTest(nil, official, nil, nil)
	t.Cleanup(func() { models.ResetFetchersForTest() })

	path := filepath.Join(t.TempDir(), "ocgo-models.json")
	if err := WriteModelCatalog(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if err := json.Unmarshal(b, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Models) < 3 {
		t.Fatalf("catalog should have at least 3 models, got %d", len(catalog.Models))
	}
	if catalog.Models[0].Slug != "minimax-m3" {
		t.Fatalf("catalog.Models[0].Slug = %q, want minimax-m3 (official order)", catalog.Models[0].Slug)
	}
	if catalog.Models[1].Slug != "kimi-k2.6" {
		t.Fatalf("catalog.Models[1].Slug = %q, want kimi-k2.6", catalog.Models[1].Slug)
	}
	if catalog.Models[2].Slug != "glm-5.1" {
		t.Fatalf("catalog.Models[2].Slug = %q, want glm-5.1", catalog.Models[2].Slug)
	}
}

func TestWriteCodexModelCatalogIncludesMappingsAfterKnown(t *testing.T) {
	mappingPath := filepath.Join(t.TempDir(), "model-mapping.json")
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return mappingPath }
	t.Cleanup(func() { config.ModelMappingFile = old })

	models.ResetFetchersForTest()
	models.SetFetchersForTest(nil, nil, errors.New("no official"), errors.New("no remote"))
	t.Cleanup(func() { models.ResetFetchersForTest() })

	mappingsContent := `{
		"claude": {},
		"codex": {"gpt-5": "deepseek-v4-pro", "gpt-4": "kimi-k2.6"}
	}`
	if err := os.WriteFile(mappingPath, []byte(mappingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "ocgo-models.json")
	if err := WriteModelCatalog(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var catalog struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if err := json.Unmarshal(b, &catalog); err != nil {
		t.Fatal(err)
	}

	knownIDs := models.KnownIDs()
	knownCount := len(knownIDs)
	if knownCount != 18 {
		t.Fatalf("expected 18 known models from fallback, got %d", knownCount)
	}

	if len(catalog.Models) < knownCount+2 {
		t.Fatalf("catalog should have at least %d models (18 known + 2 mappings), got %d", knownCount+2, len(catalog.Models))
	}

	for i, m := range catalog.Models {
		if m.Slug == "gpt-4" || m.Slug == "gpt-5" {
			if i < knownCount {
				slugs := []string{}
				for _, x := range catalog.Models {
					slugs = append(slugs, x.Slug)
				}
				t.Fatalf("mapping %s should appear after known models (at index >= %d), found at index %d\norder: %v", m.Slug, knownCount, i, slugs)
			}
		}
	}

	for i := 0; i < knownCount; i++ {
		if catalog.Models[i].Slug != knownIDs[i] {
			t.Fatalf("known model at index %d = %q, want %q (fallback order must be preserved)", i, catalog.Models[i].Slug, knownIDs[i])
		}
	}

	if catalog.Models[knownCount].Slug != "gpt-4" || catalog.Models[knownCount+1].Slug != "gpt-5" {
		slugs := []string{}
		for _, x := range catalog.Models[knownCount:] {
			slugs = append(slugs, x.Slug)
		}
		t.Fatalf("expected [gpt-4, gpt-5] after known models (alphabetical), got %v", slugs)
	}

	for _, want := range []string{"qwen3.7-plus", "hy3-preview"} {
		found := false
		for i := 0; i < knownCount; i++ {
			if catalog.Models[i].Slug == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in the first %d fallback positions", want, knownCount)
		}
	}

	content := string(b)
	if !strings.Contains(content, "OCGO mapping to deepseek-v4-pro") {
		t.Fatalf("catalog should contain mapping description:\n%s", content)
	}
}
