package proxy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"ocgo/internal/tokens"
)

// CountTokens is the HTTP handler for POST /v1/messages/count_tokens.
//
// It accepts three payload shapes:
//
//   - Anthropic Messages  ({"model":..., "system":..., "messages":[...]})
//   - OpenAI Chat         ({"model":..., "messages":[...]})
//   - Responses           ({"model":..., "instructions":..., "input":...})
//
// The format is detected by inspecting the top-level keys. Unknown
// shapes fall back to a generic JSON token estimate.
//
// The handler never calls the network: it computes a local,
// deterministic, conservative estimate using internal/tokens.
//
// Errors are returned in the OpenAI-compatible envelope produced by
// WriteOpenAIError so that the same client surface works for both
// success and failure.
func CountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteOpenAIError(w, http.StatusMethodNotAllowed,
			"method not allowed: use POST", "invalid_request_error", "", "method_not_allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteOpenAIError(w, http.StatusBadRequest,
			"read body: "+err.Error(), "invalid_request_error", "", "read_error")
		return
	}

	est, err := estimateTokensFromBody(body)
	if err != nil {
		// Empty body is a hard 400: the client gave us nothing
		// useful to count.
		WriteOpenAIError(w, http.StatusBadRequest,
			err.Error(), "invalid_request_error", "", classifyTokensError(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(est)
}

// estimateTokensFromBody returns the per-shape estimate based on a
// strict decode of the JSON body.
func estimateTokensFromBody(body []byte) (tokens.Estimate, error) {
	trimmed := trimAll(body)
	if len(trimmed) == 0 {
		return tokens.Estimate{}, tokens.ErrEmptyBody{}
	}

	// Strict decode: refuse trailing JSON, refuse non-object
	// top-level values. This is the single source of truth for
	// "is this a valid JSON object".
	var doc map[string]any
	if err := tokens.DecodeJSONObjectStrict(trimmed, &doc); err != nil {
		if errors.Is(err, tokens.ErrEmptyBody{}) {
			return tokens.Estimate{}, tokens.ErrEmptyBody{}
		}
		return tokens.Estimate{}, errTokensInvalidJSON{msg: err.Error()}
	}

	// Responses-shape detection: explicit "input" or "instructions"
	// at the top level, with or without messages.
	if _, hasInput := doc["input"]; hasInput {
		return tokens.EstimateResponsesTokens(trimmed)
	}
	if _, hasInstructions := doc["instructions"]; hasInstructions {
		return tokens.EstimateResponsesTokens(trimmed)
	}

	// Anthropic-vs-OpenAI detection. The Anthropic shape uses
	// "system" as a string or array and uses "tools" with
	// {name, description, input_schema}. The OpenAI shape uses
	// "messages" with "role"/"content" + "tools" with
	// {type:"function", function:{...}}. We default to Anthropic
	// when "system" is present, otherwise we treat "messages" as
	// either shape but prefer Anthropic when tool items look like
	// {name, input_schema} and OpenAI when tool items look like
	// {type, function}. To keep the surface simple, we dispatch
	// on the "tools" shape when present and fall back to
	// Anthropic otherwise.
	if _, hasSystem := doc["system"]; hasSystem {
		return tokens.EstimateAnthropicCountTokens(trimmed)
	}
	if tools, ok := peekTools(doc); ok && len(tools) > 0 {
		first, _ := tools[0].(map[string]any)
		if _, isAnthropic := first["input_schema"]; isAnthropic {
			return tokens.EstimateAnthropicCountTokens(trimmed)
		}
		if fn, isOpenAI := first["function"].(map[string]any); isOpenAI && fn != nil {
			if _, hasName := fn["name"]; hasName {
				return tokens.EstimateOpenAIChatTokens(trimmed)
			}
		}
		// Unknown tools shape: default to Anthropic (the more
		// common shape in the OCGO client surface).
		return tokens.EstimateAnthropicCountTokens(trimmed)
	}
	if _, hasMessages := doc["messages"]; hasMessages {
		// When messages is present without "system" and without
		// "tools", we have to disambiguate Anthropic vs OpenAI
		// by looking at the messages themselves. Anthropic uses
		// {"type": "text"|"image"|"tool_use"|"tool_result",
		// ...}; OpenAI uses {"role", "content"|"tool_calls",
		// "tool_call_id", "name"} with content parts of
		// {"type": "text"|"image_url"}. If any message looks
		// like OpenAI, we route to the OpenAI estimator so we
		// don't lose the image_url or tool_calls overhead.
		if looksLikeOpenAIChatMessages(doc) {
			return tokens.EstimateOpenAIChatTokens(trimmed)
		}
		// Default: Anthropic (the most common shape in the
		// OCGO client surface).
		return tokens.EstimateAnthropicCountTokens(trimmed)
	}
	// Unknown shape but valid JSON object: count the whole body as
	// JSON. This is a best-effort estimate for non-standard clients
	// and is intentionally tolerant.
	return tokens.Estimate{InputTokens: tokens.EstimateJSONTokens(trimmed)}, nil
}

// looksLikeOpenAIChatMessages returns true when the top-level
// "messages" array carries any field that is OpenAI-specific and
// not part of the Anthropic Messages shape. The list is:
//
//   - any message with "tool_calls" (OpenAI tool calls) or
//     "tool_call_id" (OpenAI tool result)
//   - any message content part with type "image_url"
//
// A pure {role, content:string} list with no system/tools is
// ambiguous and falls through to the default Anthropic path.
func looksLikeOpenAIChatMessages(doc map[string]any) bool {
	msgs, ok := doc["messages"].([]any)
	if !ok {
		return false
	}
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if _, hasTC := mm["tool_calls"]; hasTC {
			return true
		}
		if _, hasTCID := mm["tool_call_id"]; hasTCID {
			return true
		}
		if parts, ok := mm["content"].([]any); ok {
			for _, p := range parts {
				pp, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if t, _ := pp["type"].(string); t == "image_url" {
					return true
				}
			}
		}
	}
	return false
}

// classifyTokensError maps an internal/tokens error into a short
// machine-readable code for the OpenAI error envelope.
func classifyTokensError(err error) string {
	if err == nil {
		return ""
	}
	// Both ErrEmptyBody and invalid-JSON errors get a dedicated
	// code so clients can distinguish them.
	if isEmptyBody(err) {
		return "empty_body"
	}
	if isInvalidJSON(err) {
		return "invalid_json"
	}
	return "invalid_request"
}

func isEmptyBody(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(tokens.ErrEmptyBody)
	return ok
}

func isInvalidJSON(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(errTokensInvalidJSON)
	return ok
}

// errTokensInvalidJSON signals that the request body was not
// decodable as a JSON object. It is distinct from
// tokens.EstimateXxxTokens' own invalid-JSON errors so the handler
// can attribute the error to the right code.
type errTokensInvalidJSON struct{ msg string }

func (e errTokensInvalidJSON) Error() string { return "invalid JSON: " + e.msg }

// trimAll returns the body with all leading/trailing whitespace
// removed. It is safer than the more aggressive trim in
// internal/tokens because the proxy must not mutate the bytes it
// hands to the estimator.
func trimAll(b []byte) []byte {
	start := 0
	for start < len(b) {
		c := b[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}
	end := len(b)
	for end > start {
		c := b[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}
	return b[start:end]
}

// topLevelKeys returns the top-level keys of a JSON object without
// surfacing decoding errors. Trailing-JSON errors are swallowed and
// reported as "not a JSON object" so that callers that only need
// the key set still behave sanely. The strict version of the
// decoder (tokens.DecodeJSONObjectStrict) is the source of truth
// for the handler path.
func topLevelKeys(raw []byte) map[string]struct{} {
	keys, _ := topLevelKeysOK(raw)
	return keys
}

// topLevelKeysOK returns the top-level keys of a JSON object and a
// bool indicating whether the body strictly decoded as a JSON
// object. Trailing JSON, syntax errors, and non-object top-level
// values all return (nil, false).
func topLevelKeysOK(raw []byte) (map[string]struct{}, bool) {
	var doc map[string]any
	if err := tokens.DecodeJSONObjectStrict(raw, &doc); err != nil {
		return nil, false
	}
	out := make(map[string]struct{}, len(doc))
	for k := range doc {
		out[k] = struct{}{}
	}
	return out, true
}

// peekTools reads the "tools" array of a strictly-decoded top-level
// object. It is used to detect Anthropic-vs-OpenAI tool shapes.
// Returns ok=true only if "tools" is present (even if empty) AND
// the body was a valid JSON object. A non-object body returns
// (nil, false) so callers fall through to the "no tools" branch.
func peekTools(doc map[string]any) ([]any, bool) {
	v, ok := doc["tools"]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, true
	}
	return arr, true
}
