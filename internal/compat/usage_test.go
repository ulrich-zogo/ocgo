package compat

import (
	"encoding/json"
	"testing"
)

func TestUsageFromFieldsDerivesTotalWhenAbsent(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"input_tokens":  10,
		"output_tokens": 5,
	})
	if got.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15 (derived from input+output)", got.TotalTokens)
	}
}

func TestUsageFromFieldsDerivesTotalFromPromptCompletion(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"prompt_tokens":     7,
		"completion_tokens": 3,
	})
	if got.TotalTokens != 10 {
		t.Errorf("TotalTokens = %d, want 10 (derived from prompt+completion)", got.TotalTokens)
	}
}

func TestUsageFromFieldsPreservesExplicitTotal(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"input_tokens":  10,
		"output_tokens": 5,
		"total_tokens":  999,
	})
	if got.TotalTokens != 999 {
		t.Errorf("TotalTokens = %d, want 999 (explicit value preserved)", got.TotalTokens)
	}
}

func TestUsageFromFieldsReadsPromptCompletion(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"prompt_tokens":     7,
		"completion_tokens": 3,
	})
	if got.InputTokens != 7 {
		t.Errorf("InputTokens = %d, want 7", got.InputTokens)
	}
	if got.OutputTokens != 3 {
		t.Errorf("OutputTokens = %d, want 3", got.OutputTokens)
	}
}

func TestUsageFromFieldsReadsInputOutput(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"input_tokens":  12,
		"output_tokens": 8,
	})
	if got.InputTokens != 12 {
		t.Errorf("InputTokens = %d, want 12", got.InputTokens)
	}
	if got.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", got.OutputTokens)
	}
}

func TestUsageFromFieldsReadsCachedTokensFlat(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"input_tokens":  10,
		"output_tokens": 5,
		"cached_tokens": 4,
	})
	if got.CachedInputTokens != 4 {
		t.Errorf("CachedInputTokens = %d, want 4", got.CachedInputTokens)
	}
}

func TestUsageFromFieldsReadsCacheReadInputTokens(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"input_tokens":            10,
		"output_tokens":           5,
		"cache_read_input_tokens": 4,
	})
	if got.CachedInputTokens != 4 {
		t.Errorf("CachedInputTokens = %d, want 4", got.CachedInputTokens)
	}
}

func TestUsageFromFieldsReadsCacheCreationInputTokens(t *testing.T) {
	got := UsageFromFields(map[string]any{
		"input_tokens":                10,
		"output_tokens":               5,
		"cache_creation_input_tokens": 4,
	})
	if got.CachedInputTokens != 4 {
		t.Errorf("CachedInputTokens = %d, want 4", got.CachedInputTokens)
	}
}

func TestUsageFromFieldsReadsNestedPromptTokensDetailsCachedTokens(t *testing.T) {
	raw := `{
		"prompt_tokens": 10,
		"completion_tokens": 5,
		"prompt_tokens_details": {"cached_tokens": 4}
	}`
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := UsageFromFields(m)
	if got.CachedInputTokens != 4 {
		t.Errorf("CachedInputTokens = %d, want 4 (from prompt_tokens_details)", got.CachedInputTokens)
	}
}

func TestUsageFromFieldsReadsNestedInputTokensDetailsCachedTokens(t *testing.T) {
	raw := `{
		"input_tokens": 10,
		"output_tokens": 5,
		"input_tokens_details": {"cached_tokens": 4}
	}`
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := UsageFromFields(m)
	if got.CachedInputTokens != 4 {
		t.Errorf("CachedInputTokens = %d, want 4 (from input_tokens_details)", got.CachedInputTokens)
	}
}

func TestUsageFromFieldsPresentFlag(t *testing.T) {
	if got := UsageFromFields(map[string]any{}); got.Present {
		t.Errorf("Present = true, want false (empty input)")
	}
	got := UsageFromFields(map[string]any{"prompt_tokens": 1})
	if !got.Present {
		t.Errorf("Present = false, want true (any non-zero usage)")
	}
}

func TestMergeUsageBothPresent(t *testing.T) {
	a := TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15, CachedInputTokens: 4, Present: true}
	b := TokenUsage{InputTokens: 20, OutputTokens: 8, TotalTokens: 28, CachedInputTokens: 2, Present: true}
	got := MergeUsage(a, b)
	if got.InputTokens != 30 || got.OutputTokens != 13 || got.TotalTokens != 43 || got.CachedInputTokens != 6 {
		t.Errorf("MergeUsage = %+v, want sum of all fields", got)
	}
	if !got.Present {
		t.Errorf("Present = false, want true")
	}
}

func TestMergeUsageOnlyAPresent(t *testing.T) {
	a := TokenUsage{InputTokens: 10, Present: true}
	got := MergeUsage(a, TokenUsage{})
	if got.InputTokens != 10 || !got.Present {
		t.Errorf("MergeUsage = %+v, want a unchanged", got)
	}
}

func TestMergeUsageOnlyBPresent(t *testing.T) {
	b := TokenUsage{InputTokens: 5, Present: true}
	got := MergeUsage(TokenUsage{}, b)
	if got.InputTokens != 5 || !got.Present {
		t.Errorf("MergeUsage = %+v, want b unchanged", got)
	}
}

func TestResponsesUsageIncludesInputTokensDetailsWhenCached(t *testing.T) {
	got := ResponsesUsage(TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15, CachedInputTokens: 4})
	if got["input_tokens"].(int) != 10 {
		t.Errorf("input_tokens = %v, want 10", got["input_tokens"])
	}
	if got["output_tokens"].(int) != 5 {
		t.Errorf("output_tokens = %v, want 5", got["output_tokens"])
	}
	if got["total_tokens"].(int) != 15 {
		t.Errorf("total_tokens = %v, want 15", got["total_tokens"])
	}
	details, ok := got["input_tokens_details"].(map[string]any)
	if !ok {
		t.Fatalf("input_tokens_details missing or wrong type: %T", got["input_tokens_details"])
	}
	if details["cached_tokens"].(int) != 4 {
		t.Errorf("input_tokens_details.cached_tokens = %v, want 4", details["cached_tokens"])
	}
}

func TestResponsesUsageOmitsInputTokensDetailsWhenNoCache(t *testing.T) {
	got := ResponsesUsage(TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	if _, ok := got["input_tokens_details"]; ok {
		t.Errorf("input_tokens_details should be omitted when CachedInputTokens=0, got %v", got)
	}
}

func TestResponsesUsageDerivesTotalWhenAbsent(t *testing.T) {
	got := ResponsesUsage(TokenUsage{InputTokens: 10, OutputTokens: 5})
	if got["total_tokens"].(int) != 15 {
		t.Errorf("total_tokens = %v, want 15 (derived)", got["total_tokens"])
	}
}

func TestOpenAIUsageIncludesPromptTokensDetailsWhenCached(t *testing.T) {
	got := OpenAIUsage(TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15, CachedInputTokens: 4})
	if got["prompt_tokens"].(int) != 10 {
		t.Errorf("prompt_tokens = %v, want 10", got["prompt_tokens"])
	}
	if got["completion_tokens"].(int) != 5 {
		t.Errorf("completion_tokens = %v, want 5", got["completion_tokens"])
	}
	if got["total_tokens"].(int) != 15 {
		t.Errorf("total_tokens = %v, want 15", got["total_tokens"])
	}
	details, ok := got["prompt_tokens_details"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_tokens_details missing or wrong type: %T", got["prompt_tokens_details"])
	}
	if details["cached_tokens"].(int) != 4 {
		t.Errorf("prompt_tokens_details.cached_tokens = %v, want 4", details["cached_tokens"])
	}
}

func TestOpenAIUsageOmitsPromptTokensDetailsWhenNoCache(t *testing.T) {
	got := OpenAIUsage(TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	if _, ok := got["prompt_tokens_details"]; ok {
		t.Errorf("prompt_tokens_details should be omitted when CachedInputTokens=0, got %v", got)
	}
}

func TestOpenAIUsageDerivesTotalWhenAbsent(t *testing.T) {
	got := OpenAIUsage(TokenUsage{InputTokens: 10, OutputTokens: 5})
	if got["total_tokens"].(int) != 15 {
		t.Errorf("total_tokens = %v, want 15 (derived)", got["total_tokens"])
	}
}

func TestAnthropicUsageKeepsCacheReadInputTokens(t *testing.T) {
	got := AnthropicUsage(TokenUsage{InputTokens: 10, OutputTokens: 5, CachedInputTokens: 4})
	if got["input_tokens"] != 10 {
		t.Errorf("input_tokens = %d, want 10", got["input_tokens"])
	}
	if got["output_tokens"] != 5 {
		t.Errorf("output_tokens = %d, want 5", got["output_tokens"])
	}
	if got["cache_read_input_tokens"] != 4 {
		t.Errorf("cache_read_input_tokens = %d, want 4", got["cache_read_input_tokens"])
	}
}

func TestAnthropicUsageOmitsCacheReadWhenNoCache(t *testing.T) {
	got := AnthropicUsage(TokenUsage{InputTokens: 10, OutputTokens: 5})
	if _, ok := got["cache_read_input_tokens"]; ok {
		t.Errorf("cache_read_input_tokens should be omitted when CachedInputTokens=0, got %v", got)
	}
}

func TestAnthropicUsageHasNoTotalTokens(t *testing.T) {
	// Anthropic Messages API does not expose total_tokens. The
	// helper must not invent one.
	got := AnthropicUsage(TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	if _, ok := got["total_tokens"]; ok {
		t.Errorf("total_tokens should not be present in Anthropic usage, got %v", got)
	}
}

func TestUsageFromJSONEndToEnd(t *testing.T) {
	raw := []byte(`{
		"prompt_tokens": 10,
		"completion_tokens": 5,
		"prompt_tokens_details": {"cached_tokens": 4}
	}`)
	got := UsageFromJSON(raw)
	if got.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", got.InputTokens)
	}
	if got.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", got.OutputTokens)
	}
	if got.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", got.TotalTokens)
	}
	if got.CachedInputTokens != 4 {
		t.Errorf("CachedInputTokens = %d, want 4", got.CachedInputTokens)
	}
}

func TestUsageFromJSONInvalid(t *testing.T) {
	if got := UsageFromJSON([]byte(`{`)); got.Present {
		t.Errorf("UsageFromJSON(invalid).Present = true, want false")
	}
	if got := UsageFromJSON(nil); got.Present {
		t.Errorf("UsageFromJSON(nil).Present = true, want false")
	}
}
