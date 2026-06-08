package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"ocgo/internal/config"
	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

func EnsureConfig(base string) error {
	path := config.CodexConfigFile()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := WriteModelCatalog(config.CodexModelCatalogFile()); err != nil {
		return err
	}
	return WriteProfile(path, strings.TrimRight(base, "/")+"/v1/")
}

func WriteProfile(path, baseURL string) error {
	profilePath := filepath.Join(filepath.Dir(path), config.CodexProfileName+".config.toml")
	catalogPath := config.CodexModelCatalogFile()
	profileText := strings.Join([]string{
		fmt.Sprintf("openai_base_url = %q", baseURL),
		`forced_login_method = "api"`,
		fmt.Sprintf("model_provider = %q", config.CodexProfileName),
		fmt.Sprintf("model_catalog_json = %q", catalogPath),
		`model_reasoning_effort = "minimal"`,
		`model_reasoning_summary = "none"`,
		"",
		fmt.Sprintf("[model_providers.%s]", config.CodexProfileName),
		`name = "OpenCode Go"`,
		fmt.Sprintf("base_url = %q", baseURL),
		`wire_api = "responses"`,
		"",
	}, "\n")
	if err := os.WriteFile(profilePath, []byte(profileText), 0644); err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	text := ""
	if err == nil {
		text = string(b)
	} else if !os.IsNotExist(err) {
		return err
	}
	cleaned := StripLegacyProfile(text)
	return os.WriteFile(path, []byte(cleaned), 0644)
}

func StripLegacyProfile(text string) string {
	var out []string
	inRemovedSection := false
	currentSection := ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed
			inRemovedSection = IsLegacySection(currentSection)
			if inRemovedSection {
				continue
			}
		}
		if inRemovedSection {
			continue
		}
		if currentSection == "" && strings.HasPrefix(trimmed, "profile") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "profile" && strings.Trim(strings.TrimSpace(parts[1]), `"'`) == config.CodexProfileName {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.TrimLeft(strings.Join(out, "\n"), "\n")
}

func IsLegacySection(section string) bool {
	profiles := fmt.Sprintf("[profiles.%s", config.CodexProfileName)
	providers := fmt.Sprintf("[model_providers.%s", config.CodexProfileName)
	return section == fmt.Sprintf("[profiles.%s]", config.CodexProfileName) ||
		strings.HasPrefix(section, profiles+".") ||
		section == fmt.Sprintf("[model_providers.%s]", config.CodexProfileName) ||
		strings.HasPrefix(section, providers+".")
}

func WriteModelCatalog(path string) error {
	mappings, err := mapping.LoadModelMappings()
	if err != nil {
		mappings = mapping.DefaultModelMappings()
	}
	knownIDs := models.KnownIDs()
	modelsArr := make([]map[string]any, 0, len(knownIDs)+len(mappings["codex"]))
	seen := map[string]bool{}
	addModel := func(id, target, description string, i int) {
		if seen[id] {
			return
		}
		seen[id] = true
		meta := models.Metadata(target)
		displayName := id
		if id == target {
			displayName = meta.DisplayName
		}
		modelsArr = append(modelsArr, map[string]any{
			"slug":                             id,
			"display_name":                     displayName,
			"description":                      description,
			"default_reasoning_level":          meta.DefaultReasoningLevel,
			"supported_reasoning_levels":       meta.SupportedReasoning,
			"shell_type":                       "shell_command",
			"visibility":                       "list",
			"supported_in_api":                 true,
			"priority":                         i,
			"availability_nux":                 nil,
			"upgrade":                          nil,
			"base_instructions":                "You are Codex, a coding agent running in a terminal-based coding assistant.",
			"supports_reasoning_summaries":     meta.ReasoningSummaries,
			"default_reasoning_summary":        meta.DefaultReasoningSummary,
			"support_verbosity":                false,
			"default_verbosity":                nil,
			"apply_patch_tool_type":            nil,
			"web_search_tool_type":             "text",
			"truncation_policy":                map[string]any{"mode": "tokens", "limit": 10000},
			"supports_parallel_tool_calls":     meta.ParallelToolCalls,
			"supports_image_detail_original":   meta.SupportsImageOriginal,
			"context_window":                   meta.ContextWindow,
			"max_context_window":               meta.MaxContextWindow,
			"auto_compact_token_limit":         nil,
			"effective_context_window_percent": 95,
			"experimental_supported_tools":     []any{},
			"input_modalities":                 meta.CodexInputModalities,
			"supports_search_tool":             meta.SupportsSearchTool,
		})
	}
	for i, id := range knownIDs {
		addModel(id, id, models.Metadata(id).Description, i)
	}
	keys := make([]string, 0, len(mappings["codex"]))
	for source := range mappings["codex"] {
		keys = append(keys, source)
	}
	sort.Strings(keys)
	for i, source := range keys {
		target := mappings["codex"][source]
		addModel(source, target, "OCGO mapping to "+target, len(knownIDs)+i)
	}
	b, err := json.MarshalIndent(map[string]any{"models": modelsArr}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

func CheckVersion() error {
	if _, err := exec.LookPath("codex"); err != nil {
		return fmt.Errorf("codex is not installed, install with: npm install -g @openai/codex")
	}
	out, err := exec.Command("codex", "--version").Output()
	if err != nil {
		return fmt.Errorf("failed to get codex version: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return fmt.Errorf("unexpected codex version output: %s", string(out))
	}
	version := fields[len(fields)-1]
	if CompareVersions(version, "0.81.0") < 0 {
		return fmt.Errorf("codex version %s is too old, minimum required is 0.81.0; update with: npm update -g @openai/codex", version)
	}
	return nil
}

func CompareVersions(a, b string) int {
	ap, bp := VersionParts(a), VersionParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func VersionParts(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	fields := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < len(fields) && i < 3; i++ {
		part := fields[i]
		for j, r := range part {
			if r < '0' || r > '9' {
				part = part[:j]
				break
			}
		}
		out[i], _ = strconv.Atoi(part)
	}
	return out
}
