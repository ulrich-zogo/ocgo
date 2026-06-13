package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

func (m Manager) WriteModelCatalog() error {
	if strings.TrimSpace(m.Paths.ModelCatalogFile) == "" {
		return fmt.Errorf("codex model catalog path is required")
	}
	return writeModelCatalogTo(m.Paths.ModelCatalogFile)
}

func writeModelCatalogTo(path string) error {
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
