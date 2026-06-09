package proxy

import (
	"bytes"
	"encoding/json"
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
// quick peek at the JSON top-level keys.
func estimateTokensFromBody(body []byte) (tokens.Estimate, error) {
	trimmed := trimAll(body)
	if len(trimmed) == 0 {
		return tokens.Estimate{}, tokens.ErrEmptyBody{}
	}

	// If the body is not a JSON object at all (or is malformed),
	// surface a 400. EstimateJSONTokens silently falls back to
	// text, which would mask a client bug as a successful 0/1
	// token response.
	keys, ok := topLevelKeysOK(trimmed)
	if !ok {
		return tokens.Estimate{}, errTokensInvalidJSON{msg: "request body is not a JSON object"}
	}

	// Responses-shape detection: explicit "input" or "instructions"
	// at the top level, with or without messages.
	if _, hasInput := keys["input"]; hasInput {
		return tokens.EstimateResponsesTokens(trimmed)
	}
	if _, hasInstructions := keys["instructions"]; hasInstructions {
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
	if _, hasSystem := keys["system"]; hasSystem {
		return tokens.EstimateAnthropicCountTokens(trimmed)
	}
	if tools, ok := peekTools(trimmed); ok && len(tools) > 0 {
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
	if _, hasMessages := keys["messages"]; hasMessages {
		// No "system" and no "tools": still an Anthropic-shaped
		// request (the most common case for count_tokens).
		return tokens.EstimateAnthropicCountTokens(trimmed)
	}
	// Unknown shape but valid JSON object: count the whole body as
	// JSON. This is a best-effort estimate for non-standard clients
	// and is intentionally tolerant.
	return tokens.Estimate{InputTokens: tokens.EstimateJSONTokens(trimmed)}, nil
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

// topLevelKeys peeks at the top-level keys of a JSON object without
// doing a full decode. The estimate does not need a full parse: the
// dispatch only inspects the first level of keys. We do a minimal
// scan and bail if the body is not a JSON object.
func topLevelKeys(raw []byte) map[string]struct{} {
	keys, _ := topLevelKeysOK(raw)
	return keys
}

// topLevelKeysOK returns the top-level keys of a JSON object and a
// bool indicating whether the body decoded as a JSON object.
func topLevelKeysOK(raw []byte) (map[string]struct{}, bool) {
	out := map[string]struct{}{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var peek map[string]any
	if err := dec.Decode(&peek); err != nil {
		return nil, false
	}
	for k := range peek {
		out[k] = struct{}{}
	}
	return out, true
}

// peekTools reads just the "tools" array of a top-level object. It
// is used to detect Anthropic-vs-OpenAI tool shapes. Returns ok=true
// only if "tools" is present (even if empty).
func peekTools(raw []byte) ([]any, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var peek map[string]any
	if err := dec.Decode(&peek); err != nil {
		return nil, false
	}
	v, ok := peek["tools"]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, true
	}
	return arr, true
}
