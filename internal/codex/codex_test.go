package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ocgo/internal/config"
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
