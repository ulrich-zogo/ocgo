package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ocgo/internal/config"
)

const modelSelectionVersion = 1

type ModelSelection struct {
	Version      int       `json:"version"`
	DefaultModel string    `json:"default_model"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var ModelSelectionFile = config.ModelSelectionFile

func ReadModelSelection(path string) (ModelSelection, error) {
	if path == "" {
		return ModelSelection{}, fmt.Errorf("empty selection path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ModelSelection{}, err
	}
	var sel ModelSelection
	if err := json.Unmarshal(data, &sel); err != nil {
		return ModelSelection{}, fmt.Errorf("parse selection: %w", err)
	}
	if sel.Version != modelSelectionVersion {
		return ModelSelection{}, fmt.Errorf("selection version %d != %d", sel.Version, modelSelectionVersion)
	}
	return sel, nil
}

func WriteModelSelection(path string, sel ModelSelection) error {
	if path == "" {
		return fmt.Errorf("empty selection path")
	}
	if sel.Version == 0 {
		sel.Version = modelSelectionVersion
	}
	if sel.UpdatedAt.IsZero() {
		sel.UpdatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(sel, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal selection: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create selection dir: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func GetDefaultModel() (string, error) {
	sel, err := ReadModelSelection(ModelSelectionFile())
	if err != nil {
		return "", err
	}
	id := NormalizeID(sel.DefaultModel)
	if id == "" {
		return "", fmt.Errorf("default model is empty in selection file")
	}
	if !IsKnown(id) {
		return "", fmt.Errorf("default model %q in selection file is not a known OpenCode Go model. Run `ocgo models` to list available models.", id)
	}
	return id, nil
}

// GetDefaultModelStatus returns the model that would be used when no
// explicit model is provided. Unlike GetDefaultModel, it does not
// error when the configured default is missing or invalid: in that
// case it falls back to the first known OpenCode Go model and reports
// configured=false. It only returns an error if no known models are
// available at all (official + remote + cache + fallback all empty).
func GetDefaultModelStatus() (model string, configured bool, err error) {
	if id, err := GetDefaultModel(); err == nil && id != "" {
		return id, true, nil
	}
	known := KnownIDs()
	if len(known) == 0 {
		return "", false, fmt.Errorf("no known OpenCode Go models available")
	}
	return known[0], false, nil
}

func SetDefaultModel(model string) error {
	id := NormalizeID(strings.TrimSpace(model))
	if id == "" {
		return fmt.Errorf("default model cannot be empty")
	}
	if !IsKnown(id) {
		return fmt.Errorf("unknown OpenCode Go model %q. Run `ocgo models` to list available models.", model)
	}
	sel := ModelSelection{
		Version:      modelSelectionVersion,
		DefaultModel: id,
		UpdatedAt:    time.Now().UTC(),
	}
	return WriteModelSelection(ModelSelectionFile(), sel)
}

func ResolveEffectiveModel(explicit string) (string, error) {
	if id := NormalizeID(strings.TrimSpace(explicit)); id != "" {
		if !IsKnown(id) {
			return "", fmt.Errorf("unknown OpenCode Go model %q. Run `ocgo models` to list available models.", explicit)
		}
		return id, nil
	}
	if id, err := GetDefaultModel(); err == nil && id != "" {
		return id, nil
	}
	known := KnownIDs()
	if len(known) > 0 {
		return known[0], nil
	}
	return "", fmt.Errorf("no known OpenCode Go models available")
}
