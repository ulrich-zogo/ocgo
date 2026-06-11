package app

import (
	"fmt"

	"ocgo/internal/config"
	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

// ClaudeLaunchConfig holds the parameters needed to launch Claude Code.
type ClaudeLaunchConfig struct {
	BaseURL string
	Args    []string
	Env     []string
}

// BuildClaudeLaunchConfig computes the Claude launch configuration from a
// loaded OCGO config, an optional explicit model, and any passthrough args.
// It returns an error if the model is unknown.
func BuildClaudeLaunchConfig(cfg config.Config, model string, passthroughArgs []string) (ClaudeLaunchConfig, error) {
	base := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)

	claudeArgs := make([]string, len(passthroughArgs))
	copy(claudeArgs, passthroughArgs)

	env := []string{
		"ANTHROPIC_BASE_URL=" + base,
		"ANTHROPIC_AUTH_TOKEN=unused",
	}

	mappings, err := mapping.LoadModelMappings()
	if err != nil {
		return ClaudeLaunchConfig{}, err
	}

	modelEnv, hasEffective, err := BuildClaudeModelEnv(model)
	if err != nil {
		return ClaudeLaunchConfig{}, err
	}
	if hasEffective {
		env = append(env, modelEnv...)
	} else {
		env = append(env, BuildClaudeLegacyMappingEnv(mappings)...)
	}

	return ClaudeLaunchConfig{
		BaseURL: base,
		Args:    claudeArgs,
		Env:     env,
	}, nil
}

// BuildCodexArgs builds the argv that will be passed to the real
// `codex` binary. It always uses the ocgo-launch profile and adds
// the effective model with -m so Codex CLI always receives an
// explicit OpenCode Go model.
//
// The explicit model takes priority over the configured default.
func BuildCodexArgs(explicitModel string, extraArgs []string) ([]string, error) {
	selected, err := models.ResolveEffectiveModel(explicitModel)
	if err != nil {
		return nil, err
	}
	return BuildCodexArgsWithResolvedModel(selected, extraArgs), nil
}

// BuildCodexArgsWithResolvedModel assumes the model has already been
// resolved (typically by models.ResolveEffectiveModel) and returns
// the argv passed to the real `codex` binary.
//
// Use this when the caller needs the resolved model for other
// purposes (e.g. printing "Effective OpenCode Go model: ..." in
// `--config` mode) and does not want to re-resolve.
func BuildCodexArgsWithResolvedModel(selectedModel string, extraArgs []string) []string {
	args := []string{"--profile", config.CodexProfileName, "-m", selectedModel}
	return append(args, extraArgs...)
}

// BuildClaudeModelEnv returns the slice of environment variables
// that should be set to route Claude Code through the effective
// OpenCode Go model. The boolean is true when an effective model
// was resolved (so the caller should use the returned env instead
// of the legacy opus/sonnet/haiku mapping fallback).
func BuildClaudeModelEnv(explicitModel string) ([]string, bool, error) {
	selected, err := models.ResolveEffectiveModel(explicitModel)
	if err != nil {
		return nil, false, err
	}
	if selected == "" {
		return nil, false, nil
	}
	env := []string{
		"ANTHROPIC_MODEL=" + selected,
		"ANTHROPIC_SMALL_FAST_MODEL=" + selected,
		"ANTHROPIC_CUSTOM_MODEL_OPTION=" + selected,
		"ANTHROPIC_CUSTOM_MODEL_OPTION_NAME=" + selected + " via OCGO",
		"ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION=OpenCode Go model routed through OCGO",
	}
	return env, true, nil
}

// BuildClaudeLegacyMappingEnv returns the legacy opus/sonnet/haiku
// ANTHROPIC_* environment variables derived from the loaded
// model mappings. Used as a fallback when no effective model is
// available (rare: should only happen if the official API, remote,
// cache and fallback list are all empty).
func BuildClaudeLegacyMappingEnv(mappings map[string]map[string]string) []string {
	opus := mapping.ResolveMappedModel("claude", "claude-opus", mappings)
	sonnet := mapping.ResolveMappedModel("claude", "claude-sonnet", mappings)
	haiku := mapping.ResolveMappedModel("claude", "claude-haiku", mappings)
	env := make([]string, 0, 15)
	if opus != "claude-opus" {
		env = append(env,
			"ANTHROPIC_DEFAULT_OPUS_MODEL="+opus,
			"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME="+opus+" via OCGO",
			"ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION=OpenCode Go model routed through OCGO",
		)
	}
	if sonnet != "claude-sonnet" {
		env = append(env,
			"ANTHROPIC_DEFAULT_SONNET_MODEL="+sonnet,
			"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME="+sonnet+" via OCGO",
			"ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION=OpenCode Go model routed through OCGO",
		)
	}
	if haiku != "claude-haiku" {
		env = append(env,
			"ANTHROPIC_DEFAULT_HAIKU_MODEL="+haiku,
			"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME="+haiku+" via OCGO",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION=OpenCode Go model routed through OCGO",
			"ANTHROPIC_SMALL_FAST_MODEL="+haiku,
		)
	}
	return env
}

// DescribeSelectionSource returns a short human label describing
// where the effective default model came from. Used by the
// `ocgo opencode model current` command.
func DescribeSelectionSource(configured bool) string {
	if configured {
		return "configured"
	}
	return "first-known-model"
}
