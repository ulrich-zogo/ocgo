package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const desktopProviderName = "ocgo-desktop"
const desktopProfileName = "ocgo-desktop"

func NormalizeBaseURLWithV1(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed + "/"
	}
	return trimmed + "/v1/"
}

func (m Manager) EnableDesktopOpenCode(baseURL string, explicitModel string) (DesktopState, error) {
	normalized := NormalizeBaseURLWithV1(baseURL)
	if normalized == "" {
		return DesktopState{}, errors.New("base URL is required")
	}

	current, stateErr := ReadDesktopState(m.DesktopStateFile())
	backupFile := ""
	if stateErr == nil && current.Mode == DesktopModeOpenCode && current.BackupFile != "" {
		if _, err := os.Stat(current.BackupFile); err == nil {
			backupFile = current.BackupFile
		}
	}
	if backupFile == "" {
		bf, err := m.BackupDesktopConfig(time.Now())
		if err != nil {
			return DesktopState{}, err
		}
		backupFile = bf
	}

	if err := m.WriteModelCatalog(); err != nil {
		return DesktopState{}, fmt.Errorf("write model catalog: %w", err)
	}

	dst := m.DesktopConfigFile()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return DesktopState{}, fmt.Errorf("create codex config dir: %w", err)
	}
	configText := renderDesktopOpenCodeConfig(normalized, explicitModel, m.Paths.ModelCatalogFile)
	if err := os.WriteFile(dst, []byte(configText), 0644); err != nil {
		return DesktopState{}, fmt.Errorf("write codex desktop config: %w", err)
	}

	st := DesktopState{
		Version:    DesktopStateVersion,
		Mode:       DesktopModeOpenCode,
		UpdatedAt:  time.Now().UTC(),
		BaseURL:    normalized,
		Model:      explicitModel,
		BackupFile: backupFile,
	}
	if err := WriteDesktopState(m.DesktopStateFile(), st); err != nil {
		return DesktopState{}, err
	}
	return st, nil
}

func (m Manager) EnableDesktopChatGPT() (DesktopState, error) {
	current, err := ReadDesktopState(m.DesktopStateFile())
	if err != nil {
		return DesktopState{}, fmt.Errorf("read codex desktop state: %w", err)
	}
	if current.Mode != DesktopModeOpenCode {
		return DesktopState{}, fmt.Errorf("codex desktop is not currently configured by OCGO (mode=%q)", current.Mode)
	}
	if current.BackupFile == "" {
		return DesktopState{}, errors.New("no backup available to restore ChatGPT/OpenAI configuration")
	}
	if err := m.RestoreDesktopConfig(current.BackupFile); err != nil {
		return DesktopState{}, err
	}
	st := DesktopState{
		Version:   DesktopStateVersion,
		Mode:      DesktopModeChatGPT,
		UpdatedAt: time.Now().UTC(),
	}
	if err := WriteDesktopState(m.DesktopStateFile(), st); err != nil {
		return DesktopState{}, err
	}
	return st, nil
}

type DesktopStatusReport struct {
	Mode        DesktopMode
	Managed     bool
	BaseURL     string
	Model       string
	BackupFile  string
	StateFile   string
	UpdatedAt   time.Time
	Source      string
}

func (m Manager) DesktopStatus() (DesktopStatusReport, error) {
	st, err := ReadDesktopState(m.DesktopStateFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DesktopStatusReport{Source: "none", StateFile: m.DesktopStateFile()}, nil
		}
		return DesktopStatusReport{}, err
	}
	return DesktopStatusReport{
		Mode:       st.Mode,
		Managed:    true,
		BaseURL:    st.BaseURL,
		Model:      st.Model,
		BackupFile: st.BackupFile,
		StateFile:  m.DesktopStateFile(),
		UpdatedAt:  st.UpdatedAt,
		Source:     "state",
	}, nil
}

func renderDesktopOpenCodeConfig(baseURL, model, catalogPath string) string {
	catalog := catalogPath
	if catalog == "" {
		catalog = filepath.Join(filepath.Dir(baseURL), "ocgo-models.json")
	}
	lines := []string{
		fmt.Sprintf("model = %q", model),
		fmt.Sprintf("model_provider = %q", desktopProviderName),
		"",
		fmt.Sprintf("[model_providers.%s]", desktopProviderName),
		`name = "OCGO OpenCode Go"`,
		fmt.Sprintf("base_url = %q", baseURL),
		`env_key = "OPENAI_API_KEY"`,
		`wire_api = "responses"`,
		fmt.Sprintf("model_catalog_json = %q", catalog),
		"",
	}
	return strings.Join(lines, "\n")
}
