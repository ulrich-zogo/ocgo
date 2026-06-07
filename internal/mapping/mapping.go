package mapping

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"ocgo/internal/config"
	"ocgo/internal/models"
)

func DefaultModelMappings() map[string]map[string]string {
	return map[string]map[string]string{
		"claude": {},
		"codex":  {},
	}
}

func LoadModelMappings() (map[string]map[string]string, error) {
	mappings := DefaultModelMappings()
	b, err := os.ReadFile(config.ModelMappingFile())
	if errors.Is(err, os.ErrNotExist) {
		return mappings, nil
	}
	if err != nil {
		return nil, err
	}
	var custom map[string]map[string]string
	if err := json.Unmarshal(b, &custom); err != nil {
		return mappings, nil
	}
	for tool, entries := range custom {
		if mappings[tool] == nil {
			mappings[tool] = map[string]string{}
		}
		for source, target := range entries {
			if strings.TrimSpace(source) != "" && strings.TrimSpace(target) != "" {
				mappings[tool][strings.TrimSpace(source)] = models.NormalizeID(target)
			}
		}
	}
	return mappings, nil
}

func SaveModelMappings(mappings map[string]map[string]string) error {
	if err := os.MkdirAll(config.ConfigDir(), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.ModelMappingFile(), append(b, '\n'), 0644)
}

func ResolveMappedModel(tool, source string, mappings map[string]map[string]string) string {
	source = strings.TrimSpace(models.NormalizeID(source))
	entries := mappings[tool]
	if target := entries[source]; target != "" {
		return target
	}
	if tool == "claude" {
		for _, family := range []string{"opus", "sonnet", "haiku"} {
			if source == family || strings.Contains(source, "claude-"+family) {
				if target := entries["claude-"+family]; target != "" {
					return target
				}
			}
		}
	}
	return source
}

func ResolveToolModel(tool, source string) string {
	mappings, err := LoadModelMappings()
	if err != nil {
		mappings = DefaultModelMappings()
	}
	return ResolveMappedModel(tool, source, mappings)
}

func PrintToolMapping(tool string, mapping map[string]string) {
	fmt.Printf("%s -> OpenCode Go mapping (%s):\n", DisplayToolName(tool), config.ModelMappingFile())
	if len(mapping) == 0 {
		fmt.Println("  (empty)")
		return
	}
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-24s -> %s\n", k, mapping[k])
	}
}

func DisplayToolName(tool string) string {
	if tool == "" {
		return tool
	}
	return strings.ToUpper(tool[:1]) + tool[1:]
}

func PrintLaunchMapping(tool string, mapping map[string]string) {
	if len(mapping) == 0 {
		fmt.Fprintf(os.Stderr, "No OCGO model mappings configured for %s (%s)\n", tool, config.ModelMappingFile())
		return
	}
	fmt.Fprintf(os.Stderr, "OCGO model mapping enabled for %s (%s)\n", tool, config.ModelMappingFile())
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %s -> %s\n", k, mapping[k])
	}
}

func KnownOpenCodeModel(model string) bool {
	model = models.NormalizeID(model)
	for _, id := range models.KnownIDs() {
		if id == model {
			return true
		}
	}
	return false
}
