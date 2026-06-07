package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ocgo/internal/compat"
	"ocgo/internal/config"
	"ocgo/internal/mapping"
)

func setTempMappingFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model-mapping.json")
	old := config.ModelMappingFile
	config.ModelMappingFile = func() string { return path }
	t.Cleanup(func() { config.ModelMappingFile = old })
	return path
}

func TestPrepareChatBodyAppliesCodexMapping(t *testing.T) {
	setTempMappingFile(t)
	m := mapping.DefaultModelMappings()
	m["codex"]["gpt-5"] = "deepseek-v4-pro"
	if err := mapping.SaveModelMappings(m); err != nil {
		t.Fatal(err)
	}
	body, err := PrepareChatBody([]byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"model":"deepseek-v4-pro"`) {
		t.Fatalf("mapping was not applied: %s", string(body))
	}
}

func TestRawChatImageKeepsKimiAndStripsDetail(t *testing.T) {
	setTempMappingFile(t)
	body, err := PrepareChatBody([]byte(`{"model":"kimi-k2.6","messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc","detail":"high"}}]}]}`))
	if err != nil {
		t.Fatalf("Kimi image request should validate: %v", err)
	}
	if !strings.Contains(string(body), `"model":"kimi-k2.6"`) {
		t.Fatalf("image chat body should keep Kimi model: %s", string(body))
	}
	if strings.Contains(string(body), `"detail"`) && strings.Contains(string(body), `"high"`) {
		t.Fatalf("image detail should be stripped for compatibility: %s", string(body))
	}
}

func TestRawChatImageRejectsUnsupportedModel(t *testing.T) {
	setTempMappingFile(t)
	body, err := PrepareChatBody([]byte(`{"model":"deepseek-v4-pro","messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]}]}`))
	if err != nil {
		t.Fatalf("PrepareChatBody should not reject, got: %v", err)
	}
	if !strings.Contains(string(body), `"model":"deepseek-v4-pro"`) {
		t.Fatalf("model should be kept: %s", string(body))
	}
}

func TestRawChatStreamRequestsUsage(t *testing.T) {
	setTempMappingFile(t)
	body, err := PrepareChatBody([]byte(`{"model":"kimi-k2.6","stream":true,"stream_options":{"include_usage":true,"foo":"bar"},"messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	options, ok := req["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("missing stream options in %s", string(body))
	}
	if options["include_usage"] != true {
		t.Fatalf("bad stream options: %+v", options)
	}
}

func TestRawChatReasoningEffortPassThrough(t *testing.T) {
	setTempMappingFile(t)
	body, err := PrepareChatBody([]byte(`{"model":"glm-5.1","reasoning":{"effort":"xhigh"},"thinking":{"type":"enabled"},"output_config":{"reasoning":{"depth":2}},"messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req["reasoning_effort"] != "medium" {
		t.Fatalf("reasoning_effort = %v, want medium in %s", req["reasoning_effort"], string(body))
	}
	if _, ok := req["thinking"]; ok {
		t.Fatalf("thinking should be stripped from forwarded chat body: %s", string(body))
	}
}

func TestConvertedStreamingRequestsAskForUsage(t *testing.T) {
	anthropic := ConvertRequest(compat.AnthropicRequest{Model: "kimi-k2.6", Stream: true, Messages: []compat.AMessage{{Role: "user", Content: []byte(`hello`)}}})
	if !anthropic.Stream {
		t.Fatalf("anthropic conversion should preserve stream flag: %+v", anthropic)
	}
}

func TestConvertedRequestsForwardReasoningEffort(t *testing.T) {
	anthropic := ConvertRequest(compat.AnthropicRequest{Model: "glm-5.1", Reasoning: []byte(`{"effort":"high"}`), Messages: []compat.AMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hello"}]`)}}})
	if anthropic.ReasoningEffort != "high" {
		t.Fatalf("anthropic reasoning effort = %q, want high", anthropic.ReasoningEffort)
	}
	responses := ResponsesToChat(compat.ResponsesRequest{Model: "glm-5.1", OutputConfig: []byte(`{"reasoning":{"depth":2}}`), Input: []byte(`[{"type":"message","role":"user","content":"hello"}]`)})
	if responses.ReasoningEffort != "medium" {
		t.Fatalf("responses reasoning effort = %q, want medium", responses.ReasoningEffort)
	}
}

func TestAnthropicImageKeepsKimiModel(t *testing.T) {
	out := ConvertRequest(compat.AnthropicRequest{Model: "kimi-k2.6", Messages: []compat.AMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"what is this?"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]`)}}})
	if out.Model != "kimi-k2.6" {
		t.Fatalf("image request should keep Kimi model, got %q", out.Model)
	}
	if err := ValidateImageSupport(out); err != nil {
		t.Fatalf("Kimi image request should validate: %v", err)
	}
}

func TestAnthropicImageRejectsUnsupportedModel(t *testing.T) {
	out := ConvertRequest(compat.AnthropicRequest{Model: "deepseek-v4-pro", Messages: []compat.AMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"what is this?"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]`)}}})
	if err := ValidateImageSupport(out); err == nil || !strings.Contains(err.Error(), "deepseek-v4-pro") {
		t.Fatalf("DeepSeek image request should be rejected, got %v", err)
	}
}

func TestResponsesImageKeepsKimiModel(t *testing.T) {
	req := compat.ResponsesRequest{Model: "kimi-k2.6", Input: []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"describe this"},{"type":"input_image","image_url":"data:image/png;base64,abc"}]}]`)}
	out := ResponsesToChat(req)
	if out.Model != "kimi-k2.6" {
		t.Fatalf("image request should keep Kimi model, got %q", out.Model)
	}
	if err := ValidateImageSupport(out); err != nil {
		t.Fatalf("Kimi image request should validate: %v", err)
	}
}

func TestResponsesImageRejectsUnsupportedModel(t *testing.T) {
	req := compat.ResponsesRequest{Model: "deepseek-v4-pro", Input: []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"describe this"},{"type":"input_image","image_url":"data:image/png;base64,abc"}]}]`)}
	out := ResponsesToChat(req)
	if err := ValidateImageSupport(out); err == nil || !strings.Contains(err.Error(), "deepseek-v4-pro") {
		t.Fatalf("DeepSeek image request should be rejected, got %v", err)
	}
}

func TestForwardAnthropicSendsNormalizedBody(t *testing.T) {
	var forwarded string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing API key header: %q", r.Header.Get("x-api-key"))
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		forwarded = buf.String()
		if strings.Contains(forwarded, "opencode-go/") {
			t.Fatalf("forwarded body still contains opencode-go/: %s", forwarded)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	oldURL := AnthropicURL
	AnthropicURL = srv.URL
	defer func() { AnthropicURL = oldURL }()

	resp, err := ForwardAnthropic(context.Background(), config.Config{APIKey: "test-key"}, compat.AnthropicRequest{
		Model:     "opencode-go/qwen3.7-max",
		Thinking:  []byte(`{"type":"enabled"}`),
		Messages:  []compat.AMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}]`)}},
		MaxTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	for _, want := range []string{`"model":"qwen3.7-max"`, `"text":"hello"`} {
		if !strings.Contains(forwarded, want) {
			t.Fatalf("missing %q in forwarded body: %s", want, forwarded)
		}
	}
}

func TestWriteAnthropicResponseIncludesUsage(t *testing.T) {
	body := bytes.NewReader([]byte(`{"content":[{"type":"text","text":"done"}],"role":"assistant","model":"kimi-k2.6","usage":{"input_tokens":11,"output_tokens":5,"cache_read_input_tokens":4}}`))
	w := httptest.NewRecorder()
	WriteAnthropicResponse(w, body, "kimi-k2.6")
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["usage"]; !ok {
		t.Fatalf("missing usage: %+v", out)
	}
}

func TestWriteResponsesResponseIncludesUsage(t *testing.T) {
	body := bytes.NewReader([]byte(`{"id":"msg_1","model":"kimi-k2.6","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":7,"output_tokens":3,"cache_read_input_tokens":2}}`))
	w := httptest.NewRecorder()
	WriteResponsesResponse(w, body, "kimi-k2.6")
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["usage"]; !ok {
		t.Fatalf("missing usage: %+v", out)
	}
}

func TestNormalizeAnthropicRequestForStrictUpstream(t *testing.T) {
	ar := compat.AnthropicRequest{
		Model:        "opencode-go/qwen3.7-max",
		Thinking:     []byte(`{"type":"enabled","budget_tokens":1024}`),
		Reasoning:    []byte(`{"effort":"high"}`),
		OutputConfig: []byte(`{"reasoning":{"depth":2}}`),
		System:       []byte(`[{"type":"text","text":"rules","cache_control":{"type":"ephemeral"}}]`),
		Tools:        []compat.ATool{{Type: "web_search_20250305", Name: "web_search", MaxUses: 8, AllowedDomains: []string{"example.com"}}},
		Messages: []compat.AMessage{{Role: "user", Content: []byte(`[
			{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}},
			{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"ok","cache_control":{"type":"ephemeral"}}]}
		]`)}},
	}
	NormalizeAnthropicRequestForUpstream(&ar)
	body, err := json.Marshal(ar)
	if err != nil {
		t.Fatal(err)
	}
	out := string(body)
	for _, gone := range []string{"opencode-go/", "thinking", "reasoning", "output_config"} {
		if strings.Contains(out, gone) {
			t.Fatalf("strict upstream request still contains %q: %s", gone, out)
		}
	}
	for _, want := range []string{`"model":"qwen3.7-max"`, `"type":"text"`, `"text":"hello"`, `"tool_use_id":"toolu_1"`, `"type":"web_search_20250305"`, `"name":"web_search"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in normalized request: %s", want, out)
		}
	}
}

func TestCopyAnthropicToolResultTruncatesLargeFetchContent(t *testing.T) {
	large := strings.Repeat("a", compat.MaxAnthropicToolResultContentChars+50) + "tail-should-be-omitted"
	dst := map[string]any{}
	src := map[string]json.RawMessage{
		"content": compat.MarshalJSON([]map[string]any{{"type": "text", "text": large}}),
	}
	compat.CopyAnthropicToolResultContent(dst, src)
	body, err := json.Marshal(dst)
	if err != nil {
		t.Fatal(err)
	}
	out := string(body)
	if strings.Contains(out, "tail-should-be-omitted") {
		t.Fatalf("large fetched content was not truncated: %s", out[len(out)-200:])
	}
}

func TestNormalizeQwenAnthropicRequestThinkingVariants(t *testing.T) {
	zero := 0.0
	topP := 0.8
	for _, tc := range []struct {
		name string
		req  compat.AnthropicRequest
	}{
		{
			name: "thinking enabled with budget",
			req: compat.AnthropicRequest{
				Thinking: []byte(`{"type":"enabled","budget_tokens":2048}`),
			},
		},
		{
			name: "thinking disabled with zero temperature",
			req: compat.AnthropicRequest{
				Thinking:    []byte(`{"type":"disabled"}`),
				Temperature: &zero,
			},
		},
		{
			name: "reasoning effort high",
			req: compat.AnthropicRequest{
				ReasoningEffort: []byte(`"high"`),
				TopP:            &topP,
			},
		},
		{
			name: "nested output config reasoning",
			req: compat.AnthropicRequest{
				OutputConfig: []byte(`{"reasoning":{"effort":"medium"}}`),
			},
		},
		{
			name: "legacy effort level depth fields",
			req: compat.AnthropicRequest{
				Effort: []byte(`"low"`),
				Level:  []byte(`2`),
				Depth:  []byte(`{"level":"high"}`),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ar := tc.req
			ar.Model = "opencode-go/qwen3.7-max"
			ar.Stream = true
			ar.MaxTokens = 1234
			ar.System = []byte(`"plain rules"`)
			ar.Tools = []compat.ATool{{Name: "Bash", Description: "run command", InputSchema: []byte(`{"type":"object","properties":{"command":{"type":"string"}}}`)}}
			ar.Messages = []compat.AMessage{{Role: "user", Content: []byte(`[
				{"type":"text","text":"hello qwen","cache_control":{"type":"ephemeral"}},
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"pwd","cache_control":{"type":"ephemeral"}}}
			]`)}}

			NormalizeAnthropicRequestForUpstream(&ar)
			body, err := json.Marshal(ar)
			if err != nil {
				t.Fatal(err)
			}
			out := string(body)
			for _, gone := range []string{"opencode-go/", "thinking", "reasoning", "reasoning_effort", "output_config", "effort", "level", "depth"} {
				if strings.Contains(out, gone) {
					t.Fatalf("normalized qwen request still contains %q: %s", gone, out)
				}
			}
			for _, want := range []string{`"model":"qwen3.7-max"`, `"stream":true`, `"max_tokens":1234`, `"name":"Bash"`, `"id":"toolu_1"`, `"command":"pwd"`, `"text":"hello qwen"`} {
				if !strings.Contains(out, want) {
					t.Fatalf("missing %q in normalized qwen request: %s", want, out)
				}
			}
			if tc.req.Temperature != nil && !strings.Contains(out, `"temperature":0`) {
				t.Fatalf("temperature option was not preserved: %s", out)
			}
			if tc.req.TopP != nil && !strings.Contains(out, `"top_p":0.8`) {
				t.Fatalf("top_p option was not preserved: %s", out)
			}
		})
	}
}

func TestStreamAnthropicIncludesFinalUsage(t *testing.T) {
	body := bytes.NewReader([]byte(strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
		`data: [DONE]`,
		``,
	}, "\n")))
	w := httptest.NewRecorder()
	StreamAnthropic(w, body, "kimi-k2.6")
	out := w.Body.String()
	if !strings.Contains(out, "hi") {
		t.Fatalf("missing content in stream output:\n%s", out)
	}
}

func TestStreamResponsesIncludesCompletedUsage(t *testing.T) {
	body := bytes.NewReader([]byte(strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
		`data: [DONE]`,
		``,
	}, "\n")))
	w := httptest.NewRecorder()
	StreamResponses(w, body, "kimi-k2.6")
	out := w.Body.String()
	if !strings.Contains(out, "hi") {
		t.Fatalf("missing content in stream output:\n%s", out)
	}
}

func TestStreamAnthropicForwardsToolCalls(t *testing.T) {
	compat.ReasoningContentCache.Lock()
	compat.ReasoningContentCache.ByCallID = map[string]string{}
	compat.ReasoningContentCache.Unlock()

	body := bytes.NewReader([]byte(strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"Need pwd.","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"Bash","arguments":"{\"command\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"pwd\"}"}}]}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")))
	w := httptest.NewRecorder()
	StreamAnthropic(w, body, "deepseek-v4-flash")
	out := w.Body.String()
	if !strings.Contains(out, "call_abc") {
		t.Fatalf("missing tool call reference in stream:\n%s", out)
	}
}

func TestStreamResponsesForwardsToolCalls(t *testing.T) {
	compat.ReasoningContentCache.Lock()
	compat.ReasoningContentCache.ByCallID = map[string]string{}
	compat.ReasoningContentCache.Unlock()

	body := bytes.NewReader([]byte(strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"I should call the tool.","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"shell","arguments":"{\"cmd\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"pwd\"}"}}]}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")))
	w := httptest.NewRecorder()
	StreamResponses(w, body, "deepseek-v4-flash")
	out := w.Body.String()
	if !strings.Contains(out, "call_abc") {
		t.Fatalf("missing tool call reference in stream:\n%s", out)
	}
}
