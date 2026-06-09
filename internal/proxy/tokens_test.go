package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCountTokensMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages/count_tokens", CountTokens)
	return mux
}

func postCountTokens(t *testing.T, mux http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func decodeCountTokensOK(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("count_tokens status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	var est struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &est); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, w.Body.String())
	}
	return est.InputTokens
}

func TestCountTokensPOSTAnthropicShape(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `{
		"model":"minimax-m3",
		"system":"Tu es strict.",
		"messages":[
			{"role":"user","content":"Bonjour, explique Dataverse en 3 lignes."}
		]
	}`)
	got := decodeCountTokensOK(t, w)
	if got < 5 {
		t.Errorf("anthropic estimate = %d, want >= 5", got)
	}
}

func TestCountTokensPOSTOpenAIChatShape(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `{
		"model":"kimi-k2.6",
		"messages":[
			{"role":"system","content":"Tu es strict."},
			{"role":"user","content":"Bonjour"}
		]
	}`)
	got := decodeCountTokensOK(t, w)
	if got < 3 {
		t.Errorf("openai chat estimate = %d, want >= 3", got)
	}
}

func TestCountTokensPOSTOpenAIChatWithFunctionTool(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `{
		"model":"kimi-k2.6",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{
			"type":"function",
			"function":{"name":"lookup","description":"Lookup","parameters":{"type":"object"}}
		}]
	}`)
	got := decodeCountTokensOK(t, w)
	// tool overhead must be >= 8 in addition to message + text.
	if got < 5+ToolOverheadTokensFloor {
		t.Errorf("openai chat with tool estimate = %d, want >= %d", got, 5+ToolOverheadTokensFloor)
	}
}

func TestCountTokensPOSTResponsesShape(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `{
		"model":"kimi-k2.6",
		"instructions":"Tu es strict.",
		"input":"Réponds uniquement OK."
	}`)
	got := decodeCountTokensOK(t, w)
	if got < 3 {
		t.Errorf("responses estimate = %d, want >= 3", got)
	}
}

func TestCountTokensPOSTResponsesImageAddsOverhead(t *testing.T) {
	mux := newCountTokensMux()
	noImg := postCountTokens(t, mux, `{
		"model":"kimi-k2.6",
		"input":"hello"
	}`)
	if noImg.Code != http.StatusOK {
		t.Fatalf("no-img status = %d, want 200", noImg.Code)
	}
	withImg := postCountTokens(t, mux, `{
		"model":"kimi-k2.6",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"look"},
				{"type":"input_image"}
			]}
		]
	}`)
	noImgN := decodeCountTokensOK(t, noImg)
	withImgN := decodeCountTokensOK(t, withImg)
	if withImgN-noImgN < 85 {
		t.Errorf("image overhead delta = %d, want >= 85", withImgN-noImgN)
	}
}

func TestCountTokensPOSTToolsAddOverhead(t *testing.T) {
	mux := newCountTokensMux()
	noTool := postCountTokens(t, mux, `{
		"model":"minimax-m3",
		"messages":[{"role":"user","content":"hi"}]
	}`)
	withTool := postCountTokens(t, mux, `{
		"model":"minimax-m3",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{
			"name":"lookup",
			"description":"Lookup data",
			"input_schema":{"type":"object"}
		}]
	}`)
	noToolN := decodeCountTokensOK(t, noTool)
	withToolN := decodeCountTokensOK(t, withTool)
	if withToolN-noToolN < 8 {
		t.Errorf("tool overhead delta = %d, want >= 8", withToolN-noToolN)
	}
}

func TestCountTokensGETReturns405(t *testing.T) {
	mux := newCountTokensMux()
	req := httptest.NewRequest(http.MethodGet, "/v1/messages/count_tokens", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", w.Code)
	}
	if !strings.Contains(w.Body.String(), "method not allowed") {
		t.Errorf("GET body = %s, want error envelope", w.Body.String())
	}
}

func TestCountTokensPUTReturns405(t *testing.T) {
	mux := newCountTokensMux()
	req := httptest.NewRequest(http.MethodPut, "/v1/messages/count_tokens", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT status = %d, want 405", w.Code)
	}
}

func TestCountTokensInvalidJSONReturns400(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `{not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid-json status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("invalid-json body = %s, want error envelope", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"invalid_json"`) {
		t.Errorf("invalid-json body = %s, want code invalid_json", w.Body.String())
	}
}

func TestCountTokensEmptyBodyReturns400(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `   `)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty-body status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("empty body = %s, want error envelope", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"empty_body"`) {
		t.Errorf("empty body = %s, want code empty_body", w.Body.String())
	}
}

func TestCountTokensMinimalNonEmptyIsNotZero(t *testing.T) {
	mux := newCountTokensMux()
	w := postCountTokens(t, mux, `{"messages":[{"role":"user","content":"a"}]}`)
	got := decodeCountTokensOK(t, w)
	if got < 1 {
		t.Errorf("minimal estimate = %d, want >= 1", got)
	}
}

func TestCountTokensIsDeterministic(t *testing.T) {
	mux := newCountTokensMux()
	body := `{
		"model":"minimax-m3",
		"system":"Be helpful.",
		"messages":[
			{"role":"user","content":"hello world"},
			{"role":"assistant","content":[
				{"type":"text","text":"hi there"}
			]}
		]
	}`
	first := decodeCountTokensOK(t, postCountTokens(t, mux, body))
	for i := 0; i < 5; i++ {
		again := decodeCountTokensOK(t, postCountTokens(t, mux, body))
		if again != first {
			t.Fatalf("non-deterministic: %d vs %d", first, again)
		}
	}
}

// ToolOverheadTokensFloor is the minimum additional token contribution
// that any tool description adds to the estimate (it does not include
// the name/description text which the test does not pre-count).
const ToolOverheadTokensFloor = 8
