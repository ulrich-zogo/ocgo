package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"ocgo/internal/codex"
)

// codexCLIChecks returns the Codex CLI checks:
//   - codex binary on PATH
//   - CLI profile (~/.codex/ocgo-launch.config.toml)
//   - CLI model catalog (~/.codex/ocgo-models.json)
//
// The doctor never executes codex itself: it shells out to
// codex --version through the existing codex.NewManager
// abstraction. The actual exec is not part of these tests
// because it is hooked through codex/execLookPath and
// codex/execCommandOutput.
func (r Runner) codexCLIChecks() []Check {
	return []Check{
		r.checkCodexBinary(),
		r.checkCLIProfile(),
		r.checkCLIModelCatalog(),
	}
}

// checkCodexBinary locates the codex executable and reports
// its version. The version comparison uses the existing
// codex.CompareVersions helper against the minimum version
// configured in internal/codex.
func (r Runner) checkCodexBinary() Check {
	id := "codex.cli.binary"
	if err := codexCheckVersion(); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not installed") {
			return Error(id, "Codex CLI binary",
				"codex not found on PATH",
				"Install Codex CLI: npm install -g @openai/codex")
		}
		if strings.Contains(msg, "too old") {
			return Warning(id, "Codex CLI binary",
				msg,
				"Update Codex CLI: npm update -g @openai/codex")
		}
		return Error(id, "Codex CLI binary", msg,
			"Install or update Codex CLI: npm install -g @openai/codex")
	}
	return OK(id, "Codex CLI binary", "codex found on PATH")
}

// checkCLIProfile verifies that the CLI profile file exists
// and contains the keys required to point codex at OCGO.
func (r Runner) checkCLIProfile() Check {
	id := "codex.cli.profile"
	path := r.Paths.CodexProfileFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Codex CLI profile",
				"profile not found: "+path,
				"Run: ocgo launch codex --config")
		}
		return Error(id, "Codex CLI profile",
			"failed to read "+path+": "+err.Error(),
			"Run: ocgo launch codex --config")
	}
	text := string(b)
	missing := missingCLIProfileFields(text)
	if len(missing) > 0 {
		return Error(id, "Codex CLI profile",
			"profile is missing required keys: "+strings.Join(missing, ", "),
			"Run: ocgo launch codex --config")
	}
	return OK(id, "Codex CLI profile", path)
}

// checkCLIModelCatalog verifies the CLI model catalog.
func (r Runner) checkCLIModelCatalog() Check {
	id := "codex.cli.catalog"
	path := r.Paths.CodexCatalogFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Codex CLI model catalog",
				"catalog not found: "+path,
				"Run: ocgo launch codex --config")
		}
		return Error(id, "Codex CLI model catalog",
			"failed to read "+path+": "+err.Error(),
			"Run: ocgo launch codex --config")
	}
	var list []map[string]any
	if err := json.Unmarshal(b, &list); err != nil {
		return Error(id, "Codex CLI model catalog",
			"invalid JSON: "+err.Error(),
			"Run: ocgo launch codex --config")
	}
	if len(list) == 0 {
		return Warning(id, "Codex CLI model catalog",
			"catalog is empty: "+path,
			"Run: ocgo launch codex --config")
	}
	// Cross-check the effective model is in the catalog.
	model, _, _ := getDefaultModel()
	if model != "" {
		found := false
		for _, item := range list {
			if id, _ := item["id"].(string); id == model {
				found = true
				break
			}
		}
		if !found {
			return Warning(id, "Codex CLI model catalog",
				fmt.Sprintf("effective model %q is not in the catalog", model),
				"Run: ocgo launch codex --config")
		}
	}
	return OK(id, "Codex CLI model catalog", path)
}

// codexDesktopChecks returns the Codex Desktop checks.
func (r Runner) codexDesktopChecks() []Check {
	return []Check{
		r.checkDesktopConfig(),
		r.checkDesktopState(),
		r.checkDesktopBackup(),
	}
}

// checkDesktopConfig inspects ~/.codex/config.toml. The
// expected OCGO shape is:
//
//	model_provider = "ocgo-desktop"
//	[model_providers.ocgo-desktop]
//	base_url = "http://host:port/v1/"
//	env_key = "OPENAI_API_KEY"
//	wire_api = "responses"
//	model_catalog_json = "<path>"
//
// The doctor only inspects the file; it does not modify it.
func (r Runner) checkDesktopConfig() Check {
	id := "codex.desktop.config"
	path := r.Paths.CodexConfigFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Codex Desktop config",
				"config not found: "+path,
				"Run: ocgo codex desktop enable opencode")
		}
		return Error(id, "Codex Desktop config",
			"failed to read "+path+": "+err.Error(),
			"")
	}
	text := string(b)
	missing := missingDesktopConfigFields(text)
	if len(missing) > 0 {
		// Detect the ChatGPT/OpenAI shape and report a
		// helpful remediation.
		if looksLikeChatGPTConfig(text) {
			return Warning(id, "Codex Desktop config",
				"Desktop is currently in ChatGPT/OpenAI mode",
				"Run: ocgo codex desktop enable opencode")
		}
		return Warning(id, "Codex Desktop config",
			"config does not look like an OCGO Desktop profile (missing: "+strings.Join(missing, ", ")+")",
			"Run: ocgo codex desktop enable opencode")
	}
	return OK(id, "Codex Desktop config", path)
}

// checkDesktopState inspects the OCGO state file.
func (r Runner) checkDesktopState() Check {
	id := "codex.desktop.state"
	path := r.Paths.DesktopStateFile
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Codex Desktop state",
				"state not found: "+path,
				"Run: ocgo codex desktop enable opencode or ocgo codex desktop enable chatgpt")
		}
		return Error(id, "Codex Desktop state",
			"failed to read "+path+": "+err.Error(),
			"")
	}
	var s codex.DesktopState
	if err := json.Unmarshal(b, &s); err != nil {
		return Error(id, "Codex Desktop state",
			"invalid JSON: "+err.Error(),
			"")
	}
	if s.Version != codex.DesktopStateVersion {
		return Error(id, "Codex Desktop state",
			fmt.Sprintf("unsupported state version %d (expected %d)", s.Version, codex.DesktopStateVersion),
			"Run: ocgo codex desktop enable opencode")
	}
	switch s.Mode {
	case codex.DesktopModeOpenCode:
		return OK(id, "Codex Desktop state", "mode=opencode")
	case codex.DesktopModeChatGPT:
		return OK(id, "Codex Desktop state", "mode=chatgpt")
	default:
		return Error(id, "Codex Desktop state",
			"unknown mode: "+string(s.Mode),
			"Run: ocgo codex desktop enable opencode")
	}
}

// checkDesktopBackup verifies the backup referenced by the
// state file (if any) is present and non-empty.
func (r Runner) checkDesktopBackup() Check {
	id := "codex.desktop.backup"
	st, err := codex.ReadDesktopState(r.Paths.DesktopStateFile)
	if err != nil {
		// If the state file is missing, surface a "no
		// backup referenced" warning so the user can still
		// see what is going on.
		if errors.Is(err, os.ErrNotExist) {
			return Warning(id, "Codex Desktop backup",
				"no state file, no backup referenced",
				"Run: ocgo codex desktop enable opencode")
		}
		return Warning(id, "Codex Desktop backup",
			"state file unreadable: "+err.Error(),
			"")
	}
	if st.BackupFile == "" {
		// A ChatGPT state without a backup is normal only
		// if the user never had a Desktop config to back
		// up. For OpenCode, a missing backup is a warning
		// because we cannot restore the previous setup.
		if st.Mode == codex.DesktopModeOpenCode {
			return Warning(id, "Codex Desktop backup",
				"no backup recorded in OpenCode state",
				"")
		}
		return OK(id, "Codex Desktop backup", "no backup required in chatgpt mode")
	}
	info, err := os.Stat(st.BackupFile)
	if err != nil {
		return Error(id, "Codex Desktop backup",
			"backup file is missing: "+st.BackupFile,
			"Run: ocgo codex desktop enable opencode to recreate it, or reconfigure manually")
	}
	if info.Size() == 0 {
		return Error(id, "Codex Desktop backup",
			"backup file is empty: "+st.BackupFile,
			"Reconfigure Codex Desktop manually")
	}
	return OK(id, "Codex Desktop backup", st.BackupFile)
}

// ---------- helpers ----------

// missingCLIProfileFields returns the list of top-level keys
// expected in the CLI profile but absent from the text.
// The check is case-insensitive and tolerant of whitespace.
func missingCLIProfileFields(text string) []string {
	expected := []string{
		"model_provider",
		"openai_base_url",
		"model_catalog_json",
	}
	// The profile must also contain a [model_providers.ocgo-launch]
	// table. We check for the section header.
	wantSection := "[model_providers.ocgo-launch]"
	var missing []string
	lc := strings.ToLower(text)
	for _, k := range expected {
		if !strings.Contains(lc, k+" =") {
			missing = append(missing, k)
		}
	}
	if !strings.Contains(text, wantSection) {
		missing = append(missing, wantSection)
	}
	return missing
}

// missingDesktopConfigFields returns the list of keys
// expected in the Desktop OCGO profile but absent. The
// check is loose: as long as the section is present and
// base_url + wire_api are wired, we are happy.
func missingDesktopConfigFields(text string) []string {
	lc := strings.ToLower(text)
	if !strings.Contains(lc, "model_provider = \"ocgo-desktop\"") {
		// Possibly not an OCGO Desktop config; let the
		// caller decide if it is a ChatGPT config.
		return []string{"model_provider = \"ocgo-desktop\""}
	}
	required := []string{
		"base_url",
		"wire_api",
		"env_key",
	}
	var missing []string
	for _, k := range required {
		if !strings.Contains(lc, k+" =") {
			missing = append(missing, k)
		}
	}
	return missing
}

// looksLikeChatGPTConfig returns true if the file appears
// to be the standard Codex Desktop default with ChatGPT
// provider, e.g. a model_provider = "chatgpt" line.
func looksLikeChatGPTConfig(text string) bool {
	lc := strings.ToLower(text)
	return strings.Contains(lc, "model_provider = \"chatgpt\"") ||
		!strings.Contains(lc, "model_provider")
}
