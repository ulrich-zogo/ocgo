// Package tokens provides a deterministic, dependency-free, local token
// estimator used by the OCGO proxy for the
// POST /v1/messages/count_tokens endpoint.
//
// The estimator is intentionally approximate. It is not byte-compatible
// with any proprietary tokenizer (Anthropic, OpenAI, OpenCode Go). The
// goal is to give clients a stable, non-zero, conservative estimate that
// they can use to size contexts, decide whether to continue a
// conversation, and avoid worst-case zero-token responses that make
// upstream usage accounting impossible.
//
// No network calls. No persistent state. No analytics. No billing.
package tokens

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// Overhead constants used by the estimator. They are NOT exact token
// counts; they are conservative bumps to account for the structural
// metadata that surrounds a piece of text in a real request (role
// labels, message separators, tool descriptors, etc.). They are
// intentionally exported so callers and tests can reference them.
const (
	MessageOverheadTokens = 4
	ToolOverheadTokens    = 8
	ImageOverheadTokens   = 85
	RequestOverheadTokens = 3
)

// Estimate is the minimal shape returned to the client.
type Estimate struct {
	InputTokens int `json:"input_tokens"`
}

// ErrTrailingJSON is returned by DecodeJSONObjectStrict when the
// input contains valid JSON followed by additional, non-whitespace
// tokens. Strict decoding refuses to silently accept a body that
// may have been truncated or concatenated.
var ErrTrailingJSON = errors.New("unexpected trailing JSON after first object")

// DecodeJSONObjectStrict decodes raw as a single JSON object into
// dst. It refuses trailing tokens: any second Decode call must hit
// io.EOF, otherwise the body is rejected as malformed. The
// underlying decoder uses UseNumber so numeric fields decode to
// json.Number.
//
// Errors:
//   - syntax error in the body
//   - body is valid JSON but not an object (e.g. array, string)
//   - ErrTrailingJSON if there is content after the first value
//   - ErrEmptyBody if raw is empty or whitespace-only
func DecodeJSONObjectStrict(raw []byte, dst *map[string]any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ErrEmptyBody{}
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return ErrTrailingJSON
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// EstimateTextTokens returns a conservative token estimate for a free
// piece of text. Empty input (or whitespace-only) returns 0. The
// formula is:
//
//	tokens = max(ceil(rune_count / 4), word_count, 1)
//
// with the floor raised to 1 for any non-empty input.
//
// rune_count is the Unicode-aware rune count, not the byte length,
// which keeps the estimate stable for UTF-8 and non-Latin scripts.
// word_count is the number of whitespace-separated tokens. Words are
// a useful lower bound because a single very long word (a URL, a
// path) would otherwise be massively under-counted by a
// characters/4 heuristic alone.
func EstimateTextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	runes := utf8.RuneCountInString(text)
	tokens := (runes + 3) / 4

	words := len(strings.Fields(text))
	if words > tokens {
		tokens = words
	}

	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// EstimateJSONTokens returns a token estimate for an arbitrary
// already-serialized JSON document. It walks the value as generic
// JSON and counts every string, every object key, every non-container
// element, with structural overheads.
func EstimateJSONTokens(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		// Fall back to treating the body as plain text.
		return EstimateTextTokens(string(raw))
	}
	return estimateAny(v) + RequestOverheadTokens
}

// EstimateAnthropicCountTokens estimates the input_tokens for an
// Anthropic-style Messages payload. It accepts the raw request body
// (not the Go struct) so the handler can validate format errors
// itself and return OpenAI-shaped errors.
func EstimateAnthropicCountTokens(raw []byte) (Estimate, error) {
	var v map[string]any
	if err := DecodeJSONObjectStrict(raw, &v); err != nil {
		if errors.Is(err, ErrEmptyBody{}) {
			return Estimate{}, errEmptyBody
		}
		return Estimate{}, errInvalidJSON{err: err}
	}
	tokens := RequestOverheadTokens
	tokens += estimateStringField(v, "system")
	tokens += estimateToolsAnthropic(v["tools"])
	if msgs, ok := v["messages"].([]any); ok {
		tokens += estimateMessagesAnthropic(msgs)
	}
	if tokens < 1 {
		tokens = 1
	}
	return Estimate{InputTokens: tokens}, nil
}

// EstimateOpenAIChatTokens estimates the input_tokens for an
// OpenAI-style Chat Completions payload. The /v1/messages/count_tokens
// route tolerates this shape so that clients that send OpenAI-style
// counts are not rejected with 400.
func EstimateOpenAIChatTokens(raw []byte) (Estimate, error) {
	var v map[string]any
	if err := DecodeJSONObjectStrict(raw, &v); err != nil {
		if errors.Is(err, ErrEmptyBody{}) {
			return Estimate{}, errEmptyBody
		}
		return Estimate{}, errInvalidJSON{err: err}
	}
	tokens := RequestOverheadTokens
	tokens += estimateToolsOpenAI(v["tools"])
	if msgs, ok := v["messages"].([]any); ok {
		tokens += estimateMessagesOpenAI(msgs)
	}
	if tokens < 1 {
		tokens = 1
	}
	return Estimate{InputTokens: tokens}, nil
}

// EstimateResponsesTokens estimates the input_tokens for an OpenAI
// Responses API payload.
func EstimateResponsesTokens(raw []byte) (Estimate, error) {
	var v map[string]any
	if err := DecodeJSONObjectStrict(raw, &v); err != nil {
		if errors.Is(err, ErrEmptyBody{}) {
			return Estimate{}, errEmptyBody
		}
		return Estimate{}, errInvalidJSON{err: err}
	}
	tokens := RequestOverheadTokens
	tokens += estimateStringField(v, "instructions")
	tokens += estimateToolsResponses(v["tools"])
	tokens += estimateResponsesInput(v["input"])
	if tokens < 1 {
		tokens = 1
	}
	return Estimate{InputTokens: tokens}, nil
}

// ---------- internals ----------

func estimateStringField(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	return estimateAny(v)
}

func estimateAny(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case string:
		return EstimateTextTokens(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return 1
	case json.Number:
		return 1
	case []any:
		total := 0
		for _, e := range x {
			total += estimateAny(e)
		}
		return total
	case map[string]any:
		total := 0
		// Sort keys for determinism: callers and tests rely on a
		// stable result for the same input shape.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			total += EstimateTextTokens(k)
			total += estimateAny(x[k])
		}
		return total
	}
	return 0
}

func estimateMessagesAnthropic(msgs []any) int {
	total := 0
	for _, m := range msgs {
		total += estimateAnthropicMessage(m)
	}
	return total
}

func estimateAnthropicMessage(m any) int {
	mm, ok := m.(map[string]any)
	if !ok {
		return MessageOverheadTokens
	}
	total := MessageOverheadTokens
	total += estimateStringField(mm, "role")
	total += estimateAnthropicContent(mm["content"])
	return total
}

func estimateAnthropicContent(v any) int {
	switch x := v.(type) {
	case string:
		return EstimateTextTokens(x)
	case []any:
		total := 0
		for _, b := range x {
			total += estimateAnthropicBlock(b)
		}
		return total
	}
	return 0
}

func estimateAnthropicBlock(b any) int {
	bb, ok := b.(map[string]any)
	if !ok {
		return 0
	}
	typ, _ := bb["type"].(string)
	switch typ {
	case "text":
		return EstimateTextTokens(stringField(bb, "text"))
	case "image":
		// Anthropic image blocks carry a `source` (base64 or url).
		// We add an explicit image overhead for the block and
		// count any caption-like fields.
		total := ImageOverheadTokens
		total += estimateAny(bb["source"])
		return total
	case "tool_use":
		total := MessageOverheadTokens
		total += EstimateTextTokens(stringField(bb, "name"))
		total += estimateAny(bb["input"])
		return total
	case "tool_result":
		total := MessageOverheadTokens
		total += EstimateTextTokens(stringField(bb, "tool_use_id"))
		total += estimateAnthropicContent(bb["content"])
		return total
	}
	// Unknown block types: walk generic.
	return estimateAny(bb)
}

func estimateMessagesOpenAI(msgs []any) int {
	total := 0
	for _, m := range msgs {
		total += estimateOpenAIMessage(m)
	}
	return total
}

func estimateOpenAIMessage(m any) int {
	mm, ok := m.(map[string]any)
	if !ok {
		return MessageOverheadTokens
	}
	total := MessageOverheadTokens
	total += estimateStringField(mm, "role")
	total += estimateStringField(mm, "name")
	total += estimateOpenAIContent(mm["content"])
	if tcs, ok := mm["tool_calls"].([]any); ok {
		total += estimateOpenAIToolCalls(tcs)
	}
	if tcID, ok := mm["tool_call_id"].(string); ok && tcID != "" {
		total += EstimateTextTokens(tcID)
	}
	return total
}

func estimateOpenAIContent(v any) int {
	switch x := v.(type) {
	case string:
		return EstimateTextTokens(x)
	case []any:
		total := 0
		for _, p := range x {
			total += estimateOpenAIPart(p)
		}
		return total
	}
	return 0
}

func estimateOpenAIPart(p any) int {
	pp, ok := p.(map[string]any)
	if !ok {
		return 0
	}
	typ, _ := pp["type"].(string)
	switch typ {
	case "text":
		return EstimateTextTokens(stringField(pp, "text"))
	case "image_url":
		return ImageOverheadTokens + EstimateTextTokens(stringField(pp, "url"))
	}
	return estimateAny(pp)
}

func estimateOpenAIToolCalls(tcs []any) int {
	total := 0
	for _, c := range tcs {
		cc, ok := c.(map[string]any)
		if !ok {
			total += MessageOverheadTokens
			continue
		}
		total += MessageOverheadTokens
		total += EstimateTextTokens(stringField(cc, "id"))
		total += EstimateTextTokens(stringField(cc, "type"))
		if fn, ok := cc["function"].(map[string]any); ok {
			total += EstimateTextTokens(stringField(fn, "name"))
			total += EstimateTextTokens(stringField(fn, "arguments"))
		}
	}
	return total
}

func estimateToolsAnthropic(v any) int {
	tools, ok := v.([]any)
	if !ok {
		return 0
	}
	total := 0
	for _, t := range tools {
		tt, ok := t.(map[string]any)
		if !ok {
			total += ToolOverheadTokens
			continue
		}
		total += ToolOverheadTokens
		total += EstimateTextTokens(stringField(tt, "name"))
		total += EstimateTextTokens(stringField(tt, "description"))
		total += estimateAny(tt["input_schema"])
	}
	return total
}

func estimateToolsOpenAI(v any) int {
	tools, ok := v.([]any)
	if !ok {
		return 0
	}
	total := 0
	for _, t := range tools {
		tt, ok := t.(map[string]any)
		if !ok {
			total += ToolOverheadTokens
			continue
		}
		total += ToolOverheadTokens
		total += EstimateTextTokens(stringField(tt, "type"))
		if fn, ok := tt["function"].(map[string]any); ok {
			total += EstimateTextTokens(stringField(fn, "name"))
			total += EstimateTextTokens(stringField(fn, "description"))
			total += estimateAny(fn["parameters"])
		} else {
			total += EstimateTextTokens(stringField(tt, "name"))
			total += EstimateTextTokens(stringField(tt, "description"))
			total += estimateAny(tt["parameters"])
		}
	}
	return total
}

func estimateToolsResponses(v any) int {
	tools, ok := v.([]any)
	if !ok {
		return 0
	}
	total := 0
	for _, t := range tools {
		tt, ok := t.(map[string]any)
		if !ok {
			total += ToolOverheadTokens
			continue
		}
		total += ToolOverheadTokens
		total += EstimateTextTokens(stringField(tt, "type"))
		total += EstimateTextTokens(stringField(tt, "name"))
		total += EstimateTextTokens(stringField(tt, "description"))
		total += estimateAny(tt["parameters"])
	}
	return total
}

func estimateResponsesInput(v any) int {
	switch x := v.(type) {
	case string:
		return EstimateTextTokens(x)
	case []any:
		total := 0
		for _, item := range x {
			total += estimateResponsesItem(item)
		}
		return total
	}
	return 0
}

func estimateResponsesItem(item any) int {
	mm, ok := item.(map[string]any)
	if !ok {
		return 0
	}
	// Items can either be a content-part ({"type": "input_text",
	// "text": "..."}) or a whole message ({"role": "user",
	// "content": [...]}). We dispatch on the presence of role.
	if _, hasRole := mm["role"]; hasRole {
		return estimateResponsesMessage(mm)
	}
	return estimateResponsesPart(mm)
}

func estimateResponsesMessage(mm map[string]any) int {
	total := MessageOverheadTokens
	total += estimateStringField(mm, "role")
	total += estimateResponsesInput(mm["content"])
	if tcs, ok := mm["tool_calls"].([]any); ok {
		total += estimateOpenAIToolCalls(tcs)
	}
	return total
}

func estimateResponsesPart(p map[string]any) int {
	typ, _ := p["type"].(string)
	switch typ {
	case "input_text", "output_text", "text":
		return EstimateTextTokens(stringField(p, "text"))
	case "input_image":
		return ImageOverheadTokens
	case "function_call":
		total := MessageOverheadTokens
		total += EstimateTextTokens(stringField(p, "call_id"))
		total += EstimateTextTokens(stringField(p, "name"))
		total += EstimateTextTokens(stringField(p, "arguments"))
		return total
	case "function_call_output":
		total := MessageOverheadTokens
		total += EstimateTextTokens(stringField(p, "call_id"))
		total += EstimateTextTokens(stringField(p, "output"))
		return total
	case "tool_call":
		total := MessageOverheadTokens
		total += EstimateTextTokens(stringField(p, "id"))
		if fn, ok := p["function"].(map[string]any); ok {
			total += EstimateTextTokens(stringField(fn, "name"))
			total += EstimateTextTokens(stringField(fn, "arguments"))
		}
		return total
	case "tool_result":
		total := MessageOverheadTokens
		total += EstimateTextTokens(stringField(p, "tool_call_id"))
		total += estimateAny(p["content"])
		return total
	}
	return estimateAny(p)
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ---------- typed errors ----------

// ErrEmptyBody indicates that the request body was empty or
// whitespace-only.
type ErrEmptyBody struct{}

func (ErrEmptyBody) Error() string { return "empty request body" }

var errEmptyBody = ErrEmptyBody{}

// ErrInvalidJSON wraps a json decode failure for /v1/messages/count_tokens.
type errInvalidJSON struct{ err error }

func (e errInvalidJSON) Error() string { return "invalid JSON: " + e.err.Error() }
func (e errInvalidJSON) Unwrap() error { return e.err }
