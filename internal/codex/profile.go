package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ocgo/internal/config"
)

func (m Manager) WriteCLIProfile(baseURL string) error {
	profilePath := m.profileFile()
	catalogPath := m.Paths.ModelCatalogFile
	if catalogPath == "" {
		catalogPath = config.CodexModelCatalogFile()
	}
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
	if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(profilePath, []byte(profileText), 0644); err != nil {
		return err
	}
	b, err := os.ReadFile(m.Paths.ConfigFile)
	text := ""
	if err == nil {
		text = string(b)
	} else if !os.IsNotExist(err) {
		return err
	}
	cleaned := StripLegacyProfile(text)
	if m.Paths.ConfigFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.Paths.ConfigFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(m.Paths.ConfigFile, []byte(cleaned), 0644)
}

func (m Manager) profileFile() string {
	if m.Paths.ProfileFile != "" {
		return m.Paths.ProfileFile
	}
	return config.CodexProfileConfigFile()
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
