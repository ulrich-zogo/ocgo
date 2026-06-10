package doctor

import (
	"ocgo/internal/codex"
	"ocgo/internal/models"
)

// codexCheckVersion is a small wrapper that lets the doctor
// call the existing codex version check without coupling to
// the Manager constructor. The Manager has no fields the
// doctor needs to override, so we go through it as a value
// type.
func codexCheckVersion() error {
	return codex.NewManager().CheckVersion()
}

// getDefaultModel is a thin wrapper around
// models.GetDefaultModelStatus. It returns the model name
// and the configured flag, or an empty model name on
// failure. Errors are intentionally swallowed because the
// caller (the model-catalog check) only needs the model
// for cross-referencing the catalog.
func getDefaultModel() (string, bool, error) {
	return models.GetDefaultModelStatus()
}

// readModelSelection reads a model selection file at the
// given path. It uses the existing models.ReadModelSelection
// logic so the doctor respects version checks and JSON
// validation. The doctor passes the explicit path from
// Runner.Paths.ModelSelectionFile rather than going through
// the package-level models.ModelSelectionFile var, so the
// doctor's tests can run in temp directories.
func readModelSelection(path string) (models.ModelSelection, error) {
	return models.ReadModelSelection(path)
}

